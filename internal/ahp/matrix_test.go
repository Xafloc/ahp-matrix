package ahp

import (
	"math"
	"testing"
)

// saaty9 is the 1–9 scale and its reciprocals, ascending.
var saaty9 = []float64{1.0 / 9, 1.0 / 8, 1.0 / 7, 1.0 / 6, 1.0 / 5, 1.0 / 4, 1.0 / 3, 1.0 / 2, 1, 2, 3, 4, 5, 6, 7, 8, 9}

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (±%g)", name, got, want, tol)
	}
}

func approxSlice(t *testing.T, name string, got, want []float64, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s has length %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		approx(t, name, got[i], want[i], tol)
	}
}

// consistent returns the perfectly consistent matrix for weights w.
func consistent(w []float64) Matrix {
	m := NewMatrix(len(w))
	for i := range w {
		for j := i + 1; j < len(w); j++ {
			m.Set(i, j, w[i]/w[j])
		}
	}
	return m
}

func TestSetFillsReciprocal(t *testing.T) {
	m := NewMatrix(3)
	m.Set(0, 2, 4)
	approx(t, "m[0][2]", m[0][2], 4, 0)
	approx(t, "m[2][0]", m[2][0], 0.25, 0)
	if m.Missing() != 2 {
		t.Errorf("Missing() = %d, want 2", m.Missing())
	}
	if !m.Judged(2, 0) || m.Judged(0, 1) {
		t.Error("Judged reports the wrong pairs")
	}
}

func TestSnap(t *testing.T) {
	classic5 := []float64{0.1, 0.2, 1, 5, 10}
	cases := []struct {
		v       float64
		allowed []float64
		want    float64
	}{
		{2.6, saaty9, 3},
		{0.826, saaty9, 1},
		{0.3, classic5, 0.2},
		{3, classic5, 5},
		{50, saaty9, 9},
	}
	for _, c := range cases {
		approx(t, "Snap", Snap(c.v, c.allowed), c.want, 1e-12)
	}
}
