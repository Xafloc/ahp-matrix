package ahp

import (
	"errors"
	"fmt"
)

// Tree is a criteria hierarchy flattened into parallel slices, ordered so that
// every parent comes before its children.
type Tree struct {
	Parent []int       // parent index, or -1 for a top-level criterion
	Local  []float64   // weight among siblings; each sibling group sums to 1
	Leaf   [][]float64 // per node: alternatives' local priorities for a leaf, nil otherwise
}

// Synthesis is the result of combining a Tree.
type Synthesis struct {
	Global  []float64   // per node: product of local weights from the top down
	Subtree [][]float64 // per node: alternatives' scores within that subtree; each sums to 1
	Scores  []float64   // per alternative: overall score; sums to 1
}

// Synthesize combines local weights and leaf priorities for nAlt alternatives.
func Synthesize(t Tree, nAlt int) (Synthesis, error) {
	n := len(t.Parent)
	if len(t.Local) != n || len(t.Leaf) != n {
		return Synthesis{}, errors.New("ahp: tree slices differ in length")
	}
	s := Synthesis{Global: make([]float64, n), Subtree: make([][]float64, n), Scores: make([]float64, nAlt)}
	for i := 0; i < n; i++ {
		p := t.Parent[i]
		switch {
		case p >= i:
			return Synthesis{}, fmt.Errorf("ahp: node %d has parent %d; parents must come first", i, p)
		case p < 0:
			s.Global[i] = t.Local[i]
		default:
			s.Global[i] = s.Global[p] * t.Local[i]
		}
		s.Subtree[i] = make([]float64, nAlt)
	}
	// Bottom-up: children come after parents, so walking backwards finishes
	// every subtree before it is added to its parent.
	for i := n - 1; i >= 0; i-- {
		if t.Leaf[i] != nil {
			if len(t.Leaf[i]) != nAlt {
				return Synthesis{}, fmt.Errorf("ahp: leaf %d has %d priorities, want %d", i, len(t.Leaf[i]), nAlt)
			}
			copy(s.Subtree[i], t.Leaf[i])
		}
		target := s.Scores
		if p := t.Parent[i]; p >= 0 {
			target = s.Subtree[p]
		}
		for a := 0; a < nAlt; a++ {
			target[a] += t.Local[i] * s.Subtree[i][a]
		}
	}
	return s, nil
}
