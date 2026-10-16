package model

import (
	"errors"
	"math"
	"testing"
)

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (±%g)", name, got, want, tol)
	}
}

func scores(r *Results) []float64 {
	out := make([]float64, len(r.Scores))
	for i, s := range r.Scores {
		out[i] = s.Score
	}
	return out
}

func approxScores(t *testing.T, r *Results, want []float64) {
	t.Helper()
	got := scores(r)
	if len(got) != len(want) {
		t.Fatalf("got %d scores, want %d", len(got), len(want))
	}
	for i := range want {
		approx(t, r.Scores[i].ID, got[i], want[i], 1e-6)
	}
}

func hasIssue(r *Results, code string) bool {
	for _, i := range r.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func criterion(r *Results, id string) CriterionResult {
	for _, c := range r.Criteria {
		if c.ID == id {
			return c
		}
	}
	return CriterionResult{}
}

func TestComputeSingleEvaluator(t *testing.T) {
	r, err := Compute(fixture(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	approxScores(t, r, []float64{0.450893, 0.272321, 0.276786})
	if r.Scores[0].Rank != 1 || r.Scores[2].Rank != 2 || r.Scores[1].Rank != 3 {
		t.Errorf("ranks = %+v", r.Scores)
	}
	approx(t, "completeness", r.Completeness, 1, 0)
	if !r.Consistent || len(r.Issues) != 0 {
		t.Errorf("Consistent = %v, issues = %+v", r.Consistent, r.Issues)
	}
	if len(r.Matrices) != 3 {
		t.Errorf("got %d matrix reports, want 3 (root, Technical, Performance)", len(r.Matrices))
	}
}

func TestComputeWeightsFollowTheHierarchy(t *testing.T) {
	r, _ := Compute(fixture(), "e1")
	wantOrder := []string{"c1", "c2", "c3", "c4"}
	for i, id := range wantOrder {
		if r.Criteria[i].ID != id {
			t.Fatalf("criteria order = %v..., want %v", r.Criteria[i].ID, wantOrder)
		}
	}
	approx(t, "c1 global", criterion(r, "c1").Global, 0.75, 1e-9)
	approx(t, "c2 global", criterion(r, "c2").Global, 0.375, 1e-9)
	approx(t, "c3 local", criterion(r, "c3").Local, 0.5, 1e-9)
	approx(t, "c4 global", criterion(r, "c4").Global, 0.25, 1e-9)
	sub := criterion(r, "c1").Subtree
	approx(t, "c1 subtree a3", sub[2], 0.321429, 1e-6)
}

func TestComputeStability(t *testing.T) {
	r, _ := Compute(fixture(), "e1")
	if s := criterion(r, "c1").Stability; s != nil {
		t.Errorf("c1 stability = %v, want none (Alpha leads at every weight)", *s)
	}
	s := criterion(r, "c2").Stability
	if s == nil {
		t.Fatal("c2 stability = nil, want 13/38")
	}
	approx(t, "c2 stability", *s, 13.0/38, 1e-9)
}

func TestComputeCombinesEvaluators(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c4", B: "c1", Value: 3})
	p.Ratings = append(p.Ratings, Rating{EvaluatorID: "e2", CriterionID: "c3", AlternativeID: "a1", Value: 10})
	r, err := Compute(p, AllEvaluators)
	if err != nil {
		t.Fatal(err)
	}
	// Ann says Technical is 3× Cost and Bob says the reverse: the geometric mean is 1.
	approx(t, "c1 local", criterion(r, "c1").Local, 0.5, 1e-9)
	approxScores(t, r, []float64{0.511905, 0.269841, 0.218254})
	if len(r.Disagreements) != 1 {
		t.Fatalf("got %d disagreements, want 1", len(r.Disagreements))
	}
	d := r.Disagreements[0]
	approx(t, "spread", d.Spread, 3, 1e-9)
	approx(t, "Bob's value", d.Values["e2"], 1.0/3, 1e-12)

	// Bob alone has only judged one pair and one rating.
	rb, _ := Compute(p, "e2")
	if rb.Completeness >= 0.5 || len(rb.Disagreements) != 0 {
		t.Errorf("Bob: completeness %v, %d disagreements", rb.Completeness, len(rb.Disagreements))
	}
}

func TestComputeIncompleteButConnected(t *testing.T) {
	p := fixture()
	p.Pairwise = p.Pairwise[:4] // drop Beta vs Gamma on Performance
	r, _ := Compute(p, "e1")
	approxScores(t, r, []float64{0.450893, 0.272321, 0.276786}) // e1 is consistent, so nothing changes
	approx(t, "completeness", r.Completeness, 10.0/11, 1e-12)
	m := r.Matrices[2]
	if m.Context != AltsContext("c2") || m.Complete || m.Missing != 1 {
		t.Errorf("performance matrix = %+v", m)
	}
}

func TestComputeDisconnectedGroupFallsBackToEqualWeights(t *testing.T) {
	p := fixture()
	p.Pairwise = p.Pairwise[1:] // drop Technical vs Cost
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueGroupDisconnected) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueGroupDisconnected)
	}
	approx(t, "c1 local", criterion(r, "c1").Local, 0.5, 1e-12)
	if r.Matrices[0].Connected || len(r.Matrices[0].Components) != 2 {
		t.Errorf("root matrix = %+v", r.Matrices[0])
	}
}

