package ahp

import (
	"math"
	"sort"
)

// Suggestion flags a judgment that disagrees with the derived weights.
type Suggestion struct {
	I, J      int
	Current   float64 // M[I][J]
	Suggested float64 // w[I]/w[J] snapped to the allowed scale
	Deviation float64 // |ln(M[I][J]·w[J]/w[I])|
}

// Suggestions returns up to k judged pairs whose judgments deviate most from
// the weights w, most deviant first. Pairs with negligible deviation, and
// pairs whose nearest allowed value is the current one, are left out.
func Suggestions(m Matrix, w []float64, allowed []float64, k int) []Suggestion {
	var out []Suggestion
	for i := 0; i < m.N(); i++ {
		for j := i + 1; j < m.N(); j++ {
			if !m.Judged(i, j) {
				continue
			}
			dev := math.Abs(math.Log(m[i][j] * w[j] / w[i]))
			suggested := Snap(w[i]/w[j], allowed)
			if dev < 1e-9 || suggested == m[i][j] {
				continue // nothing to fix, or the scale has no better value to offer
			}
			out = append(out, Suggestion{I: i, J: j, Current: m[i][j], Suggested: suggested, Deviation: dev})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Deviation > out[b].Deviation })
	if len(out) > k {
		out = out[:k]
	}
	return out
}
