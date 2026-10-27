// Package meta is a small client for the Meta Model API: Muse Spark
// chat completions with a strict JSON schema, and SAM 3.1 segmentation.
//
// It is plain net/http on purpose. The API is OpenAI-compatible, but the
// two calls ProofMuse makes are small, and owning the wire format keeps
// retries, usage and cost accounting in one place we can test.
package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the Meta Model API root.
const DefaultBaseURL = "https://api.meta.ai/v1"

// Client calls the Meta Model API. The zero value is not usable; build
// one with New.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	// MaxAttempts bounds tries per call, including the first. Retries
	// happen only on 429, 5xx and transport errors.
	MaxAttempts int
	// Sleep waits between attempts. Tests replace it.
	Sleep func(context.Context, time.Duration) error
}

// New returns a client with production defaults.
func New(apiKey string) *Client {
	return &Client{
		BaseURL:     DefaultBaseURL,
		APIKey:      apiKey,
		HTTP:        &http.Client{Timeout: 120 * time.Second},
		MaxAttempts: 4,
		Sleep:       sleepCtx,
	}
}

// APIError is a non-2xx answer that was not retried or ran out of
// attempts.
type APIError struct {
	Status    int
	Body      string
	RequestID string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("meta api: http %d (request %s): %s", e.Status, e.RequestID, e.Body)
}

// Retryable reports whether the status is worth another attempt.
func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// post sends body to path and returns the open response on 2xx. On a
// retryable failure it backs off exponentially from 500 ms with full
// jitter, or waits exactly Retry-After when the server sends one.
// Attempts is how many requests were made, for the usage log.
func (c *Client) post(ctx context.Context, path string, body any, stream bool) (resp *http.Response, attempts int, err error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("meta: encode request: %w", err)
	}
	max := c.MaxAttempts
	if max < 1 {
		max = 1
	}
	var last error
	for attempts = 1; attempts <= max; attempts++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(payload))
		if err != nil {
			return nil, attempts, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		if stream {
			req.Header.Set("Accept", "text/event-stream")
		}
		resp, err := c.HTTP.Do(req)
		var wait time.Duration
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil, attempts, ctx.Err()
			}
			last = fmt.Errorf("meta: transport: %w", err)
		case resp.StatusCode/100 == 2:
			return resp, attempts, nil
		default:
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			last = &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(b)), RequestID: resp.Header.Get("x-request-id")}
			if !retryable(resp.StatusCode) {
				return nil, attempts, last
			}
			wait = retryAfter(resp.Header.Get("Retry-After"))
		}
		if attempts == max {
			break
		}
		if wait == 0 {
			wait = backoff(attempts)
		}
		if err := c.Sleep(ctx, wait); err != nil {
			return nil, attempts, err
		}
	}
	return nil, max, last
}

// backoff is full jitter over 500 ms * 2^(attempt-1), capped at 8 s.
func backoff(attempt int) time.Duration {
	ceil := 500 * time.Millisecond << (attempt - 1)
	if ceil > 8*time.Second {
		ceil = 8 * time.Second
	}
	return time.Duration(rand.Int64N(int64(ceil))) + 50*time.Millisecond
}

// retryAfter reads a delta-seconds Retry-After. Dates are rare on this
// API and fall back to the computed backoff.
func retryAfter(v string) time.Duration {
	s, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || s < 0 {
		return 0
	}
	if s > 60 {
		s = 60
	}
	return time.Duration(s) * time.Second
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ErrNoContent is a 200 with no visible answer, usually because
// reasoning used the whole output budget.
var ErrNoContent = errors.New("meta: response has no content")
