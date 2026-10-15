package ahp

import "testing"

// sampleTree is Technical (0.6) with children Performance and Security
// (0.5 each), plus Cost (0.4), scoring two alternatives.
func sampleTree() Tree {
	return Tree{
		Parent: []int{-1, -1, 0, 0},
		Local:  []float64{0.6, 0.4, 0.5, 0.5},
		Leaf:   [][]float64{nil, {0.2, 0.8}, {0.7, 0.3}, {0.4, 0.6}},
	}
}

func TestSynthesize(t *testing.T) {
	s, err := Synthesize(sampleTree(), 2)
	if err != nil {
		t.Fatal(err)
	}
	approxSlice(t, "Global", s.Global, []float64{0.6, 0.4, 0.3, 0.3}, 1e-12)
	approxSlice(t, "Subtree[0]", s.Subtree[0], []float64{0.55, 0.45}, 1e-12)
	approxSlice(t, "Subtree[1]", s.Subtree[1], []float64{0.2, 0.8}, 1e-12)
	approxSlice(t, "Scores", s.Scores, []float64{0.41, 0.59}, 1e-12)
}

func TestSynthesizeRejectsBadTrees(t *testing.T) {
	bad := Tree{Parent: []int{1, -1}, Local: []float64{1, 1}, Leaf: [][]float64{{1}, nil}}
	if _, err := Synthesize(bad, 1); err == nil {
		t.Error("child before parent: want error")
	}
	short := sampleTree()
	short.Leaf[1] = []float64{1}
	if _, err := Synthesize(short, 2); err == nil {
		t.Error("leaf with wrong length: want error")
	}
	if _, err := Synthesize(Tree{Parent: []int{-1}}, 1); err == nil {
		t.Error("mismatched slices: want error")
	}
}
