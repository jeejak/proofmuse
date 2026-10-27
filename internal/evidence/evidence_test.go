package evidence

import (
	"errors"
	"slices"
	"testing"
)

const good = `{"photos":[
 {"photo":2,"label_in_photo":"yes","label_readable":"yes","resi_text":"spxid 123-456","sealed_box_in_photo":"yes","item_in_photo":"no","boxes_in_photo":1,"image_quality_issues":[],"label_box":{"x_min":100,"y_min":200,"x_max":1004,"y_max":600}},
 {"photo":1,"label_in_photo":"no","label_readable":"uncertain","resi_text":"SPXID999","sealed_box_in_photo":"no","item_in_photo":"yes","boxes_in_photo":1,"image_quality_issues":["blur"],"label_box":{"x_min":10,"y_min":10,"x_max":50,"y_max":50}}]}`

func TestParseObservation(t *testing.T) {
	o, err := ParseObservation([]byte(good), 2)
	if err != nil {
		t.Fatal(err)
	}
	p1, p2 := o.Photos[0], o.Photos[1]
	if p1.Photo != 1 || p2.Photo != 2 {
		t.Fatalf("not sorted by photo: %+v", o.Photos)
	}
	if p1.ResiText != "" {
		t.Errorf("read kept on a label that is not readable: %q", p1.ResiText)
	}
	if !p1.LabelBox.IsZero() {
		t.Errorf("box kept on a photo with no label: %+v", p1.LabelBox)
	}
	if p2.LabelBox.XMax != 1000 {
		t.Errorf("box not clamped: %+v", p2.LabelBox)
	}
}

func TestParseObservationRejects(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown field":   `{"photos":[],"verdict":"kuat"}`,
		"wrong count":     `{"photos":[]}`,
		"trailing":        good + `{}`,
		"bad enum":        `{"photos":[{"photo":1,"label_in_photo":"maybe","label_readable":"no","resi_text":"","sealed_box_in_photo":"no","item_in_photo":"no","boxes_in_photo":0,"image_quality_issues":[],"label_box":{"x_min":0,"y_min":0,"x_max":0,"y_max":0}}]}`,
		"bad issue":       `{"photos":[{"photo":1,"label_in_photo":"no","label_readable":"no","resi_text":"","sealed_box_in_photo":"no","item_in_photo":"no","boxes_in_photo":0,"image_quality_issues":["ugly"],"label_box":{"x_min":0,"y_min":0,"x_max":0,"y_max":0}}]}`,
		"duplicate photo": `{"photos":[{"photo":1,"label_in_photo":"no","label_readable":"no","resi_text":"","sealed_box_in_photo":"no","item_in_photo":"no","boxes_in_photo":0,"image_quality_issues":[],"label_box":{"x_min":0,"y_min":0,"x_max":0,"y_max":0}},{"photo":1,"label_in_photo":"no","label_readable":"no","resi_text":"","sealed_box_in_photo":"no","item_in_photo":"no","boxes_in_photo":0,"image_quality_issues":[],"label_box":{"x_min":0,"y_min":0,"x_max":0,"y_max":0}}]}`,
		"not json":        `Foto bagus`,
	} {
		n := 1
		if name == "trailing" || name == "duplicate photo" {
			n = 2
		}
		if _, err := ParseObservation([]byte(raw), n); !errors.Is(err, ErrSchema) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

func TestAggregateResiConflict(t *testing.T) {
	ps := []PhotoObs{
		{LabelReadable: Yes, ResiText: "JP111"},
		{LabelReadable: Yes, ResiText: "JP222"},
		{LabelReadable: Yes, ResiText: "jp-222"},
	}
	r := Aggregate(ps)
	if r.LabelResiRead != "JP222" || !r.ResiConflict {
		t.Fatalf("%+v", r)
	}
	res := Decide(r, Order{Resi: "JP222", Koli: 0})
	if res.Verdict != VerdictPerluDicek || !slices.Equal(res.Reasons, []string{ReasonResiConflict}) {
		t.Fatalf("%+v", res)
	}
}

func TestDecide(t *testing.T) {
	ok := Record{ImageUsable: UsableOK, LabelVisible: Yes, LabelResiRead: "JP1", PackageSealed: Yes, ItemVisible: Yes, KoliSeen: 1}
	cases := []struct {
		name    string
		mut     func(*Record)
		order   Order
		verdict string
		reasons []string
	}{
		{"clean", func(*Record) {}, Order{Resi: "jp-1", Koli: 1}, VerdictKuat, nil},
		{"mismatch", func(*Record) {}, Order{Resi: "JP2", Koli: 1}, VerdictLemah, []string{ReasonLabelMismatch}},
		{"unreadable", func(r *Record) { r.LabelResiRead = "" }, Order{Resi: "JP1", Koli: 1}, VerdictPerluDicek, []string{ReasonLabelUnreadable}},
		{"unsure label", func(r *Record) { r.LabelResiRead, r.LabelVisible = "", Unsure }, Order{Resi: "JP1", Koli: 1}, VerdictKuat, nil},
		{"dark and unsealed", func(r *Record) { r.ImageUsable, r.PackageSealed = UsableDark, No }, Order{Resi: "JP1", Koli: 1}, VerdictLemah, []string{ReasonImageDark, ReasonNotSealed}},
		{"koli", func(r *Record) { r.KoliSeen = 2 }, Order{Resi: "JP1", Koli: 1}, VerdictPerluDicek, []string{ReasonKoliMismatch}},
		{"item mismatch", func(*Record) {}, Order{Resi: "JP1", Koli: 1, ItemMismatch: true}, VerdictPerluDicek, []string{ReasonItemMismatch}},
	}
	for _, c := range cases {
		r := ok
		c.mut(&r)
		got := Decide(r, c.order)
		if got.Verdict != c.verdict || !slices.Equal(got.Reasons, c.reasons) {
			t.Errorf("%s: got %+v", c.name, got)
		}
	}
}

func TestEveryReasonHasAMessage(t *testing.T) {
	for _, r := range []string{ReasonImageDark, ReasonImageBlurry, ReasonImageObstructed, ReasonLabelNotVisible,
		ReasonLabelUnreadable, ReasonLabelMismatch, ReasonItemNotVisible, ReasonNotSealed, ReasonKoliMismatch,
		ReasonItemMismatch, ReasonResiConflict} {
		m, ok := Messages[r]
		if !ok || m.Problem == "" || m.Retake == "" || m.ProblemEN == "" || m.RetakeEN == "" {
			t.Errorf("%s: incomplete message %+v", r, m)
		}
	}
}

func TestParseClaim(t *testing.T) {
	if c, err := ParseClaim([]byte(`{"claim_type":"paket_kosong"}`)); err != nil || c != ClaimPaketKosong {
		t.Fatalf("%q %v", c, err)
	}
	for _, bad := range []string{`{"claim_type":"refund_now"}`, `{"claim_type":"lainnya","reply":"x"}`, `kosong`, `{}`} {
		if _, err := ParseClaim([]byte(bad)); !errors.Is(err, ErrSchema) {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestEveryClaimAndGapHasText(t *testing.T) {
	for _, c := range ClaimTypes {
		if ClaimText[c].ID == "" || ClaimText[c].EN == "" {
			t.Errorf("claim %s has no text", c)
		}
	}
	for _, typ := range ClaimTypes {
		for _, g := range TriageClaim(typ, nil).Missing {
			if GapText[g].ID == "" || GapText[g].EN == "" {
				t.Errorf("gap %s has no text", g)
			}
		}
	}
}
