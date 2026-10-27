// Package packet renders the evidence packet: one PDF a seller can attach
// to a Shopee or TikTok Shop dispute. It holds the order facts, the
// verdict and its reasons, the buyer-claim triage when there is one, the
// packing photos with SAM's label box drawn on them, and each photo's
// SHA-256 so a reviewer can confirm the photos were not changed.
//
// Every sentence in it is a fixed string from internal/evidence; no
// model wrote any of it. The packet is advice about evidence quality,
// never proof of what was in the box, and it says so on every page.
package packet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // photos may arrive as PNG
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/draw"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

// Order is what the packet states about the order.
type Order struct {
	OrderID     string
	Marketplace string
	Courier     string
	Resi        string
	Koli        int
	Item        string
}

// Input is everything one packet needs.
type Input struct {
	CheckID     string
	Order       Order
	Result      *pipeline.Result
	Photos      [][]byte // the photos the check ran on, same order
	Claim       *evidence.Triage
	GeneratedAt time.Time
}

var wib = time.FixedZone("WIB", 7*60*60)

const (
	pageW  = 210.0
	margin = 14.0
	inner  = pageW - 2*margin
)

// Render builds the PDF.
func Render(in Input) ([]byte, error) {
	if in.Result == nil {
		return nil, fmt.Errorf("packet: no result")
	}
	if len(in.Photos) != len(in.Result.Photos) {
		return nil, fmt.Errorf("packet: %d photos for a check of %d", len(in.Photos), len(in.Result.Photos))
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, 22)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	gen := in.GeneratedAt.In(wib)

	pdf.SetFooterFunc(func() {
		pdf.SetY(-18)
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(90, 100, 120)
		pdf.MultiCell(inner, 3.6, tr(evidence.Advisory+" "+evidence.AdvisoryEN+
			fmt.Sprintf(" ProofMuse by Jeejak, check %s, page %d.", short(in.CheckID), pdf.PageNo())), "", "C", false)
	})
	pdf.AddPage()

	// Header.
	pdf.SetFont("Helvetica", "B", 17)
	pdf.SetTextColor(14, 33, 82)
	pdf.CellFormat(inner, 8, tr("Paket Bukti Pengemasan"), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(90, 100, 120)
	pdf.CellFormat(inner, 5, tr("Packing evidence packet · ProofMuse by Jeejak · dibuat "+gen.Format("2 Jan 2006 15.04")+" WIB"), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// Order facts.
	o := in.Order
	rows := [][2]string{
		{"Resi / tracking no.", o.Resi},
		{"Kurir / courier", o.Courier},
		{"Koli / parcels", fmt.Sprint(o.Koli)},
		{"Marketplace", o.Marketplace},
		{"Pesanan / order", o.OrderID},
		{"Barang / item", o.Item},
	}
	pdf.SetTextColor(20, 27, 51)
	for _, r := range rows {
		if strings.TrimSpace(r[1]) == "" {
			continue
		}
		pdf.SetFont("Helvetica", "", 9.5)
		pdf.CellFormat(42, 6, tr(r[0]), "B", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 9.5)
		pdf.CellFormat(inner-42, 6, tr(r[1]), "B", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	// Verdict.
	v := in.Result.Verdict
	vt := evidence.VerdictText[v.Verdict]
	fill := map[string][3]int{evidence.VerdictKuat: {227, 244, 232}, evidence.VerdictPerluDicek: {253, 240, 213}, evidence.VerdictLemah: {251, 229, 227}}[v.Verdict]
	ink := map[string][3]int{evidence.VerdictKuat: {29, 122, 70}, evidence.VerdictPerluDicek: {138, 90, 0}, evidence.VerdictLemah: {179, 38, 30}}[v.Verdict]
	pdf.SetFillColor(fill[0], fill[1], fill[2])
	pdf.SetTextColor(ink[0], ink[1], ink[2])
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(inner, 10, tr("  "+vt[0]+"  ·  "+vt[1]), "", 1, "L", true, 0, "")
	pdf.SetTextColor(20, 27, 51)
	pdf.Ln(2)
	if len(v.Reasons) == 0 {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(inner, 5, tr("Semua yang dicek terlihat jelas, dan label cocok dengan pesanan. / Everything checked is clearly shown, and the label matches the order."), "", "L", false)
	}
	for _, code := range v.Reasons {
		m := evidence.Messages[code]
		pdf.SetFont("Helvetica", "B", 10)
		pdf.MultiCell(inner, 5, tr(m.Problem+" / "+m.ProblemEN), "", "L", false)
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(inner, 4.5, tr(m.Retake+" / "+m.RetakeEN), "", "L", false)
		pdf.Ln(1.5)
	}

	// Buyer claim.
	if c := in.Claim; c != nil {
		pdf.Ln(2)
		ct := evidence.ClaimText[c.ClaimType]
		pdf.SetFont("Helvetica", "B", 11)
		pdf.MultiCell(inner, 6, tr("Komplain pembeli: "+ct.ID+" / Buyer complaint: "+ct.EN), "", "L", false)
		pdf.SetFont("Helvetica", "", 9.5)
		if len(c.Answers) > 0 {
			nums := make([]string, len(c.Answers))
			for i, n := range c.Answers {
				nums[i] = fmt.Sprintf("#%d", n)
			}
			pdf.MultiCell(inner, 5, tr("Foto yang menjawab / photos that answer it: "+strings.Join(nums, ", ")), "", "L", false)
		} else if c.ClaimType != evidence.ClaimLainnya {
			pdf.MultiCell(inner, 5, tr("Belum ada foto yang menjawab. / No photo answers it yet."), "", "L", false)
		}
		for _, g := range c.Missing {
			t := evidence.GapText[g]
			pdf.MultiCell(inner, 4.5, tr("- "+t.ID+" / "+t.EN), "", "L", false)
		}
	}

	// Photos, two per row.
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(inner, 6, tr("Foto packing / Packing photos"), "", 1, "L", false, 0, "")
	boxes := map[int]evidence.Box{}
	for _, g := range in.Result.Grounding {
		boxes[g.Photo] = g.Box
	}
	answers := map[int]bool{}
	if in.Claim != nil {
		for _, n := range in.Claim.Answers {
			answers[n] = true
		}
	}
	colW := (inner - 6) / 2
	hashes := make([]string, len(in.Photos))
	for i, raw := range in.Photos {
		sum := sha256.Sum256(raw)
		hashes[i] = hex.EncodeToString(sum[:])
		jp, w, h, err := toJPEG(raw)
		if err != nil {
			return nil, fmt.Errorf("packet: photo %d: %w", i+1, err)
		}
		imgH := colW * float64(h) / float64(w)
		if i%2 == 0 && pdf.GetY()+imgH+14 > 297-22 {
			pdf.AddPage()
		}
		x := margin + float64(i%2)*(colW+6)
		y := pdf.GetY()
		name := fmt.Sprintf("p%d", i+1)
		pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "JPG"}, bytes.NewReader(jp))
		pdf.ImageOptions(name, x, y, colW, imgH, false, fpdf.ImageOptions{ImageType: "JPG"}, 0, "")
		if b, ok := boxes[i+1]; ok && !b.IsZero() {
			pdf.SetDrawColor(19, 163, 111)
			pdf.SetLineWidth(0.6)
			pdf.Rect(x+colW*float64(b.XMin)/1000, y+imgH*float64(b.YMin)/1000,
				colW*float64(b.XMax-b.XMin)/1000, imgH*float64(b.YMax-b.YMin)/1000, "D")
		}
		pdf.SetXY(x, y+imgH+0.8)
		pdf.SetFont("Helvetica", "B", 8.5)
		cap := fmt.Sprintf("#%d", i+1)
		if answers[i+1] {
			cap += "  menjawab komplain / answers the complaint"
		}
		pdf.CellFormat(colW, 4, tr(cap), "", 2, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.MultiCell(colW, 3.6, tr(photoNote(in.Result.Photos[i])), "", "L", false)
		if i%2 == 1 || i == len(in.Photos)-1 {
			pdf.SetY(y + imgH + 14)
		} else {
			pdf.SetY(y)
		}
	}

	// Integrity.
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 9.5)
	pdf.CellFormat(inner, 5, tr("Sidik foto / Photo fingerprints (SHA-256 of the files checked)"), "", 1, "L", false, 0, "")
	pdf.SetFont("Courier", "", 7.5)
	for i, h := range hashes {
		pdf.CellFormat(inner, 4, fmt.Sprintf("#%d  %s", i+1, h), "", 1, "L", false, 0, "")
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("packet: %w", err)
	}
	return out.Bytes(), nil
}

// photoNote is the fixed per-photo line: what was seen, never model prose.
func photoNote(p evidence.PhotoObs) string {
	var parts []string
	id := map[string]string{"blur": "buram/blurry", "glare": "silau/glare", "too_dark": "gelap/too dark", "too_far": "terlalu jauh/too far", "cropped": "terpotong/cropped", "obstructed": "tertutup/obstructed"}
	for _, q := range p.ImageQualityIssues {
		parts = append(parts, id[q])
	}
	switch {
	case p.LabelReadable == evidence.Yes && p.ResiText != "":
		parts = append(parts, "resi "+evidence.NormaliseResi(p.ResiText))
	case p.LabelInPhoto == evidence.Yes:
		parts = append(parts, "label tidak terbaca/unreadable")
	}
	if p.SealedBoxInPhoto == evidence.Yes {
		parts = append(parts, "tersegel/sealed")
	}
	if p.ItemInPhoto == evidence.Yes {
		parts = append(parts, "barang terlihat/item shown")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " · ")
}

// toJPEG re-encodes a photo at a 1280 px long side for the PDF.
func toJPEG(raw []byte) ([]byte, int, int, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if long := max(w, h); long > 1280 {
		nw, nh := w*1280/long, h*1280/long
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img, w, h = dst, nw, nh
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
