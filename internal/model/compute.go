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
	Completeness  float64           `json:"completeness"`  // share of the assigned judgments made (everyone's, or one person's), plus measured values; 0–1
	Consistent    bool              `json:"consistent"`    // every connected matrix is within its threshold
	People        []PersonProgress  `json:"people"`        // each person's progress on their own tasks
	Values        Progress          `json:"values"`        // measured values entered, shared by everyone
	Method        string            `json:"method"`        // how Scores combine people: AggregateJudgments or AggregatePriorities
}

// Progress counts judgments made out of judgments asked for.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// PersonProgress is one person's progress on the tasks assigned to them.
type PersonProgress struct {
	ID         string `json:"id"`
	Done       int    `json:"done"`
	Total      int    `json:"total"`
	Finished   bool   `json:"finished"`   // every assigned judgment made
	Consistent bool   `json:"consistent"` // their own comparisons are within the thresholds
	Structure  string `json:"structure"`  // Fingerprint of their copy as it would be sent now
	OutOfDate  bool   `json:"outOfDate"`  // the copy they were sent no longer matches the decision
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
	// Matrix is the combined comparison matrix, rows and columns in Items
	// order: Matrix[i][j] is how strongly item i is preferred over item j,
	// 0 where the pair hasn't been judged.
	Matrix [][]float64 `json:"matrix"`
}

