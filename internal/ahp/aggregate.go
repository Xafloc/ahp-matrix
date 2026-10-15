package ahp

import "math"

// Weighted is one evaluator's value together with that evaluator's weight.
type Weighted struct{ Value, Weight float64 }

// GeoMean returns the weighted geometric mean of positive values. It is how
// several evaluators' pairwise judgments are combined (AIJ).
func GeoMean(vs []Weighted) float64 {
	num, den := 0.0, 0.0
	for _, v := range vs {
		num += v.Weight * math.Log(v.Value)
		den += v.Weight
	}
	return math.Exp(num / den)
}

// ArithMean returns the weighted arithmetic mean.
func ArithMean(vs []Weighted) float64 {
	num, den := 0.0, 0.0
	for _, v := range vs {
		num += v.Weight * v.Value
		den += v.Weight
	}
	return num / den
}

// GeoStdDev returns the weighted geometric standard deviation of positive
// values: 1 means full agreement, and larger values mean more disagreement.
func GeoStdDev(vs []Weighted) float64 {
	lg := math.Log(GeoMean(vs))
	num, den := 0.0, 0.0
	for _, v := range vs {
		d := math.Log(v.Value) - lg
		num += v.Weight * d * d
		den += v.Weight
	}
	return math.Exp(math.Sqrt(num / den))
}
