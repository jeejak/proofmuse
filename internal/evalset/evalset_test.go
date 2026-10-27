package evalset

import (
	"slices"
	"testing"

	"github.com/jeejak/proofmuse/internal/evidence"
)

// TestAggregateMatchesEvalset is the contract between the Python
// derivation that built the human record labels and the Go rules that
// judge the model. Feed every record's human photo labels through
// evidence.Aggregate and evidence.Decide; the result must equal the
// record row in labels.csv, field by field, verdict and reasons
// included. If the two drift, a model would be scored against labels
// built by different rules from the ones that judge it.
func TestAggregateMatchesEvalset(t *testing.T) {
	s, err := Load("../../evalset")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Labels) != 104 {
		t.Fatalf("labels: got %d records, want 104", len(s.Labels))
	}
	for _, l := range s.Labels {
		if !l.Derived {
			t.Errorf("%s: record fields are not derived from human photos", l.RecordID)
			continue
		}
		var obs []evidence.PhotoObs
		for _, p := range s.Photos[l.RecordID] {
			if !p.Reviewed {
				t.Errorf("%s/%s: photo not human-reviewed", l.RecordID, p.File)
			}
			o := p.Obs
			// The human labels say whether a label is readable, not what
			// it reads; the printed resi is in the record row.
			if o.LabelReadable == evidence.Yes {
				o.ResiText = l.Record.LabelResiRead
			}
			obs = append(obs, o)
		}
		got := evidence.Aggregate(obs)
		want := l.Record
		if got != want {
			t.Errorf("%s: aggregate\n got %+v\nwant %+v", l.RecordID, got, want)
		}
		res := evidence.Decide(got, evidence.Order{Resi: l.Resi, Koli: l.Koli, ItemMismatch: l.ItemMismatch})
		if res.Verdict != l.Verdict || !slices.Equal(res.Reasons, l.Reasons) {
			t.Errorf("%s: decide got %s %v, want %s %v", l.RecordID, res.Verdict, res.Reasons, l.Verdict, l.Reasons)
		}
	}
}

func TestOrdersMatchLabels(t *testing.T) {
	s, err := Load("../../evalset")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range s.Labels {
		o := s.Orders[l.RecordID]
		if evidence.NormaliseResi(o.Resi) != evidence.NormaliseResi(l.Resi) || o.Koli != l.Koli {
			t.Errorf("%s: order %s/%d, label %s/%d", l.RecordID, o.Resi, o.Koli, l.Resi, l.Koli)
		}
	}
}