// PairSuggestion proposes a value for a judgment that contradicts the others.
type PairSuggestion struct {
	A         string  `json:"a"`
	B         string  `json:"b"`
	Current   float64 `json:"current"` // how strongly A is preferred over B now
	Suggested float64 `json:"suggested"`
	CRAfter   float64 `json:"crAfter"` // the consistency ratio if Suggested replaced Current
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
// With AggregatePriorities, the combined Scores are a weighted geometric mean
// of each person's own scores; everything else describes the combined answers.
func Compute(p *Project, evaluator string) (*Results, error) {
	res, err := computeFor(p, evaluator, nil, nil)
	if err != nil {
		return nil, err
	}
	res.People, res.Values = progress(p)
	res.Completeness = completeness(res.People, res.Values, evaluator)
	res.Method = AggregateJudgments
	if evaluator == AllEvaluators && p.Aggregation == AggregatePriorities {
		if scores, ok := combineRankings(p); ok {
			res.Scores, res.Method = scores, AggregatePriorities
		}
	}
	return res, nil
}

// computeFor runs the calculation with the judgments of one evaluator (or
// all). When mine is set, contexts it rejects use the fallback evaluators'
// judgments instead: a person's view, filled in from the group where their
// tasks don't reach.
func computeFor(p *Project, evaluator string, mine func(context string) bool, fallback map[string]float64) (*Results, error) {
	c := &computer{
		p:        p,
		mine:     mine,
		fallback: fallback,
		weights:  map[string]float64{},
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
	mine     func(context string) bool
	fallback map[string]float64 // weights for contexts mine rejects
	altIDs   []string
	res      *Results
}

// weightsFor is the evaluators whose judgments count in a context.
func (c *computer) weightsFor(context string) map[string]float64 {
	if c.mine == nil || c.mine(context) {
		return c.weights
	}
	return c.fallback
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
		w, ok := c.weightsFor(context)[j.EvaluatorID]
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

	a := ahp.Analyze(m)
	rep := MatrixReport{
		Context: context, Items: items, Weights: a.Weights, CR: a.CR, Threshold: a.Threshold,
		Consistent: a.Consistent, Complete: a.Complete, Connected: a.Connected,
		Missing: a.Missing, Pairs: pairs, Suggestions: []PairSuggestion{}, Matrix: m,
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
		rep.Suggestions = append(rep.Suggestions, PairSuggestion{
			A: items[s.I], B: items[s.J], Current: s.Current, Suggested: s.Suggested, CRAfter: crWith(m, s.I, s.J, s.Suggested),
		})
	}
	c.res.Matrices = append(c.res.Matrices, rep)
	return a.Weights
}

// ratingPriorities turns the selected evaluators' 1–10 ratings into priorities.
func (c *computer) ratingPriorities(cr Criterion) []float64 {
	votes := make([][]ahp.Weighted, len(c.altIDs))
	pos := c.altPositions()
	for _, r := range c.p.Ratings {
		w, ok := c.weightsFor(AltsContext(cr.ID))[r.EvaluatorID]
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

// crWith is m's consistency ratio with the judgment for (i, j) set to v.
func crWith(m ahp.Matrix, i, j int, v float64) float64 {
	trial := ahp.NewMatrix(m.N())
	for r := range m {
		copy(trial[r], m[r])
	}
	trial.Set(i, j, v)
	return ahp.Analyze(trial).CR
}

// group is one set of things compared with each other: a sibling group of criteria.
type group struct {
	context string
	items   []string
}

// assigned lists the sibling groups a person weighs and the leaves they score.
func assigned(p *Project, e Evaluator) ([]group, []Criterion) {
	kids := p.childrenOf()
	var groups []group
	if e.Tasks == nil || e.Tasks.Weigh {
		parents := append([]string{""}, criteriaIDs(p)...)
		for _, parent := range parents {
			if k := kids[parent]; len(k) >= 2 {
				groups = append(groups, group{context: CriteriaContext(parent), items: criteriaIDs(&Project{Criteria: k})})
			}
		}
	}
	var leaves []Criterion
	only := map[string]bool{}
	if e.Tasks != nil {
		for _, id := range e.Tasks.Score {
			only[id] = true
		}
	}
	for _, c := range p.Criteria {
		if len(kids[c.ID]) > 0 {
			continue
		}
		if e.Tasks == nil || e.Tasks.ScoreAll || only[c.ID] {
			leaves = append(leaves, c)
		}
	}
	return groups, leaves
}

func criteriaIDs(p *Project) []string {
	ids := make([]string, len(p.Criteria))
	for i, c := range p.Criteria {
		ids[i] = c.ID
	}
	return ids
}

// progress counts each person's judgments against their tasks, and the
// measured values (which are shared, not anyone's).
func progress(p *Project) ([]PersonProgress, Progress) {
	nAlt := len(p.Alternatives)
	alts := map[string]bool{}
	for _, a := range p.Alternatives {
		alts[a.ID] = true
	}
	pairsDone := func(ev, context string, items map[string]bool) int {
		seen := map[string]bool{}
		for _, j := range p.Pairwise {
			if j.EvaluatorID != ev || j.Context != context || !items[j.A] || !items[j.B] || j.A == j.B {
				continue
			}
			lo, hi := j.A, j.B
			if lo > hi {
				lo, hi = hi, lo
			}
			seen[lo+"|"+hi] = true
		}
		return len(seen)
	}
	people := make([]PersonProgress, 0, len(p.Evaluators))
	for _, e := range p.Evaluators {
		pp := PersonProgress{ID: e.ID, Structure: Fingerprint(p, e.ID)}
		groups, leaves := assigned(p, e)
		for _, g := range groups {
			items := map[string]bool{}
			for _, id := range g.items {
				items[id] = true
			}
			pp.Total += len(g.items) * (len(g.items) - 1) / 2
			pp.Done += pairsDone(e.ID, g.context, items)
		}
		for _, c := range leaves {
			switch c.Method {
			case MethodRating:
				pp.Total += nAlt
				rated := map[string]bool{}
				for _, r := range p.Ratings {
					if r.EvaluatorID == e.ID && r.CriterionID == c.ID && alts[r.AlternativeID] {
						rated[r.AlternativeID] = true
					}
				}
				pp.Done += len(rated)
			case MethodValue:
				// measured values are counted once, below
			default:
				pp.Total += nAlt * (nAlt - 1) / 2
				pp.Done += pairsDone(e.ID, AltsContext(c.ID), alts)
			}
		}
		pp.Finished = pp.Total > 0 && pp.Done >= pp.Total
		if own, err := computeFor(p, e.ID, nil, nil); err == nil {
			pp.Consistent = own.Consistent
		}
		pp.OutOfDate = e.Invite != nil && e.Invite.Structure != pp.Structure
		people = append(people, pp)
	}
	var values Progress
	kids := p.childrenOf()
	for _, c := range p.Criteria {
		if len(kids[c.ID]) > 0 || c.Method != MethodValue {
			continue
		}
		values.Total += nAlt
		for _, v := range p.Values {
			if v.CriterionID == c.ID && alts[v.AlternativeID] {
				values.Done++
			}
		}
	}
	return people, values
}

// completeness is the share of assigned judgments made: one person's, or
// everyone's together, plus the measured values.
func completeness(people []PersonProgress, values Progress, evaluator string) float64 {
	done, total := values.Done, values.Total
	for _, pp := range people {
		if evaluator == AllEvaluators || pp.ID == evaluator {
			done += pp.Done
			total += pp.Total
		}
	}
	if total == 0 {
		return 1
	}
	return float64(done) / float64(total)
}

// combineRankings works out each person's own scores (using the group's
// combined answers where their tasks don't reach) and combines them with a
// weighted geometric mean. People who haven't judged anything are left out.
func combineRankings(p *Project) ([]AltScore, bool) {
	everyone := map[string]float64{}
	for _, e := range p.Evaluators {
		everyone[e.ID] = e.Weight
	}
	judged := map[string]bool{}
	for _, j := range p.Pairwise {
		judged[j.EvaluatorID] = true
	}
	for _, r := range p.Ratings {
		judged[r.EvaluatorID] = true
	}
	n := len(p.Alternatives)
	logSum := make([]float64, n)
	totalWeight := 0.0
	for _, e := range p.Evaluators {
		if !judged[e.ID] {
			continue
		}
		groups, leaves := assigned(p, e)
		mine := map[string]bool{}
		for _, g := range groups {
			mine[g.context] = true
		}
		for _, c := range leaves {
			mine[AltsContext(c.ID)] = true
		}
		r, err := computeFor(p, e.ID, func(ctx string) bool { return mine[ctx] }, everyone)
		if err != nil || len(r.Scores) != n {
			return nil, false
		}
		for a, s := range r.Scores {
			logSum[a] += e.Weight * math.Log(math.Max(s.Score, 1e-300))
		}
		totalWeight += e.Weight
	}
	if totalWeight == 0 || n == 0 {
		return nil, false
	}
	raw := make([]float64, n)
	for a := range raw {
		raw[a] = math.Exp(logSum[a] / totalWeight)
	}
	combined := normalize(raw)
	scores := make([]AltScore, n)
	for a, alt := range p.Alternatives {
		scores[a] = AltScore{ID: alt.ID, Score: combined[a], Rank: rank(combined, a)}
	}
	return scores, true
}
