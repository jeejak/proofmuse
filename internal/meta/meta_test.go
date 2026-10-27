package meta

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A real SAM 3.1 line captured 2026-10-27 (R04/04.jpg, "cardboard box"),
// with mask payloads shortened. The payload alphabet includes "," and
// digits, which is why the parser reads box tokens only.
const twoBoxes = `<0f>0<|box;x1=6;y1=178;x2=895;y2=923;w=1280;h=960;c=0.96|><|mask;x=0;y=0;c=0.96;data=769,1024,!!!!*g,9<|box;x1=1|D>|>,1<|box;x1=780;y1=93;x2=1161;y2=452;w=1280;h=960;c=0.89|><|mask;x=0;y=0;c=0.89;data=769,1024,!!#Kq|>`

func TestParseSegmentation(t *testing.T) {
	got, err := ParseSegmentation(twoBoxes)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d detections, want 2: %+v", len(got), got)
	}
	if got[0].ID != 0 || got[0].X2 != 895 || got[0].Confidence != 0.96 || got[1].ID != 1 || got[1].Y1 != 93 {
		t.Fatalf("wrong detections: %+v", got)
	}
	if g := got[0].Grid(); g != [4]int{5, 185, 699, 961} {
		t.Fatalf("grid: got %v", g)
	}
}

func TestParseSegmentationEmptyAndBad(t *testing.T) {
	if d, err := ParseSegmentation("  "); err != nil || d != nil {
		t.Fatalf("empty: %v %v", d, err)
	}
	for _, bad := range []string{
		`0<|box;x1=1;y1=1;x2=2;y2=2;w=9;h=9|>`,         // no frame marker
		`<0f>0<|box;x1=1;y1=1;x2=2;w=9;h=9|>`,          // missing y2
		`<0f>0<|box;x1=5;y1=1;x2=2;y2=2;w=9;h=9|>`,     // inverted
		`<0f>0<|box;x1=a;y1=1;x2=2;y2=2;w=9;h=9|>`,     // not a number
		`<0f>0<|box;x1=1;y1=1;x2=2;y2=2;w=9;h=9;c=x|>`, // bad confidence
	} {
		if _, err := ParseSegmentation(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func testClient(url string) (*Client, *[]time.Duration) {
	var waits []time.Duration
	c := New("test-key")
	c.BaseURL = url
	c.Sleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	return c, &waits
}

const chatOK = `{"choices":[{"finish_reason":"stop","message":{"content":"{\"photos\":[]}"}}],
"usage":{"prompt_tokens":1000,"completion_tokens":200,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":150}}}`

func TestJSONRetriesThenSucceeds(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		switch n.Add(1) {
		case 1:
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
		case 2:
			w.WriteHeader(503)
		default:
			fmt.Fprint(w, chatOK)
		}
	}))
	defer srv.Close()
	c, waits := testClient(srv.URL)
	out, u, err := c.JSON(context.Background(), JSONRequest{Model: "muse-spark-1.3", Parts: []Part{Text("x"), JPEG([]byte{1})}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"photos":[]}` || u.Attempts != 3 || u.Images != 1 || u.ReasoningTokens != 150 {
		t.Fatalf("out %s usage %+v", out, u)
	}
	if (*waits)[0] != 2*time.Second {
		t.Fatalf("Retry-After not honoured: %v", *waits)
	}
	want := (1000*1.25 + 200*4.25) / 1e6
	if diff := u.CostUSD - want; diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("cost %v, want %v", u.CostUSD, want)
	}
}

func TestJSONDoesNotRetry400(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("x-request-id", "req-1")
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":"bad schema"}`)
	}))
	defer srv.Close()
	c, _ := testClient(srv.URL)
	_, _, err := c.JSON(context.Background(), JSONRequest{Model: "muse-spark-1.3"})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 400 || ae.RequestID != "req-1" || n.Load() != 1 {
		t.Fatalf("err %v, calls %d", err, n.Load())
	}
}

func TestJSONGivesUpAfterMaxAttempts(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c, waits := testClient(srv.URL)
	_, u, err := c.JSON(context.Background(), JSONRequest{Model: "muse-spark-1.3"})
	if err == nil || n.Load() != 4 || u.Attempts != 4 || len(*waits) != 3 {
		t.Fatalf("err %v calls %d attempts %d waits %d", err, n.Load(), u.Attempts, len(*waits))
	}
}

func TestJSONReasoningAteBudget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"finish_reason":"length","message":{"content":null}}],"usage":{"prompt_tokens":15,"completion_tokens":20,"completion_tokens_details":{"reasoning_tokens":18}}}`)
	}))
	defer srv.Close()
	c, _ := testClient(srv.URL)
	_, u, err := c.JSON(context.Background(), JSONRequest{Model: "muse-spark-1.3"})
	if !errors.Is(err, ErrNoContent) || !strings.Contains(err.Error(), "length") || u.OutputTokens != 20 {
		t.Fatalf("err %v usage %+v", err, u)
	}
}

func TestSegmentStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("accept %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.created\ndata: {\"type\":\"response.created\"}\n\n")
		fmt.Fprintf(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", twoBoxes)
		fmt.Fprintf(w, "event: response.output_text.done\ndata: {\"type\":\"response.output_text.done\",\"text\":%q}\n\n", twoBoxes)
		fmt.Fprint(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"images_processed\":1}}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	c, _ := testClient(srv.URL)
	d, u, err := c.Segment(context.Background(), "sam-3.1", []byte{1}, "cardboard box")
	if err != nil || len(d) != 2 || u.Images != 1 || u.CostUSD != 0.0025 {
		t.Fatalf("d %v usage %+v err %v", d, u, err)
	}
}

func TestSegmentStreamCutShort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", twoBoxes)
	}))
	defer srv.Close()
	c, _ := testClient(srv.URL)
	if _, _, err := c.Segment(context.Background(), "sam-3.1", []byte{1}, "label"); err == nil {
		t.Fatal("accepted a stream with no response.completed")
	}
}

func TestSplitRoutesCalls(t *testing.T) {
	var vision, sam atomic.Int32
	vs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vision.Add(1)
		if r.URL.Path != "/chat/completions" {
			t.Errorf("vision path %s", r.URL.Path)
		}
		fmt.Fprint(w, chatOK)
	}))
	defer vs.Close()
	ss := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sam.Add(1)
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"images_processed\":1}}}\n\n")
	}))
	defer ss.Close()
	v, _ := testClient(vs.URL)
	g, _ := testClient(ss.URL)
	sp := Split{Vision: v, Segmenter: g}
	if _, _, err := sp.JSON(context.Background(), JSONRequest{Model: "meta-models/muse-spark-open-1026"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sp.Segment(context.Background(), "sam-3.1", []byte{1}, "label"); err != nil {
		t.Fatal(err)
	}
	if vision.Load() != 1 || sam.Load() != 1 {
		t.Fatalf("vision %d sam %d", vision.Load(), sam.Load())
	}
	SetPrice("meta-models/muse-spark-open-1026", 1, 2)
	if c := Cost(Usage{Model: "meta-models/muse-spark-open-1026", InputTokens: 1e6, OutputTokens: 1e6}); c != 3 {
		t.Fatalf("cost %v", c)
	}
}
