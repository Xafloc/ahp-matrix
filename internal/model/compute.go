package model

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/Xafloc/ahp-matrix/internal/ahp"
)

// AllEvaluators asks Compute to combine every evaluator's judgments.
const AllEvaluators = "all"

// ErrUnknownEvaluator means Compute was asked for an evaluator the project lacks.
var ErrUnknownEvaluator = errors.New("unknown evaluator")

// Issue codes reported in Results.Issues.
const (
	IssueNoCriteria        = "no-criteria"        // there is nothing to score against
	IssueGroupDisconnected = "group-disconnected" // a sibling group's comparisons don't link every criterion; weights treated as equal
	IssueAltsDisconnected  = "alts-disconnected"  // a leaf's comparisons don't link every alternative; treated as equal
	IssueRatingsMissing    = "ratings-missing"    // some alternatives are unrated; they get the average rating
	IssueValuesMissing     = "values-missing"     // some alternatives have no value; they get the average value
	IssueValuesInvalid     = "values-invalid"     // values can't be turned into priorities; treated as equal
)

const (
	maxSuggestions   = 3
	maxDisagreements = 10
)

// Results is everything needed to show a project's outcome.
type Results struct {
	Evaluator     string            `json:"evaluator"` // AllEvaluators or an evaluator ID
	Scores        []AltScore        `json:"scores"`    // in project order
	Criteria      []CriterionResult `json:"criteria"`  // parents before children
	Matrices      []MatrixReport    `json:"matrices"`  // one per pairwise comparison group
	Issues        []Issue           `json:"issues"`
	Disagreements []Disagreement    `json:"disagreements"` // most contested pairs first
	Completeness  float64           `json:"completeness"`  // share of required judgments made, 0–1
	Consistent    bool              `json:"consistent"`    // every connected matrix is within its threshold
}

// AltScore is an alternative's overall score (all scores sum to 1) and rank (1 is best).
type AltScore struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
	Rank  int     `json:"rank"`
}

// CriterionResult gives a criterion's weights and the inputs for sensitivity
// analysis: varying this criterion's local weight changes overall scores
// linearly (see ahp.SensitivityLine).
type CriterionResult struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parentId,omitempty"`
	Local     float64   `json:"local"`     // weight among siblings
	Global    float64   `json:"global"`    // share of the whole decision
	Subtree   []float64 `json:"subtree"`   // alternatives' scores within this criterion, in project order
	Stability *float64  `json:"stability"` // smallest change to Local that changes the winner; null if none
}

// MatrixReport describes one pairwise comparison group.
type MatrixReport struct {
	Context     string           `json:"context"`
	Items       []string         `json:"items"` // criterion or alternative IDs, in matrix order
	Weights     []float64        `json:"weights"`
	CR          float64          `json:"cr"`
	Threshold   float64          `json:"threshold"`
	Consistent  bool             `json:"consistent"`
	Complete    bool             `json:"complete"`
	Connected   bool             `json:"connected"`
	Missing     int              `json:"missing"`
	Pairs       int              `json:"pairs"`
	Components  [][]string       `json:"components,omitempty"` // linked groups, when not connected
	Suggestions []PairSuggestion `json:"suggestions"`
}

// PairSuggestion proposes a value for a judgment that contradicts the others.
type PairSuggestion struct {
	A         string  `json:"a"`
	B         string  `json:"b"`
	Current   float64 `json:"current"` // how strongly A is preferred over B now
	Suggested float64 `json:"suggested"`
}

// Issue is a problem that makes the results less trustworthy.
type Issue struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	CriterionID string `json:"criterionId,omitempty"`
	Context     string `json:"context,omitempty"`
}

// Disagreement is a pair that evaluators judged differently.
type Disagreement struct {
	Context string             `json:"context"`
	A       string             `json:"a"`
	B       string             `json:"b"`
	Spread  float64            `json:"spread"` // geometric standard deviation; 1 means full agreement
	Values  map[string]float64 `json:"values"` // evaluator ID → how strongly A is preferred over B
}

