// Command eval runs the pipeline over a split of the eval set and scores
// it against the human labels.
//
//	go run ./cmd/eval -split dev -name full
//	go run ./cmd/eval -split dev -name spark-only -no-ground
//
// Results are appended to docs/eval/<name>-<split>.jsonl one record at a
// time; a rerun skips records already there, so a crash or a rate limit
// never pays twice. The report is written to docs/eval/<name>-<split>.md.
// Prompt tuning looks at dev only; test is run once per published
// configuration.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jeejak/proofmuse/internal/evalset"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

// Row is one record's run, as stored in the JSONL.
type Row struct {
	RecordID string           `json:"record_id"`
	Split    string           `json:"split"`
	Group    string           `json:"group"`
	Human    string           `json:"human_verdict"`
	Error    string           `json:"error,omitempty"`
	Invalid  bool             `json:"invalid,omitempty"`
	Result   *pipeline.Result `json:"result,omitempty"`
	RunAt    time.Time        `json:"run_at"`
}

func main() {
	dir := flag.String("set", "evalset", "eval set directory")
	split := flag.String("split", "dev", "dev, test or all")
	name := flag.String("name", "full", "run name, used in output file names")
	out := flag.String("out", "docs/eval", "output directory")
	workers := flag.Int("workers", 4, "records in flight")
	effort := flag.String("effort", "", "override observation reasoning effort")
	maxSide := flag.Int("max-side", 0, "override long side sent")
	model := flag.String("model", "", "override vision model")
	noGround := flag.Bool("no-ground", false, "Muse Spark only: no SAM, no reread, no verify")
	reportOnly := flag.Bool("report", false, "only rebuild the report from the JSONL")
	limit := flag.Int("limit", 0, "stop after this many new records (0: all)")
	claims := flag.Bool("claims", false, "evaluate buyer-claim triage instead of photos")
	visionURL := flag.String("vision-base-url", "", "OpenAI-compatible endpoint for the vision model (e.g. https://inference.baseten.co/v1); SAM stays on the Meta Model API")
	visionKeyEnv := flag.String("vision-key-env", "BASETEN_API_KEY", "env var holding the key for -vision-base-url")
	priceIn := flag.Float64("price-in", 0, "USD per 1M input tokens for a model the client does not know")
	priceOut := flag.Float64("price-out", 0, "USD per 1M output tokens for a model the client does not know")
	flag.Parse()

	s, err := evalset.Load(*dir)
	if err != nil {
		log.Fatal(err)
	}
	cfg := pipeline.Defaults()
	if *effort != "" {
		cfg.Effort = *effort
	}
	if cfg.Effort == "omit" { // some partner hosts reject reasoning_effort
		cfg.Effort, cfg.RereadEffort = "", ""
	}
	if *maxSide > 0 {
		cfg.MaxSide = *maxSide
	}
	if *model != "" {
		cfg.VisionModel = *model
	}
	if *noGround {
		cfg.Ground, cfg.Reread, cfg.Verify = false, false, false
	}
	if *claims {
		key := os.Getenv("MODEL_API_KEY")
		if key == "" {
			log.Fatal("MODEL_API_KEY is not set")
		}
		runClaims(s, cfg, meta.New(key), *split, *out)
		return
	}
	base := filepath.Join(*out, *name+"-"+*split)
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	done, err := readRows(base + ".jsonl")
	if err != nil {
		log.Fatal(err)
	}
	if !*reportOnly {
		key := os.Getenv("MODEL_API_KEY")
		if key == "" {
			log.Fatal("MODEL_API_KEY is not set")
		}
		var api pipeline.API = meta.New(key)
		if *visionURL != "" {
			vkey := os.Getenv(*visionKeyEnv)
			if vkey == "" {
				log.Fatalf("%s is not set", *visionKeyEnv)
			}
			v := meta.New(vkey)
			v.BaseURL = *visionURL
			api = meta.Split{Vision: v, Segmenter: meta.New(key)}
		}
		if *priceIn > 0 || *priceOut > 0 {
			meta.SetPrice(cfg.VisionModel, *priceIn, *priceOut)
		}
		run(s, cfg, api, *split, base+".jsonl", done, *workers, *limit)
		if done, err = readRows(base + ".jsonl"); err != nil {
			log.Fatal(err)
		}
	}
	rep := score(s, done, *split)
	md := render(*name, *split, cfg, rep)
	if err := os.WriteFile(base+".md", []byte(md), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Print(md)
}

func readRows(path string) (map[string]Row, error) {
	rows := map[string]Row{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return rows, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 32<<20)
	for sc.Scan() {
		var r Row
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		// A transient error is retried on the next run; an answer that
		// failed the schema twice is a result and stays.
		if r.Error != "" && !r.Invalid {
			continue
		}
		rows[r.RecordID] = r
	}
	return rows, sc.Err()
}

func run(s *evalset.Set, cfg pipeline.Config, api pipeline.API, split, path string, done map[string]Row, workers, limit int) {
	var todo []evalset.Label
	for _, l := range s.Labels {
		if (split == "all" || s.Split[l.RecordID] == split) && done[l.RecordID].RecordID == "" {
			todo = append(todo, l)
		}
	}
	if limit > 0 && len(todo) > limit {
		todo = todo[:limit]
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var mu sync.Mutex
	jobs := make(chan evalset.Label)
	var wg sync.WaitGroup
	for range max(workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for l := range jobs {
				row := runOne(s, cfg, api, l)
				b, _ := json.Marshal(row)
				mu.Lock()
				f.Write(append(b, '\n'))
				mu.Unlock()
				v := "-"
				if row.Result != nil {
					v = row.Result.Verdict.Verdict
				}
				log.Printf("%s %-28s human %-11s model %-11s %s", l.RecordID, l.Group, l.Verdict, v, row.Error)
			}
		}()
	}
	for _, l := range todo {
		jobs <- l
	}
	close(jobs)
	wg.Wait()
}

func runOne(s *evalset.Set, cfg pipeline.Config, api pipeline.API, l evalset.Label) Row {
	row := Row{RecordID: l.RecordID, Split: s.Split[l.RecordID], Group: l.Group, Human: l.Verdict, RunAt: time.Now().UTC()}
	var photos [][]byte
	for _, p := range s.Photos[l.RecordID] {
		b, err := os.ReadFile(s.PhotoPath(p))
		if err != nil {
			row.Error = err.Error()
			return row
		}
		photos = append(photos, b)
	}
	o := s.Orders[l.RecordID]
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	res, err := pipeline.Run(ctx, api, cfg, photos, evidence.Order{Resi: o.Resi, Koli: o.Koli, ItemMismatch: l.ItemMismatch})
	row.Result = res
	if err != nil {
		row.Error = err.Error()
		row.Invalid = errorsIsInvalid(err)
	}
	return row
}

func sortedIDs(m map[string]Row) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
