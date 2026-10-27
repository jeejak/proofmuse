// Command probe runs the full pipeline on eval records and prints the
// trace, per-photo answers beside the human labels, the verdict and the
// cost. A development tool; the eval harness is cmd/eval.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jeejak/proofmuse/internal/evalset"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

func main() {
	dir := flag.String("set", "evalset", "eval set directory")
	recs := flag.String("records", "R04", "comma-separated record ids")
	effort := flag.String("effort", "", "override reasoning effort")
	maxSide := flag.Int("max-side", 0, "override long side sent")
	noGround := flag.Bool("no-ground", false, "skip SAM and reread")
	flag.Parse()
	key := os.Getenv("MODEL_API_KEY")
	if key == "" {
		log.Fatal("MODEL_API_KEY is not set")
	}
	s, err := evalset.Load(*dir)
	if err != nil {
		log.Fatal(err)
	}
	cfg := pipeline.Defaults()
	if *effort != "" {
		cfg.Effort = *effort
	}
	if *maxSide > 0 {
		cfg.MaxSide = *maxSide
	}
	if *noGround {
		cfg.Ground, cfg.Reread = false, false
	}
	labels := map[string]evalset.Label{}
	for _, l := range s.Labels {
		labels[l.RecordID] = l
	}
	api := meta.New(key)
	for _, id := range strings.Split(*recs, ",") {
		l, ok := labels[id]
		if !ok {
			log.Fatalf("no record %s", id)
		}
		var photos [][]byte
		for _, p := range s.Photos[id] {
			b, err := os.ReadFile(s.PhotoPath(p))
			if err != nil {
				log.Fatal(err)
			}
			photos = append(photos, b)
		}
		o := s.Orders[id]
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := pipeline.Run(ctx, api, cfg, photos, evidence.Order{Resi: o.Resi, Koli: o.Koli, ItemMismatch: l.ItemMismatch})
		cancel()
		fmt.Printf("== %s (%s, %s split)\n", id, l.Group, s.Split[id])
		if res != nil {
			for _, st := range res.Trace {
				fmt.Printf("  %-7s p%d %-55s -> %s ($%.4f, %.1fs)\n", st.Name, st.Photo, st.Why, st.Result, st.Usage.CostUSD, st.Usage.Latency.Seconds())
			}
		}
		if err != nil {
			fmt.Println("  error:", err)
			continue
		}
		for i, p := range res.Photos {
			h := s.Photos[id][i].Obs
			fmt.Printf("  p%d model label=%s read=%s resi=%q sealed=%s item=%s boxes=%d issues=%v\n", p.Photo, p.LabelInPhoto, p.LabelReadable, p.ResiText, p.SealedBoxInPhoto, p.ItemInPhoto, p.BoxesInPhoto, p.ImageQualityIssues)
			fmt.Printf("     human label=%s read=%s            sealed=%s item=%s boxes=%d issues=%v\n", h.LabelInPhoto, h.LabelReadable, h.SealedBoxInPhoto, h.ItemInPhoto, h.BoxesInPhoto, h.ImageQualityIssues)
		}
		fmt.Printf("  verdict model %s %v | human %s %v\n", res.Verdict.Verdict, res.Verdict.Reasons, l.Verdict, l.Reasons)
		fmt.Printf("  total $%.4f, %.1fs, in %d out %d (reasoning %d), images %d\n", res.Usage.CostUSD, res.Latency.Seconds(), res.Usage.InputTokens, res.Usage.OutputTokens, res.Usage.ReasoningTokens, res.Usage.Images)
	}
}
