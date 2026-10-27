package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

func postPhotos(t *testing.T, url string, photos [][]byte, fields map[string]string) (*http.Response, []byte) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	for _, p := range photos {
		fw, _ := mw.CreateFormFile("photo", "p.jpg")
		fw.Write(p)
	}
	mw.Close()
	resp, err := http.Post(url, mw.FormDataContentType(), &b)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func TestPacketOnlyForTheCheckedPhotos(t *testing.T) {
	run := func(ctx context.Context, photos [][]byte, o evidence.Order) (*pipeline.Result, error) {
		obs := []evidence.PhotoObs{
			{Photo: 1, ItemInPhoto: "yes", LabelInPhoto: "no", LabelReadable: "no", SealedBoxInPhoto: "no"},
			{Photo: 2, LabelInPhoto: "yes", LabelReadable: "yes", ResiText: "SPXID5774637064", SealedBoxInPhoto: "yes", ItemInPhoto: "no"},
		}
		return &pipeline.Result{Config: pipeline.Defaults(), Photos: obs,
			Grounding: []pipeline.Grounding{{Photo: 2, Box: evidence.Box{XMin: 120, YMin: 110, XMax: 590, YMax: 480}, Confidence: 0.95}},
			Verdict:   evidence.Result{Verdict: evidence.VerdictKuat}}, nil
	}
	ts := testServer(t, nil, run)
	a := jpegBytes(t)
	b := append(append([]byte{}, a...), 0) // a different file that still decodes
	resp, body := postPhotos(t, ts.URL+"/api/checks", [][]byte{a, b}, map[string]string{"order_id": "SYN-R01"})
	if resp.StatusCode != 202 && resp.StatusCode != 200 {
		t.Fatalf("create check: %d %s", resp.StatusCode, body)
	}
	var c struct{ ID, Status string }
	json.Unmarshal(body, &c)
	for i := 0; i < 100 && c.Status != "done"; i++ {
		time.Sleep(10 * time.Millisecond)
		r, _ := http.Get(ts.URL + "/api/checks/" + c.ID)
		json.NewDecoder(r.Body).Decode(&c)
		r.Body.Close()
	}
	url := ts.URL + "/api/checks/" + c.ID + "/packet"

	resp, body = postPhotos(t, url, [][]byte{a, b}, map[string]string{"claim_type": "paket_kosong"})
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatalf("packet: %d %s %q", resp.StatusCode, resp.Header.Get("Content-Type"), body[:min(len(body), 40)])
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="bukti-paket-SPXID5774637064.pdf"` {
		t.Fatalf("content-disposition %q", cd)
	}
	if resp, _ := postPhotos(t, url, [][]byte{b, a}, nil); resp.StatusCode != 409 {
		t.Fatalf("reordered photos: %d, want 409", resp.StatusCode)
	}
	if resp, _ := postPhotos(t, url, [][]byte{a}, nil); resp.StatusCode != 409 {
		t.Fatalf("fewer photos: %d, want 409", resp.StatusCode)
	}
	if resp, _ := postPhotos(t, url, [][]byte{a, b}, map[string]string{"claim_type": "refund"}); resp.StatusCode != 400 {
		t.Fatalf("bad claim type: %d, want 400", resp.StatusCode)
	}
	if resp, _ := postPhotos(t, ts.URL+"/api/checks/nope/packet", [][]byte{a}, nil); resp.StatusCode != 404 {
		t.Fatalf("unknown check: %d, want 404", resp.StatusCode)
	}
}
