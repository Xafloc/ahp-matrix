// Package model defines the project document stored in .ahp.json files,
// keeps it valid, and turns it into AHP results.
package model

import "time"

// SchemaVersion is the newest project file version this build reads and writes.
const SchemaVersion = 1

// Method is how a leaf criterion scores the alternatives.
type Method string

const (
	MethodPairwise Method = "pairwise" // compare alternatives two at a time
	MethodRating   Method = "rating"   // rate each alternative 1–10
	MethodValue    Method = "value"    // enter a measured value per alternative
)

// Direction says whether larger measured values are better.
type Direction string

const (
	DirHigher Direction = "higher"
	DirLower  Direction = "lower"
)

// Project is one decision: alternatives scored against weighted criteria.
type Project struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Version       int                `json:"version"` // incremented on every save
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
	Scale         Scale              `json:"scale"`
	Alternatives  []Alternative      `json:"alternatives"`
	Criteria      []Criterion        `json:"criteria"`
	Evaluators    []Evaluator        `json:"evaluators"`
	Pairwise      []PairwiseJudgment `json:"pairwise"`
	Ratings       []Rating           `json:"ratings"`
	Values        []Value            `json:"values"`
}

// Alternative is one option being chosen between.
type Alternative struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// Criterion is a node in the criteria tree. Only leaves (criteria without
// children) have a Method and score alternatives.
type Criterion struct {
	ID          string    `json:"id"`
	ParentID    string    `json:"parentId,omitempty"` // empty for a top-level criterion
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Notes       string    `json:"notes"`
	Method      Method    `json:"method,omitempty"`
	Direction   Direction `json:"direction,omitempty"` // MethodValue only
	Unit        string    `json:"unit,omitempty"`      // MethodValue only, for display
}

// Evaluator is a person whose judgments count, with a relative weight.
type Evaluator struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// PairwiseJudgment says how strongly A is preferred over B within Context
// (see CriteriaContext and AltsContext). Each unordered pair is stored once
// per evaluator and context; the reciprocal is implied.
type PairwiseJudgment struct {
	EvaluatorID string  `json:"evaluatorId"`
	Context     string  `json:"context"`
	A           string  `json:"a"`
	B           string  `json:"b"`
	Value       float64 `json:"value"`
	Note        string  `json:"note,omitempty"`
}

// Rating is one evaluator's 1–10 score for an alternative on a leaf criterion.
type Rating struct {
	EvaluatorID   string  `json:"evaluatorId"`
	CriterionID   string  `json:"criterionId"`
	AlternativeID string  `json:"alternativeId"`
	Value         float64 `json:"value"`
}

// Value is an objective measurement of an alternative on a leaf criterion.
type Value struct {
	CriterionID   string  `json:"criterionId"`
	AlternativeID string  `json:"alternativeId"`
	Value         float64 `json:"value"`
}

// New returns an empty project with a single evaluator, ready to save.
func New(name string) *Project {
	now := time.Now().UTC()
	return &Project{
		SchemaVersion: SchemaVersion,
		ID:            NewProjectID(),
		Name:          name,
		CreatedAt:     now,
		UpdatedAt:     now,
		Scale:         ScaleSaaty9,
		Alternatives:  []Alternative{},
		Criteria:      []Criterion{},
		Evaluators:    []Evaluator{{ID: NewID(), Name: "Me", Weight: 1}},
		Pairwise:      []PairwiseJudgment{},
		Ratings:       []Rating{},
		Values:        []Value{},
	}
}
