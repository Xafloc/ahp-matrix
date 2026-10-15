package model

import (
	"errors"
	"math"
	"testing"
)

func TestPrepareAcceptsFixture(t *testing.T) {
	p := fixture()
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if len(p.Pairwise) != 5 || len(p.Ratings) != 3 || len(p.Values) != 3 {
		t.Errorf("Prepare dropped valid judgments: %d pairwise, %d ratings, %d values", len(p.Pairwise), len(p.Ratings), len(p.Values))
	}
}

func TestPrepareFillsDefaults(t *testing.T) {
	p := &Project{
		Name:         "  Spaced  ",
		Evaluators:   []Evaluator{{Name: "Me"}},
		Alternatives: []Alternative{{Name: "A"}},
		Criteria:     []Criterion{{ID: "c1", Name: "Leaf"}, {ID: "c2", Name: "Measured", Method: MethodValue}},
	}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if p.Name != "Spaced" || p.Scale != ScaleSaaty9 || p.SchemaVersion != SchemaVersion {
		t.Errorf("defaults not filled: name %q, scale %q, schema %d", p.Name, p.Scale, p.SchemaVersion)
	}
	if p.Evaluators[0].ID == "" || p.Evaluators[0].Weight != 1 || p.Alternatives[0].ID == "" {
		t.Error("missing IDs or evaluator weight were not filled")
	}
	if p.Criteria[0].Method != MethodPairwise || p.Criteria[1].Direction != DirHigher {
		t.Errorf("leaf defaults: method %q, direction %q", p.Criteria[0].Method, p.Criteria[1].Direction)
	}
	if p.Pairwise == nil || p.Ratings == nil || p.Values == nil {
		t.Error("nil slices would encode as JSON null")
	}
}

func TestPrepareDropsOrphansAndDuplicates(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise,
		PairwiseJudgment{EvaluatorID: "ghost", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 2}, // unknown evaluator
		PairwiseJudgment{EvaluatorID: "e1", Context: AltsContext("gone"), A: "a1", B: "a2", Value: 2},    // criterion deleted
		PairwiseJudgment{EvaluatorID: "e1", Context: CriteriaContext(""), A: "c1", B: "c2", Value: 2},    // c2 is not top-level
		PairwiseJudgment{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a3", B: "a1", Value: 0.5},    // duplicate of a1/a3, reversed
		PairwiseJudgment{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a1", B: "a1", Value: 1},      // self-comparison
		PairwiseJudgment{EvaluatorID: "e1", Context: "nonsense", A: "a1", B: "a2", Value: 1},             // bad context
	)
	p.Ratings = append(p.Ratings, Rating{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "gone", Value: 3})
	p.Values = append(p.Values, Value{CriterionID: "c4", AlternativeID: "a1", Value: 150})
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if len(p.Pairwise) != 5 || len(p.Ratings) != 3 || len(p.Values) != 3 {
		t.Fatalf("got %d pairwise, %d ratings, %d values; want 5, 3, 3", len(p.Pairwise), len(p.Ratings), len(p.Values))
	}
	if j := p.Pairwise[3]; j.A != "a3" || j.B != "a1" || j.Value != 0.5 {
		t.Errorf("later duplicate should replace the earlier one in place, got %+v", j)
	}
	if p.Values[0].Value != 150 {
		t.Errorf("later value should win, got %v", p.Values[0].Value)
	}
}

func TestPrepareSnapsToScale(t *testing.T) {
	p := fixture()
	p.Pairwise[0].Value = 2.6
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if p.Pairwise[0].Value != 3 {
		t.Errorf("saaty9 snap: got %v, want 3", p.Pairwise[0].Value)
	}
	p.Scale = ScaleClassic5
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if p.Pairwise[0].Value != 5 {
		t.Errorf("classic5 snap: got %v, want 5", p.Pairwise[0].Value)
	}
}

func TestValidateReportsProblems(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(p *Project)
		field  string
	}{
		{"empty name", func(p *Project) { p.Name = " " }, "name"},
		{"bad scale", func(p *Project) { p.Scale = "huge" }, "scale"},
		{"no evaluators", func(p *Project) { p.Evaluators = nil; p.Pairwise = nil; p.Ratings = nil }, "evaluators"},
		{"negative evaluator weight", func(p *Project) { p.Evaluators[0].Weight = -1 }, "evaluators[0].weight"},
		{"duplicate alternative", func(p *Project) { p.Alternatives[1].ID = "a1" }, "alternatives[1].id"},
		{"unnamed alternative", func(p *Project) { p.Alternatives[2].Name = "" }, "alternatives[2].name"},
		{"reserved criterion id", func(p *Project) { p.Criteria[3].ID = "root" }, "criteria[3].id"},
		{"missing parent", func(p *Project) { p.Criteria[3].ParentID = "nope" }, "criteria[3].parentId"},
		{"cycle", func(p *Project) { p.Criteria[0].ParentID = "c2" }, "criteria[0].parentId"},
		{"bad method", func(p *Project) { p.Criteria[1].Method = "vibes" }, "criteria[1].method"},
		{"bad direction", func(p *Project) { p.Criteria[3].Direction = "sideways" }, "criteria[3].direction"},
		{"negative judgment", func(p *Project) { p.Pairwise[0].Value = -2 }, "pairwise[0].value"},
		{"NaN judgment", func(p *Project) { p.Pairwise[0].Value = math.NaN() }, "pairwise[0].value"},
		{"rating too high", func(p *Project) { p.Ratings[0].Value = 11 }, "ratings[0].value"},
		{"infinite value", func(p *Project) { p.Values[0].Value = math.Inf(1) }, "values[0].value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := fixture()
			c.mutate(p)
			var ve *ValidationError
			if err := p.Prepare(); !errors.As(err, &ve) {
				t.Fatalf("Prepare() = %v, want *ValidationError", err)
			}
			for _, pr := range ve.Problems {
				if pr.Field == c.field {
					return
				}
			}
			t.Errorf("no problem for field %q; got %+v", c.field, ve.Problems)
		})
	}
}

func TestValidateRejectsUnsafeProjectIDs(t *testing.T) {
	for _, bad := range []string{"x/abc", "a b", "a?b", "../../etc"} {
		p := fixture()
		p.ID = bad
		var ve *ValidationError
		if err := p.Prepare(); !errors.As(err, &ve) || ve.Problems[0].Field != "id" {
			t.Errorf("id %q: err = %v, want a problem on field id", bad, err)
		}
	}
	if !ValidID(NewProjectID()) {
		t.Error("NewProjectID is not a ValidID")
	}
	p := fixture()
	p.ID = ""
	if err := p.Prepare(); err != nil || !ValidID(p.ID) {
		t.Errorf("empty id: err = %v, id %q; want a generated id", err, p.ID)
	}
}