// Compute derives weights, scores and diagnostics for a prepared project
// (see Prepare), combining all evaluators or using one evaluator's judgments.
func Compute(p *Project, evaluator string) (*Results, error) {
	c := &computer{
		p:       p,
		weights: map[string]float64{},
		res: &Results{
			Evaluator: evaluator, Scores: []AltScore{}, Criteria: []CriterionResult{},
			Matrices: []MatrixReport{}, Issues: []Issue{}, Disagreements: []Disagreement{},
		},
	}
	if evaluator == AllEvaluators {
		for _, e := range p.Evaluators {
			c.weights[e.ID] = e.Weight
		}
	} else {
		for _, e := range p.Evaluators {
			if e.ID == evaluator {
				c.weights[e.ID] = 1
			}
		}
		if len(c.weights) == 0 {
			return nil, fmt.Errorf("%w %q", ErrUnknownEvaluator, evaluator)
		}
	}
	for _, a := range p.Alternatives {
		c.altIDs = append(c.altIDs, a.ID)
	}
	return c.run()
}

type computer struct {
	p        *Project
	weights  map[string]float64 // evaluator ID → weight, for the evaluators being combined
	altIDs   []string
	res      *Results
	required int // judgments needed for a complete project
	answered int // judgments made
}

type vote struct {
	evaluator string
	value     float64
	weight    float64
}

func (c *computer) run() (*Results, error) {
	order, parent := c.hierarchy()
	nAlt := len(c.altIDs)
	if len(order) == 0 && nAlt > 0 {
		c.issue(Issue{Code: IssueNoCriteria, Message: "Add criteria to score the alternatives against."})
	}

	tree := ahp.Tree{Parent: parent, Local: make([]float64, len(order)), Leaf: make([][]float64, len(order))}
	isParent := make([]bool, len(order))
	for _, pi := range parent {
		if pi >= 0 {
			isParent[pi] = true
		}
	}

	// Local weights, one sibling group at a time, top level first.
	groups := map[int][]int{} // parent index (-1 for top level) → child indices
	var groupOrder []int
	for i, pi := range parent {
		if _, seen := groups[pi]; !seen {
			groupOrder = append(groupOrder, pi)
		}
		groups[pi] = append(groups[pi], i)
	}
	for _, pi := range groupOrder {
		members := groups[pi]
		ids := make([]string, len(members))
		for k, i := range members {
			ids[k] = order[i].ID
		}
		parentID := ""
		if pi >= 0 {
			parentID = order[pi].ID
		}
		w := c.compare(CriteriaContext(parentID), ids, IssueGroupDisconnected, parentID)
		for k, i := range members {
			tree.Local[i] = w[k]
		}
	}

	// Alternatives' local priorities on each leaf.
	for i, cr := range order {
		if isParent[i] {
			continue
		}
		switch {
		case nAlt == 0:
			tree.Leaf[i] = []float64{}
		case cr.Method == MethodRating:
			tree.Leaf[i] = c.ratingPriorities(cr)
		case cr.Method == MethodValue:
			tree.Leaf[i] = c.valuePriorities(cr)
		default:
			tree.Leaf[i] = c.compare(AltsContext(cr.ID), c.altIDs, IssueAltsDisconnected, cr.ID)
		}
	}

	syn, err := ahp.Synthesize(tree, nAlt)
	if err != nil {
		return nil, err
	}
	for a, id := range c.altIDs {
		c.res.Scores = append(c.res.Scores, AltScore{ID: id, Score: syn.Scores[a], Rank: rank(syn.Scores, a)})
	}
	for i, cr := range order {
		r := CriterionResult{ID: cr.ID, ParentID: cr.ParentID, Local: tree.Local[i], Global: syn.Global[i], Subtree: syn.Subtree[i]}
		if line, ok := ahp.SensitivityLine(tree, syn, i); ok && nAlt >= 2 {
			if d, found := line.Stability(tree.Local[i]); found {
				r.Stability = &d
			}
		}
		c.res.Criteria = append(c.res.Criteria, r)
	}

	c.res.Completeness = 1
	if c.required > 0 {
		c.res.Completeness = float64(c.answered) / float64(c.required)
	}
	c.res.Consistent = true
	for _, m := range c.res.Matrices {
		if m.Connected && !m.Consistent {
			c.res.Consistent = false
		}
	}
	d := c.res.Disagreements
	sort.SliceStable(d, func(i, j int) bool {
		if d[i].Spread != d[j].Spread {
			return d[i].Spread > d[j].Spread
		}
		if d[i].Context != d[j].Context {
			return d[i].Context < d[j].Context
		}
		return d[i].A+d[i].B < d[j].A+d[j].B
	})
	if len(d) > maxDisagreements {
		c.res.Disagreements = d[:maxDisagreements]
	}
	return c.res, nil
}

