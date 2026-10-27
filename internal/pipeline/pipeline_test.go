package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
)

type fakeAPI struct {
	mu        sync.Mutex
	observe   []string // answers to observation calls, in order
	reread    string
	sam       map[int][]meta.Detection // by photo width, to tell photos apart
	calls     []string
	readSizes []int
}

func (f *fakeAPI) JSON(_ context.Context, r meta.JSONRequest) ([]byte, meta.Usage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u := meta.Usage{Model: r.Model, InputTokens: 100, OutputTokens: 10}
	if r.SchemaName == "ResiRead" {
		f.calls = append(f.calls, "reread")
		img, _ := jpeg.Decode(bytes.NewReader(mustB64(r.Parts[1].ImageURL.URL)))
		f.readSizes = append(f.readSizes, max(img.Bounds().Dx(), img.Bounds().Dy()))
		return []byte(f.reread), u, nil
	}
	f.calls = append(f.calls, "observe")
	a := f.observe[0]
	f.observe = f.observe[1:]
	return []byte(a), u, nil
}

func (f *fakeAPI) Segment(_ context.Context, _ string, b []byte, phrase string) ([]meta.Detection, meta.Usage, error) {
	img, _ := jpeg.Decode(bytes.NewReader(b))
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "segment:"+phrase)
	return f.sam[img.Bounds().Dx()], meta.Usage{Model: "sam-3.1", Images: 1, CostUSD: 0.0025}, nil
}

func jpg(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.White)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func photo(n int, label, readable, resi string) string {
	box := `{"x_min":0,"y_min":0,"x_max":0,"y_max":0}`
	if label == "yes" {
		box = `{"x_min":100,"y_min":100,"x_max":400,"y_max":300}`
	}
	return fmt.Sprintf(`{"photo":%d,"label_in_photo":%q,"label_readable":%q,"resi_text":%q,"sealed_box_in_photo":"yes","item_in_photo":"yes","boxes_in_photo":1,"image_quality_issues":[],"label_box":%s}`, n, label, readable, resi, box)
}

func TestRunRereadsOnlyUnreadLabels(t *testing.T) {
	// Photo 1 (800 px wide): no label, SAM must not run on it.
	// Photo 2 (1000 px): label read on the first pass, no reread.
	// Photo 3 (1200 px): label located but unreadable, reread succeeds.
	obs := `{"photos":[` + photo(1, "no", "no", "") + "," + photo(2, "yes", "yes", "JP1") + "," + photo(3, "yes", "no", "") + `]}`
	f := &fakeAPI{
		observe: []string{obs},
		reread:  `{"readable":"yes","resi_text":"jp-1"}`,
		sam: map[int][]meta.Detection{
			1000: {{X1: 100, Y1: 100, X2: 400, Y2: 300, W: 1000, H: 750, Confidence: 0.9}},
			1024: {{X1: 102, Y1: 77, X2: 410, Y2: 230, W: 1024, H: 512, Confidence: 0.95}, {X1: 0, Y1: 0, X2: 9, Y2: 9, W: 1024, H: 512, Confidence: 0.2}},
		},
	}
	photos := [][]byte{jpg(t, 800, 600), jpg(t, 1000, 750), jpg(t, 2400, 1200)}
	res, err := Run(context.Background(), f, Defaults(), photos, evidence.Order{Resi: "JP1", Koli: 1})
	if err != nil {
		t.Fatal(err)
	}
	segs := 0
	for _, c := range f.calls {
		if c == "segment:label" {
			segs++
		}
	}
	if segs != 2 || slices.Index(f.calls, "reread") < 0 || strings.Count(strings.Join(f.calls, ","), "reread") != 1 {
		t.Fatalf("calls %v", f.calls)
	}
	if f.readSizes[0] < 1024 {
		t.Errorf("reread crop long side %d, want at least 1024 (upscaled)", f.readSizes[0])
	}
	if res.FirstPass[2].LabelReadable != "no" || res.Photos[2].LabelReadable != "yes" {
		t.Errorf("first pass not preserved or reread not applied: %+v / %+v", res.FirstPass[2], res.Photos[2])
	}
	if res.Verdict.Verdict != evidence.VerdictKuat || res.Record.ResiConflict {
		t.Errorf("verdict %+v record %+v", res.Verdict, res.Record)
	}
	if len(res.Grounding) != 2 || res.Grounding[1].Confidence != 0.95 {
		t.Errorf("grounding %+v", res.Grounding)
	}
	if res.Usage.Images != 2 || res.Usage.InputTokens != 200 {
		t.Errorf("usage %+v", res.Usage)
	}
}

