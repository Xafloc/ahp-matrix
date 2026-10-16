package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrDifferentProject means a returned file belongs to another decision.
var ErrDifferentProject = errors.New("this file is a different decision; only files exported from this one can be merged")

// MergePolicy says what to do when a returned file holds judgments for
// someone who already has judgments in the project.
type MergePolicy string

const (
	MergeAsk     MergePolicy = ""        // refuse with *EvaluatorConflictError so the person can choose
	MergeReplace MergePolicy = "replace" // the file's judgments replace theirs
	MergeSkip    MergePolicy = "skip"    // keep theirs and ignore the file's
)

// EvaluatorConflictError lists the people already present when merging with MergeAsk.
type EvaluatorConflictError struct{ Names []string }

func (e *EvaluatorConflictError) Error() string {
	return fmt.Sprintf("%s already %s judgments in this decision", strings.Join(e.Names, ", "), map[bool]string{true: "has", false: "have"}[len(e.Names) == 1])
}

// MergeReport says what a merge did.
type MergeReport struct {
	Added     []string `json:"added"`     // people new to the project
	Replaced  []string `json:"replaced"`  // people whose judgments were replaced
	Skipped   []string `json:"skipped"`   // people left as they were
	Judgments int      `json:"judgments"` // judgments brought in
	Dropped   int      `json:"dropped"`   // judgments about options or criteria the project no longer has
}

// Packet is a copy of the project to send to one person: the full structure
// and measured values, but only that person's judgments.
func Packet(p *Project, evaluatorID string) (*Project, error) {
	var who *Evaluator
	for i := range p.Evaluators {
		if p.Evaluators[i].ID == evaluatorID {
			who = &p.Evaluators[i]
		}
	}
	if who == nil {
		return nil, fmt.Errorf("%w %q", ErrUnknownEvaluator, evaluatorID)
	}
	out, err := clone(p)
	if err != nil {
		return nil, err
	}
	out.Evaluators = []Evaluator{*who}
	out.Pairwise = keep(out.Pairwise, func(j PairwiseJudgment) bool { return j.EvaluatorID == evaluatorID })
	out.Ratings = keep(out.Ratings, func(r Rating) bool { return r.EvaluatorID == evaluatorID })
	return out, nil
}

// Merge brings the judgments in a returned packet (or any export of the same
// project) into owner. Only people's judgments come across; the owner's
// options, criteria, measured values and names always win. Judgments about
// items the owner no longer has are dropped and counted. On error, owner is
// unchanged.
func Merge(owner, packet *Project, policy MergePolicy) (*MergeReport, error) {
	if packet.ID != owner.ID {
		return nil, ErrDifferentProject
	}
	next, err := clone(owner)
	if err != nil {
		return nil, err
	}
	existing := map[string]string{}
	for _, e := range next.Evaluators {
		existing[e.ID] = e.Name
	}
	rep := &MergeReport{Added: []string{}, Replaced: []string{}, Skipped: []string{}}
	var conflicts []string
	incoming := map[string]bool{}
	for _, e := range packet.Evaluators {
		name, ok := existing[e.ID]
		switch {
		case !ok:
			next.Evaluators = append(next.Evaluators, e)
			rep.Added = append(rep.Added, e.Name)
			incoming[e.ID] = true
		case policy == MergeAsk:
			conflicts = append(conflicts, name)
		case policy == MergeReplace:
			rep.Replaced = append(rep.Replaced, name)
			incoming[e.ID] = true
		default:
			rep.Skipped = append(rep.Skipped, name)
		}
	}
	if len(conflicts) > 0 {
		return nil, &EvaluatorConflictError{Names: conflicts}
	}

	// Replace means replace: drop what the owner had for these people first.
	next.Pairwise = keep(next.Pairwise, func(j PairwiseJudgment) bool { return !incoming[j.EvaluatorID] })
	next.Ratings = keep(next.Ratings, func(r Rating) bool { return !incoming[r.EvaluatorID] })
	k := next.known()
	for _, j := range packet.Pairwise {
		if !incoming[j.EvaluatorID] {
			continue
		}
		if k.pair(j) {
			next.Pairwise = append(next.Pairwise, j)
		} else {
			rep.Dropped++
		}
	}
	for _, r := range packet.Ratings {
		if !incoming[r.EvaluatorID] {
			continue
		}
		if k.rating(r) {
			next.Ratings = append(next.Ratings, r)
		} else {
			rep.Dropped++
		}
	}
	if err := next.Prepare(); err != nil {
		return nil, err
	}
	for _, j := range next.Pairwise {
		if incoming[j.EvaluatorID] {
			rep.Judgments++
		}
	}
	for _, r := range next.Ratings {
		if incoming[r.EvaluatorID] {
			rep.Judgments++
		}
	}
	*owner = *next
	return rep, nil
}

func keep[T any](items []T, ok func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, it := range items {
		if ok(it) {
			out = append(out, it)
		}
	}
	return out
}

// clone deep-copies a project.
func clone(p *Project) (*Project, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var out Project
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
