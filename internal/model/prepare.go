package model

import (
	"fmt"
	"math"
	"strings"

	"github.com/Xafloc/ahp-matrix/internal/ahp"
)

// Problem is one validation failure, tied to a field path such as
// "criteria[2].method".
type Problem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError lists every problem found in a project.
type ValidationError struct{ Problems []Problem }

func (e *ValidationError) Error() string {
	msg := fmt.Sprintf("invalid project: %s: %s", e.Problems[0].Field, e.Problems[0].Message)
	if n := len(e.Problems) - 1; n > 0 {
		msg += fmt.Sprintf(" (and %d more)", n)
	}
	return msg
}

// Prepare readies a project for saving: it fills in defaults, drops judgments
// that refer to deleted items (and duplicates), validates what is left, and
// snaps pairwise values to the project's scale. Prepare returns a
// *ValidationError when the project cannot be saved.
func (p *Project) Prepare() error {
	p.fillDefaults()
	p.prune()
	if err := p.Validate(); err != nil {
		return err
	}
	allowed := p.Scale.Allowed()
	for i := range p.Pairwise {
		p.Pairwise[i].Value = ahp.Snap(p.Pairwise[i].Value, allowed)
	}
	return nil
}

// childrenOf maps each parent ID ("" for the top level) to its children, in
// project order.
func (p *Project) childrenOf() map[string][]Criterion {
	kids := map[string][]Criterion{}
	for _, c := range p.Criteria {
		kids[c.ParentID] = append(kids[c.ParentID], c)
	}
	return kids
}

