package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jeejak/proofmuse/client"
	"github.com/jeejak/proofmuse/internal/checks"
	"github.com/jeejak/proofmuse/internal/evalset"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

const testKey = "jeejak-console-test-key-0123456789"

func testServer(t *testing.T, keys map[string]string, run checks.Runner) *httptest.Server {
	t.Helper()
	set, err := evalset.Load("../../evalset")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := checks.New(run, "cfg", "", 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{
		set: set, orders: map[string]evalset.Order{}, svc: svc,
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		limit:    &checks.Limiter{PerClientHour: 100, PerDay: 100},
		keys:     keys,
		apiLimit: &checks.Limiter{PerClientHour: 100, PerDay: 100},
	}
	for _, o := range set.Orders {
		s.orders[o.OrderID] = o
	}
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return ts
}

func jpegBytes(t *testing.T) []byte {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestObserveOffWithoutKeys(t *testing.T) {
	ts := testServer(t, nil, nil)
	resp, err := http.Post(ts.URL+"/v1/observe", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("status %d, want 404 when API_KEYS is empty", resp.StatusCode)
	}
}

func TestObserveEndToEnd(t *testing.T) {
	var runs atomic.Int32
	var gotOrder evidence.Order
	run := func(ctx context.Context, photos [][]byte, o evidence.Order) (*pipeline.Result, error) {
		runs.Add(1)
		gotOrder = o
		return &pipeline.Result{
			PromptVersion: evidence.PromptVersion,
			Config:        pipeline.Defaults(),
			Record:        evidence.Record{ImageUsable: "ok", LabelVisible: "yes", LabelResiRead: "JP123", PackageSealed: "yes", ItemVisible: "unsure", KoliSeen: 1},
			Verdict:       evidence.Result{Verdict: evidence.VerdictKuat},
			Trace:         []pipeline.Step{{Name: "observe", Result: "ok"}, {Name: "ground", Result: "skipped"}, {Name: "verify", Result: "confirmed"}},
		}, nil
	}
	ts := testServer(t, map[string]string{testKey: "jeejak-console"}, run)
	photos := [][]byte{jpegBytes(t), jpegBytes(t)}

	if _, err := client.New(ts.URL, "wrong-key-wrong-key-wrong-key").Observe(context.Background(), client.Request{Photos: photos}); !errors.Is(err, client.ErrAuth) {
		t.Fatalf("wrong key: %v", err)
	}
	c := client.New(ts.URL, testKey)
	res, err := c.Observe(context.Background(), client.Request{Photos: photos, Resi: "jp-123", Koli: 1})
	if err != nil {
		t.Fatal(err)
	}
	o := res.Observation
	if o.ImageUsable != "ok" || o.LabelVisible != "yes" || o.LabelResiRead == nil || *o.LabelResiRead != "JP123" ||
		o.ItemVisible != "unsure" || o.KoliSeen == nil || *o.KoliSeen != 1 || res.Verdict != "kuat" {
		t.Fatalf("result %+v obs %+v", res, o)
	}
	if len(res.Steps) != 2 || res.Steps[1] != "verify" || res.Reasons == nil {
		t.Fatalf("steps %v reasons %v", res.Steps, res.Reasons)
	}
	if gotOrder.Resi != "jp-123" || gotOrder.Koli != 1 {
		t.Fatalf("order passed to the pipeline: %+v", gotOrder)
	}
	again, err := c.Observe(context.Background(), client.Request{Photos: photos, Resi: "JP123", Koli: 1})
	if err != nil || !again.Reused || runs.Load() != 1 {
		t.Fatalf("repeat not answered from the stored result: %+v %v runs %d", again, err, runs.Load())
	}
}

func TestObserveSchemaFailure(t *testing.T) {
	run := func(ctx context.Context, photos [][]byte, o evidence.Order) (*pipeline.Result, error) {
		return nil, pipeline.ErrInvalid
	}
	ts := testServer(t, map[string]string{testKey: "jeejak-console"}, run)
	_, err := client.New(ts.URL, testKey).Observe(context.Background(), client.Request{Photos: [][]byte{jpegBytes(t)}})
	if !errors.Is(err, client.ErrInvalid) {
		t.Fatalf("err %v, want ErrInvalid", err)
	}
}

func TestObserveErrorsAndResiNotStored(t *testing.T) {
	var gotOrder evidence.Order
	run := func(ctx context.Context, photos [][]byte, o evidence.Order) (*pipeline.Result, error) {
		gotOrder = o
		return &pipeline.Result{Config: pipeline.Defaults(), Record: evidence.Record{ImageUsable: "ok", LabelVisible: "no", PackageSealed: "yes", ItemVisible: "yes"}}, nil
	}
	ts := testServer(t, map[string]string{testKey: "jeejak-console"}, run)
	c := client.New(ts.URL, testKey)
	photo := [][]byte{jpegBytes(t)}
	if _, err := c.Observe(context.Background(), client.Request{Photos: photo, Koli: 99}); !errors.Is(err, client.ErrRejected) {
		t.Fatalf("koli 99: err %v, want ErrRejected", err)
	}
	if _, err := c.Observe(context.Background(), client.Request{Photos: photo, Resi: "SPXID5774637064", Koli: 1}); err != nil {
		t.Fatal(err)
	}
	if gotOrder.Resi != "SPXID5774637064" {
		t.Fatalf("pipeline did not get the resi: %+v", gotOrder)
	}
	off := testServer(t, nil, run)
	if _, err := client.New(off.URL, testKey).Observe(context.Background(), client.Request{Photos: photo}); !errors.Is(err, client.ErrOff) {
		t.Fatalf("API off: err %v, want ErrOff", err)
	}
}

func TestOrderIDHidesResi(t *testing.T) {
	id := orderIDFor("jeejak-console", "SPXID5774637064", 1)
	if strings.Contains(id, "5774637064") || !strings.HasPrefix(id, "api:jeejak-console:") || !strings.HasSuffix(id, ":1") {
		t.Fatalf("order id %q", id)
	}
}
