// Package evalset reads the synthetic eval set in evalset/: human photo
// labels, derived record labels, mock orders and the dev/test split.
package evalset

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jeejak/proofmuse/internal/evidence"
)

// Photo is one row of photos.csv.
type Photo struct {
	RecordID string
	File     string // "01.jpg"
	Obs      evidence.PhotoObs
	Reviewed bool
}

// Label is one row of labels.csv.
type Label struct {
	RecordID     string
	Resi         string // the order's resi
	Koli         int
	Courier      string
	Group        string
	Record       evidence.Record
	Verdict      string
	Reasons      []string
	Derived      bool // record_fields_source == derived_from_human_photos
	ListingItem  string
	RetakeOf     string
	ItemMismatch bool
}

// Order is one entry of orders.json.
type Order struct {
	OrderID     string `json:"order_id"`
	Marketplace string `json:"marketplace"`
	Courier     string `json:"courier"`
	Resi        string `json:"resi"`
	Koli        int    `json:"koli"`
	ListingItem string `json:"listing_item"`
	CreatedAt   string `json:"created_at"`
}

// Set is the loaded eval set.
type Set struct {
	Dir    string
	Labels []Label            // sorted by record id
	Photos map[string][]Photo // by record id, in shooting order
	Orders map[string]Order   // by record id
	Split  map[string]string  // record id to dev or test
}

// PhotoPath is the image file for a photo.
func (s *Set) PhotoPath(p Photo) string {
	return filepath.Join(s.Dir, "records", p.RecordID, p.File)
}

// Load reads dir (the evalset directory).
func Load(dir string) (*Set, error) {
	s := &Set{Dir: dir, Photos: map[string][]Photo{}, Orders: map[string]Order{}, Split: map[string]string{}}
	photos, err := readCSV(filepath.Join(dir, "photos.csv"))
	if err != nil {
		return nil, err
	}
	for i, r := range photos {
		p, err := parsePhoto(r)
		if err != nil {
			return nil, fmt.Errorf("photos.csv row %d: %w", i+2, err)
		}
		s.Photos[p.RecordID] = append(s.Photos[p.RecordID], p)
	}
	for id := range s.Photos {
		sort.Slice(s.Photos[id], func(a, b int) bool { return s.Photos[id][a].File < s.Photos[id][b].File })
		for i := range s.Photos[id] {
			s.Photos[id][i].Obs.Photo = i + 1
		}
	}
	labels, err := readCSV(filepath.Join(dir, "labels.csv"))
	if err != nil {
		return nil, err
	}
	for i, r := range labels {
		l, err := parseLabel(r)
		if err != nil {
			return nil, fmt.Errorf("labels.csv row %d: %w", i+2, err)
		}
		s.Labels = append(s.Labels, l)
	}
	sort.Slice(s.Labels, func(a, b int) bool { return s.Labels[a].RecordID < s.Labels[b].RecordID })
	split, err := readCSV(filepath.Join(dir, "split.csv"))
	if err != nil {
		return nil, err
	}
	for _, r := range split {
		s.Split[r["record_id"]] = r["split"]
	}
	b, err := os.ReadFile(filepath.Join(dir, "orders.json"))
	if err != nil {
		return nil, err
	}
	var orders []Order
	if err := json.Unmarshal(b, &orders); err != nil {
		return nil, fmt.Errorf("orders.json: %w", err)
	}
	for _, o := range orders {
		s.Orders[strings.TrimPrefix(o.OrderID, "SYN-")] = o
	}
	for _, l := range s.Labels {
		if len(s.Photos[l.RecordID]) == 0 {
			return nil, fmt.Errorf("record %s has no photos", l.RecordID)
		}
		if _, ok := s.Orders[l.RecordID]; !ok {
			return nil, fmt.Errorf("record %s has no order", l.RecordID)
		}
		if s.Split[l.RecordID] == "" {
			return nil, fmt.Errorf("record %s has no split", l.RecordID)
		}
	}
	return s, nil
}

func readCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cr := csv.NewReader(f)
	head, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: header: %w", path, err)
	}
	var out []map[string]string
	for {
		row, err := cr.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		m := make(map[string]string, len(head))
		for i, h := range head {
			m[h] = row[i]
		}
		out = append(out, m)
	}
}

func parsePhoto(r map[string]string) (Photo, error) {
	boxes, err := strconv.Atoi(r["boxes_in_photo"])
	if err != nil {
		return Photo{}, fmt.Errorf("boxes_in_photo %q", r["boxes_in_photo"])
	}
	var issues []string
	for _, q := range strings.Split(r["image_quality_issues"], "|") {
		if q != "" {
			issues = append(issues, q)
		}
	}
	var box evidence.Box
	if f := strings.Fields(r["label_box"]); len(f) == 4 {
		v := make([]int, 4)
		for i := range f {
			if v[i], err = strconv.Atoi(f[i]); err != nil {
				return Photo{}, fmt.Errorf("label_box %q", r["label_box"])
			}
		}
		box = evidence.Box{XMin: v[0], YMin: v[1], XMax: v[2], YMax: v[3]}
	}
	return Photo{
		RecordID: r["record_id"],
		File:     r["photo"],
		Reviewed: r["needs_review"] == "no",
		Obs: evidence.PhotoObs{
			LabelInPhoto:       r["label_in_photo"],
			LabelReadable:      r["label_readable"],
			SealedBoxInPhoto:   r["sealed_box_in_photo"],
			ItemInPhoto:        r["item_in_photo"],
			BoxesInPhoto:       boxes,
			ImageQualityIssues: issues,
			LabelBox:           box,
		},
	}, nil
}

func parseLabel(r map[string]string) (Label, error) {
	koli, err := strconv.Atoi(r["koli"])
	if err != nil {
		return Label{}, fmt.Errorf("koli %q", r["koli"])
	}
	seen := 0
	if r["koli_seen"] != "" {
		if seen, err = strconv.Atoi(r["koli_seen"]); err != nil {
			return Label{}, fmt.Errorf("koli_seen %q", r["koli_seen"])
		}
	}
	var reasons []string
	for _, x := range strings.Split(r["verdict_reasons"], "|") {
		if x != "" {
			reasons = append(reasons, x)
		}
	}
	return Label{
		RecordID: r["record_id"],
		Resi:     r["resi"],
		Koli:     koli,
		Courier:  r["courier"],
		Group:    r["group"],
		Record: evidence.Record{
			ImageUsable:   r["image_usable"],
			LabelVisible:  r["label_visible"],
			LabelResiRead: r["label_resi_read"],
			PackageSealed: r["package_sealed"],
			ItemVisible:   r["item_visible"],
			KoliSeen:      seen,
		},
		Verdict:      r["human_verdict"],
		Reasons:      reasons,
		Derived:      r["record_fields_source"] == "derived_from_human_photos",
		ListingItem:  r["listing_item"],
		RetakeOf:     r["retake_of"],
		ItemMismatch: r["group"] == "item_mismatch",
	}, nil
}

// Claim is one row of claims.csv.
type Claim struct {
	ClaimID   string
	RecordID  string
	Text      string
	ClaimType string
	Answers   []string // "R05/02.jpg"
	Missing   []string
}

// LoadClaims reads claims.csv.
func LoadClaims(dir string) ([]Claim, error) {
	rows, err := readCSV(filepath.Join(dir, "claims.csv"))
	if err != nil {
		return nil, err
	}
	split := func(s string) []string {
		var out []string
		for _, x := range strings.Split(s, "|") {
			if x != "" {
				out = append(out, x)
			}
		}
		return out
	}
	var out []Claim
	for _, r := range rows {
		out = append(out, Claim{ClaimID: r["claim_id"], RecordID: strings.TrimPrefix(r["order_id"], "SYN-"), Text: r["text"],
			ClaimType: r["claim_type"], Answers: split(r["evidence_that_answers"]), Missing: split(r["evidence_missing"])})
	}
	return out, nil
}
