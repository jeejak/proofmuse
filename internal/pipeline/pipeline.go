// Package pipeline runs one parcel's photos through ProofMuse:
//
//  1. prepare: downscale every photo once, so token cost is bounded.
//  2. observe: one Muse Spark call labels every photo (strict schema).
//  3. ground: SAM 3.1 segments "label" on each photo where a label may be
//     visible, giving a pixel box for the overlay and the crop.
//  4. reread: only where a label is found but not read, Muse Spark reads
//     the resi again from the SAM crop at higher resolution.
//  5. verify: where a read differs from the order, read that label again
//     from the crop; a mismatch stands only if both reads agree.
//  6. decide: evidence.Aggregate and evidence.Decide, no model.
//
// Steps 3 and 4 are conditional on what step 2 saw, and each step is
// written to the trace with the reason it ran or was skipped. Models
// observe; the rules decide.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
)

// API is the slice of the Meta client the pipeline uses. Tests fake it.
type API interface {
	JSON(ctx context.Context, r meta.JSONRequest) ([]byte, meta.Usage, error)
	Segment(ctx context.Context, model string, jpeg []byte, phrase string) ([]meta.Detection, meta.Usage, error)
}

// Config selects models and the cost knobs the eval sweeps.
type Config struct {
	VisionModel  string // muse-spark-1.3
	SegmentModel string // sam-3.1
	Effort       string // reasoning effort for the observation call
	MaxTokens    int    // output budget, reasoning included
	MaxSide      int    // long side of photos sent to Muse Spark, px
	Ground       bool   // run SAM on photos that may show a label
	Reread       bool   // second-pass read on the SAM crop
	RereadEffort string // reasoning effort for the second-pass read
	Verify       bool   // reread a label whose number differs from the order
	// LabelPhrase is SAM's noun phrase. "label" works where "shipping
	// label" returns nothing (measured 2026-10-27 on dev photos).
	LabelPhrase string
	// MinLabelConfidence ignores weaker SAM label matches.
	MinLabelConfidence float64
}

// Defaults are the settings the eval chose (see README results).
func Defaults() Config {
	return Config{
		VisionModel:        "muse-spark-1.3",
		SegmentModel:       "sam-3.1",
		Effort:             "minimal",
		MaxTokens:          8000,
		MaxSide:            1024,
		Ground:             true,
		Reread:             true,
		RereadEffort:       "low",
		Verify:             true,
		LabelPhrase:        "label",
		MinLabelConfidence: 0.5,
	}
}

// Step is one entry of the trace: what ran, why, and what it cost.
type Step struct {
	Name   string     `json:"name"`
	Photo  int        `json:"photo,omitempty"`
	Why    string     `json:"why"`
	Result string     `json:"result"`
	Usage  meta.Usage `json:"usage"`
}

// Grounding is SAM's label box for one photo, on the 0 to 1000 grid.
type Grounding struct {
	Photo      int          `json:"photo"`
	Box        evidence.Box `json:"box"`
	Confidence float64      `json:"confidence"`
}

// Result is everything a caller shows or stores.
type Result struct {
	PromptVersion string              `json:"prompt_version"`
	Config        Config              `json:"config"`
	Photos        []evidence.PhotoObs `json:"photos"` // after reread
	FirstPass     []evidence.PhotoObs `json:"first_pass"`
	Grounding     []Grounding         `json:"grounding"`
	Record        evidence.Record     `json:"record"`
	Verdict       evidence.Result     `json:"verdict"`
	Messages      []evidence.Message  `json:"messages"`
	Trace         []Step              `json:"trace"`
	Usage         meta.Usage          `json:"usage"` // all calls; Model left empty
	Latency       time.Duration       `json:"latency_ns"`
}

// ErrInvalid wraps an observation that failed the schema twice.
var ErrInvalid = errors.New("pipeline: observation invalid")

// Run checks one parcel. photos are the raw image bytes in shooting
// order; order holds the facts the models never see.
func Run(ctx context.Context, api API, cfg Config, photos [][]byte, order evidence.Order) (*Result, error) {
	start := time.Now()
	if len(photos) == 0 || len(photos) > 20 {
		return nil, fmt.Errorf("pipeline: %d photos, want 1 to 20", len(photos))
	}
	res := &Result{PromptVersion: evidence.PromptVersion, Config: cfg}
	add := func(s Step) {
		res.Trace = append(res.Trace, s)
		res.Usage.Add(s.Usage)
	}

	// 1. prepare
	full := make([]imageAndBytes, len(photos))
	for i, b := range photos {
		img, err := decode(b)
		if err != nil {
			return nil, fmt.Errorf("photo %d: %w", i+1, err)
		}
		small, err := encode(fit(img, cfg.MaxSide, 0))
		if err != nil {
			return nil, err
		}
		full[i] = imageAndBytes{img: img, sent: small}
	}

	// 2. observe, with one retry on a schema failure
	parts := []meta.Part{meta.Text(evidence.Prompt(len(photos)))}
	for _, f := range full {
		parts = append(parts, meta.JPEG(f.sent))
	}
	req := meta.JSONRequest{Model: cfg.VisionModel, Effort: cfg.Effort, MaxTokens: cfg.MaxTokens,
		SchemaName: "PackingObservation", Schema: evidence.Schema(), Parts: parts}
	var obs evidence.Observation
	for attempt := 1; ; attempt++ {
		raw, u, err := api.JSON(ctx, req)
		if err == nil {
			obs, err = evidence.ParseObservation(raw, len(photos))
		}
		step := Step{Name: "observe", Why: fmt.Sprintf("label %d photos", len(photos)), Usage: u}
		if err == nil {
			step.Result = "ok"
			add(step)
			break
		}
		step.Result = err.Error()
		add(step)
		retry := errors.Is(err, evidence.ErrSchema) || errors.Is(err, meta.ErrNoContent)
		if !retry || attempt == 2 {
			if retry {
				return res, fmt.Errorf("%w: %v", ErrInvalid, err)
			}
			return res, err
		}
	}
	res.FirstPass = append([]evidence.PhotoObs(nil), obs.Photos...)
	for i := range res.FirstPass {
		res.FirstPass[i].ImageQualityIssues = append([]string(nil), obs.Photos[i].ImageQualityIssues...)
	}

	// 3. ground
	if cfg.Ground {
		res.Grounding = ground(ctx, api, cfg, full, obs.Photos, add)
	}

	// 4. reread
	if cfg.Reread {
		reread(ctx, api, cfg, full, obs.Photos, res.Grounding, add)
	}

	// 4b. verify a mismatch before the rules can call it one
	if cfg.Verify {
		verify(ctx, api, cfg, full, obs.Photos, res.Grounding, order.Resi, add)
	}

	// 5. decide
	res.Photos = obs.Photos
	res.Record = evidence.Aggregate(obs.Photos)
	res.Verdict = evidence.Decide(res.Record, order)
	for _, r := range res.Verdict.Reasons {
		res.Messages = append(res.Messages, evidence.Messages[r])
	}
	res.Latency = time.Since(start)
	return res, ctx.Err()
}

// imageAndBytes keeps the decoded original (for crops) beside the
// downscaled JPEG that was sent.
type imageAndBytes struct {
	img  image.Image
	sent []byte
}
