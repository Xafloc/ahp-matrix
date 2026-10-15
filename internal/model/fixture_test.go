package model

// fixture returns a small valid project:
//
//	Technical (c1) ─┬─ Performance (c2, pairwise)
//	                └─ Security    (c3, rating)
//	Cost (c4, value, lower is better)
//
// with alternatives a1–a3 and evaluators e1 and e2. Only e1 has judged.
// e1's judgments are fully consistent, so the expected results are exact:
// root weights 0.75/0.25, Technical's children 0.5/0.5, and Performance,
// Security and Cost priorities of [4,2,1]/7, [0.25,0.25,0.5] and [4,2,1]/7.
func fixture() *Project {
	p := New("Fixture")
	p.Evaluators = []Evaluator{{ID: "e1", Name: "Ann", Weight: 1}, {ID: "e2", Name: "Bob", Weight: 1}}
	p.Alternatives = []Alternative{{ID: "a1", Name: "Alpha"}, {ID: "a2", Name: "Beta"}, {ID: "a3", Name: "Gamma"}}
	p.Criteria = []Criterion{
		{ID: "c1", Name: "Technical"},
		{ID: "c2", ParentID: "c1", Name: "Performance", Method: MethodPairwise},
		{ID: "c3", ParentID: "c1", Name: "Security", Method: MethodRating},
		{ID: "c4", Name: "Cost", Method: MethodValue, Direction: DirLower, Unit: "USD"},
	}
	p.Pairwise = []PairwiseJudgment{
		{EvaluatorID: "e1", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 3},
		{EvaluatorID: "e1", Context: CriteriaContext("c1"), A: "c2", B: "c3", Value: 1},
		{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a1", B: "a2", Value: 2},
		{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a1", B: "a3", Value: 4},
		{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a2", B: "a3", Value: 2},
	}
	p.Ratings = []Rating{
		{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "a1", Value: 5},
		{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "a2", Value: 5},
		{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "a3", Value: 10},
	}
	p.Values = []Value{
		{CriterionID: "c4", AlternativeID: "a1", Value: 100},
		{CriterionID: "c4", AlternativeID: "a2", Value: 200},
		{CriterionID: "c4", AlternativeID: "a3", Value: 400},
	}
	return p
}
