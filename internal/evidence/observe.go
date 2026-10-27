// Package evidence turns packing photos into an advisory verdict.
//
// The model observes; this package decides. Muse Spark fills a fixed set
// of per-photo fields (PhotoObs) and nothing else. Aggregate folds them
// into one record with the same rules the eval set uses to derive record
// labels from human photo labels (evalset/tools/derive_records.py), and
// Decide turns the record into a verdict and reason codes. The model is
// never told which resi or koli to expect, so a misread label can only
// ever produce "label_mismatch", never a wrong resi in anything a seller
// sees.
package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Tri-state answers. "uncertain" is a real answer and never produces a
// warning on its own.
const (
	Yes       = "yes"
	No        = "no"
	Uncertain = "uncertain"
)

// Quality issues a photo can carry, as in evalset/photos.csv.
var QualityIssues = []string{"blur", "glare", "too_dark", "too_far", "cropped", "obstructed"}

// Box is a region on the 0 to 1000 grid of the image as sent, origin top
// left. The zero box means "none".
type Box struct {
	XMin int `json:"x_min"`
	YMin int `json:"y_min"`
	XMax int `json:"x_max"`
	YMax int `json:"y_max"`
}

// IsZero reports the "no box" sentinel.
func (b Box) IsZero() bool { return b == Box{} }

// PhotoObs is what the model reports for one photo. Field names match
// the human labels in evalset/photos.csv so they compare one to one.
type PhotoObs struct {
	Photo              int      `json:"photo"`
	LabelInPhoto       string   `json:"label_in_photo"`
	LabelReadable      string   `json:"label_readable"`
	ResiText           string   `json:"resi_text"`
	SealedBoxInPhoto   string   `json:"sealed_box_in_photo"`
	ItemInPhoto        string   `json:"item_in_photo"`
	BoxesInPhoto       int      `json:"boxes_in_photo"`
	ImageQualityIssues []string `json:"image_quality_issues"`
	LabelBox           Box      `json:"label_box"`
}

// Observation is the whole model answer for one record's photos.
type Observation struct {
	Photos []PhotoObs `json:"photos"`
}

func tri(desc string) map[string]any {
	return map[string]any{"type": "string", "enum": []string{Yes, No, Uncertain}, "description": desc}
}

// Schema is the strict JSON schema sent as response_format. Strict mode
// on this API requires every property listed in required and
// additionalProperties false on every object.
func Schema() map[string]any {
	box := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"x_min", "y_min", "x_max", "y_max"},
		"description":          "The shipping label on a 0 to 1000 grid, origin top left. All zeros when no label is visible.",
		"properties": map[string]any{
			"x_min": map[string]any{"type": "integer"},
			"y_min": map[string]any{"type": "integer"},
			"x_max": map[string]any{"type": "integer"},
			"y_max": map[string]any{"type": "integer"},
		},
	}
	photo := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{"photo", "label_in_photo", "label_readable", "resi_text", "sealed_box_in_photo",
			"item_in_photo", "boxes_in_photo", "image_quality_issues", "label_box"},
		"properties": map[string]any{
			"photo":               map[string]any{"type": "integer", "description": "1-based position of the photo in this message."},
			"label_in_photo":      tri("A courier shipping label (waybill) is visible."),
			"label_readable":      tri("Every character of the tracking number on the label can be read."),
			"resi_text":           map[string]any{"type": "string", "description": "The tracking number exactly as printed, only if label_readable is yes. Otherwise empty."},
			"sealed_box_in_photo": tri("A shipping carton is visibly closed and taped shut."),
			"item_in_photo":       tri("The product being shipped is visible, outside or inside an open carton."),
			"boxes_in_photo":      map[string]any{"type": "integer", "description": "Number of shipping cartons visible. Retail product packaging does not count."},
			"image_quality_issues": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string", "enum": QualityIssues},
			},
			"label_box": box,
		},
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"photos"},
		"properties": map[string]any{
			"photos": map[string]any{"type": "array", "items": photo},
		},
	}
}

