package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jeejak/proofmuse/internal/checks"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/packet"
)

// packetPDF renders the evidence packet for a finished check. The server
// keeps no photos, so the page sends them again; they must be exactly the
// photos that were checked (same SHA-256, same order), and nothing is
// stored. The claim triage is recomputed here from the claim type and the
// stored observations, never taken from the browser. No model is called.
func (s *server) packetPDF(w http.ResponseWriter, r *http.Request) {
	c, ok := s.svc.Get(r.PathValue("id"))
	if !ok || c.Status != checks.Done || c.Result == nil {
		writeErr(w, 404, "no finished check with that id")
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
	if len(photos) != len(c.PhotoHash) {
		writeErr(w, 409, "these are not the photos that were checked")
		return
	}
	for i, p := range photos {
		sum := sha256.Sum256(p)
		if hex.EncodeToString(sum[:]) != c.PhotoHash[i] {
			writeErr(w, 409, "these are not the photos that were checked")
			return
		}
	}
	var claim *evidence.Triage
	if ct := r.FormValue("claim_type"); ct != "" {
		if !slices.Contains(evidence.ClaimTypes, ct) {
			writeErr(w, 400, "unknown claim_type")
			return
		}
		t := evidence.TriageClaim(ct, c.Result.Photos)
		claim = &t
	}
	o := s.orders[c.OrderID]
	pdf, err := packet.Render(packet.Input{
		CheckID: c.ID,
		Order: packet.Order{OrderID: o.OrderID, Marketplace: o.Marketplace, Courier: o.Courier,
			Resi: o.Resi, Koli: o.Koli, Item: o.ListingItem},
		Result:      c.Result,
		Photos:      photos,
		Claim:       claim,
		GeneratedAt: time.Now(),
	})
	if err != nil {
		s.log.Warn("packet", "check", c.ID, "err", err)
		writeErr(w, 500, "could not render the packet")
		return
	}
	name := "bukti-paket"
	if o.Resi != "" {
		name += "-" + evidence.NormaliseResi(o.Resi)
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".pdf"))
	w.Header().Set("Cache-Control", "no-store")
	w.Write(pdf)
}
