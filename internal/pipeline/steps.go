package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
)

// ground runs SAM on every photo where the first pass did not rule a
// label out, in parallel, and keeps the most confident match per photo.
func ground(ctx context.Context, api API, cfg Config, full []imageAndBytes, photos []evidence.PhotoObs, add func(Step)) []Grounding {
	type out struct {
		g    *Grounding
		step Step
	}
	outs := make([]out, len(photos))
	var wg sync.WaitGroup
	for i, p := range photos {
		if p.LabelInPhoto == evidence.No {
			outs[i].step = Step{Name: "ground", Photo: p.Photo, Why: "skipped: first pass saw no label", Result: "skipped"}
			continue
		}
		wg.Add(1)
		go func(i int, p evidence.PhotoObs) {
			defer wg.Done()
			dets, u, err := api.Segment(ctx, cfg.SegmentModel, full[i].sent, cfg.LabelPhrase)
			st := Step{Name: "ground", Photo: p.Photo, Why: "first pass label_in_photo=" + p.LabelInPhoto, Usage: u}
			if err != nil {
				st.Result = "error: " + err.Error()
				outs[i] = out{step: st}
				return
			}
			var best *meta.Detection
			for j := range dets {
				if dets[j].Confidence >= cfg.MinLabelConfidence && (best == nil || dets[j].Confidence > best.Confidence) {
					best = &dets[j]
				}
			}
			if best == nil {
				st.Result = fmt.Sprintf("no label (%d weak matches)", len(dets))
				outs[i] = out{step: st}
				return
			}
			g := best.Grid()
			gr := &Grounding{Photo: p.Photo, Box: evidence.Box{XMin: g[0], YMin: g[1], XMax: g[2], YMax: g[3]}, Confidence: best.Confidence}
			st.Result = fmt.Sprintf("label at %v c=%.2f", g, best.Confidence)
			outs[i] = out{g: gr, step: st}
		}(i, p)
	}
	wg.Wait()
	var gs []Grounding
	for _, o := range outs {
		add(o.step)
		if o.g != nil {
			gs = append(gs, *o.g)
		}
	}
	return gs
}

// rereadSchema is the second-pass contract: is the number legible on
// this crop, and what is it.
func rereadSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"readable", "resi_text"},
		"properties": map[string]any{
			"readable":  map[string]any{"type": "string", "enum": []string{evidence.Yes, evidence.No, evidence.Uncertain}},
			"resi_text": map[string]any{"type": "string"},
		},
	}
}

const rereadPrompt = `This is a close crop of a courier shipping label (waybill) from an Indonesian parcel. Read the tracking number (nomor resi), usually printed as large text near the barcode. Set readable to "yes" only if every character is legible, and copy it exactly in resi_text. Otherwise set readable to "no" or "uncertain" and leave resi_text empty. Do not guess characters.`

// cropRead is one second-pass read: is the resi legible on a high
// resolution crop of the label, and what does it say. The crop comes
// from the original photo, not the downscaled one, which is the point:
// more pixels on the label than the first pass had.
type cropRead struct {
	readable string
	resi     string // normalised; empty unless readable is yes
	usage    meta.Usage
	err      error
}

func readCrop(ctx context.Context, api API, cfg Config, f imageAndBytes, box evidence.Box) cropRead {
	b, err := encode(fit(crop(f.img, box, 0.15), 1600, 1024))
	if err != nil {
		return cropRead{err: err}
	}
	raw, u, err := api.JSON(ctx, meta.JSONRequest{Model: cfg.VisionModel, Effort: cfg.RereadEffort, MaxTokens: cfg.MaxTokens,
		SchemaName: "ResiRead", Schema: rereadSchema(), Parts: []meta.Part{meta.Text(rereadPrompt), meta.JPEG(b)}})
	if err != nil {
		return cropRead{usage: u, err: err}
	}
	var r struct {
		Readable string `json:"readable"`
		ResiText string `json:"resi_text"`
	}
	if err := strictJSON(raw, &r); err != nil || (r.Readable != evidence.Yes && r.Readable != evidence.No && r.Readable != evidence.Uncertain) {
		return cropRead{usage: u, err: fmt.Errorf("invalid answer, discarded")}
	}
	n := evidence.NormaliseResi(r.ResiText)
	if r.Readable != evidence.Yes || n == "" {
		return cropRead{readable: evidence.Uncertain, usage: u}
	}
	return cropRead{readable: evidence.Yes, resi: n, usage: u}
}

