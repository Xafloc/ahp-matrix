package ahp

import (
	"math"
	"sort"
)

// Line gives each alternative's overall score as a linear function of one
// criterion's local weight x: score[a](x) = Intercept[a] + Slope[a]·x.
type Line struct{ Intercept, Slope []float64 }

// At returns every alternative's score at weight x.
func (l Line) At(x float64) []float64 {
	out := make([]float64, len(l.Slope))
	for a := range out {
		out[a] = l.Intercept[a] + l.Slope[a]*x
	}
	return out
}

// SensitivityLine describes how overall scores change when node's local
// weight is set to x and its siblings are rescaled proportionally to share
// 1−x. If the siblings currently have no weight at all, they share 1−x
// equally. ok is false when node has no siblings, so its weight cannot change.
func SensitivityLine(t Tree, s Synthesis, node int) (l Line, ok bool) {
	p := t.Parent[node]
	gp := 1.0
	if p >= 0 {
		gp = s.Global[p]
	}
	var sibs []int
	for i := range t.Parent {
		if i != node && t.Parent[i] == p {
			sibs = append(sibs, i)
		}
	}
	if len(sibs) == 0 {
		return Line{}, false
	}
	nAlt := len(s.Scores)
	w := t.Local[node]
	l = Line{Intercept: make([]float64, nAlt), Slope: make([]float64, nAlt)}
	for a := 0; a < nAlt; a++ {
		others := 0.0
		for _, sb := range sibs {
			others += t.Local[sb] * s.Subtree[sb][a]
		}
		group := w*s.Subtree[node][a] + others // the sibling group's current share, within the parent
		var q float64                          // the siblings' score per unit of weight they share
		if 1-w > 1e-12 {
			q = others / (1 - w)
		} else {
			for _, sb := range sibs {
				q += s.Subtree[sb][a]
			}
			q /= float64(len(sibs))
		}
		rest := s.Scores[a] - gp*group
		l.Intercept[a] = rest + gp*q
		l.Slope[a] = gp * (s.Subtree[node][a] - q)
	}
	return l, true
}

// Crossover is a weight at which alternatives A and B score equally.
type Crossover struct {
	X    float64
	A, B int
}

// Crossovers returns every weight in [0, 1] where two alternatives' scores
// cross, sorted by weight.
func (l Line) Crossovers() []Crossover {
	var out []Crossover
	for a := range l.Slope {
		for b := a + 1; b < len(l.Slope); b++ {
			if x, ok := l.meet(a, b); ok {
				out = append(out, Crossover{X: x, A: a, B: b})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].X < out[j].X })
	return out
}

// Stability returns the smallest change from weight w at which another
// alternative overtakes the leader at w. ok is false when the leader never
// changes anywhere in [0, 1].
func (l Line) Stability(w float64) (delta float64, ok bool) {
	top := argmax(l.At(w))
	delta = math.Inf(1)
	for b := range l.Slope {
		if b == top {
			continue
		}
		if x, met := l.meet(top, b); met && math.Abs(x-w) < delta {
			delta, ok = math.Abs(x-w), true
		}
	}
	if !ok {
		return 0, false
	}
	return delta, true
}

// meet returns the weight in [0, 1] where alternatives a and b score equally.
func (l Line) meet(a, b int) (float64, bool) {
	ds := l.Slope[a] - l.Slope[b]
	if math.Abs(ds) < 1e-15 {
		return 0, false
	}
	x := (l.Intercept[b] - l.Intercept[a]) / ds
	return x, x >= 0 && x <= 1
}

func argmax(v []float64) int {
	best := 0
	for i := range v {
		if v[i] > v[best] {
			best = i
		}
	}
	return best
}