// hierarchy returns the criteria in depth-first order (parents first) and each
// one's parent index (-1 for the top level).
func (c *computer) hierarchy() ([]Criterion, []int) {
	kids := c.p.childrenOf()
	var order []Criterion
	var parent []int
	var walk func(parentID string, parentIdx int)
	walk = func(parentID string, parentIdx int) {
		for _, cr := range kids[parentID] {
			order = append(order, cr)
			parent = append(parent, parentIdx)
			walk(cr.ID, len(order)-1)
		}
	}
	walk("", -1)
	return order, parent
}

// compare builds the combined pairwise matrix for items in context, records
// its report, and returns the items' weights. Groups that can't be resolved
// get equal weights and an issue with the given code.
func (c *computer) compare(context string, items []string, code, criterionID string) []float64 {
	n := len(items)
	if n == 1 {
		return []float64{1}
	}
	idx := map[string]int{}
	for i, id := range items {
		idx[id] = i
	}
	votes := map[[2]int][]vote{}
	for _, j := range c.p.Pairwise {
		w, ok := c.weights[j.EvaluatorID]
		if !ok || j.Context != context {
			continue
		}
		a, okA := idx[j.A]
		b, okB := idx[j.B]
		if !okA || !okB {
			continue
		}
		v := j.Value
		if a > b {
			a, b, v = b, a, 1/v
		}
		key := [2]int{a, b}
		votes[key] = append(votes[key], vote{evaluator: j.EvaluatorID, value: v, weight: w})
	}

	m := ahp.NewMatrix(n)
	for key, vs := range votes {
		ws := make([]ahp.Weighted, len(vs))
		for k, v := range vs {
			ws[k] = ahp.Weighted{Value: v.value, Weight: v.weight}
		}
		m.Set(key[0], key[1], ahp.GeoMean(ws))
		if len(vs) >= 2 {
			d := Disagreement{Context: context, A: items[key[0]], B: items[key[1]], Spread: ahp.GeoStdDev(ws), Values: map[string]float64{}}
			for _, v := range vs {
				d.Values[v.evaluator] = v.value
			}
			c.res.Disagreements = append(c.res.Disagreements, d)
		}
	}
	pairs := n * (n - 1) / 2
	c.required += pairs
	c.answered += len(votes)

	a := ahp.Analyze(m)
	rep := MatrixReport{
		Context: context, Items: items, Weights: a.Weights, CR: a.CR, Threshold: a.Threshold,
		Consistent: a.Consistent, Complete: a.Complete, Connected: a.Connected,
		Missing: a.Missing, Pairs: pairs, Suggestions: []PairSuggestion{},
	}
	if !a.Connected {
		for _, g := range a.Components {
			ids := make([]string, len(g))
			for k, i := range g {
				ids[k] = items[i]
			}
			rep.Components = append(rep.Components, ids)
		}
		rep.Weights = uniform(n)
		c.res.Matrices = append(c.res.Matrices, rep)
		c.issue(Issue{Code: code, Context: context, CriterionID: criterionID,
			Message: fmt.Sprintf("Comparisons don't link all %d items yet, so they are weighted equally.", n)})
		return uniform(n)
	}
	for _, s := range ahp.Suggestions(m, a.Weights, c.p.Scale.Allowed(), maxSuggestions) {
		rep.Suggestions = append(rep.Suggestions, PairSuggestion{A: items[s.I], B: items[s.J], Current: s.Current, Suggested: s.Suggested})
	}
	c.res.Matrices = append(c.res.Matrices, rep)
	return a.Weights
}