// labelBox picks the crop box for a photo: SAM's when it found one,
// otherwise the first pass's own box.
func labelBox(p evidence.PhotoObs, byPhoto map[int]Grounding) (evidence.Box, string) {
	if g, ok := byPhoto[p.Photo]; ok {
		return g.Box, "SAM box"
	}
	return p.LabelBox, "model box"
}

// forEachCrop runs readCrop in parallel on the photos pick selects and
// hands each answer to apply in photo order, so the trace is stable.
func forEachCrop(ctx context.Context, api API, cfg Config, full []imageAndBytes, photos []evidence.PhotoObs, gs []Grounding,
	name string, pick func(evidence.PhotoObs) (string, bool), apply func(i int, r cropRead) string, add func(Step)) {
	byPhoto := map[int]Grounding{}
	for _, g := range gs {
		byPhoto[g.Photo] = g
	}
	type job struct {
		why string
		r   cropRead
		run bool
	}
	jobs := make([]job, len(photos))
	var wg sync.WaitGroup
	for i, p := range photos {
		why, ok := pick(p)
		if !ok {
			continue
		}
		box, src := labelBox(p, byPhoto)
		if box.IsZero() {
			continue
		}
		jobs[i] = job{why: why + " (" + src + ")", run: true}
		wg.Add(1)
		go func(i int, box evidence.Box) {
			defer wg.Done()
			jobs[i].r = readCrop(ctx, api, cfg, full[i], box)
		}(i, box)
	}
	wg.Wait()
	for i, j := range jobs {
		if !j.run {
			continue
		}
		st := Step{Name: name, Photo: photos[i].Photo, Why: j.why, Usage: j.r.usage}
		if j.r.err != nil {
			st.Result = "error: " + j.r.err.Error()
		} else {
			st.Result = apply(i, j.r)
		}
		add(st)
	}
}

// reread: where a label is located but its number was not read, try the
// crop. A legible crop upgrades the photo to readable.
func reread(ctx context.Context, api API, cfg Config, full []imageAndBytes, photos []evidence.PhotoObs, gs []Grounding, add func(Step)) {
	forEachCrop(ctx, api, cfg, full, photos, gs, "reread",
		func(p evidence.PhotoObs) (string, bool) {
			if p.LabelReadable == evidence.Yes || p.LabelInPhoto == evidence.No {
				return "", false
			}
			return "label located but label_readable=" + p.LabelReadable, true
		},
		func(i int, r cropRead) string {
			if r.readable != evidence.Yes {
				return "still not readable"
			}
			photos[i].LabelInPhoto, photos[i].LabelReadable, photos[i].ResiText = evidence.Yes, evidence.Yes, r.resi
			return "read " + r.resi
		}, add)
}

// verify: before the rules may call a label the wrong one, every photo
// whose read differs from the order is read again from its crop. The
// model is still never told the expected number; the server only
// decides that a mismatch is worth a second look. If the two reads
// agree, the mismatch stands. If they differ, or the crop is not
// legible, the photo's read is withdrawn and the label counts as
// unreadable: "please check" rather than "wrong label".
func verify(ctx context.Context, api API, cfg Config, full []imageAndBytes, photos []evidence.PhotoObs, gs []Grounding, orderResi string, add func(Step)) {
	want := evidence.NormaliseResi(orderResi)
	forEachCrop(ctx, api, cfg, full, photos, gs, "verify",
		func(p evidence.PhotoObs) (string, bool) {
			got := evidence.NormaliseResi(p.ResiText)
			if got == "" || got == want {
				return "", false
			}
			return "read differs from the order", true
		},
		func(i int, r cropRead) string {
			first := evidence.NormaliseResi(photos[i].ResiText)
			if r.readable == evidence.Yes && r.resi == first {
				return "confirmed: both reads agree"
			}
			photos[i].LabelReadable, photos[i].ResiText = evidence.Uncertain, ""
			if r.readable == evidence.Yes {
				return "withdrawn: second read " + r.resi + " differs from first " + first
			}
			return "withdrawn: crop not legible"
		}, add)
}
