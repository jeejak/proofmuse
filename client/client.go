// Package client calls a ProofMuse server from another backend. It is
// what the Jeejak console uses to run ProofMuse on packing photos its
// phones have synced, and it has no dependencies beyond the standard
// library so any Go service can import it.
//
//	c := client.New("https://proofmuse.jeejak.id", os.Getenv("PROOFMUSE_API_KEY"))
//	res, err := c.Observe(ctx, client.Request{Photos: jpegs, Resi: "JP123", Koli: 1})
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Request is one parcel's packing photos and the two order facts the
// server compares against. The models never see Resi or Koli; ProofMuse
// uses Resi only to decide whether a mismatching read must be verified.
type Request struct {
	Photos [][]byte // JPEG or PNG, in shooting order, 1 to 8
	Resi   string   // the order's tracking number; optional
	Koli   int      // parcels in the order; 0 when unknown
}

// Observation is the record-level answer in the six-field shape the
// Jeejak console's evidence check stores. Values: image_usable is ok,
// dark, blurry or obstructed; the tri-state fields are yes, no or unsure.
type Observation struct {
	ImageUsable   string  `json:"image_usable"`
	LabelVisible  string  `json:"label_visible"`
	LabelResiRead *string `json:"label_resi_read"`
	PackageSealed string  `json:"package_sealed"`
	ItemVisible   string  `json:"item_visible"`
	KoliSeen      *int    `json:"koli_seen"`
}

// Usage is what the check cost, all model calls together.
type Usage struct {
	Model           string  `json:"model"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	ReasoningTokens int     `json:"reasoning_tokens"`
	SAMImages       int     `json:"sam_images"`
	CostUSD         float64 `json:"cost_usd"`
	LatencyMS       int64   `json:"latency_ms"`
}

// Result is the server's answer.
type Result struct {
	ID            string      `json:"id"`
	Reused        bool        `json:"reused"`
	PromptVersion string      `json:"prompt_version"`
	Observation   Observation `json:"observation"`
	Verdict       string      `json:"verdict"`
	Reasons       []string    `json:"reasons"`
	Usage         Usage       `json:"usage"`
	// Steps names the pipeline steps that ran (observe, ground, reread,
	// verify), for logs.
	Steps []string `json:"steps"`
}

// Errors the caller can act on.
var (
	ErrAuth    = errors.New("proofmuse: API key rejected")
	ErrLimit   = errors.New("proofmuse: daily limit reached")
	ErrInvalid = errors.New("proofmuse: model answer did not match the schema")
)

// Client talks to one ProofMuse server.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New returns a client with a timeout that covers a full pipeline run.
func New(baseURL, apiKey string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, HTTP: &http.Client{Timeout: 150 * time.Second}}
}

// Observe runs ProofMuse on one parcel and waits for the answer. The
// same photos for the same order return the stored result without a
// new model call.
func (c *Client) Observe(ctx context.Context, r Request) (*Result, error) {
	if len(r.Photos) == 0 {
		return nil, errors.New("proofmuse: no photos")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i, p := range r.Photos {
		fw, err := mw.CreateFormFile("photo", strconv.Itoa(i+1)+".jpg")
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(p); err != nil {
			return nil, err
		}
	}
	if r.Resi != "" {
		_ = mw.WriteField("resi", r.Resi)
	}
	if r.Koli > 0 {
		_ = mw.WriteField("koli", strconv.Itoa(r.Koli))
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/observe", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proofmuse: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrAuth
	case http.StatusTooManyRequests:
		return nil, ErrLimit
	case http.StatusUnprocessableEntity:
		return nil, ErrInvalid
	default:
		return nil, fmt.Errorf("proofmuse: http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var res Result
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, fmt.Errorf("proofmuse: decode: %w", err)
	}
	return &res, nil
}
