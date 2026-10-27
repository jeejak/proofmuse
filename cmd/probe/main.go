// Command probe runs the observation step on one eval record and prints
// the model's per-photo answer, the verdict, and what it cost. It is a
// development tool; the eval harness is cmd/eval.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jeejak/proofmuse/internal/evalset"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
)

func main() {
	dir := flag.String("set", "evalset", "eval set directory")
	rec := flag.String("record", "R04", "record id")
	model := flag.String("model", "muse-spark-1.3", "vision model")
	effort := flag.String("effort", "low", "reasoning effort")
	maxTokens := flag.Int("max-tokens", 8000, "output budget, reasoning included")
	flag.Parse()
	key := os.Getenv("MODEL_API_KEY")
	if key == "" {
		log.Fatal("MODEL_API_KEY is not set")
	}
	s, err := evalset.Load(*dir)
	if err != nil {
		log.Fatal(err)
	}
	photos := s.Photos[*rec]
	parts := []meta.Part{meta.Text(evidence.Prompt(len(photos)))}
	for _, p := range photos {
		b, err := os.ReadFile(s.PhotoPath(p))
		if err != nil {
			log.Fatal(err)
		}
		parts = append(parts, meta.JPEG(b))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	raw, u, err := meta.New(key).JSON(ctx, meta.JSONRequest{
		Model: *model, Effort: *effort, MaxTokens: *maxTokens,
		SchemaName: "PackingObservation", Schema: evidence.Schema(), Parts: parts,
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	fmt.Printf("usage: %+v\n", u)
	if err != nil {
		log.Fatal(err)
	}
	obs, err := evidence.ParseObservation(raw, len(photos))
	if err != nil {
		fmt.Println(string(raw))
		log.Fatal(err)
	}
	var label evalset.Label
	for _, l := range s.Labels {
		if l.RecordID == *rec {
			label = l
		}
	}
	o := s.Orders[*rec]
	r := evidence.Aggregate(obs.Photos)
	res := evidence.Decide(r, evidence.Order{Resi: o.Resi, Koli: o.Koli, ItemMismatch: label.ItemMismatch})
	for i, p := range obs.Photos {
		h := photos[i].Obs
		fmt.Printf("photo %d model %+v\n        human label=%s readable=%s sealed=%s item=%s boxes=%d issues=%v box=%v\n", p.Photo, p,
			h.LabelInPhoto, h.LabelReadable, h.SealedBoxInPhoto, h.ItemInPhoto, h.BoxesInPhoto, h.ImageQualityIssues, h.LabelBox)
	}
	fmt.Printf("record model %+v\nrecord human %+v\n", r, label.Record)
	fmt.Printf("verdict model %s %v | human %s %v\n", res.Verdict, res.Reasons, label.Verdict, label.Reasons)
}
