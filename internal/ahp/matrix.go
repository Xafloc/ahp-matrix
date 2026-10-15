// Package ahp implements the Analytic Hierarchy Process math: priority
// vectors, consistency, incomplete matrices, aggregation, hierarchy synthesis
// and sensitivity. It knows nothing about projects, files or HTTP.
package ahp

import "math"

// Matrix is a square pairwise comparison matrix. M[i][j] is how strongly item
// i is preferred over item j. A zero off-diagonal entry means the pair has not
// been judged.
type Matrix [][]float64

// NewMatrix returns an n×n matrix with ones on the diagonal and no judgments.
func NewMatrix(n int) Matrix {
	m := make(Matrix, n)
	for i := range m {
		m[i] = make([]float64, n)
		m[i][i] = 1
	}
	return m
}

// Set records that i is preferred over j by v and fills in the reciprocal.
func (m Matrix) Set(i, j int, v float64) {
	m[i][j] = v
	m[j][i] = 1 / v
}

// N returns the matrix dimension.
func (m Matrix) N() int { return len(m) }

// Judged reports whether the pair (i, j) has a judgment.
func (m Matrix) Judged(i, j int) bool { return m[i][j] != 0 }

// Missing returns the number of unordered pairs without a judgment.
func (m Matrix) Missing() int {
	n := 0
	for i := range m {
		for j := i + 1; j < len(m); j++ {
			if !m.Judged(i, j) {
				n++
			}
		}
	}
	return n
}

// Snap returns the value in allowed that is closest to v on a log scale.
// allowed must be non-empty and hold only positive values.
func Snap(v float64, allowed []float64) float64 {
	best, bestDist := allowed[0], math.Inf(1)
	lv := math.Log(v)
	for _, a := range allowed {
		if d := math.Abs(math.Log(a) - lv); d < bestDist {
			best, bestDist = a, d
		}
	}
	return best
}
