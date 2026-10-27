package pipeline

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
)

// MaxClaimRunes bounds a pasted buyer message.
const MaxClaimRunes = 1000

// ClaimResult is a triaged buyer claim.
type ClaimResult struct {
	evidence.Triage
	Usage meta.Usage `json:"usage"`
}

// ClassifyClaim asks Muse Spark which kind of complaint a buyer's message
// is (one enum value, one retry on a schema failure). Everything after
// that, which photos answer it and what is missing, is TriageClaim.
func ClassifyClaim(ctx context.Context, api API, cfg Config, text string) (string, meta.Usage, error) {
	if utf8.RuneCountInString(text) > MaxClaimRunes {
		return "", meta.Usage{}, fmt.Errorf("pipeline: claim longer than %d characters", MaxClaimRunes)
	}
	req := meta.JSONRequest{Model: cfg.VisionModel, Effort: cfg.Effort, MaxTokens: 2000,
		SchemaName: "BuyerClaim", Schema: evidence.ClaimSchema(), Parts: []meta.Part{meta.Text(evidence.ClaimPrompt(text))}}
	var total meta.Usage
	for attempt := 1; ; attempt++ {
		raw, u, err := api.JSON(ctx, req)
		total.Model = u.Model
		total.Add(u)
		if err == nil {
			var c string
			if c, err = evidence.ParseClaim(raw); err == nil {
				return c, total, nil
			}
		}
		retry := errors.Is(err, evidence.ErrSchema) || errors.Is(err, meta.ErrNoContent)
		if !retry || attempt == 2 {
			return "", total, err
		}
	}
}

// TriageClaim classifies the claim and maps it onto a finished check's
// photos.
func TriageClaim(ctx context.Context, api API, cfg Config, text string, photos []evidence.PhotoObs) (*ClaimResult, error) {
	c, u, err := ClassifyClaim(ctx, api, cfg, text)
	if err != nil {
		return &ClaimResult{Usage: u}, err
	}
	return &ClaimResult{Triage: evidence.TriageClaim(c, photos), Usage: u}, nil
}
