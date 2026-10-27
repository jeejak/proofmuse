package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // phones and the eval set may hand over PNG

	"golang.org/x/image/draw"

	"github.com/jeejak/proofmuse/internal/evidence"
)

// decode reads a JPEG or PNG.
func decode(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("pipeline: decode photo: %w", err)
	}
	return img, nil
}

// fit scales img so its long side is at most maxSide (never up) or, with
// minSide, at least minSide (up only). Zero disables either bound.
func fit(img image.Image, maxSide, minSide int) image.Image {
	b := img.Bounds()
	long := max(b.Dx(), b.Dy())
	target := long
	if maxSide > 0 && long > maxSide {
		target = maxSide
	}
	if minSide > 0 && long < minSide {
		target = minSide
	}
	if target == long {
		return img
	}
	w := b.Dx() * target / long
	h := b.Dy() * target / long
	dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

func encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("pipeline: encode photo: %w", err)
	}
	return buf.Bytes(), nil
}

// crop cuts a 0 to 1000 grid box out of img with a margin (share of the
// box size on each side), clipped to the frame.
func crop(img image.Image, box evidence.Box, margin float64) image.Image {
	b := img.Bounds()
	px := func(v, size int) int { return v * size / 1000 }
	x0, y0 := px(box.XMin, b.Dx()), px(box.YMin, b.Dy())
	x1, y1 := px(box.XMax, b.Dx()), px(box.YMax, b.Dy())
	mx, my := int(float64(x1-x0)*margin), int(float64(y1-y0)*margin)
	r := image.Rect(x0-mx, y0-my, x1+mx, y1+my).Add(b.Min).Intersect(b)
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Copy(dst, image.Point{}, img, r, draw.Src, nil)
	return dst
}

func strictJSON(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("trailing data")
	}
	return nil
}