// Prompt is the instruction placed next to the images. It is versioned
// so eval results and stored checks name the prompt that produced them.
const PromptVersion = "2026-10-27.1"

// Prompt returns the instruction for n photos.
func Prompt(n int) string {
	return fmt.Sprintf(`You are an object grounding expert inspecting %d packing photos from an Indonesian online seller, in shooting order. For each photo, report only what is visible in that photo. Return one entry per photo, photo 1 to %d.

Rules:
- Answer "uncertain" when you cannot tell. Do not guess.
- resi_text: copy the tracking number character by character only when every character is legible; otherwise leave it empty and set label_readable to "no" or "uncertain".
- boxes_in_photo counts shipping cartons only, not the product's own retail box.
- image_quality_issues lists problems that hide something: blur, glare, too_dark, too_far (the parcel is small in the frame), cropped (the parcel or label is cut off by the frame edge), obstructed (a hand or object covers the item, label or seal). Empty when the photo is clear.
- label_box: the shipping label on a 0 to 1000 grid of the image, origin top left. All zeros when no label is visible. Do not miss it.`, n, n)
}

// ErrSchema marks an answer that does not fit the contract. Such an
// answer is discarded and counted; it is never repaired.
var ErrSchema = errors.New("evidence: model answer does not match schema")

// ParseObservation decodes strictly (unknown fields, trailing data and
// out-of-range values are ErrSchema) and checks there is exactly one
// entry per photo sent, numbered 1 to n.
func ParseObservation(raw []byte, n int) (Observation, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var o Observation
	if err := dec.Decode(&o); err != nil {
		return Observation{}, fmt.Errorf("%w: %v", ErrSchema, err)
	}
	if dec.More() {
		return Observation{}, fmt.Errorf("%w: trailing data", ErrSchema)
	}
	if len(o.Photos) != n {
		return Observation{}, fmt.Errorf("%w: %d photo entries for %d photos", ErrSchema, len(o.Photos), n)
	}
	seen := make([]bool, n+1)
	for i := range o.Photos {
		p := &o.Photos[i]
		if p.Photo < 1 || p.Photo > n || seen[p.Photo] {
			return Observation{}, fmt.Errorf("%w: photo number %d", ErrSchema, p.Photo)
		}
		seen[p.Photo] = true
		for _, v := range []string{p.LabelInPhoto, p.LabelReadable, p.SealedBoxInPhoto, p.ItemInPhoto} {
			if v != Yes && v != No && v != Uncertain {
				return Observation{}, fmt.Errorf("%w: photo %d value %q", ErrSchema, p.Photo, v)
			}
		}
		if p.BoxesInPhoto < 0 || p.BoxesInPhoto > 20 {
			return Observation{}, fmt.Errorf("%w: photo %d boxes %d", ErrSchema, p.Photo, p.BoxesInPhoto)
		}
		for _, q := range p.ImageQualityIssues {
			if !slices.Contains(QualityIssues, q) {
				return Observation{}, fmt.Errorf("%w: photo %d issue %q", ErrSchema, p.Photo, q)
			}
		}
		p.LabelBox = clampBox(p.LabelBox)
		// Consistency the schema cannot express. A read without a
		// readable label is not trusted: it is dropped, not repaired
		// into a "yes".
		if p.LabelReadable != Yes {
			p.ResiText = ""
		}
		if p.LabelInPhoto == No {
			p.LabelBox = Box{}
		}
	}
	slices.SortFunc(o.Photos, func(a, b PhotoObs) int { return a.Photo - b.Photo })
	return o, nil
}

// clampBox keeps coordinates on the grid (the model may emit 1001) and
// turns an inverted or empty box into the zero box.
func clampBox(b Box) Box {
	c := func(v int) int { return min(max(v, 0), 1000) }
	b = Box{c(b.XMin), c(b.YMin), c(b.XMax), c(b.YMax)}
	if b.XMax <= b.XMin || b.YMax <= b.YMin {
		return Box{}
	}
	return b
}