func TestComputeMissingRatingGetsTheAverage(t *testing.T) {
	p := fixture()
	p.Ratings = p.Ratings[:2] // Gamma unrated; Alpha and Beta both have 5
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueRatingsMissing) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueRatingsMissing)
	}
	sub := criterion(r, "c3").Subtree
	for a := range sub {
		approx(t, "security priority", sub[a], 1.0/3, 1e-12)
	}
}

func TestComputeInvalidValues(t *testing.T) {
	p := fixture()
	p.Values[0].Value = 0 // zero cost can't be inverted
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueValuesInvalid) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueValuesInvalid)
	}
	approx(t, "cost priority", criterion(r, "c4").Subtree[0], 1.0/3, 1e-12)

	p.Criteria[3].Direction = DirHigher
	p.Values[0].Value = -5
	r, _ = Compute(p, "e1")
	if !hasIssue(r, IssueValuesInvalid) {
		t.Error("negative value with higher-is-better: want values-invalid")
	}
}

func TestComputeSuggestsFixesForContradictions(t *testing.T) {
	p := fixture()
	p.Pairwise[3].Value = 0.25 // Alpha vs Gamma reversed: contradicts the other two
	r, _ := Compute(p, "e1")
	m := r.Matrices[2]
	if m.Consistent || r.Consistent {
		t.Errorf("want an inconsistent Performance matrix, got CR %v", m.CR)
	}
	// In a three-item cycle every pair deviates equally, so the reversed pair
	// must be among the suggestions rather than necessarily first.
	for _, s := range m.Suggestions {
		if s.A == "a1" && s.B == "a3" && s.Current == 0.25 {
			return
		}
	}
	t.Errorf("suggestions = %+v, want one for a1 vs a3", m.Suggestions)
}

func TestComputeEdgeCases(t *testing.T) {
	if _, err := Compute(fixture(), "nobody"); !errors.Is(err, ErrUnknownEvaluator) {
		t.Errorf("err = %v, want ErrUnknownEvaluator", err)
	}

	empty := New("Empty")
	r, err := Compute(empty, AllEvaluators)
	if err != nil || len(r.Scores) != 0 || r.Completeness != 1 {
		t.Errorf("empty project: %+v, %v", r, err)
	}

	noCriteria := New("No criteria")
	noCriteria.Alternatives = []Alternative{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	r, _ = Compute(noCriteria, AllEvaluators)
	if !hasIssue(r, IssueNoCriteria) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueNoCriteria)
	}

	one := fixture()
	one.Alternatives = one.Alternatives[:1]
	r, err = Compute(one, "e1")
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "single alternative", r.Scores[0].Score, 1, 1e-12)
}

func TestPrepareKeepsJudgmentsWhenALeafGainsChildren(t *testing.T) {
	p := fixture()
	// Nest a new criterion under Cost, then save: Cost is no longer a leaf.
	p.Criteria = append(p.Criteria, Criterion{ID: "c5", ParentID: "c4", Name: "Licence"})
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if len(p.Values) != 3 {
		t.Fatalf("nesting dropped Cost's values: %d left, want 3", len(p.Values))
	}
	// Un-nest it again: Cost's values are still there to use.
	p.Criteria = p.Criteria[:4]
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	r, _ := Compute(p, "e1")
	if hasIssue(r, IssueValuesMissing) {
		t.Errorf("Cost lost its values after un-nesting: %+v", r.Issues)
	}
}

func TestComputeTinyLowerIsBetterValueStaysFinite(t *testing.T) {
	p := fixture()
	p.Values[0].Value = 1e-310 // valid input, but 1/v overflows to +Inf
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueValuesInvalid) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueValuesInvalid)
	}
	for _, s := range r.Scores {
		if math.IsNaN(s.Score) || math.IsInf(s.Score, 0) {
			t.Fatalf("score %v is not finite", s.Score)
		}
	}
}

func TestComputeReportsTheCombinedMatrix(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c4", B: "c1", Value: 3})
	r, _ := Compute(p, AllEvaluators)
	m := r.Matrices[0].Matrix // root: Technical vs Cost; Ann 3, Bob 1/3 → geometric mean 1
	if len(m) != 2 || m[0][0] != 1 {
		t.Fatalf("matrix = %v", m)
	}
	approx(t, "combined", m[0][1], 1, 1e-12)
	perf := r.Matrices[2].Matrix // Performance: a1/a2 = 2, a1/a3 = 4, a2/a3 = 2
	approx(t, "a3 over a1", perf[2][0], 0.25, 1e-12)

	p.Pairwise = p.Pairwise[:4] // leave Beta vs Gamma unjudged
	r, _ = Compute(p, "e1")
	if got := r.Matrices[2].Matrix[1][2]; got != 0 {
		t.Errorf("unjudged cell = %v, want 0", got)
	}
}
