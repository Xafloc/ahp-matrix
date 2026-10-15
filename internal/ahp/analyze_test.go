package ahp

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestAnalyzeSaatyExample(t *testing.T) {
	m := NewMatrix(3)
	m.Set(0, 1, 3)
	m.Set(0, 2, 5)
	m.Set(1, 2, 3)
	a := Analyze(m)
	approxSlice(t, "weights", a.Weights, []float64{0.636986, 0.258285, 0.104729}, 1e-6)
	approx(t, "λmax", a.LambdaMax, 3.038511, 1e-6)
	approx(t, "CR", a.CR, 0.033199, 1e-6)
	if !a.Consistent || !a.Complete || !a.Connected {
		t.Errorf("got Consistent=%v Complete=%v Connected=%v, want all true", a.Consistent, a.Complete, a.Connected)
	}
}

func TestAnalyzeConsistentMatrixRecoversWeights(t *testing.T) {
	w := []float64{0.4, 0.3, 0.2, 0.1}
	a := Analyze(consistent(w))
	approxSlice(t, "weights", a.Weights, w, 1e-9)
	approx(t, "CR", a.CR, 0, 1e-9)
}

func TestAnalyzeInconsistentMatrix(t *testing.T) {
	m := consistent([]float64{0.4, 0.3, 0.2, 0.1})
	m.Set(0, 3, 0.25) // contradicts the other judgments, which imply 4
	a := Analyze(m)
	approx(t, "CR", a.CR, 0.402553, 1e-6)
	if a.Consistent {
		t.Error("Consistent = true, want false")
	}
}

func TestAnalyzeIncompleteConsistentMatrixUsesHarker(t *testing.T) {
	w := []float64{0.4, 0.3, 0.2, 0.1}
	m := consistent(w)
	m[0][3], m[3][0] = 0, 0
	m[1][2], m[2][1] = 0, 0
	a := Analyze(m)
	if a.Missing != 2 || a.Complete || !a.Connected {
		t.Fatalf("got Missing=%d Complete=%v Connected=%v", a.Missing, a.Complete, a.Connected)
	}
	approxSlice(t, "weights", a.Weights, w, 1e-9)
	approx(t, "CR", a.CR, 0, 1e-9)
}

func TestAnalyzeDisconnected(t *testing.T) {
	m := NewMatrix(4)
	m.Set(0, 1, 2)
	m.Set(2, 3, 5)
	a := Analyze(m)
	if a.Connected || a.Weights != nil {
		t.Fatalf("got Connected=%v Weights=%v, want false and nil", a.Connected, a.Weights)
	}
	if want := [][]int{{0, 1}, {2, 3}}; !reflect.DeepEqual(a.Components, want) {
		t.Errorf("Components = %v, want %v", a.Components, want)
	}
}

func TestAnalyzeTinyMatrices(t *testing.T) {
	if a := Analyze(NewMatrix(0)); a.Weights != nil || !a.Consistent {
		t.Errorf("n=0: got %+v", a)
	}
	approxSlice(t, "n=1 weights", Analyze(NewMatrix(1)).Weights, []float64{1}, 0)
	m := NewMatrix(2)
	m.Set(0, 1, 3)
	a := Analyze(m)
	approxSlice(t, "n=2 weights", a.Weights, []float64{0.75, 0.25}, 1e-12)
	approx(t, "n=2 CR", a.CR, 0, 0)
}

func TestRandomIndex(t *testing.T) {
	approx(t, "RI(3)", RandomIndex(3), 0.58, 0)
	approx(t, "RI(15)", RandomIndex(15), 1.59, 0)
	approx(t, "RI(16)", RandomIndex(16), 1.597807, 1e-6)
}

func TestAnalyzeWeightsAlwaysSumToOne(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for trial := 0; trial < 200; trial++ {
		n := 2 + r.Intn(29) // up to 30 items, past the legacy sheet's 25 and the n > 15 RI branch
		m := NewMatrix(n)
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				m.Set(i, j, saaty9[r.Intn(len(saaty9))])
			}
		}
		a := Analyze(m)
		if math.IsNaN(a.CR) || math.IsInf(a.CR, 0) || a.CR < 0 {
			t.Fatalf("trial %d (n=%d): CR = %v", trial, n, a.CR)
		}
		sum := 0.0
		for _, w := range a.Weights {
			if w <= 0 {
				t.Fatalf("trial %d: non-positive weight %v", trial, w)
			}
			sum += w
		}
		approx(t, "sum of weights", sum, 1, 1e-9)
	}
}

// chain returns a consistent but sparse matrix: only neighbours i, i+1 are
// judged. Harker's method recovers the exact weights, but its large diagonal
// puts the second eigenvalue close to λmax, which stalls power iteration.
func chain(w []float64) Matrix {
	m := NewMatrix(len(w))
	for i := 0; i+1 < len(w); i++ {
		m.Set(i, i+1, w[i]/w[i+1])
	}
	return m
}

func TestAnalyzeSparseChainsAreExact(t *testing.T) {
	for _, n := range []int{15, 25, 50} {
		w := make([]float64, n)
		sum := 0.0
		for i := range w {
			w[i] = math.Pow(1.3, float64(n-i))
			sum += w[i]
		}
		for i := range w {
			w[i] /= sum
		}
		a := Analyze(chain(w))
		for i := range w {
			if rel := math.Abs(a.Weights[i]/w[i] - 1); rel > 1e-8 {
				t.Fatalf("n=%d: weight %d off by %.2g relative (got %v, want %v)", n, i, rel, a.Weights[i], w[i])
			}
		}
		approx(t, "CR of a consistent chain", a.CR, 0, 1e-9)
	}
}
