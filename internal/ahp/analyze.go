package ahp

import (
	"math"
	"sort"
)

const (
	bisectTolerance  = 1e-14 // relative width of the bracket around λmax
	bisectMaxIter    = 200
	inverseTolerance = 1e-15
	inverseMaxIter   = 20
)

// saatyRI holds Saaty's random consistency indices, indexed by n.
var saatyRI = [...]float64{0, 0, 0, 0.58, 0.90, 1.12, 1.24, 1.32, 1.41, 1.45, 1.49, 1.51, 1.48, 1.56, 1.57, 1.59}

// RandomIndex returns the random consistency index for an n×n matrix:
// Saaty's table for n ≤ 15 and Alonso & Lamata's regression above that.
func RandomIndex(n int) float64 {
	if n < len(saatyRI) {
		return saatyRI[n]
	}
	fn := float64(n)
	return (1.7699*fn - 4.3513) / (fn - 1)
}

// CRThreshold returns the largest acceptable consistency ratio for n items.
func CRThreshold(n int) float64 {
	switch n {
	case 3:
		return 0.05
	case 4:
		return 0.08
	default:
		return 0.10
	}
}

// Analysis is the result of analyzing one comparison matrix.
type Analysis struct {
	Weights    []float64 // principal eigenvector summing to 1; nil when not connected
	LambdaMax  float64
	CI, CR     float64
	Threshold  float64
	Consistent bool
	Missing    int     // unjudged pairs
	Complete   bool    // Missing == 0
	Connected  bool    // judgments link every item to every other
	Components [][]int // groups of linked item indices; a single group when connected
}

// Analyze computes priorities and consistency for m. Incomplete matrices use
// Harker's method: a missing entry becomes 0 and each diagonal entry becomes 1
// plus the number of missing entries in its row. Weights are only defined when
// the judgments connect all items.
func Analyze(m Matrix) Analysis {
	n := m.N()
	a := Analysis{Missing: m.Missing(), Threshold: CRThreshold(n)}
	a.Complete = a.Missing == 0
	a.Components = components(m)
	a.Connected = len(a.Components) <= 1
	if n == 0 {
		a.Consistent = true
		return a
	}
	if !a.Connected {
		return a
	}

	h := NewMatrix(n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			h[i][j] = m[i][j]
			if !m.Judged(i, j) {
				h[i][i]++
			}
		}
	}
	a.Weights, a.LambdaMax = principal(h)
	if n > 2 {
		a.CI = math.Max(0, (a.LambdaMax-float64(n))/float64(n-1))
		a.CR = a.CI / RandomIndex(n)
	}
	a.Consistent = a.CR <= a.Threshold
	return a
}

// principal returns the normalized principal (Perron) eigenvector of a
// non-negative, irreducible matrix with a positive diagonal, and its
// eigenvalue λmax.
//
// Plain power iteration stalls on sparse Harker matrices: their large
// diagonals put the second eigenvalue close to λmax. Instead λmax is
// bracketed by bisection, using the fact that μI−M is a nonsingular
// M-matrix exactly when μ > λmax, which Gaussian elimination reveals through
// its pivots. Inverse iteration with μ just above λmax then converges in a
// step or two, whatever the eigenvalue gap.
func principal(m Matrix) ([]float64, float64) {
	n := m.N()
	if n == 1 {
		return []float64{1}, m[0][0]
	}
	// λmax lies between the smallest and largest row sums.
	lo, hi := math.Inf(1), 0.0
	for i := range m {
		s := 0.0
		for _, v := range m[i] {
			s += v
		}
		lo, hi = math.Min(lo, s), math.Max(hi, s)
	}
	hi++ // strictly above λmax, so μI−M starts out nonsingular
	for iter := 0; iter < bisectMaxIter && hi-lo > bisectTolerance*hi; iter++ {
		mid := (lo + hi) / 2
		if _, ok := shiftedSolve(m, mid, nil); ok {
			hi = mid
		} else {
			lo = mid
		}
	}

	w := make([]float64, n)
	for i := range w {
		w[i] = 1 / float64(n)
	}
	for iter := 0; iter < inverseMaxIter; iter++ {
		x, _ := shiftedSolve(m, hi, w)
		sum := 0.0
		for _, v := range x {
			sum += v
		}
		delta := 0.0
		for i := range x {
			x[i] /= sum
			delta = math.Max(delta, math.Abs(x[i]-w[i]))
		}
		w = x
		if delta < inverseTolerance {
			break
		}
	}
	lambda := 0.0 // w sums to 1, so the sum of Mw is λmax
	for i := range m {
		for j, v := range m[i] {
			lambda += v * w[j]
		}
	}
	return w, lambda
}

// shiftedSolve factors μI−M by Gaussian elimination without pivoting. ok is
// false when a pivot is not positive, i.e. when μI−M is not a nonsingular
// M-matrix (μ ≤ λmax). When ok and b is non-nil, it also solves (μI−M)x = b.
func shiftedSolve(m Matrix, mu float64, b []float64) (x []float64, ok bool) {
	n := m.N()
	a := make([][]float64, n)
	for i := range a {
		a[i] = make([]float64, n)
		for j := range a[i] {
			a[i][j] = -m[i][j]
		}
		a[i][i] += mu
	}
	if b != nil {
		x = append([]float64(nil), b...)
	}
	for k := 0; k < n; k++ {
		if !(a[k][k] > 0) {
			return nil, false
		}
		for i := k + 1; i < n; i++ {
			f := a[i][k] / a[k][k]
			for j := k; j < n; j++ {
				a[i][j] -= f * a[k][j]
			}
			if x != nil {
				x[i] -= f * x[k]
			}
		}
	}
	for i := n - 1; x != nil && i >= 0; i-- {
		s := x[i]
		for j := i + 1; j < n; j++ {
			s -= a[i][j] * x[j]
		}
		x[i] = s / a[i][i]
	}
	return x, true
}

// components groups item indices that are linked by judgments. Each group is
// sorted, and groups are ordered by their smallest index.
func components(m Matrix) [][]int {
	n := m.N()
	seen := make([]bool, n)
	var groups [][]int
	for start := 0; start < n; start++ {
		if seen[start] {
			continue
		}
		seen[start] = true
		group := []int{}
		stack := []int{start}
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			group = append(group, i)
			for j := 0; j < n; j++ {
				if j != i && !seen[j] && m.Judged(i, j) {
					seen[j] = true
					stack = append(stack, j)
				}
			}
		}
		sort.Ints(group)
		groups = append(groups, group)
	}
	return groups
}
