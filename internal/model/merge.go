package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrDifferentProject means a returned file belongs to another decision.
var ErrDifferentProject = errors.New("this file is a different decision; only files exported from this one can be merged")

// MergePolicy says what to do when a returned file holds judgments for
// someone who already has judgments in the project.
type MergePolicy string

const (
	MergeAsk     MergePolicy = ""        // refuse with *EvaluatorConflictError (only for people who have judged) so the person can choose
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
	Added     []string `json:"added"`     // people whose judgments arrived for the first time
	Replaced  []string `json:"replaced"`  // people whose judgments were replaced
	Skipped   []string `json:"skipped"`   // people left as they were
	Judgments int      `json:"judgments"` // judgments brought in
	Dropped   []string `json:"dropped"`   // judgments that couldn't come across, in words

	droppedBy map[string][]string // the same lines, by evaluator id
}

// MergePreview says what merging a file would do, without doing it.
type MergePreview struct {
	ID        string          `json:"id"`   // the decision it would merge into
	Name      string          `json:"name"` // ...and its name
	People    []PreviewPerson `json:"people"`
	Dropped   []string        `json:"dropped"`   // judgments that can't come across, in words
	OutOfDate bool            `json:"outOfDate"` // the file was made from an older version of the decision
}

// PreviewPerson is one person whose answers are in the file.
type PreviewPerson struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	Judgments            int        `json:"judgments"`        // that would come across
	ReplacesExisting     bool       `json:"replacesExisting"` // they already have judgments here
	PreviouslyReturnedAt *time.Time `json:"previouslyReturnedAt,omitempty"`
	Dropped              []string   `json:"dropped"` // their answers that can't come across
}

