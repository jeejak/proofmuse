package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jeejak/proofmuse/internal/evalset"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

func errorsIsInvalid(err error) bool { return errors.Is(err, pipeline.ErrInvalid) }

var verdicts = []string{evidence.VerdictKuat, evidence.VerdictPerluDicek, evidence.VerdictLemah}

type fieldScore struct{ agree, total, humanUncertain int }

type report struct {
	Records, Scored, Errors, Invalid int
	Confusion                        map[string]map[string]int // human -> model
	Agree                            int
	LemahTP, LemahFP, LemahFN        int
	KuatHuman, KuatFlagged           int // false alarms: human kuat, model not kuat
	Fields                           map[string]*fieldScore
	BoxModelIoU, BoxSAMIoU           []float64
	SAMMissed, SAMExtra              int // SAM found no label where human sees one, or the reverse
	Latency                          []time.Duration
	Cost                             []float64
	Photos                           int
	InTok, OutTok, ReasonTok         int
	Steps                            map[string]int
	VerifyWithdrawn, VerifyConfirmed int
	RereadUpgraded                   int
	Misses                           []string
}

func score(s *evalset.Set, rows map[string]Row, split string) report {
	r := report{Confusion: map[string]map[string]int{}, Fields: map[string]*fieldScore{}, Steps: map[string]int{}}
	for _, v := range verdicts {
		r.Confusion[v] = map[string]int{}
	}
	for _, f := range []string{"label_in_photo", "label_readable", "sealed_box_in_photo", "item_in_photo", "boxes_in_photo", "image_quality_issues"} {
		r.Fields[f] = &fieldScore{}
	}
	labels := map[string]evalset.Label{}
	for _, l := range s.Labels {
		labels[l.RecordID] = l
	}
	for _, id := range sortedIDs(rows) {
		row := rows[id]
		if split != "all" && row.Split != split {
			continue
		}
		r.Records++
		if row.Result != nil {
			for _, st := range row.Result.Trace {
				r.Steps[st.Name]++
				switch {
				case st.Name == "verify" && strings.HasPrefix(st.Result, "withdrawn"):
					r.VerifyWithdrawn++
				case st.Name == "verify" && strings.HasPrefix(st.Result, "confirmed"):
					r.VerifyConfirmed++
				case st.Name == "reread" && strings.HasPrefix(st.Result, "read "):
					r.RereadUpgraded++
				}
			}
		}
		if row.Error != "" {
			if row.Invalid {
				r.Invalid++
			} else {
				r.Errors++
			}
			continue
		}
		res, l := row.Result, labels[id]
		r.Scored++
		h, m := l.Verdict, res.Verdict.Verdict
		r.Confusion[h][m]++
		if h == m {
			r.Agree++
		} else {
			r.Misses = append(r.Misses, fmt.Sprintf("%s (%s): human %s %v, model %s %v", id, l.Group, h, l.Reasons, m, res.Verdict.Reasons))
		}
		switch {
		case m == evidence.VerdictLemah && h == evidence.VerdictLemah:
			r.LemahTP++
		case m == evidence.VerdictLemah:
			r.LemahFP++
		case h == evidence.VerdictLemah:
			r.LemahFN++
		}
		if h == evidence.VerdictKuat {
			r.KuatHuman++
			if m != evidence.VerdictKuat {
				r.KuatFlagged++
			}
		}
		r.Latency = append(r.Latency, res.Latency)
		r.Cost = append(r.Cost, res.Usage.CostUSD)
		r.InTok += res.Usage.InputTokens
		r.OutTok += res.Usage.OutputTokens
		r.ReasonTok += res.Usage.ReasoningTokens
		human := s.Photos[id]
		r.Photos += len(human)
		sam := map[int]pipeline.Grounding{}
		for _, g := range res.Grounding {
			sam[g.Photo] = g
		}
		// Per-photo fields are scored on the first pass: that is what
		// the model itself saw, before the pipeline's own rereads.
		for i, mp := range res.FirstPass {
			hp := human[i].Obs
			tri := func(name, hv, mv string) {
				f := r.Fields[name]
				if hv == evidence.Uncertain {
					f.humanUncertain++
					return
				}
				f.total++
				if hv == mv {
					f.agree++
				}
			}
			tri("label_in_photo", hp.LabelInPhoto, mp.LabelInPhoto)
			tri("label_readable", hp.LabelReadable, mp.LabelReadable)
			tri("sealed_box_in_photo", hp.SealedBoxInPhoto, mp.SealedBoxInPhoto)
			tri("item_in_photo", hp.ItemInPhoto, mp.ItemInPhoto)
			b := r.Fields["boxes_in_photo"]
			b.total++
			if hp.BoxesInPhoto == mp.BoxesInPhoto {
				b.agree++
			}
			q := r.Fields["image_quality_issues"]
			q.total++
			if sameSet(hp.ImageQualityIssues, mp.ImageQualityIssues) {
				q.agree++
			}
			if hp.LabelInPhoto == evidence.Yes && !hp.LabelBox.IsZero() {
				r.BoxModelIoU = append(r.BoxModelIoU, iou(hp.LabelBox, mp.LabelBox))
				if g, ok := sam[mp.Photo]; ok {
					r.BoxSAMIoU = append(r.BoxSAMIoU, iou(hp.LabelBox, g.Box))
				} else if res.Config.Ground && mp.LabelInPhoto != evidence.No {
					r.SAMMissed++
				}
			} else if _, ok := sam[mp.Photo]; ok && hp.LabelInPhoto == evidence.No {
				r.SAMExtra++
			}
		}
	}
	return r
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]bool{}
	for _, x := range a {
		m[x] = true
	}
	for _, x := range b {
		if !m[x] {
			return false
		}
	}
	return true
}

