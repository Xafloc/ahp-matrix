package model

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestVersionOneFilesMigrate(t *testing.T) {
	p := fixture()
	data, _ := json.Marshal(p)
	v1 := strings.Replace(string(data), `"schemaVersion":2`, `"schemaVersion":1`, 1)
	got, err := Decode([]byte(v1))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.Name != p.Name {
		t.Errorf("migrated project: version %d, name %q", got.SchemaVersion, got.Name)
	}
	var fv *FutureVersionError
	if _, err := Decode([]byte(`{"schemaVersion":3}`)); !errors.As(err, &fv) {
		t.Errorf("version 3: err = %v, want FutureVersionError", err)
	}
}

func TestFingerprintFollowsStructureNotWording(t *testing.T) {
	base := Fingerprint(fixture(), "e1")
	if len(base) != 12 {
		t.Fatalf("fingerprint %q, want 12 hex characters", base)
	}
	same := func(name string, edit func(p *Project)) {
		p := fixture()
		edit(p)
		if got := Fingerprint(p, "e1"); got != base {
			t.Errorf("%s changed the fingerprint", name)
		}
	}
	differs := func(name string, edit func(p *Project)) {
		p := fixture()
		edit(p)
		if got := Fingerprint(p, "e1"); got == base {
			t.Errorf("%s didn't change the fingerprint", name)
		}
	}
	same("renaming a criterion", func(p *Project) { p.Criteria[0].Name = "Tech" })
	same("editing notes", func(p *Project) { p.Criteria[1].Notes = "new" })
	same("renaming an option", func(p *Project) { p.Alternatives[0].Name = "A" })
	same("another person's tasks", func(p *Project) { p.Evaluators[1].Tasks = &Tasks{Weigh: true} })
	differs("adding a criterion", func(p *Project) { p.Criteria = append(p.Criteria, Criterion{ID: "c5", Name: "New"}) })
	differs("moving a criterion", func(p *Project) { p.Criteria[3].ParentID = "c1" })
	differs("changing a method", func(p *Project) { p.Criteria[1].Method = MethodRating })
	differs("removing an option", func(p *Project) { p.Alternatives = p.Alternatives[:2] })
	differs("changing the scale", func(p *Project) { p.Scale = ScaleClassic5 })
	differs("changing their tasks", func(p *Project) { p.Evaluators[0].Tasks = &Tasks{Weigh: true} })
}

func TestPrepareTidiesTasksAndAggregation(t *testing.T) {
	p := fixture()
	p.Evaluators[1].Tasks = &Tasks{Score: []string{"c2", "gone", "c1"}} // c1 isn't a leaf
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if got := p.Evaluators[1].Tasks.Score; len(got) != 1 || got[0] != "c2" {
		t.Errorf("task leaves = %v, want [c2]", got)
	}
	if p.Aggregation != AggregateJudgments {
		t.Errorf("aggregation = %q, want the default", p.Aggregation)
	}
	p.Aggregation = "votes"
	var ve *ValidationError
	if err := p.Prepare(); !errors.As(err, &ve) {
		t.Errorf("unknown aggregation: err = %v, want a validation error", err)
	}
}

func TestPacketsSayWhoTheyAreFor(t *testing.T) {
	pk, err := Packet(fixture(), "e2")
	if err != nil {
		t.Fatal(err)
	}
	if pk.Packet == nil || pk.Packet.For != "e2" || pk.Packet.From != "Ann" || pk.Packet.Structure != Fingerprint(fixture(), "e2") || pk.Packet.SentAt.IsZero() {
		t.Errorf("packet info = %+v", pk.Packet)
	}
}

func person(r *Results, id string) PersonProgress {
	for _, pp := range r.People {
		if pp.ID == id {
			return pp
		}
	}
	return PersonProgress{}
}

func TestProgressFollowsEachPersonsTasks(t *testing.T) {
	// fixture: two sibling groups (1 pair each), Performance in pairs (3 pairs),
	// Security rated (3), Cost measured (3 shared values). Ann has done everything.
	for _, tc := range []struct {
		name  string
		tasks *Tasks
		total int
	}{
		{"everything", nil, 8},
		{"weigh only", &Tasks{Weigh: true}, 2},
		{"score only", &Tasks{ScoreAll: true}, 6},
		{"score Security only", &Tasks{Score: []string{"c3"}}, 3},
		{"weigh and score Performance", &Tasks{Weigh: true, Score: []string{"c2"}}, 5},
	} {
		p := fixture()
		p.Evaluators[1].Tasks = tc.tasks
		r, _ := Compute(p, AllEvaluators)
		bob := person(r, "e2")
		if bob.Total != tc.total || bob.Done != 0 || bob.Finished {
			t.Errorf("%s: Bob %+v, want total %d and nothing done", tc.name, bob, tc.total)
		}
	}
	r, _ := Compute(fixture(), AllEvaluators)
	ann := person(r, "e1")
	if ann.Total != 8 || ann.Done != 8 || !ann.Finished || !ann.Consistent {
		t.Errorf("Ann = %+v, want 8 of 8, finished and consistent", ann)
	}
	if r.Values.Total != 3 || r.Values.Done != 3 {
		t.Errorf("values = %+v, want 3 of 3", r.Values)
	}
}

func TestCompletenessCountsEveryone(t *testing.T) {
	p := fixture() // Ann 8 of 8, Bob 0 of 8, values 3 of 3
	r, _ := Compute(p, AllEvaluators)
	approx(t, "everyone", r.Completeness, 11.0/19, 1e-12)
	rb, _ := Compute(p, "e2")
	approx(t, "Bob", rb.Completeness, 3.0/11, 1e-12)
}

