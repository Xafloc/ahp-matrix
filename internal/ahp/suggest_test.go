package ahp

import "testing"

func TestSuggestionsFindsTheContradiction(t *testing.T) {
	m := consistent([]float64{0.4, 0.3, 0.2, 0.1})
	m.Set(0, 3, 0.25)
	a := Analyze(m)
	got := Suggestions(m, a.Weights, saaty9, 3)
	if len(got) != 3 {
		t.Fatalf("got %d suggestions, want 3", len(got))
	}
	top := got[0]
	if top.I != 0 || top.J != 3 {
		t.Errorf("top suggestion is pair (%d,%d), want (0,3)", top.I, top.J)
	}
	approx(t, "Current", top.Current, 0.25, 0)
	approx(t, "Suggested", top.Suggested, 1, 0)
	approx(t, "Deviation", top.Deviation, 1.195483, 1e-6)
	for i := 1; i < len(got); i++ {
		if got[i].Deviation > got[i-1].Deviation {
			t.Error("suggestions are not sorted by deviation")
		}
	}
}

func TestSuggestionsEmptyWhenConsistent(t *testing.T) {
	w := []float64{0.5, 0.3, 0.2}
	if got := Suggestions(consistent(w), w, saaty9, 3); len(got) != 0 {
		t.Errorf("got %d suggestions for a consistent matrix, want 0", len(got))
	}
}

func TestMeans(t *testing.T) {
	approx(t, "GeoMean reciprocal", GeoMean([]Weighted{{3, 1}, {1.0 / 3, 1}}), 1, 1e-12)
	approx(t, "GeoMean weighted", GeoMean([]Weighted{{4, 3}, {1, 1}}), 2.828427, 1e-6)
	approx(t, "GeoMean identical", GeoMean([]Weighted{{5, 1}, {5, 2}}), 5, 1e-12)
	approx(t, "ArithMean", ArithMean([]Weighted{{2, 1}, {8, 3}}), 6.5, 1e-12)
	approx(t, "GeoStdDev agree", GeoStdDev([]Weighted{{7, 1}, {7, 1}}), 1, 1e-12)
	approx(t, "GeoStdDev spread", GeoStdDev([]Weighted{{9, 1}, {1, 1}}), 3, 1e-12)
}