func (p *Project) fillDefaults() {
	p.SchemaVersion = SchemaVersion
	if p.ID == "" {
		p.ID = NewProjectID()
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Scale == "" {
		p.Scale = ScaleSaaty9
	}
	if p.Alternatives == nil {
		p.Alternatives = []Alternative{}
	}
	if p.Criteria == nil {
		p.Criteria = []Criterion{}
	}
	if p.Evaluators == nil {
		p.Evaluators = []Evaluator{}
	}
	if p.Pairwise == nil {
		p.Pairwise = []PairwiseJudgment{}
	}
	if p.Ratings == nil {
		p.Ratings = []Rating{}
	}
	if p.Values == nil {
		p.Values = []Value{}
	}
	for i := range p.Alternatives {
		a := &p.Alternatives[i]
		a.Name = strings.TrimSpace(a.Name)
		if a.ID == "" {
			a.ID = NewID()
		}
	}
	for i := range p.Evaluators {
		e := &p.Evaluators[i]
		e.Name = strings.TrimSpace(e.Name)
		if e.ID == "" {
			e.ID = NewID()
		}
		if e.Weight == 0 {
			e.Weight = 1
		}
	}
	for i := range p.Criteria {
		c := &p.Criteria[i]
		c.Name = strings.TrimSpace(c.Name)
		if c.ID == "" {
			c.ID = NewID()
		}
	}
	kids := p.childrenOf()
	for i := range p.Criteria {
		c := &p.Criteria[i]
		if len(kids[c.ID]) > 0 {
			continue
		}
		if c.Method == "" {
			c.Method = MethodPairwise
		}
		if c.Method == MethodValue && c.Direction == "" {
			c.Direction = DirHigher
		}
	}
}

// prune drops judgments that refer to missing evaluators, criteria or
// alternatives, or to a context they don't belong to. Judgments of
// alternatives are kept for any existing criterion, not just leaves, so
// nesting a criterion and un-nesting it again loses nothing; only leaves use
// them. When the same pair, rating or value appears twice, the later one wins.
func (p *Project) prune() {
	evals := map[string]bool{}
	for _, e := range p.Evaluators {
		evals[e.ID] = true
	}
	alts := map[string]bool{}
	for _, a := range p.Alternatives {
		alts[a.ID] = true
	}
	crit := map[string]Criterion{}
	for _, c := range p.Criteria {
		crit[c.ID] = c
	}
	exists := func(id string) bool { _, ok := crit[id]; return ok }

	belongs := func(j PairwiseJudgment) bool {
		kind, id, ok := ParseContext(j.Context)
		if !ok || !evals[j.EvaluatorID] || j.A == j.B {
			return false
		}
		if kind == "alts" {
			return exists(id) && alts[j.A] && alts[j.B]
		}
		if id != "" && !exists(id) {
			return false
		}
		a, okA := crit[j.A]
		b, okB := crit[j.B]
		return okA && okB && a.ParentID == id && b.ParentID == id
	}
	pairs := []PairwiseJudgment{}
	pairAt := map[string]int{}
	for _, j := range p.Pairwise {
		if !belongs(j) {
			continue
		}
		lo, hi := j.A, j.B
		if lo > hi {
			lo, hi = hi, lo
		}
		key := j.EvaluatorID + "\x00" + j.Context + "\x00" + lo + "\x00" + hi
		if i, dup := pairAt[key]; dup {
			pairs[i] = j
			continue
		}
		pairAt[key] = len(pairs)
		pairs = append(pairs, j)
	}
	p.Pairwise = pairs

	ratings := []Rating{}
	ratingAt := map[string]int{}
	for _, r := range p.Ratings {
		if !evals[r.EvaluatorID] || !exists(r.CriterionID) || !alts[r.AlternativeID] {
			continue
		}
		key := r.EvaluatorID + "\x00" + r.CriterionID + "\x00" + r.AlternativeID
		if i, dup := ratingAt[key]; dup {
			ratings[i] = r
			continue
		}
		ratingAt[key] = len(ratings)
		ratings = append(ratings, r)
	}
	p.Ratings = ratings

	values := []Value{}
	valueAt := map[string]int{}
	for _, v := range p.Values {
		if !exists(v.CriterionID) || !alts[v.AlternativeID] {
			continue
		}
		key := v.CriterionID + "\x00" + v.AlternativeID
		if i, dup := valueAt[key]; dup {
			values[i] = v
			continue
		}
		valueAt[key] = len(values)
		values = append(values, v)
	}
	p.Values = values
}

// Validate reports every problem that prevents the project from being saved.
// It returns nil or a *ValidationError.
func (p *Project) Validate() error {
	var probs []Problem
	add := func(field, format string, args ...any) {
		probs = append(probs, Problem{Field: field, Message: fmt.Sprintf(format, args...)})
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

	if !ValidID(p.ID) {
		add("id", "must be 1–64 letters, digits, '-' or '_'")
	}
	if strings.TrimSpace(p.Name) == "" {
		add("name", "is required")
	}
	if !p.Scale.Valid() {
		add("scale", "must be %q or %q", ScaleSaaty9, ScaleClassic5)
	}

	seen := map[string]bool{}
	for i, a := range p.Alternatives {
		f := fmt.Sprintf("alternatives[%d]", i)
		if seen[a.ID] {
			add(f+".id", "duplicate id %q", a.ID)
		}
		seen[a.ID] = true
		if a.Name == "" {
			add(f+".name", "is required")
		}
	}

	if len(p.Evaluators) == 0 {
		add("evaluators", "at least one evaluator is required")
	}
	seen = map[string]bool{}
	for i, e := range p.Evaluators {
		f := fmt.Sprintf("evaluators[%d]", i)
		if seen[e.ID] {
			add(f+".id", "duplicate id %q", e.ID)
		}
		seen[e.ID] = true
		if e.Name == "" {
			add(f+".name", "is required")
		}
		if !finite(e.Weight) || e.Weight <= 0 {
			add(f+".weight", "must be greater than 0")
		}
	}

	byID := map[string]Criterion{}
	for i, c := range p.Criteria {
		f := fmt.Sprintf("criteria[%d]", i)
		if _, dup := byID[c.ID]; dup {
			add(f+".id", "duplicate id %q", c.ID)
		}
		if c.ID == rootKey {
			add(f+".id", "%q is reserved", rootKey)
		}
		byID[c.ID] = c
		if c.Name == "" {
			add(f+".name", "is required")
		}
	}
	kids := p.childrenOf()
	for i, c := range p.Criteria {
		f := fmt.Sprintf("criteria[%d]", i)
		if c.ParentID != "" {
			if _, ok := byID[c.ParentID]; !ok || c.ParentID == c.ID {
				add(f+".parentId", "refers to a missing criterion %q", c.ParentID)
			} else if inCycle(c.ID, byID) {
				add(f+".parentId", "creates a cycle")
			}
		}
		if len(kids[c.ID]) > 0 {
			continue
		}
		switch c.Method {
		case MethodPairwise, MethodRating:
		case MethodValue:
			if c.Direction != DirHigher && c.Direction != DirLower {
				add(f+".direction", "must be %q or %q", DirHigher, DirLower)
			}
		default:
			add(f+".method", "must be %q, %q or %q", MethodPairwise, MethodRating, MethodValue)
		}
	}

	for i, j := range p.Pairwise {
		if !finite(j.Value) || j.Value <= 0 {
			add(fmt.Sprintf("pairwise[%d].value", i), "must be a positive number")
		}
	}
	for i, r := range p.Ratings {
		if !finite(r.Value) || r.Value < 1 || r.Value > 10 {
			add(fmt.Sprintf("ratings[%d].value", i), "must be between 1 and 10")
		}
	}
	for i, v := range p.Values {
		if !finite(v.Value) {
			add(fmt.Sprintf("values[%d].value", i), "must be a number")
		}
	}

	if len(probs) > 0 {
		return &ValidationError{Problems: probs}
	}
	return nil
}

// inCycle reports whether following parents up from id returns to id.
func inCycle(id string, byID map[string]Criterion) bool {
	cur := byID[id].ParentID
	for steps := 0; cur != "" && steps <= len(byID); steps++ {
		if cur == id {
			return true
		}
		cur = byID[cur].ParentID
	}
	return false
}
