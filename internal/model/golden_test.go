package model

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the current code")

// goldenProjects are computed by Go and replayed by the web UI's tests
// (web/src/lib/sensitivity.test.ts), so the TypeScript copy of the
// sensitivity math stays identical to internal/ahp.
func goldenProjects() map[string]*Project {
	fixed := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	small := fixture()
	small.ID = "01GOLDENFIXTURE0000000000A"

	wide := New("Wide")
	wide.ID = "01GOLDENWIDE00000000000000"
	wide.Evaluators = []Evaluator{{ID: "e1", Name: "Ann", Weight: 1}, {ID: "e2", Name: "Bob", Weight: 2}}
	for i, name := range []string{"Atlas", "Birch", "Cedar", "Dune", "Ember"} {
		wide.Alternatives = append(wide.Alternatives, Alternative{ID: "a" + string(rune('1'+i)), Name: name, Color: "series-" + string(rune('1'+i))})
	}
	wide.Criteria = []Criterion{
		{ID: "t", Name: "Technical"},
		{ID: "t1", ParentID: "t", Name: "Speed", Method: MethodPairwise},
		{ID: "t2", ParentID: "t", Name: "Security", Method: MethodRating},
		{ID: "t3", ParentID: "t", Name: "Uptime", Method: MethodValue, Direction: DirHigher},
		{ID: "b", Name: "Business"},
		{ID: "b1", ParentID: "b", Name: "Price", Method: MethodValue, Direction: DirLower},
		{ID: "b2", ParentID: "b", Name: "Vendor", Method: MethodPairwise},
		{ID: "s", Name: "Support", Method: MethodRating},
	}
	scale := ScaleSaaty9.Allowed()
	pick := func(k int) float64 { return scale[(k*7+3)%len(scale)] }
	k := 0
	judge := func(ev, ctx string, items []string) {
		for i := range items {
			for j := i + 1; j < len(items); j++ {
				k++
				wide.Pairwise = append(wide.Pairwise, PairwiseJudgment{EvaluatorID: ev, Context: ctx, A: items[i], B: items[j], Value: pick(k)})
			}
		}
	}
	alts := []string{"a1", "a2", "a3", "a4", "a5"}
	for _, ev := range []string{"e1", "e2"} {
		judge(ev, CriteriaContext(""), []string{"t", "b", "s"})
		judge(ev, CriteriaContext("t"), []string{"t1", "t2", "t3"})
		judge(ev, CriteriaContext("b"), []string{"b1", "b2"})
		judge(ev, AltsContext("t1"), alts)
		judge(ev, AltsContext("b2"), alts)
		for i, a := range alts {
			wide.Ratings = append(wide.Ratings,
				Rating{EvaluatorID: ev, CriterionID: "t2", AlternativeID: a, Value: float64(1 + (i*3+k)%10)},
				Rating{EvaluatorID: ev, CriterionID: "s", AlternativeID: a, Value: float64(1 + (i*5+k)%10)})
		}
	}
	for i, a := range alts {
		wide.Values = append(wide.Values,
			Value{CriterionID: "t3", AlternativeID: a, Value: 99 + float64(i)*0.2},
			Value{CriterionID: "b1", AlternativeID: a, Value: 1000 + float64((i*37)%5)*250})
	}

	out := map[string]*Project{"fixture": small, "wide": wide}
	for _, p := range out {
		p.CreatedAt, p.UpdatedAt = fixed, fixed
		if err := p.Prepare(); err != nil {
			panic(err)
		}
	}
	return out
}

func TestGoldenResults(t *testing.T) {
	type run struct {
		p         *Project
		evaluator string
	}
	runs := map[string]run{}
	for name, p := range goldenProjects() {
		runs[name] = run{p, AllEvaluators}
		// One person's own results too: the judging page computes these alone.
		runs[name+"-"+p.Evaluators[0].ID] = run{p, p.Evaluators[0].ID}
	}
	for name, r := range runs {
		p := r.p
		res, err := Compute(p, r.evaluator)
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.MarshalIndent(struct {
			Project *Project `json:"project"`
			Results *Results `json:"results"`
		}{p, res}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, '\n')
		path := filepath.Join("testdata", "golden", name+".json")
		if *update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run `go test ./internal/model -run Golden -update` to create it)", err)
		}
		if diff := goldenDiff(got, want); diff != "" {
			t.Errorf("%s: results changed at %s; if that is intended, run `go test ./internal/model -run Golden -update`", path, diff)
		}
	}
}

// goldenDiff compares two JSON documents, allowing numbers to differ in their
// last bits: CPUs round differently (arm64 fuses multiply-adds), so golden
// files written on one machine must still pass on another. It returns the
// path of the first difference, or "" if there is none.
func goldenDiff(got, want []byte) string {
	var g, w any
	if json.Unmarshal(got, &g) != nil || json.Unmarshal(want, &w) != nil {
		return "(invalid JSON)"
	}
	return jsonDiff("$", g, w)
}

func jsonDiff(path string, g, w any) string {
	switch wv := w.(type) {
	case float64:
		gv, ok := g.(float64)
		if !ok || math.Abs(gv-wv) > 1e-9*math.Max(1, math.Max(math.Abs(gv), math.Abs(wv))) {
			return fmt.Sprintf("%s (%v, want %v)", path, g, w)
		}
	case map[string]any:
		gv, ok := g.(map[string]any)
		if !ok || len(gv) != len(wv) {
			return path
		}
		for k := range wv {
			if d := jsonDiff(path+"."+k, gv[k], wv[k]); d != "" {
				return d
			}
		}
	case []any:
		gv, ok := g.([]any)
		if !ok || len(gv) != len(wv) {
			return path
		}
		for i := range wv {
			if d := jsonDiff(fmt.Sprintf("%s[%d]", path, i), gv[i], wv[i]); d != "" {
				return d
			}
		}
	default:
		if fmt.Sprint(g) != fmt.Sprint(w) {
			return fmt.Sprintf("%s (%v, want %v)", path, g, w)
		}
	}
	return ""
}