func TestSentCopiesGoOutOfDate(t *testing.T) {
	p := fixture()
	p.Evaluators[1].Invite = &Invite{Structure: Fingerprint(p, "e2")}
	r, _ := Compute(p, AllEvaluators)
	if person(r, "e2").OutOfDate || person(r, "e2").Structure != Fingerprint(p, "e2") {
		t.Errorf("fresh copy: %+v", person(r, "e2"))
	}
	p.Criteria = append(p.Criteria, Criterion{ID: "c5", Name: "Support", Method: MethodRating})
	r, _ = Compute(p, AllEvaluators)
	if !person(r, "e2").OutOfDate || person(r, "e1").OutOfDate {
		t.Errorf("after adding a criterion: Bob %+v, Ann %+v", person(r, "e2"), person(r, "e1"))
	}
}

// twoPeople: criteria X and Y, options a1 and a2. Ann: X 3× Y, a1 3× a2 on X,
// a2 3× a1 on Y. Bob: Y 3× X, and the options equal on both.
func twoPeople() *Project {
	p := New("Two people")
	p.Evaluators = []Evaluator{{ID: "ann", Name: "Ann", Weight: 1}, {ID: "bob", Name: "Bob", Weight: 1}}
	p.Alternatives = []Alternative{{ID: "a1", Name: "One"}, {ID: "a2", Name: "Two"}}
	p.Criteria = []Criterion{{ID: "x", Name: "X", Method: MethodPairwise}, {ID: "y", Name: "Y", Method: MethodPairwise}}
	j := func(ev, ctx, a, b string, v float64) PairwiseJudgment {
		return PairwiseJudgment{EvaluatorID: ev, Context: ctx, A: a, B: b, Value: v}
	}
	p.Pairwise = []PairwiseJudgment{
		j("ann", CriteriaContext(""), "x", "y", 3), j("ann", AltsContext("x"), "a1", "a2", 3), j("ann", AltsContext("y"), "a1", "a2", 1.0/3),
		j("bob", CriteriaContext(""), "x", "y", 1.0/3), j("bob", AltsContext("x"), "a1", "a2", 1), j("bob", AltsContext("y"), "a1", "a2", 1),
	}
	return p
}

func TestCombiningRankingsDiffersFromCombiningAnswers(t *testing.T) {
	p := twoPeople()
	r, _ := Compute(p, AllEvaluators)
	if r.Method != AggregateJudgments {
		t.Errorf("method = %q", r.Method)
	}
	approx(t, "answers combined: One", r.Scores[0].Score, 0.5, 1e-9)

	p.Aggregation = AggregatePriorities
	r, _ = Compute(p, AllEvaluators)
	if r.Method != AggregatePriorities {
		t.Errorf("method = %q", r.Method)
	}
	// Ann ranks One at 0.625, Bob at 0.5: geometric means √(0.625·0.5) and √(0.375·0.5), renormalised.
	approx(t, "rankings combined: One", r.Scores[0].Score, 0.563508, 1e-6)
	approx(t, "rankings combined: Two", r.Scores[1].Score, 0.436492, 1e-6)
	if r.Scores[0].Rank != 1 {
		t.Errorf("ranks = %+v", r.Scores)
	}
	// One person's view is that person's own, whatever the method.
	ra, _ := Compute(p, "ann")
	approx(t, "Ann alone", ra.Scores[0].Score, 0.625, 1e-9)
}

func TestCombiningRankingsFillsUnassignedPartsFromTheGroup(t *testing.T) {
	p := twoPeople()
	p.Aggregation = AggregatePriorities
	// Bob only weighs the criteria; his view of the options comes from the group.
	p.Evaluators[1].Tasks = &Tasks{Weigh: true}
	p.Pairwise = p.Pairwise[:4] // drop Bob's option judgments
	r, err := Compute(p, AllEvaluators)
	if err != nil {
		t.Fatal(err)
	}
	// Group view of the options = Ann's alone: One 0.75 on X, 0.25 on Y.
	// Bob (Y 3× X): 0.25·0.75 + 0.75·0.25 = 0.375; Ann 0.625.
	one := math.Sqrt(0.625 * 0.375)
	two := math.Sqrt(0.375 * 0.625)
	approx(t, "One", r.Scores[0].Score, one/(one+two), 1e-6)
}

func TestEachPersonsRankingIsTheOneCombined(t *testing.T) {
	p := fixture()
	// Bob only weighs, and has: his own scores would all be equal without the group's answers.
	p.Evaluators[1].Tasks = &Tasks{Weigh: true}
	p.Pairwise = append(p.Pairwise,
		PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 1.0 / 3},
		PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext("c1"), A: "c2", B: "c3", Value: 2})
	res, err := Compute(p, AllEvaluators)
	if err != nil {
		t.Fatal(err)
	}
	bob := res.People[1]
	if !bob.Answered || len(bob.Scores) != 3 {
		t.Fatalf("bob: answered %v, scores %+v", bob.Answered, bob.Scores)
	}
	if bob.Scores[0].Score == bob.Scores[1].Score && bob.Scores[1].Score == bob.Scores[2].Score {
		t.Errorf("bob's ranking is a three-way tie; the group's scoring should fill in what he wasn't asked: %+v", bob.Scores)
	}

	// Answers outside someone's role still count, so they count as answered.
	p.Evaluators[1].Tasks = &Tasks{Score: []string{"c3"}}
	res, _ = Compute(p, AllEvaluators)
	if b := res.People[1]; !b.Answered || b.Done != 0 {
		t.Errorf("off-role answers: answered %v, done %d", b.Answered, b.Done)
	}
}
