package evidence

import (
	"strings"
)

// Record-level values, as in evalset/labels.csv.
const (
	UsableOK         = "ok"
	UsableDark       = "dark"
	UsableBlurry     = "blurry"
	UsableObstructed = "obstructed"
	Unsure           = "unsure"
)

// Record is one parcel's evidence folded from its photos.
type Record struct {
	ImageUsable   string `json:"image_usable"`
	LabelVisible  string `json:"label_visible"`
	LabelResiRead string `json:"label_resi_read"`
	PackageSealed string `json:"package_sealed"`
	ItemVisible   string `json:"item_visible"`
	KoliSeen      int    `json:"koli_seen"`
	// ResiConflict is set when readable photos disagree on the resi.
	ResiConflict bool `json:"resi_conflict"`
}

// Aggregate folds per-photo observations into a record. It mirrors
// evalset/tools/derive_records.py rule for rule, and TestAggregateMatchesEvalset
// pins the two together on all 104 records:
//
//   - label_visible, package_sealed, item_visible: yes if any photo is
//     yes, no if every photo is no, otherwise unsure.
//   - label_resi_read: the resi read on a readable label; when readable
//     photos disagree, the most frequent read wins (ties: first in shooting
//     order) and ResiConflict is set.
//   - koli_seen: the most cartons visible in any one photo.
//   - image_usable: dark, blurry or obstructed only when every photo has
//     that issue.
func Aggregate(photos []PhotoObs) Record {
	r := Record{
		LabelVisible:  anyAll(photos, func(p PhotoObs) string { return p.LabelInPhoto }),
		PackageSealed: anyAll(photos, func(p PhotoObs) string { return p.SealedBoxInPhoto }),
		ItemVisible:   anyAll(photos, func(p PhotoObs) string { return p.ItemInPhoto }),
		ImageUsable:   usable(photos),
	}
	counts := map[string]int{}
	var order []string
	for _, p := range photos {
		r.KoliSeen = max(r.KoliSeen, p.BoxesInPhoto)
		if p.LabelReadable != Yes {
			continue
		}
		n := NormaliseResi(p.ResiText)
		if n == "" {
			continue
		}
		if counts[n] == 0 {
			order = append(order, n)
		}
		counts[n]++
	}
	best := 0
	for _, n := range order {
		if counts[n] > best {
			best, r.LabelResiRead = counts[n], n
		}
	}
	r.ResiConflict = len(order) > 1
	return r
}

func anyAll(photos []PhotoObs, f func(PhotoObs) string) string {
	all := len(photos) > 0
	for _, p := range photos {
		v := f(p)
		if v == Yes {
			return Yes
		}
		if v != No {
			all = false
		}
	}
	if all {
		return No
	}
	return Unsure
}

func usable(photos []PhotoObs) string {
	for _, c := range []struct{ issue, value string }{
		{"too_dark", UsableDark}, {"blur", UsableBlurry}, {"obstructed", UsableObstructed},
	} {
		every := len(photos) > 0
		for _, p := range photos {
			if !contains(p.ImageQualityIssues, c.issue) {
				every = false
				break
			}
		}
		if every {
			return c.value
		}
	}
	return UsableOK
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// Verdicts.
const (
	VerdictKuat       = "kuat"
	VerdictPerluDicek = "perlu_dicek"
	VerdictLemah      = "lemah"
)

// Reason codes. ParcelProof's nine plus ProofMuse's item_mismatch and
// resi_conflict. Seller-facing text is fixed per code (messages.go).
const (
	ReasonImageDark       = "image_dark"
	ReasonImageBlurry     = "image_blurry"
	ReasonImageObstructed = "image_obstructed"
	ReasonLabelNotVisible = "label_not_visible"
	ReasonLabelUnreadable = "label_unreadable"
	ReasonLabelMismatch   = "label_mismatch"
	ReasonItemNotVisible  = "item_not_visible"
	ReasonNotSealed       = "not_sealed"
	ReasonKoliMismatch    = "koli_mismatch"
	ReasonItemMismatch    = "item_mismatch"
	ReasonResiConflict    = "resi_conflict"
)

// weak reasons mean the evidence would likely lose a dispute on its own.
var weak = map[string]bool{
	ReasonImageDark:       true,
	ReasonImageBlurry:     true,
	ReasonImageObstructed: true,
	ReasonLabelNotVisible: true,
	ReasonLabelMismatch:   true,
	ReasonItemNotVisible:  true,
}

// Order is what Decide compares against: facts the model never sees.
type Order struct {
	Resi string
	Koli int
	// ItemMismatch is set by a separate item check (or, in the eval, by
	// construction for the item_mismatch group).
	ItemMismatch bool
}

// Result is a verdict with the reasons that produced it, in a stable
// order.
type Result struct {
	Verdict string   `json:"verdict"`
	Reasons []string `json:"reasons"`
}

// Decide is ParcelProof's rule (evidencecheck/verdict.go, ported) plus
// ProofMuse's two new reasons. Pure and deterministic.
//
//   - "unsure" never produces a reason.
//   - The resi comparison happens here on normalised strings: a read that
//     does not match is label_mismatch; a visible label with no read is
//     label_unreadable.
//   - Any weak reason makes the verdict lemah; any other reason
//     perlu_dicek; none is kuat.
func Decide(r Record, o Order) Result {
	var reasons []string
	switch r.ImageUsable {
	case UsableDark:
		reasons = append(reasons, ReasonImageDark)
	case UsableBlurry:
		reasons = append(reasons, ReasonImageBlurry)
	case UsableObstructed:
		reasons = append(reasons, ReasonImageObstructed)
	}
	read := NormaliseResi(r.LabelResiRead)
	switch {
	case read != "":
		if read != NormaliseResi(o.Resi) {
			reasons = append(reasons, ReasonLabelMismatch)
		}
	case r.LabelVisible == No:
		reasons = append(reasons, ReasonLabelNotVisible)
	case r.LabelVisible == Yes:
		reasons = append(reasons, ReasonLabelUnreadable)
	}
	if r.ItemVisible == No {
		reasons = append(reasons, ReasonItemNotVisible)
	}
	if r.PackageSealed == No {
		reasons = append(reasons, ReasonNotSealed)
	}
	if o.Koli > 0 && r.KoliSeen != o.Koli {
		reasons = append(reasons, ReasonKoliMismatch)
	}
	if o.ItemMismatch {
		reasons = append(reasons, ReasonItemMismatch)
	}
	if r.ResiConflict {
		reasons = append(reasons, ReasonResiConflict)
	}
	verdict := VerdictKuat
	for _, x := range reasons {
		if weak[x] {
			verdict = VerdictLemah
			break
		}
		verdict = VerdictPerluDicek
	}
	return Result{Verdict: verdict, Reasons: reasons}
}

// NormaliseResi uppercases and keeps only letters and digits, so
// "spxid 0123-456" and "SPXID0123456" compare equal.
func NormaliseResi(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
