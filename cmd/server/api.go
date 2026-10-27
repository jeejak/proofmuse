package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jeejak/proofmuse/client"
	"github.com/jeejak/proofmuse/internal/checks"
	"github.com/jeejak/proofmuse/internal/evidence"
)

// apiKeys reads API_KEYS ("name:key,name:key"). Each key names its caller
// in the logs and gets its own daily cap. Empty means the server-to-server
// API is off and /v1/observe answers 404.
func apiKeys() map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(os.Getenv("API_KEYS"), ",") {
		name, key, ok := strings.Cut(strings.TrimSpace(kv), ":")
		if ok && name != "" && len(key) >= 24 {
			out[key] = name
		}
	}
	return out
}

// caller finds the key's name in constant time per key.
func (s *server) caller(r *http.Request) (string, bool) {
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || got == "" {
		return "", false
	}
	for key, name := range s.keys {
		if subtle.ConstantTimeCompare([]byte(got), []byte(key)) == 1 {
			return name, true
		}
	}
	return "", false
}

// observe is the server-to-server API: photos in, the console-shaped
// observation and the verdict out, synchronously. It shares the check
// service, so an identical request is answered from the stored result.
func (s *server) observe(w http.ResponseWriter, r *http.Request) {
	if len(s.keys) == 0 {
		http.NotFound(w, r)
		return
	}
	name, ok := s.caller(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "missing or unknown API key")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotos*maxPhotoBytes+1<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, 400, "upload too large or malformed")
		return
	}
	defer r.MultipartForm.RemoveAll()
	photos, msg := readPhotos(r)
	if msg != "" {
		writeErr(w, 400, msg)
		return
	}
	resi := strings.TrimSpace(r.FormValue("resi"))
	koli, _ := strconv.Atoi(r.FormValue("koli"))
	if koli < 0 || koli > 50 || len(resi) > 64 {
		writeErr(w, 400, "bad resi or koli")
		return
	}
	orderID := fmt.Sprintf("api:%s:%s:%d", name, evidence.NormaliseResi(resi), koli)
	id, _ := checks.Key(orderID, s.svc.ConfigKey, photos)
	if c, ok := s.svc.Get(id); !ok || c.Status == checks.Failed {
		if ok, why := s.apiLimit.Allow(name); !ok {
			writeErr(w, http.StatusTooManyRequests, why)
			return
		}
	}
	c := s.svc.Submit(orderID, evidence.Order{Resi: resi, Koli: koli}, photos)
	ctx, cancel := context.WithTimeout(r.Context(), 140*time.Second)
	defer cancel()
	for c.Status == checks.Queued || c.Status == checks.Running {
		select {
		case <-ctx.Done():
			writeErr(w, http.StatusGatewayTimeout, "check still running; retry with the same photos to collect it")
			return
		case <-time.After(250 * time.Millisecond):
		}
		c, _ = s.svc.Get(id)
	}
	if c.Status == checks.Failed {
		code := http.StatusBadGateway
		if strings.Contains(c.Error, "schema") {
			code = http.StatusUnprocessableEntity
		}
		writeErr(w, code, c.Error)
		return
	}
	s.log.Info("api observe", "caller", name, "id", c.ID, "reused", c.Reused, "verdict", c.Result.Verdict.Verdict)
	writeJSON(w, 200, toAPI(c))
}

// readPhotos reads and type-checks the "photo" parts.
func readPhotos(r *http.Request) ([][]byte, string) {
	files := r.MultipartForm.File["photo"]
	if len(files) == 0 || len(files) > maxPhotos {
		return nil, fmt.Sprintf("send 1 to %d photos", maxPhotos)
	}
	photos := make([][]byte, 0, len(files))
	for _, fh := range files {
		if fh.Size > maxPhotoBytes {
			return nil, "a photo is larger than 12 MB"
		}
		f, err := fh.Open()
		if err != nil {
			return nil, "cannot read photo"
		}
		b, err := io.ReadAll(io.LimitReader(f, maxPhotoBytes+1))
		f.Close()
		if err != nil {
			return nil, "cannot read photo"
		}
		if ct := http.DetectContentType(b); ct != "image/jpeg" && ct != "image/png" {
			return nil, "photos must be JPEG or PNG"
		}
		photos = append(photos, b)
	}
	return photos, ""
}

// toAPI maps a finished check to the client's Result.
func toAPI(c *checks.Check) client.Result {
	res := c.Result
	rec := res.Record
	obs := client.Observation{
		ImageUsable:   rec.ImageUsable,
		LabelVisible:  rec.LabelVisible,
		PackageSealed: rec.PackageSealed,
		ItemVisible:   rec.ItemVisible,
	}
	if rec.LabelResiRead != "" {
		v := rec.LabelResiRead
		obs.LabelResiRead = &v
	}
	k := rec.KoliSeen
	obs.KoliSeen = &k
	var steps []string
	for _, st := range res.Trace {
		if st.Result != "skipped" {
			steps = append(steps, st.Name)
		}
	}
	return client.Result{
		ID:            c.ID,
		Reused:        c.Reused,
		PromptVersion: res.PromptVersion,
		Observation:   obs,
		Verdict:       res.Verdict.Verdict,
		Reasons:       nonNil(res.Verdict.Reasons),
		Steps:         steps,
		Usage: client.Usage{
			Model:           res.Config.VisionModel,
			InputTokens:     res.Usage.InputTokens,
			OutputTokens:    res.Usage.OutputTokens,
			ReasoningTokens: res.Usage.ReasoningTokens,
			SAMImages:       res.Usage.Images,
			CostUSD:         res.Usage.CostUSD,
			LatencyMS:       res.Latency.Milliseconds(),
		},
	}
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