func iou(a, b evidence.Box) float64 {
	if a.IsZero() || b.IsZero() {
		return 0
	}
	ix := max(0, min(a.XMax, b.XMax)-max(a.XMin, b.XMin))
	iy := max(0, min(a.YMax, b.YMax)-max(a.YMin, b.YMin))
	inter := float64(ix * iy)
	area := func(x evidence.Box) float64 { return float64((x.XMax - x.XMin) * (x.YMax - x.YMin)) }
	return inter / (area(a) + area(b) - inter)
}

func pct(n, d int) string {
	if d == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", 100*float64(n)/float64(d), n, d)
}

func quantile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	i := int(q*float64(len(s)-1) + 0.5)
	return s[i]
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t / float64(len(xs))
}

func atLeast(xs []float64, t float64) int {
	n := 0
	for _, x := range xs {
		if x >= t {
			n++
		}
	}
	return n
}

func render(name, split string, cfg pipeline.Config, r report) string {
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	lat := make([]float64, len(r.Latency))
	for i, d := range r.Latency {
		lat[i] = d.Seconds()
	}
	w("# Eval: %s on %s", name, split)
	w("")
	w("Synthetic photos with dummy waybills, scored against Samsul's human photo labels (record labels derived from them). Not accuracy on real packing photos.")
	w("")
	w("Config: vision `%s` effort `%s`, photos at %d px, SAM `%s` ground=%v reread=%v verify=%v, prompt `%s`.",
		cfg.VisionModel, cfg.Effort, cfg.MaxSide, cfg.SegmentModel, cfg.Ground, cfg.Reread, cfg.Verify, evidence.PromptVersion)
	w("")
	w("| Measure | Result |")
	w("|---|---|")
	w("| Records scored | %d of %d (errors %d, invalid after retry %d) |", r.Scored, r.Records, r.Errors, r.Invalid)
	w("| Verdict agrees with human | %s |", pct(r.Agree, r.Scored))
	w("| Bukti lemah precision | %s |", pct(r.LemahTP, r.LemahTP+r.LemahFP))
	w("| Bukti lemah recall | %s |", pct(r.LemahTP, r.LemahTP+r.LemahFN))
	w("| False alarms on human kuat | %s |", pct(r.KuatFlagged, r.KuatHuman))
	w("| Invalid answers (schema, after one retry) | %s |", pct(r.Invalid, r.Records))
	w("| Latency per parcel p50 / p95 | %.1f s / %.1f s |", quantile(lat, .5), quantile(lat, .95))
	w("| Cost per parcel mean / p95 | $%.4f / $%.4f |", mean(r.Cost), quantile(r.Cost, .95))
	if r.Photos > 0 {
		w("| Cost per photo | $%.4f |", mean(r.Cost)*float64(r.Scored)/float64(r.Photos))
	}
	if r.Scored > 0 {
		w("| Tokens per parcel in / out (reasoning) | %d / %d (%d) |", r.InTok/r.Scored, r.OutTok/r.Scored, r.ReasonTok/r.Scored)
	}
	w("")
	w("Confusion (rows human, columns model):")
	w("")
	w("| human \\ model | kuat | perlu_dicek | lemah |")
	w("|---|---|---|---|")
	for _, h := range verdicts {
		w("| %s | %d | %d | %d |", h, r.Confusion[h][verdicts[0]], r.Confusion[h][verdicts[1]], r.Confusion[h][verdicts[2]])
	}
	w("")
	w("Per-photo agreement, first pass (photos the human marked uncertain are left out and counted):")
	w("")
	w("| Field | Agreement | Human uncertain |")
	w("|---|---|---|")
	for _, f := range []string{"label_in_photo", "label_readable", "sealed_box_in_photo", "item_in_photo", "boxes_in_photo", "image_quality_issues"} {
		s := r.Fields[f]
		w("| %s | %s | %d |", f, pct(s.agree, s.total), s.humanUncertain)
	}
	w("")
	w("Label boxes against the human box (IoU on the 0 to 1000 grid, photos with a human-labelled label):")
	w("")
	w("| Source | Mean IoU | IoU >= 0.5 |")
	w("|---|---|---|")
	w("| Muse Spark first pass | %.2f | %s |", mean(r.BoxModelIoU), pct(atLeast(r.BoxModelIoU, .5), len(r.BoxModelIoU)))
	if cfg.Ground {
		w("| SAM 3.1 (`%s`) | %.2f | %s |", cfg.LabelPhrase, mean(r.BoxSAMIoU), pct(atLeast(r.BoxSAMIoU, .5), len(r.BoxSAMIoU)))
		w("")
		w("SAM missed a human-labelled label on %d grounded photos and found one where the human saw none on %d.", r.SAMMissed, r.SAMExtra)
		w("Pipeline steps: %v. Rereads that read an unread label: %d. Verify: %d mismatches confirmed, %d withdrawn.", r.Steps, r.RereadUpgraded, r.VerifyConfirmed, r.VerifyWithdrawn)
	}
	if len(r.Misses) > 0 {
		w("")
		w("Disagreements:")
		w("")
		for _, m := range r.Misses {
			w("- %s", m)
		}
	}
	return b.String()
}
