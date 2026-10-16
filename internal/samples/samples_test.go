package samples

import (
	"errors"
	"testing"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

func TestSamplesAreCompleteAndConsistent(t *testing.T) {
	list := Samples()
	if len(list) != 3 {
		t.Fatalf("got %d samples, want 3", len(list))
	}
	for _, e := range list {
		p, err := Sample(e.ID)
		if err != nil {
			t.Fatal(err)
		}
		before := len(p.Pairwise) + len(p.Ratings) + len(p.Values)
		if err := p.Prepare(); err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		if after := len(p.Pairwise) + len(p.Ratings) + len(p.Values); after != before {
			t.Errorf("%s: Prepare dropped %d judgments", e.ID, before-after)
		}
		r, err := model.Compute(p, model.AllEvaluators)
		if err != nil {
			t.Fatal(err)
		}
		if r.Completeness != 1 || !r.Consistent || len(r.Issues) != 0 {
			t.Errorf("%s: completeness %v, consistent %v, issues %+v", e.ID, r.Completeness, r.Consistent, r.Issues)
		}
		if e.Alternatives != len(p.Alternatives) || e.Criteria != len(p.Criteria) || e.Name != p.Name {
			t.Errorf("%s: listing %+v doesn't match the file", e.ID, e)
		}
	}
}

func TestSampleCopiesAreNewProjects(t *testing.T) {
	a, _ := Sample("cloud-provider")
	b, _ := Sample("cloud-provider")
	if a.ID == b.ID || !model.ValidID(a.ID) || a.CreatedAt.IsZero() {
		t.Errorf("copies share an id or lack a creation time: %q %q", a.ID, b.ID)
	}
	if _, err := Sample("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown sample: err = %v", err)
	}
}

func TestTemplatesAreStructureOnly(t *testing.T) {
	list := Templates()
	if len(list) != 3 {
		t.Fatalf("got %d templates, want 3", len(list))
	}
	for _, e := range list {
		p, err := Template(e.ID, "My decision")
		if err != nil {
			t.Fatal(err)
		}
		if p.Name != "My decision" || len(p.Alternatives) != 0 || len(p.Pairwise) != 0 || len(p.Criteria) == 0 {
			t.Errorf("%s: %+v", e.ID, p)
		}
		if err := p.Prepare(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
	}
	vendor, _ := Template("vendor-selection", "")
	leaves := 0
	for _, c := range vendor.Criteria {
		if c.ParentID != "" {
			leaves++
		}
	}
	if leaves != 23 || vendor.Name != "Vendor selection" {
		t.Errorf("vendor template has %d criteria under groups and name %q; want the spreadsheet's 23", leaves, vendor.Name)
	}
}
