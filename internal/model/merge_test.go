package model

import (
	"errors"
	"testing"
)

func countFor(p *Project, ev string) (pairs, ratings int) {
	for _, j := range p.Pairwise {
		if j.EvaluatorID == ev {
			pairs++
		}
	}
	for _, r := range p.Ratings {
		if r.EvaluatorID == ev {
			ratings++
		}
	}
	return
}

func TestPacketHoldsOnlyThatPerson(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 2})
	pk, err := Packet(p, "e2")
	if err != nil {
		t.Fatal(err)
	}
	if pk.ID != p.ID || len(pk.Evaluators) != 1 || pk.Evaluators[0].ID != "e2" {
		t.Errorf("packet evaluators = %+v", pk.Evaluators)
	}
	if pairs, ratings := countFor(pk, "e1"); pairs+ratings != 0 {
		t.Error("packet carries someone else's judgments")
	}
	if pairs, _ := countFor(pk, "e2"); pairs != 1 {
		t.Errorf("packet has %d of Bob's judgments, want 1", pairs)
	}
	if len(pk.Values) != len(p.Values) || len(pk.Criteria) != len(p.Criteria) {
		t.Error("packet lost the shared structure or measured values")
	}
	if pk.Pairwise[0].Value = 9; p.Pairwise[len(p.Pairwise)-1].Value == 9 {
		t.Error("packet shares memory with the project")
	}
	if _, err := Packet(p, "nobody"); !errors.Is(err, ErrUnknownEvaluator) {
		t.Errorf("unknown evaluator: err = %v", err)
	}
}

// returned simulates Bob's packet coming back with his judgments filled in.
func returned(t *testing.T, owner *Project) *Project {
	t.Helper()
	pk, err := Packet(owner, "e2")
	if err != nil {
		t.Fatal(err)
	}
	pk.Pairwise = append(pk.Pairwise,
		PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c4", B: "c1", Value: 3},
		PairwiseJudgment{EvaluatorID: "e2", Context: AltsContext("c2"), A: "a1", B: "a2", Value: 5},
		PairwiseJudgment{EvaluatorID: "e2", Context: AltsContext("c2"), A: "a1", B: "zz", Value: 5}, // an option the owner deleted
	)
	pk.Ratings = append(pk.Ratings, Rating{EvaluatorID: "e2", CriterionID: "c3", AlternativeID: "a1", Value: 8})
	pk.Values[0].Value = 1 // the colleague edited a measured value: ignored
	pk.Name = "Renamed by Bob"
	return pk
}

func TestMergeAsksBeforeReplacingSomeone(t *testing.T) {
	owner := fixture()
	owner.Pairwise = append(owner.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 2})
	var ce *EvaluatorConflictError
	if _, err := Merge(owner, returned(t, owner), MergeAsk); !errors.As(err, &ce) || len(ce.Names) != 1 || ce.Names[0] != "Bob" {
		t.Fatalf("err = %v, want a conflict naming Bob", err)
	}
	if pairs, _ := countFor(owner, "e2"); pairs != 1 {
		t.Error("a refused merge still changed the project")
	}

	rep, err := Merge(owner, returned(t, owner), MergeReplace)
	if err != nil {
		t.Fatal(err)
	}
	if pairs, ratings := countFor(owner, "e2"); pairs != 2 || ratings != 1 {
		t.Errorf("after replace Bob has %d judgments and %d ratings, want 2 and 1", pairs, ratings)
	}
	if len(rep.Replaced) != 1 || rep.Replaced[0] != "Bob" || rep.Dropped != 1 || rep.Judgments != 3 {
		t.Errorf("report = %+v", rep)
	}
	if owner.Name != "Fixture" || owner.Values[0].Value != 100 {
		t.Error("the packet's structural edits leaked into the owner's project")
	}
	if pairs, _ := countFor(owner, "e1"); pairs != 5 {
		t.Error("someone else's judgments changed")
	}
}

func TestMergeSkipKeepsTheCurrentJudgments(t *testing.T) {
	owner := fixture()
	owner.Pairwise = append(owner.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 2})
	rep, err := Merge(owner, returned(t, owner), MergeSkip)
	if err != nil {
		t.Fatal(err)
	}
	if pairs, ratings := countFor(owner, "e2"); pairs != 1 || ratings != 0 || len(rep.Skipped) != 1 {
		t.Errorf("skip changed Bob's judgments (%d, %d), report %+v", pairs, ratings, rep)
	}
}

func TestMergeAddsNewPeople(t *testing.T) {
	owner := fixture()
	pk := returned(t, owner)
	pk.Evaluators[0] = Evaluator{ID: "e3", Name: "Cleo", Weight: 2}
	for i := range pk.Pairwise {
		pk.Pairwise[i].EvaluatorID = "e3"
	}
	for i := range pk.Ratings {
		pk.Ratings[i].EvaluatorID = "e3"
	}
	rep, err := Merge(owner, pk, MergeAsk)
	if err != nil {
		t.Fatal(err)
	}
	if len(owner.Evaluators) != 3 || owner.Evaluators[2].Name != "Cleo" || owner.Evaluators[2].Weight != 2 {
		t.Errorf("evaluators = %+v", owner.Evaluators)
	}
	if len(rep.Added) != 1 || rep.Added[0] != "Cleo" {
		t.Errorf("report = %+v", rep)
	}
}

func TestMergeRefusesAnotherProject(t *testing.T) {
	owner := fixture()
	other := New("Other")
	if _, err := Merge(owner, other, MergeReplace); !errors.Is(err, ErrDifferentProject) {
		t.Errorf("err = %v, want ErrDifferentProject", err)
	}
}

func TestAFailedMergeChangesNothing(t *testing.T) {
	owner := fixture()
	before, _ := clone(owner)
	pk := returned(t, owner)
	pk.Ratings[len(pk.Ratings)-1].Value = 99 // out of range: the merge must fail
	var ve *ValidationError
	if _, err := Merge(owner, pk, MergeReplace); !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a validation error", err)
	}
	if len(owner.Evaluators) != len(before.Evaluators) || len(owner.Pairwise) != len(before.Pairwise) || len(owner.Ratings) != len(before.Ratings) {
		t.Error("a failed merge changed the project")
	}
}

func TestMergeDoesNotAskAboutSomeoneWithNoJudgmentsYet(t *testing.T) {
	owner := fixture() // Bob (e2) exists but has judged nothing: his copy is coming back for the first time
	rep, err := Merge(owner, returned(t, owner), MergeAsk)
	if err != nil {
		t.Fatalf("err = %v, want a silent merge", err)
	}
	if pairs, ratings := countFor(owner, "e2"); pairs != 2 || ratings != 1 || len(rep.Added) != 1 || rep.Added[0] != "Bob" {
		t.Errorf("Bob has %d judgments and %d ratings, report %+v", pairs, ratings, rep)
	}
}

func TestMergeAcceptsACopyImportedUnderANewID(t *testing.T) {
	owner := fixture()
	pk := returned(t, owner)
	pk.ID, pk.Origin = NewProjectID(), owner.ID // the colleague already had a copy, so the import got a new id
	if _, err := Merge(owner, pk, MergeReplace); err != nil {
		t.Errorf("err = %v, want the copy to merge via its origin", err)
	}
}