func TestRunRetriesSchemaFailureOnce(t *testing.T) {
	ok := `{"photos":[` + photo(1, "no", "no", "") + `]}`
	f := &fakeAPI{observe: []string{`{"photos":[]}`, ok}}
	cfg := Defaults()
	cfg.Ground, cfg.Reread = false, false
	res, err := Run(context.Background(), f, cfg, [][]byte{jpg(t, 640, 480)}, evidence.Order{Resi: "JP1", Koli: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trace) != 2 || res.Trace[0].Result == "ok" || res.Verdict.Verdict != evidence.VerdictLemah {
		t.Fatalf("trace %+v verdict %+v", res.Trace, res.Verdict)
	}
	f = &fakeAPI{observe: []string{`nope`, `{"photos":[]}`}}
	if _, err := Run(context.Background(), f, cfg, [][]byte{jpg(t, 640, 480)}, evidence.Order{}); err == nil {
		t.Fatal("two schema failures accepted")
	}
}

func TestRunDownscalesBeforeSending(t *testing.T) {
	var sent int
	f := &fakeAPI{observe: []string{`{"photos":[` + photo(1, "no", "no", "") + `]}`}}
	cfg := Defaults()
	cfg.Ground, cfg.Reread = false, false
	api := &sizeSpy{fakeAPI: f, size: &sent}
	if _, err := Run(context.Background(), api, cfg, [][]byte{jpg(t, 4000, 3000)}, evidence.Order{}); err != nil {
		t.Fatal(err)
	}
	if sent != 1024 {
		t.Fatalf("sent long side %d, want 1024", sent)
	}
}

type sizeSpy struct {
	*fakeAPI
	size *int
}

func (s *sizeSpy) JSON(ctx context.Context, r meta.JSONRequest) ([]byte, meta.Usage, error) {
	img, _ := jpeg.Decode(bytes.NewReader(mustB64(r.Parts[1].ImageURL.URL)))
	*s.size = max(img.Bounds().Dx(), img.Bounds().Dy())
	return s.fakeAPI.JSON(ctx, r)
}

func TestVerifyBeforeMismatch(t *testing.T) {
	obs := `{"photos":[` + photo(1, "yes", "yes", "JP4975753028") + `]}`
	sam := map[int][]meta.Detection{1000: {{X1: 100, Y1: 100, X2: 400, Y2: 300, W: 1000, H: 750, Confidence: 0.9}}}
	order := evidence.Order{Resi: "JP4975753128", Koli: 1}
	for _, c := range []struct {
		name, reread, verdict, reason string
	}{
		{"second read agrees", `{"readable":"yes","resi_text":"JP 4975753028"}`, evidence.VerdictLemah, evidence.ReasonLabelMismatch},
		{"second read differs", `{"readable":"yes","resi_text":"JP4975753128"}`, evidence.VerdictPerluDicek, evidence.ReasonLabelUnreadable},
		{"crop not legible", `{"readable":"uncertain","resi_text":""}`, evidence.VerdictPerluDicek, evidence.ReasonLabelUnreadable},
	} {
		f := &fakeAPI{observe: []string{obs}, reread: c.reread, sam: sam}
		res, err := Run(context.Background(), f, Defaults(), [][]byte{jpg(t, 1000, 750)}, order)
		if err != nil {
			t.Fatal(err)
		}
		if res.Verdict.Verdict != c.verdict || !slices.Equal(res.Verdict.Reasons, []string{c.reason}) {
			t.Errorf("%s: %+v trace %+v", c.name, res.Verdict, res.Trace)
		}
		if res.FirstPass[0].ResiText != "JP4975753028" {
			t.Errorf("%s: first pass overwritten", c.name)
		}
	}
	// A read that matches the order is never re-checked.
	f := &fakeAPI{observe: []string{`{"photos":[` + photo(1, "yes", "yes", "JP4975753128") + `]}`}, sam: sam}
	if _, err := Run(context.Background(), f, Defaults(), [][]byte{jpg(t, 1000, 750)}, order); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(f.calls, "reread") {
		t.Errorf("verified a matching read: %v", f.calls)
	}
}

func TestTriageClaimUsesCheckPhotos(t *testing.T) {
	f := &fakeAPI{observe: []string{`{"claim_type":"refund"}`, `{"claim_type":"salah_kirim"}`}}
	photos := []evidence.PhotoObs{
		{Photo: 1, ItemInPhoto: "yes", LabelReadable: "no", SealedBoxInPhoto: "no"},
		{Photo: 2, SealedBoxInPhoto: "yes", LabelReadable: "yes", ItemInPhoto: "no"},
	}
	r, err := TriageClaim(context.Background(), f, Defaults(), "warnanya beda kak", photos)
	if err != nil {
		t.Fatal(err)
	}
	if r.ClaimType != "salah_kirim" || !slices.Equal(r.Answers, []int{1, 2}) || len(r.Missing) != 0 || r.Usage.InputTokens != 200 {
		t.Fatalf("%+v", r)
	}
	if _, err := TriageClaim(context.Background(), f, Defaults(), strings.Repeat("x", MaxClaimRunes+1), nil); err == nil {
		t.Fatal("accepted an over-long claim")
	}
}
