package ahp

import (
	"math"
	"sort"
)

const (
	powerTolerance = 1e-12
	powerMaxIter   = 1000
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

// principal returns the normalized principal eigenvector of a non-negative,
// irreducible matrix with a positive diagonal, and its eigenvalue, using power
// iteration.
func principal(m Matrix) ([]float64, float64) {
	n := m.N()
	w := make([]float64, n)
	for i := range w {
		w[i] = 1 / float64(n)
	}
	next := make([]float64, n)
	lambda := 0.0
	for iter := 0; iter < powerMaxIter; iter++ {
		sum := 0.0
		for i := 0; i < n; i++ {
			s := 0.0
			for j := 0; j < n; j++ {
				s += m[i][j] * w[j]
			}
			next[i] = s
			sum += s
		}
		lambda = sum // w sums to 1, so the sum of Mw estimates λmax
		delta := 0.0
		for i := range next {
			next[i] /= sum
			delta = math.Max(delta, math.Abs(next[i]-w[i]))
		}
		w, next = next, w
		if delta < powerTolerance {
			break
		}
	}
	return w, lambda
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