// PreviewMerge reports what merging packet into owner would bring across and
// what it would have to leave out. owner is not changed.
func PreviewMerge(owner, packet *Project) (*MergePreview, error) {
	if packet.ID != owner.ID && packet.Origin != owner.ID {
		return nil, ErrDifferentProject
	}
	trial, err := clone(owner)
	if err != nil {
		return nil, err
	}
	rep, err := Merge(trial, packet, MergeReplace)
	if err != nil {
		return nil, err
	}
	judged := map[string]bool{}
	for _, j := range owner.Pairwise {
		judged[j.EvaluatorID] = true
	}
	for _, r := range owner.Ratings {
		judged[r.EvaluatorID] = true
	}
	known := map[string]Evaluator{}
	for _, e := range owner.Evaluators {
		known[e.ID] = e
	}
	pv := &MergePreview{ID: owner.ID, Name: owner.Name, People: []PreviewPerson{}, Dropped: rep.Dropped}
	for _, e := range packet.Evaluators {
		person := PreviewPerson{ID: e.ID, Name: e.Name, ReplacesExisting: judged[e.ID], Dropped: rep.droppedBy[e.ID]}
		if person.Dropped == nil {
			person.Dropped = []string{}
		}
		if mine, ok := known[e.ID]; ok {
			person.Name = mine.Name
			if mine.Invite != nil {
				person.PreviouslyReturnedAt = mine.Invite.ReturnedAt
			}
		}
		for _, j := range trial.Pairwise {
			if j.EvaluatorID == e.ID {
				person.Judgments++
			}
		}
		for _, r := range trial.Ratings {
			if r.EvaluatorID == e.ID {
				person.Judgments++
			}
		}
		pv.People = append(pv.People, person)
	}
	pv.OutOfDate = packet.Packet != nil && packet.Packet.Structure != Fingerprint(owner, packet.Packet.For)
	return pv, nil
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
	them := *who
	them.Invite = nil // the owner's bookkeeping, not theirs
	out.Evaluators = []Evaluator{them}
	from := ""
	if len(p.Evaluators) > 0 {
		from = p.Evaluators[0].Name // the first person is the decision's owner
	}
	out.Packet = &PacketInfo{For: evaluatorID, From: from, SentAt: time.Now().UTC(), Structure: Fingerprint(p, evaluatorID)}
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
	if packet.ID != owner.ID && packet.Origin != owner.ID {
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
	// Only people who have judged something can conflict; someone whose copy
	// is coming back for the first time just gets their judgments.
	judged := map[string]bool{}
	for _, j := range next.Pairwise {
		judged[j.EvaluatorID] = true
	}
	for _, r := range next.Ratings {
		judged[r.EvaluatorID] = true
	}
	rep := &MergeReport{Added: []string{}, Replaced: []string{}, Skipped: []string{}, Dropped: []string{}, droppedBy: map[string][]string{}}
	var conflicts []string
	incoming := map[string]bool{}
	for _, e := range packet.Evaluators {
		name, ok := existing[e.ID]
		switch {
		case !ok:
			next.Evaluators = append(next.Evaluators, e)
			rep.Added = append(rep.Added, e.Name)
			incoming[e.ID] = true
		case !judged[e.ID]:
			rep.Added = append(rep.Added, name)
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
	why := newExplainer(next, packet)
	for _, j := range packet.Pairwise {
		if !incoming[j.EvaluatorID] {
			continue
		}
		if k.pair(j) {
			next.Pairwise = append(next.Pairwise, j)
		} else {
			line := why.pair(j)
			rep.Dropped = append(rep.Dropped, line)
			rep.droppedBy[j.EvaluatorID] = append(rep.droppedBy[j.EvaluatorID], line)
		}
	}
	for _, r := range packet.Ratings {
		if !incoming[r.EvaluatorID] {
			continue
		}
		if k.rating(r) {
			next.Ratings = append(next.Ratings, r)
		} else {
			line := why.rating(r)
			rep.Dropped = append(rep.Dropped, line)
			rep.droppedBy[r.EvaluatorID] = append(rep.droppedBy[r.EvaluatorID], line)
		}
	}
	if err := next.Prepare(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range next.Evaluators {
		e := &next.Evaluators[i]
		if !incoming[e.ID] {
			continue
		}
		if e.Invite == nil {
			e.Invite = &Invite{SentAt: now}
			if packet.Packet != nil {
				e.Invite.SentAt, e.Invite.Structure = packet.Packet.SentAt, packet.Packet.Structure
			} else {
				// Not an invitation copy (say, a whole export): their answers
				// fit the decision as it is now, so they're up to date.
				e.Invite.Structure = Fingerprint(next, e.ID)
			}
		}
		e.Invite.ReturnedAt = &now
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

// explainer words why a judgment from a returned file can't be merged,
// naming things as the file knew them.
type explainer struct {
	owner known
	names map[string]string // id → name, from the file and the owner
}

func newExplainer(owner, packet *Project) explainer {
	x := explainer{owner: owner.known(), names: map[string]string{}}
	for _, p := range []*Project{owner, packet} {
		for _, a := range p.Alternatives {
			x.names[a.ID] = a.Name
		}
		for _, c := range p.Criteria {
			x.names[c.ID] = c.Name
		}
	}
	return x
}

func (x explainer) name(id string) string {
	if n := x.names[id]; n != "" {
		return n
	}
	return id
}

func (x explainer) gone(id string) string { return x.name(id) + " is no longer in this decision" }

func (x explainer) pair(j PairwiseJudgment) string {
	kind, ctx, _ := ParseContext(j.Context)
	subject := fmt.Sprintf("Comparison of %s with %s", x.name(j.A), x.name(j.B))
	if kind == "alts" {
		subject += " on " + x.name(ctx)
		switch {
		case !x.owner.crit[ctx]:
			return subject + ": " + x.gone(ctx)
		case !x.owner.alts[j.A]:
			return subject + ": " + x.gone(j.A)
		case !x.owner.alts[j.B]:
			return subject + ": " + x.gone(j.B)
		}
		return subject + ": it can't be used"
	}
	switch {
	case !x.owner.crit[j.A]:
		return subject + ": " + x.gone(j.A)
	case !x.owner.crit[j.B]:
		return subject + ": " + x.gone(j.B)
	case ctx != "" && !x.owner.crit[ctx]:
		return subject + ": " + x.gone(ctx)
	}
	return subject + ": they're no longer in the same group"
}

func (x explainer) rating(r Rating) string {
	subject := fmt.Sprintf("Rating of %s on %s", x.name(r.AlternativeID), x.name(r.CriterionID))
	switch {
	case !x.owner.crit[r.CriterionID]:
		return subject + ": " + x.gone(r.CriterionID)
	case !x.owner.alts[r.AlternativeID]:
		return subject + ": " + x.gone(r.AlternativeID)
	}
	return subject + ": it can't be used"
}
