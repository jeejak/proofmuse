package packet

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

func photo(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.White)
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRender(t *testing.T) {
	obs := []evidence.PhotoObs{{Photo: 1, ImageQualityIssues: []string{"too_dark"}}, {Photo: 2, LabelInPhoto: "yes", LabelReadable: "yes", ResiText: "jp-123"}, {Photo: 3, SealedBoxInPhoto: "yes"}}
	res := &pipeline.Result{Photos: obs, Grounding: []pipeline.Grounding{{Photo: 2, Box: evidence.Box{XMin: 100, YMin: 100, XMax: 500, YMax: 400}}},
		Verdict: evidence.Result{Verdict: evidence.VerdictLemah, Reasons: []string{evidence.ReasonImageDark, evidence.ReasonNotSealed}}}
	tri := evidence.TriageClaim(evidence.ClaimPaketKosong, obs)
	out, err := Render(Input{CheckID: "abc123", Order: Order{Resi: "JP123", Courier: "JNE", Koli: 1, Marketplace: "shopee"},
		Result: res, Photos: [][]byte{photo(t, 2000, 1500), photo(t, 640, 480), photo(t, 480, 640)}, Claim: &tri, GeneratedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) || len(out) < 2000 {
		t.Fatalf("not a PDF: %d bytes", len(out))
	}
	if _, err := Render(Input{Result: res, Photos: [][]byte{photo(t, 10, 10)}}); err == nil {
		t.Fatal("rendered with a photo count that does not match the check")
	}
}

func TestPhotoNoteUsesFixedStrings(t *testing.T) {
	n := photoNote(evidence.PhotoObs{ImageQualityIssues: []string{"blur"}, LabelInPhoto: "yes", LabelReadable: "no", SealedBoxInPhoto: "yes"})
	if n != "buram/blurry · label tidak terbaca/unreadable · tersegel/sealed" {
		t.Fatalf("note %q", n)
	}
	if n := photoNote(evidence.PhotoObs{LabelReadable: "yes", ResiText: "jp 12-3"}); n != "resi JP123" {
		t.Fatalf("note %q", n)
	}
}
