package meta

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Part is one piece of a user message: text or an image.
type Part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// Text is a text part.
func Text(s string) Part { return Part{Type: "text", Text: s} }

// JPEG is an inline image part.
func JPEG(b []byte) Part {
	return Part{Type: "image_url", ImageURL: &imageURL{URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b)}}
}

// JSONRequest is a single-turn structured-output call.
type JSONRequest struct {
	Model string
	// Effort is reasoning_effort: minimal, low, medium, high, xhigh, max.
	// Muse Spark cannot turn reasoning off; "none" is a 400.
	Effort string
	// MaxTokens caps reasoning plus visible output together.
	MaxTokens  int
	SchemaName string
	Schema     map[string]any
	Parts      []Part
}

// Usage is what one call cost, as the API reported it.
type Usage struct {
	Model           string        `json:"model"`
	InputTokens     int           `json:"input_tokens"`
	CachedTokens    int           `json:"cached_tokens"`
	OutputTokens    int           `json:"output_tokens"`
	ReasoningTokens int           `json:"reasoning_tokens"`
	Images          int           `json:"images"`
	Attempts        int           `json:"attempts"`
	Latency         time.Duration `json:"latency_ns"`
	CostUSD         float64       `json:"cost_usd"`
}

// Add folds another call's usage into u (for a pipeline total).
func (u *Usage) Add(o Usage) {
	u.InputTokens += o.InputTokens
	u.CachedTokens += o.CachedTokens
	u.OutputTokens += o.OutputTokens
	u.ReasoningTokens += o.ReasoningTokens
	u.Images += o.Images
	u.Attempts += o.Attempts
	u.Latency += o.Latency
	u.CostUSD += o.CostUSD
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content *string `json:"content"`
			Refusal *string `json:"refusal"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

// JSON runs a structured-output chat completion and returns the raw
// JSON text of the answer. The caller validates it; decoding is
// constrained to the schema, but ProofMuse never trusts that alone.
func (c *Client) JSON(ctx context.Context, r JSONRequest) ([]byte, Usage, error) {
	body := map[string]any{
		"model": r.Model,
		"messages": []map[string]any{{
			"role":    "user",
			"content": r.Parts,
		}},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   r.SchemaName,
				"strict": true,
				"schema": r.Schema,
			},
		},
	}
	if r.Effort != "" {
		body["reasoning_effort"] = r.Effort
	}
	if r.MaxTokens > 0 {
		body["max_tokens"] = r.MaxTokens
	}
	start := time.Now()
	resp, attempts, err := c.post(ctx, "/chat/completions", body, false)
	usage := Usage{Model: r.Model, Attempts: attempts}
	if err != nil {
		usage.Latency = time.Since(start)
		return nil, usage, err
	}
	defer resp.Body.Close()
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, usage, fmt.Errorf("meta: decode chat response: %w", err)
	}
	usage.Latency = time.Since(start)
	usage.InputTokens = cr.Usage.PromptTokens
	usage.CachedTokens = cr.Usage.PromptTokensDetails.CachedTokens
	usage.OutputTokens = cr.Usage.CompletionTokens
	usage.ReasoningTokens = cr.Usage.CompletionTokensDetails.ReasoningTokens
	for _, p := range r.Parts {
		if p.Type == "image_url" {
			usage.Images++
		}
	}
	usage.CostUSD = Cost(usage)
	if len(cr.Choices) == 0 {
		return nil, usage, ErrNoContent
	}
	ch := cr.Choices[0]
	if ch.Message.Refusal != nil && *ch.Message.Refusal != "" {
		return nil, usage, fmt.Errorf("meta: refused: %s", *ch.Message.Refusal)
	}
	if ch.Message.Content == nil || *ch.Message.Content == "" {
		return nil, usage, fmt.Errorf("%w (finish_reason %s)", ErrNoContent, ch.FinishReason)
	}
	return []byte(*ch.Message.Content), usage, nil
}
