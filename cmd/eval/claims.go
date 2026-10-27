package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jeejak/proofmuse/internal/evalset"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

// runClaims classifies every buyer claim in the split and writes
// docs/eval/claims-<split>.md. The expected types are the types the 40
// synthetic messages were written for, not a human review.
func runClaims(s *evalset.Set, cfg pipeline.Config, api *meta.Client, split, out string) {
	claims, err := evalset.LoadClaims(s.Dir)
	if err != nil {
		log.Fatal(err)
	}
	type res struct {
		c       evalset.Claim
		got     string
		err     error
		u       meta.Usage
		latency time.Duration
	}
	var todo []evalset.Claim
	for _, c := range claims {
		if split == "all" || s.Split[c.RecordID] == split {
			todo = append(todo, c)
		}
	}
	results := make([]res, len(todo))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, c := range todo {
		wg.Add(1)
		go func(i int, c evalset.Claim) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			start := time.Now()
			got, u, err := pipeline.ClassifyClaim(ctx, api, cfg, c.Text)
			results[i] = res{c: c, got: got, err: err, u: u, latency: time.Since(start)}
		}(i, c)
	}
	wg.Wait()
	conf := map[string]map[string]int{}
	agree, errs := 0, 0
	var lat, cost []float64
	var misses []string
	for _, r := range results {
		if r.err != nil {
			errs++
			misses = append(misses, fmt.Sprintf("%s: error %v", r.c.ClaimID, r.err))
			continue
		}
		if conf[r.c.ClaimType] == nil {
			conf[r.c.ClaimType] = map[string]int{}
		}
		conf[r.c.ClaimType][r.got]++
		if r.got == r.c.ClaimType {
			agree++
		} else {
			misses = append(misses, fmt.Sprintf("%s %q: expected %s, model %s", r.c.ClaimID, r.c.Text, r.c.ClaimType, r.got))
		}
		lat = append(lat, r.latency.Seconds())
		cost = append(cost, r.u.CostUSD)
	}
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	w("# Eval: buyer claim triage on %s", split)
	w("")
	w("40 synthetic informal Indonesian buyer messages, 8 per type. Expected types are the types each message was written for, not a human review. Model `%s`, effort `%s`.", cfg.VisionModel, cfg.Effort)
	w("")
	w("| Measure | Result |")
	w("|---|---|")
	w("| Claim type agrees | %s |", pct(agree, len(results)-errs))
	w("| Errors | %d of %d |", errs, len(results))
	w("| Latency p50 / p95 | %.1f s / %.1f s |", quantile(lat, .5), quantile(lat, .95))
	w("| Cost per claim | $%.5f |", mean(cost))
	w("")
	w("| expected \\ model | %s |", strings.Join(evidence.ClaimTypes, " | "))
	w("|---|%s", strings.Repeat("---|", len(evidence.ClaimTypes)))
	for _, e := range evidence.ClaimTypes[:5] {
		row := []string{}
		for _, m := range evidence.ClaimTypes {
			row = append(row, fmt.Sprint(conf[e][m]))
		}
		w("| %s | %s |", e, strings.Join(row, " | "))
	}
	if len(misses) > 0 {
		w("")
		w("Disagreements:")
		w("")
		for _, m := range misses {
			w("- %s", m)
		}
	}
	path := filepath.Join(out, "claims-"+split+".md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Print(b.String())
}
