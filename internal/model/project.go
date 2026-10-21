// Package model defines the project document stored in .ahp.json files,
// keeps it valid, and turns it into AHP results.
package model

import "time"

// SchemaVersion is the newest project file version this build reads and writes.
// Version 2 added evaluator tasks and invitations, the combining method, and
// packet details; version 1 files load unchanged.
const SchemaVersion = 2

// How several people's judgments become one result.
const (
	AggregateJudgments  = "judgments"  // combine their answers (AIJ): the group acts as one
	AggregatePriorities = "priorities" // combine their final rankings (AIP): independent experts
)

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
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id"`
	Version       int    `json:"version"` // incremented on every save
	Name          string `json:"name"`
	// Origin is the id of the decision this file was copied from when it was
	// imported under a new id (its own id was taken), so judgments made in the
	// copy can still be merged back into the original.
	Origin      string    `json:"origin,omitempty"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Scale       Scale     `json:"scale"`
	Aggregation string    `json:"aggregation,omitempty"` // AggregateJudgments (default) or AggregatePriorities
	// Packet is set only in a copy sent to someone to judge (see Packet).
	Packet       *PacketInfo        `json:"packet,omitempty"`
	Alternatives []Alternative      `json:"alternatives"`
	Criteria     []Criterion        `json:"criteria"`
	Evaluators   []Evaluator        `json:"evaluators"`
	Pairwise     []PairwiseJudgment `json:"pairwise"`
	Ratings      []Rating           `json:"ratings"`
	Values       []Value            `json:"values"`
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
	Tasks  *Tasks  `json:"tasks,omitempty"`  // nil: everything
	Invite *Invite `json:"invite,omitempty"` // set once a copy has been sent to them
}

// Tasks is the part of a decision one person is asked to judge.
type Tasks struct {
	Weigh    bool     `json:"weigh"`           // compare the criteria in every sibling group
	ScoreAll bool     `json:"scoreAll"`        // score the options on every leaf criterion...
	Score    []string `json:"score,omitempty"` // ...or only on these leaves
}

// Invite records a copy sent to someone, so the app can tell when it's out of date.
type Invite struct {
	SentAt     time.Time  `json:"sentAt"`
	Structure  string     `json:"structure"` // Fingerprint of the decision when the copy was made
	ReturnedAt *time.Time `json:"returnedAt,omitempty"`
}

// PacketInfo marks a copy made for one person to judge and send back.
type PacketInfo struct {
	For       string    `json:"for"`  // the evaluator id it was made for
	From      string    `json:"from"` // who sent it
	SentAt    time.Time `json:"sentAt"`
	Structure string    `json:"structure"` // Fingerprint when it was made
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
