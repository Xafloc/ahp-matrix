package ahp

import (
	"math"
	"testing"
)

// withWeight returns a copy of t where node's local weight is x and its
// siblings share 1−x in proportion to their current weights.
func withWeight(t Tree, node int, x float64) Tree {
	out := Tree{Parent: t.Parent, Leaf: t.Leaf, Local: append([]float64(nil), t.Local...)}
	w := t.Local[node]
	for i := range t.Parent {
		if i != node && t.Parent[i] == t.Parent[node] {
			out.Local[i] = t.Local[i] * (1 - x) / (1 - w)
		}
	}
	out.Local[node] = x
	return out
}

func TestSensitivityLineMatchesResynthesis(t *testing.T) {
	tree := sampleTree()
	s, _ := Synthesize(tree, 2)
	for node := range tree.Parent {
		l, ok := SensitivityLine(tree, s, node)
		if !ok {
			t.Fatalf("node %d: ok = false, want true", node)
		}
		for x := 0.0; x <= 1.0; x += 0.05 {
			want, err := Synthesize(withWeight(tree, node, x), 2)
			if err != nil {
				t.Fatal(err)
			}
			approxSlice(t, "score", l.At(x), want.Scores, 1e-12)
		}
	}
}

func TestSensitivityLineZeroWeightSiblings(t *testing.T) {
	tree := Tree{Parent: []int{-1, -1}, Local: []float64{1, 0}, Leaf: [][]float64{{0.9, 0.1}, {0.2, 0.8}}}
	s, _ := Synthesize(tree, 2)
	l, _ := SensitivityLine(tree, s, 0)
	approxSlice(t, "score at 0.25", l.At(0.25), []float64{0.375, 0.625}, 1e-12)
}

func TestSensitivityLineOnlyChild(t *testing.T) {
	tree := Tree{Parent: []int{-1}, Local: []float64{1}, Leaf: [][]float64{{0.5, 0.5}}}
	s, _ := Synthesize(tree, 2)
	if _, ok := SensitivityLine(tree, s, 0); ok {
		t.Error("only child: ok = true, want false")
	}
}

func TestCrossoversAndStability(t *testing.T) {
	// Scores are 0.2+0.7x and 0.8−0.7x, which meet at x = 3/7.
	tree := Tree{Parent: []int{-1, -1}, Local: []float64{0.5, 0.5}, Leaf: [][]float64{{0.9, 0.1}, {0.2, 0.8}}}
	s, _ := Synthesize(tree, 2)
	l, _ := SensitivityLine(tree, s, 0)
	cs := l.Crossovers()
	if len(cs) != 1 || cs[0].A != 0 || cs[0].B != 1 {
		t.Fatalf("Crossovers = %+v, want one crossing of 0 and 1", cs)
	}
	approx(t, "crossover", cs[0].X, 3.0/7, 1e-12)
	d, ok := l.Stability(0.5)
	if !ok {
		t.Fatal("Stability ok = false, want true")
	}
	approx(t, "stability", d, 0.5-3.0/7, 1e-12)
}

func TestStabilityMatchesBruteForce(t *testing.T) {
	tree := sampleTree()
	s, _ := Synthesize(tree, 2)
	for node := range tree.Parent {
		l, _ := SensitivityLine(tree, s, node)
		w := tree.Local[node]
		leader := argmax(l.At(w))
		want := math.Inf(1)
		for x := 0.0; x <= 1.0; x += 1e-5 {
			if argmax(l.At(x)) != leader {
				want = math.Min(want, math.Abs(x-w))
			}
		}
		got, ok := l.Stability(w)
		if math.IsInf(want, 1) {
			if ok {
				t.Errorf("node %d: Stability = %v, want none", node, got)
			}
			continue
		}
		approx(t, "stability", got, want, 2e-5)
	}
}