// ratingPriorities turns the selected evaluators' 1–10 ratings into priorities.
func (c *computer) ratingPriorities(cr Criterion) []float64 {
	votes := make([][]ahp.Weighted, len(c.altIDs))
	pos := c.altPositions()
	for _, r := range c.p.Ratings {
		w, ok := c.weights[r.EvaluatorID]
		a, okA := pos[r.AlternativeID]
		if ok && okA && r.CriterionID == cr.ID {
			votes[a] = append(votes[a], ahp.Weighted{Value: r.Value, Weight: w})
		}
	}
	raw := make([]float64, len(c.altIDs))
	have := make([]bool, len(c.altIDs))
	for a, vs := range votes {
		if len(vs) > 0 {
			raw[a], have[a] = ahp.ArithMean(vs), true
		}
	}
	if !c.fillMissing(raw, have, cr, IssueRatingsMissing, "rated") {
		return uniform(len(raw))
	}
	return normalize(raw)
}

// valuePriorities turns measured values into priorities, inverting them when
// lower is better.
func (c *computer) valuePriorities(cr Criterion) []float64 {
	raw := make([]float64, len(c.altIDs))
	have := make([]bool, len(c.altIDs))
	pos := c.altPositions()
	for _, v := range c.p.Values {
		if a, ok := pos[v.AlternativeID]; ok && v.CriterionID == cr.ID {
			raw[a], have[a] = v.Value, true
		}
	}
	if !c.fillMissing(raw, have, cr, IssueValuesMissing, "given a value") {
		return uniform(len(raw))
	}
	sum, invalid := 0.0, false
	for _, v := range raw {
		sum += v
		if v < 0 || (cr.Direction == DirLower && v == 0) {
			invalid = true
		}
	}
	if invalid || sum == 0 {
		msg := "Values must be zero or more, and not all zero."
		if cr.Direction == DirLower {
			msg = "When lower is better, every value must be greater than zero."
		}
		c.issue(Issue{Code: IssueValuesInvalid, CriterionID: cr.ID, Message: msg})
		return uniform(len(raw))
	}
	if cr.Direction == DirLower {
		for a := range raw {
			raw[a] = 1 / raw[a]
		}
	}
	out := normalize(raw)
	for _, v := range out {
		if math.IsNaN(v) || math.IsInf(v, 0) { // extreme magnitudes overflow
			c.issue(Issue{Code: IssueValuesInvalid, CriterionID: cr.ID, Message: "These values are too extreme to compare; try different units."})
			return uniform(len(raw))
		}
	}
	return out
}

// fillMissing counts completeness for one leaf and gives alternatives without
// data the average of those with data. It returns false when no alternative has data.
func (c *computer) fillMissing(raw []float64, have []bool, cr Criterion, code, verb string) bool {
	sum, n := 0.0, 0
	for a := range raw {
		if have[a] {
			sum += raw[a]
			n++
		}
	}
	c.required += len(raw)
	c.answered += n
	if n == len(raw) {
		return true
	}
	c.issue(Issue{Code: code, CriterionID: cr.ID,
		Message: fmt.Sprintf("%d of %d alternatives haven't been %s yet.", len(raw)-n, len(raw), verb)})
	if n == 0 {
		return false
	}
	for a := range raw {
		if !have[a] {
			raw[a] = sum / float64(n)
		}
	}
	return true
}

func (c *computer) altPositions() map[string]int {
	pos := make(map[string]int, len(c.altIDs))
	for i, id := range c.altIDs {
		pos[id] = i
	}
	return pos
}

func (c *computer) issue(i Issue) { c.res.Issues = append(c.res.Issues, i) }

func uniform(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = 1 / float64(n)
	}
	return out
}

func normalize(v []float64) []float64 {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / sum
	}
	return out
}

// rank returns 1 plus the number of scores clearly higher than scores[a].
func rank(scores []float64, a int) int {
	r := 1
	for _, s := range scores {
		if s > scores[a]+1e-12 {
			r++
		}
	}
	return r
}
