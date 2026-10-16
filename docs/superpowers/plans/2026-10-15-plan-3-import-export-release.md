# AHP Matrix — Plan 3: Import, export and distribution

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:**
- Read workbooks made with the original "Priority Matrix" Excel template.
- Export results to Excel and CSV.
- Let people judge remotely: export one person's copy, then merge it back.
- Ship sample decisions and templates.
- Clear the Plan 2 review's deferred minors.
- Add golangci-lint, GoReleaser and a release workflow.

**Architecture:**
- **`internal/xlsx`** (excelize) reads the legacy template, and writes results workbooks and the table the CSV export shares.
- **`model.Packet` / `model.Merge`** keep merging pure and testable. The owner's structure always wins, and on error nothing changes.
- **`internal/samples`** embeds the sample and template JSON.
- **`internal/server/exchange.go`** exposes these over HTTP.
- **The UI** adds templates and examples to the library, spreadsheet import with a summary, Excel/CSV downloads on Results, and "Send their copy" / "Merge a returned file" under People.

**Tech Stack:** Go 1.24 + github.com/xuri/excelize/v2 v2.10.1 (the newest that still allows Go 1.24); Svelte UI from Plan 2; golangci-lint v2.14.0; GoReleaser v2.18.3; GitHub Actions (checkout/setup-go/setup-node/upload-artifact v7, golangci-lint-action v9, goreleaser-action v7).

**Spec:** `docs/superpowers/specs/2026-10-15-ahp-matrix-design.md` §3 (releases), §9 (import, export, merge), §11 (bundled content). It builds on Plans 1 and 2.

Everything below was built and checked before this plan was written:
- golangci-lint: 0 issues
- `go test -race ./...` and Vitest (61 tests)
- Playwright (21 end-to-end tests)
- a GoReleaser snapshot of all six platform archives, with the macOS binary started and checked
- an import of the real 2006 template (3 options, 23 criteria, 322 comparisons, nothing skipped). That file isn't committed.

## Global Constraints

- **Go floor stays at 1.24:** excelize is pinned to v2.10.1 (v2.11 needs Go 1.25). No CGO.
- **Clean on every change:** `make lint`, `make test`, `make web-check`, `make e2e`, and golangci-lint (`.golangci.yml`).
- **Legacy layout** (spec §9):
  - options in Summary!C19:C28, criteria in Summary!D7:AB7
  - lower triangles of Criteria Summary!D5:AB29 and of each "Criterion N"!D5:M14, where a cell compares its row with its column
  - notes in "Criterion N"!R5
  - the 10/5/1 scale, one evaluator named "Imported"
- **Merging:** only people's judgments come across. The owner's options, criteria, measured values and names win. Asking before replacing anyone is the default.
- **Same rules as before:** API errors use `{"error":{"code","message","details"}}`, UI copy follows Plan 2's rules, and the real 2006 workbook is never committed.

## Spec refinements made while planning

- **Results now include each comparison matrix:** `MatrixReport.matrix`, holding the combined values, with 0 for unjudged pairs. The Excel export's per-matrix sheets (spec §9: "one sheet per pairwise matrix") need it. The golden files were regenerated on purpose.
- **Packets** come from `export.ahp.json?evaluator=<id>`, named `<decision>-for-<person>.ahp.json`.
- **Merging** uses `POST /api/projects/{id}/merge[?existing=replace|skip]`. With no choice and someone already present, it returns 409 `evaluator-exists` with `details.evaluators`, and the UI asks.
- **Importing a spreadsheet shows a summary first** (counts and skipped cells, with an "Open it" button), instead of jumping straight into the decision.
- **Weights apply when you finish editing the field** (on change), so a half-typed "0." isn't turned into 1 mid-keystroke. Blank or 0 means 1, as on the server.
- **Homebrew uses GoReleaser's `homebrew_casks`** (`brews` is deprecated), with a quarantine-removal hook because the binaries aren't notarized. The upload is skipped while `HOMEBREW_TAP_GITHUB_TOKEN` is unset.
- **Clean-up done along the way:**
  - prune's validity checks were extracted into `known`, so `Merge` reuses them
  - fix suggestions already break ties deterministically (done in Plan 2)
  - golangci-lint's error-wrapping, exit-after-defer and character-check findings were fixed in code; harmless `Close`/`Fprintf` returns were excluded in `.golangci.yml`

## Review Focus

1. **A merge that fails partway** (an invalid judgment in the returned file). Expected: the decision is unchanged. *(Task 4: `TestAFailedMergeChangesNothing`)*
2. **A returned file that is a different decision.** Expected: a clear refusal, not a silent mix. *(Task 4 `TestMergeRefusesAnotherProject`, Task 6 `different-project`)*
3. **A template spreadsheet with stray text in a judgment cell, or blank names.** Expected: skipped and reported, never imported as nonsense. *(Task 1 `TestImportLegacy`, Task 8 e2e summary)*
4. **Sample or template ids that look like paths** (`../../etc/passwd`). Expected: 404, never a file read. *(Task 6 `TestSamplesAndTemplates`)*
5. **Excel sheet names from long or odd criterion names** (more than 31 characters, `[]:*?/\`, duplicates). Expected: valid, unique sheets. *(Task 3 `TestSheetNamesAreValidAndUnique`)*

---

### Task 1: Legacy template import

**Files:**
- Modify: `go.mod`, `go.sum` (excelize v2.10.1)
- Create: `internal/xlsx/legacy.go`
- Test: `internal/xlsx/legacy_test.go`

**Interfaces:**
- Consumes: `model.New`, `model.CriteriaContext`, `model.AltsContext`, `model.ScaleClassic5`.
- Produces: `xlsx.ImportLegacy(r io.Reader, name string) (*model.Project, *ImportReport, error)`; `type ImportReport {Alternatives, Criteria, Judgments int; Skipped []string}`; `ErrNotWorkbook`, `ErrNotTemplate`. Test helper `legacyWorkbook(t) *bytes.Buffer`, which builds a synthetic template with made-up data and is reused by Tasks 3 and 8.

- [ ] **Step 1: Add excelize**

```bash
mkdir -p internal/xlsx
go get github.com/xuri/excelize/v2@v2.10.1
```
Expected: `go.mod` keeps `go 1.24.0`.

- [ ] **Step 2: Write the failing test `internal/xlsx/legacy_test.go`** (complete file)

```go
package xlsx

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// legacyWorkbook builds a workbook laid out like the original "Priority
// Matrix" template, filled with made-up data:
//
//	options Alpha, Beta, Gamma; criteria Price, Speed, Support
//	criteria judgments (row over column): Speed/Price = 5, Support/Price = 1/5, Support/Speed = 1/10
//	Criterion 1 (Price): Beta/Alpha = 10, Gamma/Alpha = 5, Gamma/Beta = 0.2, notes "From the 2024 quotes"
//	Criterion 2 (Speed): only Beta/Alpha = 1 (the rest left blank)
//	Criterion 3 (Support): Gamma/Beta = "n/a" (not a number)
func legacyWorkbook(t *testing.T) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	set := func(sheet, cell string, v any) {
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetSheetName("Sheet1", "Summary"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Criteria Summary", "Notes Summary", "Criterion 1", "Criterion 2", "Criterion 3", "Criterion 4"} {
		if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
	}
	set("Summary", "A1", "Prioritization Matrix")
	for k, name := range []string{"Alpha", "Beta", "Gamma"} {
		set("Summary", fmt.Sprintf("C%d", 19+k), name)
	}
	for k, name := range []string{"Price", "Speed", "Support"} {
		col, _ := excelize.ColumnNumberToName(4 + k)
		set("Summary", col+"7", name)
	}
	// Criteria Summary: row 5+i, column D+j, holds criterion i over criterion j (j < i).
	set("Criteria Summary", "D6", 5)
	set("Criteria Summary", "D7", 0.2)
	set("Criteria Summary", "E7", 0.1)
	set("Criteria Summary", "D8", 1) // criterion 4 has no name: ignored
	// Criterion sheets: row 5+i, column D+j, holds option i over option j.
	set("Criterion 1", "D6", 10)
	set("Criterion 1", "D7", 5)
	set("Criterion 1", "E7", 0.2)
	set("Criterion 1", "R5", "From the 2024 quotes")
	set("Criterion 2", "D6", 1)
	set("Criterion 3", "E7", "n/a")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func judgment(p *model.Project, ctx, a, b string) (float64, bool) {
	for _, j := range p.Pairwise {
		if j.Context == ctx && j.A == a && j.B == b {
			return j.Value, true
		}
	}
	return 0, false
}

func TestImportLegacy(t *testing.T) {
	p, rep, err := ImportLegacy(legacyWorkbook(t), "Vendor shortlist")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Vendor shortlist" || p.Scale != model.ScaleClassic5 {
		t.Errorf("name %q, scale %q", p.Name, p.Scale)
	}
	if len(p.Evaluators) != 1 || p.Evaluators[0].Name != "Imported" {
		t.Errorf("evaluators = %+v", p.Evaluators)
	}
	names := func(n int, get func(i int) string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = get(i)
		}
		return out
	}
	if got := names(len(p.Alternatives), func(i int) string { return p.Alternatives[i].Name }); fmt.Sprint(got) != "[Alpha Beta Gamma]" {
		t.Errorf("alternatives = %v", got)
	}
	if got := names(len(p.Criteria), func(i int) string { return p.Criteria[i].Name }); fmt.Sprint(got) != "[Price Speed Support]" {
		t.Errorf("criteria = %v", got)
	}
	if p.Criteria[0].Notes != "From the 2024 quotes" {
		t.Errorf("notes = %q", p.Criteria[0].Notes)
	}

	c := func(i int) string { return p.Criteria[i].ID }
	a := func(i int) string { return p.Alternatives[i].ID }
	for _, tc := range []struct {
		ctx, a, b string
		want      float64
	}{
		{model.CriteriaContext(""), c(1), c(0), 5},
		{model.CriteriaContext(""), c(2), c(0), 0.2},
		{model.CriteriaContext(""), c(2), c(1), 0.1},
		{model.AltsContext(c(0)), a(1), a(0), 10},
		{model.AltsContext(c(0)), a(2), a(0), 5},
		{model.AltsContext(c(0)), a(2), a(1), 0.2},
		{model.AltsContext(c(1)), a(1), a(0), 1},
	} {
		if got, ok := judgment(p, tc.ctx, tc.a, tc.b); !ok || got != tc.want {
			t.Errorf("%s %s/%s = %v (found %v), want %v", tc.ctx, tc.a, tc.b, got, ok, tc.want)
		}
	}
	if len(p.Pairwise) != 7 {
		t.Errorf("got %d judgments, want 7", len(p.Pairwise))
	}
	if rep.Alternatives != 3 || rep.Criteria != 3 || rep.Judgments != 7 {
		t.Errorf("report = %+v", rep)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0] != "Criterion 3!E7: “n/a” is not a number" {
		t.Errorf("skipped = %q", rep.Skipped)
	}
	if err := p.Prepare(); err != nil {
		t.Errorf("imported project doesn't validate: %v", err)
	}
}

func TestImportLegacyRejectsOtherFiles(t *testing.T) {
	if _, _, err := ImportLegacy(bytes.NewReader([]byte("not a zip")), "x"); !errors.Is(err, ErrNotWorkbook) {
		t.Errorf("garbage: err = %v, want ErrNotWorkbook", err)
	}
	f := excelize.NewFile()
	var buf bytes.Buffer
	_ = f.Write(&buf)
	if _, _, err := ImportLegacy(&buf, "x"); !errors.Is(err, ErrNotTemplate) {
		t.Errorf("blank workbook: err = %v, want ErrNotTemplate", err)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/xlsx`
Expected: FAIL — `undefined: ImportLegacy`.

- [ ] **Step 4: Write `internal/xlsx/legacy.go`** (complete file)

```go
// Package xlsx reads the original "Priority Matrix" Excel template and
// writes results workbooks.
package xlsx

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

var (
	ErrNotWorkbook = errors.New("not an Excel workbook (.xlsx)")
	ErrNotTemplate = errors.New("this workbook isn't the Priority Matrix template (it needs Summary and Criteria Summary sheets)")
)

// The template's fixed layout.
const (
	maxAlternatives = 10 // Summary!C19:C28
	maxCriteria     = 25 // Summary!D7:AB7
	maxSkipped      = 20 // problems listed in the report; the rest are only counted
)

// ImportReport says what an import found.
type ImportReport struct {
	Alternatives int      `json:"alternatives"`
	Criteria     int      `json:"criteria"`
	Judgments    int      `json:"judgments"`
	Skipped      []string `json:"skipped"` // cells that couldn't be used, e.g. "Criterion 3!E7: “n/a” is not a number"
}

// ImportLegacy reads a workbook made from the "Priority Matrix" template:
// options from Summary!C19:C28, criteria from Summary!D7:AB7, criteria
// comparisons from the lower triangle of Criteria Summary!D5:AB29, option
// comparisons from the lower triangle of each "Criterion N"!D5:M14, and notes
// from "Criterion N"!R5. Every cell in a lower triangle compares its row with
// its column. The project uses the template's 10/5/1 scale and one evaluator
// named "Imported".
func ImportLegacy(r io.Reader, name string) (*model.Project, *ImportReport, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrNotWorkbook, err)
	}
	defer f.Close()
	for _, sheet := range []string{"Summary", "Criteria Summary"} {
		if idx, err := f.GetSheetIndex(sheet); err != nil || idx < 0 {
			return nil, nil, ErrNotTemplate
		}
	}

	p := model.New(name)
	p.Scale = model.ScaleClassic5
	ev := p.Evaluators[0].ID
	p.Evaluators[0].Name = "Imported"
	rep := &ImportReport{Skipped: []string{}}
	skipped := 0

	text := func(sheet, cell string) string {
		v, _ := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
		return strings.TrimSpace(v)
	}
	// number reads a judgment: blank cells are unanswered, anything else must be a positive number.
	number := func(sheet, cell string) (float64, bool) {
		raw := text(sheet, cell)
		if raw == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 {
			if skipped++; len(rep.Skipped) < maxSkipped {
				rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s!%s: “%s” is not a number", sheet, cell, raw))
			}
			return 0, false
		}
		return v, true
	}
	cell := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}

	alts := make([]string, maxAlternatives) // index → id, "" where the template row is blank
	for k := range alts {
		if n := text("Summary", cell(3, 19+k)); n != "" {
			a := model.Alternative{ID: fmt.Sprintf("a%d", k+1), Name: n}
			if len(p.Alternatives) < 8 {
				a.Color = fmt.Sprintf("series-%d", len(p.Alternatives)+1)
			}
			alts[k] = a.ID
			p.Alternatives = append(p.Alternatives, a)
		}
	}
	crits := make([]string, maxCriteria)
	for k := range crits {
		if n := text("Summary", cell(4+k, 7)); n != "" {
			c := model.Criterion{ID: fmt.Sprintf("c%d", k+1), Name: n, Method: model.MethodPairwise}
			c.Notes = text(fmt.Sprintf("Criterion %d", k+1), "R5")
			crits[k] = c.ID
			p.Criteria = append(p.Criteria, c)
		}
	}

	// readTriangle adds a judgment for every filled lower-triangle cell whose
	// row and column both name an item.
	readTriangle := func(sheet, context string, ids []string) {
		for i := range ids {
			for j := 0; j < i; j++ {
				if ids[i] == "" || ids[j] == "" {
					continue
				}
				if v, ok := number(sheet, cell(4+j, 5+i)); ok {
					p.Pairwise = append(p.Pairwise, model.PairwiseJudgment{EvaluatorID: ev, Context: context, A: ids[i], B: ids[j], Value: v})
				}
			}
		}
	}
	readTriangle("Criteria Summary", model.CriteriaContext(""), crits)
	for k, id := range crits {
		if id == "" {
			continue
		}
		sheet := fmt.Sprintf("Criterion %d", k+1)
		if idx, err := f.GetSheetIndex(sheet); err == nil && idx >= 0 {
			readTriangle(sheet, model.AltsContext(id), alts)
		}
	}

	if extra := skipped - len(rep.Skipped); extra > 0 {
		rep.Skipped = append(rep.Skipped, fmt.Sprintf("and %d more", extra))
	}
	rep.Alternatives, rep.Criteria, rep.Judgments = len(p.Alternatives), len(p.Criteria), len(p.Pairwise)
	return p, rep, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/xlsx`
Expected: PASS

- [ ] **Step 6: Check against the real template (not committed)**

Run a throwaway test that imports `~/Documents/Templates/Priority Matrix - 25.xlsx`. Expected: 3 options, 23 criteria, 322 judgments, nothing skipped, and `Prepare()` succeeds. Delete the throwaway test.

- [ ] **Step 7: Commit**

```bash
make lint && make test
git add go.mod go.sum internal/xlsx/legacy.go internal/xlsx/legacy_test.go
git commit -m "feat(xlsx): import workbooks made with the Priority Matrix template"
```

---

### Task 2: Each comparison matrix in the results

**Files:**
- Modify: `internal/model/compute.go` (MatrixReport.Matrix)
- Test: `internal/model/compute_test.go` (adds `TestComputeReportsTheCombinedMatrix`)
- Modify: `internal/model/testdata/golden/*.json` (regenerated)
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/components/compare.test.ts` (report gains `matrix: []`)

**Interfaces:**
- Consumes: `computer.compare` (Plan 1).
- Produces: `MatrixReport.Matrix [][]float64` (JSON `matrix`), the combined matrix in `Items` order with 0 where unjudged; `MatrixReport.matrix: number[][]` in TS. `types.ts` here is its final Plan 3 form, which also adds `StarterEntry`, `ImportReport` and `MergeReport`, used by Task 8.

- [ ] **Step 1: Write the failing test `internal/model/compute_test.go`** (complete file)

```go
package model

import (
	"errors"
	"math"
	"testing"
)

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (±%g)", name, got, want, tol)
	}
}

func scores(r *Results) []float64 {
	out := make([]float64, len(r.Scores))
	for i, s := range r.Scores {
		out[i] = s.Score
	}
	return out
}

func approxScores(t *testing.T, r *Results, want []float64) {
	t.Helper()
	got := scores(r)
	if len(got) != len(want) {
		t.Fatalf("got %d scores, want %d", len(got), len(want))
	}
	for i := range want {
		approx(t, r.Scores[i].ID, got[i], want[i], 1e-6)
	}
}

func hasIssue(r *Results, code string) bool {
	for _, i := range r.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func criterion(r *Results, id string) CriterionResult {
	for _, c := range r.Criteria {
		if c.ID == id {
			return c
		}
	}
	return CriterionResult{}
}

func TestComputeSingleEvaluator(t *testing.T) {
	r, err := Compute(fixture(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	approxScores(t, r, []float64{0.450893, 0.272321, 0.276786})
	if r.Scores[0].Rank != 1 || r.Scores[2].Rank != 2 || r.Scores[1].Rank != 3 {
		t.Errorf("ranks = %+v", r.Scores)
	}
	approx(t, "completeness", r.Completeness, 1, 0)
	if !r.Consistent || len(r.Issues) != 0 {
		t.Errorf("Consistent = %v, issues = %+v", r.Consistent, r.Issues)
	}
	if len(r.Matrices) != 3 {
		t.Errorf("got %d matrix reports, want 3 (root, Technical, Performance)", len(r.Matrices))
	}
}

func TestComputeWeightsFollowTheHierarchy(t *testing.T) {
	r, _ := Compute(fixture(), "e1")
	wantOrder := []string{"c1", "c2", "c3", "c4"}
	for i, id := range wantOrder {
		if r.Criteria[i].ID != id {
			t.Fatalf("criteria order = %v..., want %v", r.Criteria[i].ID, wantOrder)
		}
	}
	approx(t, "c1 global", criterion(r, "c1").Global, 0.75, 1e-9)
	approx(t, "c2 global", criterion(r, "c2").Global, 0.375, 1e-9)
	approx(t, "c3 local", criterion(r, "c3").Local, 0.5, 1e-9)
	approx(t, "c4 global", criterion(r, "c4").Global, 0.25, 1e-9)
	sub := criterion(r, "c1").Subtree
	approx(t, "c1 subtree a3", sub[2], 0.321429, 1e-6)
}

func TestComputeStability(t *testing.T) {
	r, _ := Compute(fixture(), "e1")
	if s := criterion(r, "c1").Stability; s != nil {
		t.Errorf("c1 stability = %v, want none (Alpha leads at every weight)", *s)
	}
	s := criterion(r, "c2").Stability
	if s == nil {
		t.Fatal("c2 stability = nil, want 13/38")
	}
	approx(t, "c2 stability", *s, 13.0/38, 1e-9)
}

func TestComputeCombinesEvaluators(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c4", B: "c1", Value: 3})
	p.Ratings = append(p.Ratings, Rating{EvaluatorID: "e2", CriterionID: "c3", AlternativeID: "a1", Value: 10})
	r, err := Compute(p, AllEvaluators)
	if err != nil {
		t.Fatal(err)
	}
	// Ann says Technical is 3× Cost and Bob says the reverse: the geometric mean is 1.
	approx(t, "c1 local", criterion(r, "c1").Local, 0.5, 1e-9)
	approxScores(t, r, []float64{0.511905, 0.269841, 0.218254})
	if len(r.Disagreements) != 1 {
		t.Fatalf("got %d disagreements, want 1", len(r.Disagreements))
	}
	d := r.Disagreements[0]
	approx(t, "spread", d.Spread, 3, 1e-9)
	approx(t, "Bob's value", d.Values["e2"], 1.0/3, 1e-12)

	// Bob alone has only judged one pair and one rating.
	rb, _ := Compute(p, "e2")
	if rb.Completeness >= 0.5 || len(rb.Disagreements) != 0 {
		t.Errorf("Bob: completeness %v, %d disagreements", rb.Completeness, len(rb.Disagreements))
	}
}

func TestComputeIncompleteButConnected(t *testing.T) {
	p := fixture()
	p.Pairwise = p.Pairwise[:4] // drop Beta vs Gamma on Performance
	r, _ := Compute(p, "e1")
	approxScores(t, r, []float64{0.450893, 0.272321, 0.276786}) // e1 is consistent, so nothing changes
	approx(t, "completeness", r.Completeness, 10.0/11, 1e-12)
	m := r.Matrices[2]
	if m.Context != AltsContext("c2") || m.Complete || m.Missing != 1 {
		t.Errorf("performance matrix = %+v", m)
	}
}

func TestComputeDisconnectedGroupFallsBackToEqualWeights(t *testing.T) {
	p := fixture()
	p.Pairwise = p.Pairwise[1:] // drop Technical vs Cost
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueGroupDisconnected) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueGroupDisconnected)
	}
	approx(t, "c1 local", criterion(r, "c1").Local, 0.5, 1e-12)
	if r.Matrices[0].Connected || len(r.Matrices[0].Components) != 2 {
		t.Errorf("root matrix = %+v", r.Matrices[0])
	}
}

func TestComputeMissingRatingGetsTheAverage(t *testing.T) {
	p := fixture()
	p.Ratings = p.Ratings[:2] // Gamma unrated; Alpha and Beta both have 5
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueRatingsMissing) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueRatingsMissing)
	}
	sub := criterion(r, "c3").Subtree
	for a := range sub {
		approx(t, "security priority", sub[a], 1.0/3, 1e-12)
	}
}

func TestComputeInvalidValues(t *testing.T) {
	p := fixture()
	p.Values[0].Value = 0 // zero cost can't be inverted
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueValuesInvalid) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueValuesInvalid)
	}
	approx(t, "cost priority", criterion(r, "c4").Subtree[0], 1.0/3, 1e-12)

	p.Criteria[3].Direction = DirHigher
	p.Values[0].Value = -5
	r, _ = Compute(p, "e1")
	if !hasIssue(r, IssueValuesInvalid) {
		t.Error("negative value with higher-is-better: want values-invalid")
	}
}

func TestComputeSuggestsFixesForContradictions(t *testing.T) {
	p := fixture()
	p.Pairwise[3].Value = 0.25 // Alpha vs Gamma reversed: contradicts the other two
	r, _ := Compute(p, "e1")
	m := r.Matrices[2]
	if m.Consistent || r.Consistent {
		t.Errorf("want an inconsistent Performance matrix, got CR %v", m.CR)
	}
	// In a three-item cycle every pair deviates equally, so the reversed pair
	// must be among the suggestions rather than necessarily first.
	for _, s := range m.Suggestions {
		if s.A == "a1" && s.B == "a3" && s.Current == 0.25 {
			return
		}
	}
	t.Errorf("suggestions = %+v, want one for a1 vs a3", m.Suggestions)
}

func TestComputeEdgeCases(t *testing.T) {
	if _, err := Compute(fixture(), "nobody"); !errors.Is(err, ErrUnknownEvaluator) {
		t.Errorf("err = %v, want ErrUnknownEvaluator", err)
	}

	empty := New("Empty")
	r, err := Compute(empty, AllEvaluators)
	if err != nil || len(r.Scores) != 0 || r.Completeness != 1 {
		t.Errorf("empty project: %+v, %v", r, err)
	}

	noCriteria := New("No criteria")
	noCriteria.Alternatives = []Alternative{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	r, _ = Compute(noCriteria, AllEvaluators)
	if !hasIssue(r, IssueNoCriteria) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueNoCriteria)
	}

	one := fixture()
	one.Alternatives = one.Alternatives[:1]
	r, err = Compute(one, "e1")
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "single alternative", r.Scores[0].Score, 1, 1e-12)
}

func TestPrepareKeepsJudgmentsWhenALeafGainsChildren(t *testing.T) {
	p := fixture()
	// Nest a new criterion under Cost, then save: Cost is no longer a leaf.
	p.Criteria = append(p.Criteria, Criterion{ID: "c5", ParentID: "c4", Name: "Licence"})
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if len(p.Values) != 3 {
		t.Fatalf("nesting dropped Cost's values: %d left, want 3", len(p.Values))
	}
	// Un-nest it again: Cost's values are still there to use.
	p.Criteria = p.Criteria[:4]
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	r, _ := Compute(p, "e1")
	if hasIssue(r, IssueValuesMissing) {
		t.Errorf("Cost lost its values after un-nesting: %+v", r.Issues)
	}
}

func TestComputeTinyLowerIsBetterValueStaysFinite(t *testing.T) {
	p := fixture()
	p.Values[0].Value = 1e-310 // valid input, but 1/v overflows to +Inf
	r, _ := Compute(p, "e1")
	if !hasIssue(r, IssueValuesInvalid) {
		t.Errorf("issues = %+v, want %s", r.Issues, IssueValuesInvalid)
	}
	for _, s := range r.Scores {
		if math.IsNaN(s.Score) || math.IsInf(s.Score, 0) {
			t.Fatalf("score %v is not finite", s.Score)
		}
	}
}

func TestComputeReportsTheCombinedMatrix(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c4", B: "c1", Value: 3})
	r, _ := Compute(p, AllEvaluators)
	m := r.Matrices[0].Matrix // root: Technical vs Cost; Ann 3, Bob 1/3 → geometric mean 1
	if len(m) != 2 || m[0][0] != 1 {
		t.Fatalf("matrix = %v", m)
	}
	approx(t, "combined", m[0][1], 1, 1e-12)
	perf := r.Matrices[2].Matrix // Performance: a1/a2 = 2, a1/a3 = 4, a2/a3 = 2
	approx(t, "a3 over a1", perf[2][0], 0.25, 1e-12)

	p.Pairwise = p.Pairwise[:4] // leave Beta vs Gamma unjudged
	r, _ = Compute(p, "e1")
	if got := r.Matrices[2].Matrix[1][2]; got != 0 {
		t.Errorf("unjudged cell = %v, want 0", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/model -run CombinedMatrix`
Expected: FAIL — `r.Matrices[0].Matrix undefined`.

- [ ] **Step 3: Write `internal/model/compute.go`** (complete file)

```go
package model

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/Xafloc/ahp-matrix/internal/ahp"
)

// AllEvaluators asks Compute to combine every evaluator's judgments.
const AllEvaluators = "all"

// ErrUnknownEvaluator means Compute was asked for an evaluator the project lacks.
var ErrUnknownEvaluator = errors.New("unknown evaluator")

// Issue codes reported in Results.Issues.
const (
	IssueNoCriteria        = "no-criteria"        // there is nothing to score against
	IssueGroupDisconnected = "group-disconnected" // a sibling group's comparisons don't link every criterion; weights treated as equal
	IssueAltsDisconnected  = "alts-disconnected"  // a leaf's comparisons don't link every alternative; treated as equal
	IssueRatingsMissing    = "ratings-missing"    // some alternatives are unrated; they get the average rating
	IssueValuesMissing     = "values-missing"     // some alternatives have no value; they get the average value
	IssueValuesInvalid     = "values-invalid"     // values can't be turned into priorities; treated as equal
)

const (
	maxSuggestions   = 3
	maxDisagreements = 10
)

// Results is everything needed to show a project's outcome.
type Results struct {
	Evaluator     string            `json:"evaluator"` // AllEvaluators or an evaluator ID
	Scores        []AltScore        `json:"scores"`    // in project order
	Criteria      []CriterionResult `json:"criteria"`  // parents before children
	Matrices      []MatrixReport    `json:"matrices"`  // one per pairwise comparison group
	Issues        []Issue           `json:"issues"`
	Disagreements []Disagreement    `json:"disagreements"` // most contested pairs first
	Completeness  float64           `json:"completeness"`  // share of required judgments made, 0–1
	Consistent    bool              `json:"consistent"`    // every connected matrix is within its threshold
}

// AltScore is an alternative's overall score (all scores sum to 1) and rank (1 is best).
type AltScore struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
	Rank  int     `json:"rank"`
}

// CriterionResult gives a criterion's weights and the inputs for sensitivity
// analysis: varying this criterion's local weight changes overall scores
// linearly (see ahp.SensitivityLine).
type CriterionResult struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parentId,omitempty"`
	Local     float64   `json:"local"`     // weight among siblings
	Global    float64   `json:"global"`    // share of the whole decision
	Subtree   []float64 `json:"subtree"`   // alternatives' scores within this criterion, in project order
	Stability *float64  `json:"stability"` // smallest change to Local that changes the winner; null if none
}

// MatrixReport describes one pairwise comparison group.
type MatrixReport struct {
	Context     string           `json:"context"`
	Items       []string         `json:"items"` // criterion or alternative IDs, in matrix order
	Weights     []float64        `json:"weights"`
	CR          float64          `json:"cr"`
	Threshold   float64          `json:"threshold"`
	Consistent  bool             `json:"consistent"`
	Complete    bool             `json:"complete"`
	Connected   bool             `json:"connected"`
	Missing     int              `json:"missing"`
	Pairs       int              `json:"pairs"`
	Components  [][]string       `json:"components,omitempty"` // linked groups, when not connected
	Suggestions []PairSuggestion `json:"suggestions"`
	// Matrix is the combined comparison matrix, rows and columns in Items
	// order: Matrix[i][j] is how strongly item i is preferred over item j,
	// 0 where the pair hasn't been judged.
	Matrix [][]float64 `json:"matrix"`
}

// PairSuggestion proposes a value for a judgment that contradicts the others.
type PairSuggestion struct {
	A         string  `json:"a"`
	B         string  `json:"b"`
	Current   float64 `json:"current"` // how strongly A is preferred over B now
	Suggested float64 `json:"suggested"`
}

// Issue is a problem that makes the results less trustworthy.
type Issue struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	CriterionID string `json:"criterionId,omitempty"`
	Context     string `json:"context,omitempty"`
}

// Disagreement is a pair that evaluators judged differently.
type Disagreement struct {
	Context string             `json:"context"`
	A       string             `json:"a"`
	B       string             `json:"b"`
	Spread  float64            `json:"spread"` // geometric standard deviation; 1 means full agreement
	Values  map[string]float64 `json:"values"` // evaluator ID → how strongly A is preferred over B
}

// Compute derives weights, scores and diagnostics for a prepared project
// (see Prepare), combining all evaluators or using one evaluator's judgments.
func Compute(p *Project, evaluator string) (*Results, error) {
	c := &computer{
		p:       p,
		weights: map[string]float64{},
		res: &Results{
			Evaluator: evaluator, Scores: []AltScore{}, Criteria: []CriterionResult{},
			Matrices: []MatrixReport{}, Issues: []Issue{}, Disagreements: []Disagreement{},
		},
	}
	if evaluator == AllEvaluators {
		for _, e := range p.Evaluators {
			c.weights[e.ID] = e.Weight
		}
	} else {
		for _, e := range p.Evaluators {
			if e.ID == evaluator {
				c.weights[e.ID] = 1
			}
		}
		if len(c.weights) == 0 {
			return nil, fmt.Errorf("%w %q", ErrUnknownEvaluator, evaluator)
		}
	}
	for _, a := range p.Alternatives {
		c.altIDs = append(c.altIDs, a.ID)
	}
	return c.run()
}

type computer struct {
	p        *Project
	weights  map[string]float64 // evaluator ID → weight, for the evaluators being combined
	altIDs   []string
	res      *Results
	required int // judgments needed for a complete project
	answered int // judgments made
}

type vote struct {
	evaluator string
	value     float64
	weight    float64
}

func (c *computer) run() (*Results, error) {
	order, parent := c.hierarchy()
	nAlt := len(c.altIDs)
	if len(order) == 0 && nAlt > 0 {
		c.issue(Issue{Code: IssueNoCriteria, Message: "Add criteria to score the alternatives against."})
	}

	tree := ahp.Tree{Parent: parent, Local: make([]float64, len(order)), Leaf: make([][]float64, len(order))}
	isParent := make([]bool, len(order))
	for _, pi := range parent {
		if pi >= 0 {
			isParent[pi] = true
		}
	}

	// Local weights, one sibling group at a time, top level first.
	groups := map[int][]int{} // parent index (-1 for top level) → child indices
	var groupOrder []int
	for i, pi := range parent {
		if _, seen := groups[pi]; !seen {
			groupOrder = append(groupOrder, pi)
		}
		groups[pi] = append(groups[pi], i)
	}
	for _, pi := range groupOrder {
		members := groups[pi]
		ids := make([]string, len(members))
		for k, i := range members {
			ids[k] = order[i].ID
		}
		parentID := ""
		if pi >= 0 {
			parentID = order[pi].ID
		}
		w := c.compare(CriteriaContext(parentID), ids, IssueGroupDisconnected, parentID)
		for k, i := range members {
			tree.Local[i] = w[k]
		}
	}

	// Alternatives' local priorities on each leaf.
	for i, cr := range order {
		if isParent[i] {
			continue
		}
		switch {
		case nAlt == 0:
			tree.Leaf[i] = []float64{}
		case cr.Method == MethodRating:
			tree.Leaf[i] = c.ratingPriorities(cr)
		case cr.Method == MethodValue:
			tree.Leaf[i] = c.valuePriorities(cr)
		default:
			tree.Leaf[i] = c.compare(AltsContext(cr.ID), c.altIDs, IssueAltsDisconnected, cr.ID)
		}
	}

	syn, err := ahp.Synthesize(tree, nAlt)
	if err != nil {
		return nil, err
	}
	for a, id := range c.altIDs {
		c.res.Scores = append(c.res.Scores, AltScore{ID: id, Score: syn.Scores[a], Rank: rank(syn.Scores, a)})
	}
	for i, cr := range order {
		r := CriterionResult{ID: cr.ID, ParentID: cr.ParentID, Local: tree.Local[i], Global: syn.Global[i], Subtree: syn.Subtree[i]}
		if line, ok := ahp.SensitivityLine(tree, syn, i); ok && nAlt >= 2 {
			if d, found := line.Stability(tree.Local[i]); found {
				r.Stability = &d
			}
		}
		c.res.Criteria = append(c.res.Criteria, r)
	}

	c.res.Completeness = 1
	if c.required > 0 {
		c.res.Completeness = float64(c.answered) / float64(c.required)
	}
	c.res.Consistent = true
	for _, m := range c.res.Matrices {
		if m.Connected && !m.Consistent {
			c.res.Consistent = false
		}
	}
	d := c.res.Disagreements
	sort.SliceStable(d, func(i, j int) bool {
		if d[i].Spread != d[j].Spread {
			return d[i].Spread > d[j].Spread
		}
		if d[i].Context != d[j].Context {
			return d[i].Context < d[j].Context
		}
		return d[i].A+d[i].B < d[j].A+d[j].B
	})
	if len(d) > maxDisagreements {
		c.res.Disagreements = d[:maxDisagreements]
	}
	return c.res, nil
}

// hierarchy returns the criteria in depth-first order (parents first) and each
// one's parent index (-1 for the top level).
func (c *computer) hierarchy() ([]Criterion, []int) {
	kids := c.p.childrenOf()
	var order []Criterion
	var parent []int
	var walk func(parentID string, parentIdx int)
	walk = func(parentID string, parentIdx int) {
		for _, cr := range kids[parentID] {
			order = append(order, cr)
			parent = append(parent, parentIdx)
			walk(cr.ID, len(order)-1)
		}
	}
	walk("", -1)
	return order, parent
}

// compare builds the combined pairwise matrix for items in context, records
// its report, and returns the items' weights. Groups that can't be resolved
// get equal weights and an issue with the given code.
func (c *computer) compare(context string, items []string, code, criterionID string) []float64 {
	n := len(items)
	if n == 1 {
		return []float64{1}
	}
	idx := map[string]int{}
	for i, id := range items {
		idx[id] = i
	}
	votes := map[[2]int][]vote{}
	for _, j := range c.p.Pairwise {
		w, ok := c.weights[j.EvaluatorID]
		if !ok || j.Context != context {
			continue
		}
		a, okA := idx[j.A]
		b, okB := idx[j.B]
		if !okA || !okB {
			continue
		}
		v := j.Value
		if a > b {
			a, b, v = b, a, 1/v
		}
		key := [2]int{a, b}
		votes[key] = append(votes[key], vote{evaluator: j.EvaluatorID, value: v, weight: w})
	}

	m := ahp.NewMatrix(n)
	for key, vs := range votes {
		ws := make([]ahp.Weighted, len(vs))
		for k, v := range vs {
			ws[k] = ahp.Weighted{Value: v.value, Weight: v.weight}
		}
		m.Set(key[0], key[1], ahp.GeoMean(ws))
		if len(vs) >= 2 {
			d := Disagreement{Context: context, A: items[key[0]], B: items[key[1]], Spread: ahp.GeoStdDev(ws), Values: map[string]float64{}}
			for _, v := range vs {
				d.Values[v.evaluator] = v.value
			}
			c.res.Disagreements = append(c.res.Disagreements, d)
		}
	}
	pairs := n * (n - 1) / 2
	c.required += pairs
	c.answered += len(votes)

	a := ahp.Analyze(m)
	rep := MatrixReport{
		Context: context, Items: items, Weights: a.Weights, CR: a.CR, Threshold: a.Threshold,
		Consistent: a.Consistent, Complete: a.Complete, Connected: a.Connected,
		Missing: a.Missing, Pairs: pairs, Suggestions: []PairSuggestion{}, Matrix: m,
	}
	if !a.Connected {
		for _, g := range a.Components {
			ids := make([]string, len(g))
			for k, i := range g {
				ids[k] = items[i]
			}
			rep.Components = append(rep.Components, ids)
		}
		rep.Weights = uniform(n)
		c.res.Matrices = append(c.res.Matrices, rep)
		c.issue(Issue{Code: code, Context: context, CriterionID: criterionID,
			Message: fmt.Sprintf("Comparisons don't link all %d items yet, so they are weighted equally.", n)})
		return uniform(n)
	}
	for _, s := range ahp.Suggestions(m, a.Weights, c.p.Scale.Allowed(), maxSuggestions) {
		rep.Suggestions = append(rep.Suggestions, PairSuggestion{A: items[s.I], B: items[s.J], Current: s.Current, Suggested: s.Suggested})
	}
	c.res.Matrices = append(c.res.Matrices, rep)
	return a.Weights
}

// ratingPriorities turns the selected evaluators' 1–10 ratings into priorities.
func (c *computer) ratingPriorities(cr Criterion) []float64 {
	votes := make([][]ahp.Weighted, len(c.altIDs))
	pos := c.altPositions()
	for _, r := range c.p.Ratings {
		w, ok := c.weights[r.EvaluatorID]
		a, okA := pos[r.AlternativeID]
		if ok && okA && r.CriterionID == cr.ID {
			votes[a] = append(votes[a], ahp.Weighted{Value: r.Value, Weight: w})
		}
	}
	raw := make([]float64, len(c.altIDs))
	have := make([]bool, len(c.altIDs))
	for a, vs := range votes {
		if len(vs) > 0 {
			raw[a], have[a] = ahp.ArithMean(vs), true
		}
	}
	if !c.fillMissing(raw, have, cr, IssueRatingsMissing, "rated") {
		return uniform(len(raw))
	}
	return normalize(raw)
}

// valuePriorities turns measured values into priorities, inverting them when
// lower is better.
func (c *computer) valuePriorities(cr Criterion) []float64 {
	raw := make([]float64, len(c.altIDs))
	have := make([]bool, len(c.altIDs))
	pos := c.altPositions()
	for _, v := range c.p.Values {
		if a, ok := pos[v.AlternativeID]; ok && v.CriterionID == cr.ID {
			raw[a], have[a] = v.Value, true
		}
	}
	if !c.fillMissing(raw, have, cr, IssueValuesMissing, "given a value") {
		return uniform(len(raw))
	}
	sum, invalid := 0.0, false
	for _, v := range raw {
		sum += v
		if v < 0 || (cr.Direction == DirLower && v == 0) {
			invalid = true
		}
	}
	if invalid || sum == 0 {
		msg := "Values must be zero or more, and not all zero."
		if cr.Direction == DirLower {
			msg = "When lower is better, every value must be greater than zero."
		}
		c.issue(Issue{Code: IssueValuesInvalid, CriterionID: cr.ID, Message: msg})
		return uniform(len(raw))
	}
	if cr.Direction == DirLower {
		for a := range raw {
			raw[a] = 1 / raw[a]
		}
	}
	out := normalize(raw)
	for _, v := range out {
		if math.IsNaN(v) || math.IsInf(v, 0) { // extreme magnitudes overflow
			c.issue(Issue{Code: IssueValuesInvalid, CriterionID: cr.ID, Message: "These values are too extreme to compare; try different units."})
			return uniform(len(raw))
		}
	}
	return out
}

// fillMissing counts completeness for one leaf and gives alternatives without
// data the average of those with data. It returns false when no alternative has data.
func (c *computer) fillMissing(raw []float64, have []bool, cr Criterion, code, verb string) bool {
	sum, n := 0.0, 0
	for a := range raw {
		if have[a] {
			sum += raw[a]
			n++
		}
	}
	c.required += len(raw)
	c.answered += n
	if n == len(raw) {
		return true
	}
	c.issue(Issue{Code: code, CriterionID: cr.ID,
		Message: fmt.Sprintf("%d of %d alternatives haven't been %s yet.", len(raw)-n, len(raw), verb)})
	if n == 0 {
		return false
	}
	for a := range raw {
		if !have[a] {
			raw[a] = sum / float64(n)
		}
	}
	return true
}

func (c *computer) altPositions() map[string]int {
	pos := make(map[string]int, len(c.altIDs))
	for i, id := range c.altIDs {
		pos[id] = i
	}
	return pos
}

func (c *computer) issue(i Issue) { c.res.Issues = append(c.res.Issues, i) }

func uniform(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = 1 / float64(n)
	}
	return out
}

func normalize(v []float64) []float64 {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / sum
	}
	return out
}

// rank returns 1 plus the number of scores clearly higher than scores[a].
func rank(scores []float64, a int) int {
	r := 1
	for _, s := range scores {
		if s > scores[a]+1e-12 {
			r++
		}
	}
	return r
}
```

- [ ] **Step 4: Write `web/src/lib/types.ts`** (complete file)

```ts
// Mirrors the JSON written by internal/model (project.go, compute.go) and the
// summaries from internal/server (projects.go). Keep the field names in sync.

export type Scale = 'saaty9' | 'classic5';
export type Method = 'pairwise' | 'rating' | 'value';
export type Direction = 'higher' | 'lower';

export interface Alternative {
  id: string;
  name: string;
  description: string;
  color: string;
}

export interface Criterion {
  id: string;
  parentId?: string;
  name: string;
  description: string;
  notes: string;
  method?: Method;
  direction?: Direction;
  unit?: string;
}

export interface Evaluator {
  id: string;
  name: string;
  weight: number;
}

export interface PairwiseJudgment {
  evaluatorId: string;
  context: string;
  a: string;
  b: string;
  value: number;
  note?: string;
}

export interface Rating {
  evaluatorId: string;
  criterionId: string;
  alternativeId: string;
  value: number;
}

export interface Value {
  criterionId: string;
  alternativeId: string;
  value: number;
}

export interface Project {
  schemaVersion: number;
  id: string;
  version: number;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  scale: Scale;
  alternatives: Alternative[];
  criteria: Criterion[];
  evaluators: Evaluator[];
  pairwise: PairwiseJudgment[];
  ratings: Rating[];
  values: Value[];
}

export interface AltScore {
  id: string;
  score: number;
  rank: number;
}

export interface CriterionResult {
  id: string;
  parentId?: string;
  local: number;
  global: number;
  subtree: number[];
  stability: number | null;
}

export interface PairSuggestion {
  a: string;
  b: string;
  current: number;
  suggested: number;
}

export interface MatrixReport {
  context: string;
  items: string[];
  weights: number[] | null;
  cr: number;
  threshold: number;
  consistent: boolean;
  complete: boolean;
  connected: boolean;
  missing: number;
  pairs: number;
  components?: string[][];
  suggestions: PairSuggestion[];
  /** Combined matrix in items order: matrix[i][j] is how strongly item i is preferred over j; 0 if unjudged. */
  matrix: number[][];
}

export interface Issue {
  code: string;
  message: string;
  criterionId?: string;
  context?: string;
}

export interface Disagreement {
  context: string;
  a: string;
  b: string;
  spread: number;
  values: Record<string, number>;
}

export interface Results {
  evaluator: string;
  scores: AltScore[];
  criteria: CriterionResult[];
  matrices: MatrixReport[];
  issues: Issue[];
  disagreements: Disagreement[];
  completeness: number;
  consistent: boolean;
}

export interface RankedName {
  name: string;
  color: string;
  score: number;
}

export interface ProjectSummary {
  id?: string;
  file: string;
  name: string;
  description: string;
  updatedAt: string;
  alternatives: number;
  criteria: number;
  ranking: RankedName[];
  completeness: number;
  consistent: boolean;
  error?: string;
}

/** A bundled sample decision or template (internal/samples). */
export interface StarterEntry {
  id: string;
  name: string;
  description: string;
  alternatives: number;
  criteria: number;
}

/** What importing a legacy workbook found (internal/xlsx). */
export interface ImportReport {
  alternatives: number;
  criteria: number;
  judgments: number;
  skipped: string[];
}

/** What merging a returned file did (model.MergeReport). */
export interface MergeReport {
  added: string[];
  replaced: string[];
  skipped: string[];
  judgments: number;
  dropped: number;
}

export const ALL_EVALUATORS = 'all';
```

- [ ] **Step 5: Write `web/src/components/compare.test.ts`** (complete file)

```ts
import { render } from 'svelte/server';
import { expect, test } from 'vitest';
import Balance from './Balance.svelte';
import ConsistencyMeter from './ConsistencyMeter.svelte';
import MatrixGrid from './MatrixGrid.svelte';

const text = (html: string) => html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');

test('the balance states the judgment in words and marks the chosen stop', () => {
  const { body } = render(Balance, {
    props: { a: { name: 'Alpha' }, b: { name: 'Beta' }, value: 1 / 5, scale: 'saaty9', kind: 'alternatives', on: 'Price', onchange: () => {} },
  });
  expect(text(body)).toContain('Beta is strongly better than Alpha on Price');
  expect(body).toMatch(/aria-valuenow="12"/); // 1/5 sits four stops right of equal (8)
  expect(body.match(/class="stop[^"]*\bon\b/g)).toHaveLength(1);
});

test('an unanswered balance sits level and says so', () => {
  const { body } = render(Balance, {
    props: { a: { name: 'A' }, b: { name: 'B' }, value: undefined, scale: 'classic5', kind: 'criteria', onchange: () => {} },
  });
  expect(text(body)).toContain('Not compared yet');
  expect(body).toContain('rotate(0deg)');
  expect(body.match(/class="stop\b/g)).toHaveLength(5);
});

test('the grid shows reciprocals and flags suggested pairs', () => {
  const items = [{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }, { id: 'c', name: 'C' }];
  const values: Record<string, number> = { '0,1': 3, '1,0': 1 / 3 };
  const { body } = render(MatrixGrid, {
    props: { items, get: (i: number, j: number) => values[`${i},${j}`], suggestions: [{ a: 'b', b: 'a', current: 1 / 3, suggested: 1 }], onpick: () => {} },
  });
  expect(text(body)).toContain('1/3');
  expect(body.match(/class="cell[^"]*\bflag\b/g)).toHaveLength(2); // both orientations of the pair
  expect(body.match(/class="cell[^"]*\bempty\b/g)).toHaveLength(4);
});

test('the consistency meter explains contradictions and offers fixes', () => {
  const report = {
    context: 'alts:x', items: ['a', 'b', 'c'], weights: [0.5, 0.3, 0.2], cr: 0.3, threshold: 0.05, consistent: false,
    complete: true, connected: true, missing: 0, pairs: 3, suggestions: [{ a: 'a', b: 'c', current: 0.25, suggested: 3 }], matrix: [],
  };
  const { body } = render(ConsistencyMeter, {
    props: { report, items: [{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }, { id: 'c', name: 'C' }], kind: 'alternatives', scale: 'saaty9', onfix: () => {} },
  });
  expect(text(body)).toContain('Some answers contradict the others');
  expect(text(body)).toContain('You said: C is moderately to strongly better than A.');
  expect(text(body)).toContain('Your other answers suggest: A is moderately better than C.');
  expect(text(body)).toContain('Use that');
});
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/model -run CombinedMatrix`
Expected: PASS

- [ ] **Step 7: Regenerate the golden files on purpose, and re-check the UI**

```bash
go test ./internal/model -run Golden -update && go test ./...
cd web && npm run check && npx vitest run
```
Expected: all green; the golden JSON gains `matrix` arrays.

- [ ] **Step 8: Commit**

```bash
make lint && make test && (cd web && npm run check)
git add internal/model/compute.go internal/model/compute_test.go internal/model/testdata/golden web/src/components/compare.test.ts web/src/lib/types.ts
git commit -m "feat(model): include each combined comparison matrix in the results"
```

---

### Task 3: Results workbook and the shared results table

**Files:**
- Create: `internal/xlsx/export.go`
- Test: `internal/xlsx/export_test.go`

**Interfaces:**
- Consumes: `ImportLegacy`, `legacyWorkbook` (Task 1); `model.Compute`, `MatrixReport.Matrix` (Task 2); `model.ParseContext`.
- Produces: `xlsx.ResultsTable(p, r) ([]string, [][]any)`, whose rows are rank, option, score, then each leaf criterion's contribution, best first; `xlsx.Export(w io.Writer, p, r) error`, which writes the sheets Summary, Results, Weights, Consistency, Judgments, Ratings and values, and `M<k> <comparison>`; `sheetName(used, name)`.

- [ ] **Step 1: Write the failing test `internal/xlsx/export_test.go`** (complete file)

```go
package xlsx

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// imported returns the synthetic legacy project with its combined results.
func imported(t *testing.T) (*model.Project, *model.Results) {
	t.Helper()
	p, _, err := ImportLegacy(legacyWorkbook(t), "Vendor shortlist")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	r, err := model.Compute(p, model.AllEvaluators)
	if err != nil {
		t.Fatal(err)
	}
	return p, r
}

func TestResultsTable(t *testing.T) {
	p, r := imported(t)
	header, rows := ResultsTable(p, r)
	if strings.Join(header, "|") != "Rank|Option|Score|Price|Speed|Support" {
		t.Errorf("header = %q", header)
	}
	if len(rows) != 3 {
		t.Fatalf("%d rows, want 3", len(rows))
	}
	total := 0.0
	for k, row := range rows {
		if row[0] != k+1 {
			t.Errorf("row %d rank = %v", k, row[0])
		}
		parts := 0.0
		for _, v := range row[3:] {
			parts += v.(float64)
		}
		if score := row[2].(float64); parts < score-1e-9 || parts > score+1e-9 {
			t.Errorf("%s: contributions %v don't add up to the score %v", row[1], parts, score)
		}
		total += row[2].(float64)
		if k > 0 && row[2].(float64) > rows[k-1][2].(float64) {
			t.Error("rows aren't sorted best first")
		}
	}
	if total < 1-1e-9 || total > 1+1e-9 {
		t.Errorf("scores sum to %v", total)
	}
}

func TestExportWorkbook(t *testing.T) {
	p, r := imported(t)
	var buf bytes.Buffer
	if err := Export(&buf, p, r); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want := []string{"Summary", "Results", "Weights", "Consistency", "Judgments", "Ratings and values", "M1 Top-level criteria", "M2 Options on Price", "M3 Options on Speed", "M4 Options on Support"}
	if got := f.GetSheetList(); !slices.Equal(got, want) {
		t.Errorf("sheets = %q\nwant     %q", got, want)
	}
	header, rows := ResultsTable(p, r)
	cell := func(sheet, ref string) string {
		v, _ := f.GetCellValue(sheet, ref, excelize.Options{RawCellValue: true})
		return v
	}
	if cell("Results", "A1") != header[0] || cell("Results", "B2") != rows[0][1] {
		t.Errorf("Results: A1 %q, B2 %q", cell("Results", "A1"), cell("Results", "B2"))
	}
	if cell("Summary", "A1") != "Vendor shortlist" {
		t.Errorf("Summary!A1 = %q", cell("Summary", "A1"))
	}
	// The top-level matrix: Speed over Price is 5 (row Speed, column Price).
	if cell("M1 Top-level criteria", "A3") != "Speed" || cell("M1 Top-level criteria", "B3") != "5" {
		t.Errorf("matrix sheet: A3 %q, B3 %q", cell("M1 Top-level criteria", "A3"), cell("M1 Top-level criteria", "B3"))
	}
	// Unjudged pairs are blank, not zero.
	if v := cell("M3 Options on Speed", "D2"); v != "" {
		t.Errorf("unjudged cell = %q, want blank", v)
	}
	if v := cell("Judgments", "A2"); v != "Imported" {
		t.Errorf("Judgments!A2 = %q", v)
	}
}

func TestSheetNamesAreValidAndUnique(t *testing.T) {
	used := map[string]bool{}
	a := sheetName(used, "M1 Options on Price: a [very] long name / with * odd ? characters")
	b := sheetName(used, "M1 Options on Price: a [very] long name / with * odd ? characters")
	if len([]rune(a)) > 31 || strings.ContainsAny(a, `[]:*?/\`) || a == b {
		t.Errorf("sheet names %q and %q", a, b)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/xlsx`
Expected: FAIL — `undefined: ResultsTable`, `undefined: Export`.

- [ ] **Step 3: Write `internal/xlsx/export.go`** (complete file)

```go
package xlsx

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// ResultsTable is the results as a table: a header, then one row per option,
// best first, holding its rank (int), name (string), overall score (float64)
// and each leaf criterion's contribution to that score (float64), which add up
// to the score. The Excel and CSV exports share it.
func ResultsTable(p *model.Project, r *model.Results) ([]string, [][]any) {
	labels := labeler(p)
	leaves := []model.CriterionResult{}
	isParent := map[string]bool{}
	for _, c := range r.Criteria {
		isParent[c.ParentID] = true
	}
	for _, c := range r.Criteria {
		if !isParent[c.ID] {
			leaves = append(leaves, c)
		}
	}
	header := []string{"Rank", "Option", "Score"}
	for _, c := range leaves {
		header = append(header, labels.criterion(c.ID))
	}
	order := make([]int, len(r.Scores))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(x, y int) bool { return r.Scores[order[x]].Score > r.Scores[order[y]].Score })
	rows := make([][]any, 0, len(order))
	for k, a := range order {
		row := []any{k + 1, p.Alternatives[a].Name, r.Scores[a].Score}
		for _, c := range leaves {
			row = append(row, c.Global*c.Subtree[a])
		}
		rows = append(rows, row)
	}
	return header, rows
}

// Export writes a results workbook: a summary, the ranking with each
// criterion's contribution, criteria weights, consistency, every judgment,
// and one sheet per comparison matrix (like the original template's
// "Criterion N" sheets).
func Export(w io.Writer, p *model.Project, r *model.Results) error {
	f := excelize.NewFile()
	defer f.Close()
	x := &book{f: f, used: map[string]bool{}}
	labels := labeler(p)

	who := "Everyone, combined"
	for _, e := range p.Evaluators {
		if e.ID == r.Evaluator {
			who = e.Name
		}
	}
	consistent := "Yes"
	if !r.Consistent {
		consistent = "No: some comparisons contradict the others"
	}
	x.sheet("Summary", nil, [][]any{
		{p.Name},
		{"Description", p.Description},
		{"Judgments from", who},
		{"Judgments made", r.Completeness},
		{"Consistent", consistent},
		{"Exported", time.Now().Format("2 January 2006")},
	})
	x.bold("Summary", "A1")
	x.percent("Summary", "B4") // "Judgments made"

	header, rows := ResultsTable(p, r)
	x.sheet("Results", header, rows)
	x.percentColumns("Results", 3, len(header), len(rows))

	weights := [][]any{}
	for _, c := range r.Criteria {
		weights = append(weights, []any{labels.criterion(c.ID), c.Local, c.Global})
	}
	x.sheet("Weights", []string{"Criterion", "Within its group", "Of the decision"}, weights)
	x.percentColumns("Weights", 2, 3, len(weights))

	checks := [][]any{}
	for _, m := range r.Matrices {
		status := "Consistent"
		switch {
		case !m.Connected:
			status = "Not enough comparisons yet"
		case len(m.Items) < 3:
			status = "Two items: always consistent"
		case !m.Consistent:
			status = "Contradictions"
		}
		checks = append(checks, []any{labels.context(m.Context), len(m.Items), m.CR, m.Threshold, status})
	}
	x.sheet("Consistency", []string{"Comparison", "Items", "Consistency ratio", "Acceptable up to", "Status"}, checks)

	people := map[string]string{}
	for _, e := range p.Evaluators {
		people[e.ID] = e.Name
	}
	judged := [][]any{}
	for _, j := range p.Pairwise {
		judged = append(judged, []any{people[j.EvaluatorID], labels.context(j.Context), labels.item(j.A), labels.item(j.B), j.Value, j.Note})
	}
	x.sheet("Judgments", []string{"Person", "Comparison", "First", "Second", "How strongly the first is preferred", "Note"}, judged)

	scored := [][]any{}
	for _, rt := range p.Ratings {
		scored = append(scored, []any{labels.criterion(rt.CriterionID), labels.item(rt.AlternativeID), people[rt.EvaluatorID], rt.Value})
	}
	for _, v := range p.Values {
		scored = append(scored, []any{labels.criterion(v.CriterionID), labels.item(v.AlternativeID), "Measured", v.Value})
	}
	x.sheet("Ratings and values", []string{"Criterion", "Option", "Rated by", "Value"}, scored)

	for k, m := range r.Matrices {
		names := make([]string, len(m.Items))
		for i, id := range m.Items {
			names[i] = labels.item(id)
		}
		grid := make([][]any, len(m.Items))
		for i := range m.Items {
			row := []any{names[i]}
			for j := range m.Items {
				if v := m.Matrix[i][j]; v != 0 {
					row = append(row, v)
				} else {
					row = append(row, nil)
				}
			}
			if m.Weights != nil {
				row = append(row, m.Weights[i])
			}
			grid[i] = row
		}
		title := x.sheet(fmt.Sprintf("M%d %s", k+1, labels.context(m.Context)), append(append([]string{"Row compared with column"}, names...), "Weight"), grid)
		x.percentColumns(title, len(names)+2, len(names)+2, len(grid))
	}

	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}
	return f.Write(w)
}

// book writes sheets with unique, valid names.
type book struct {
	f    *excelize.File
	used map[string]bool
}

// sheet adds a sheet with an optional bold header row and returns its name.
func (b *book) sheet(name string, header []string, rows [][]any) string {
	name = sheetName(b.used, name)
	_, _ = b.f.NewSheet(name)
	start := 1
	if header != nil {
		hs := make([]any, len(header))
		for i, h := range header {
			hs[i] = h
		}
		_ = b.f.SetSheetRow(name, "A1", &hs)
		if style, err := b.f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}}); err == nil {
			end, _ := excelize.CoordinatesToCellName(len(header), 1)
			_ = b.f.SetCellStyle(name, "A1", end, style)
		}
		start = 2
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, start+i)
		_ = b.f.SetSheetRow(name, cell, &row)
	}
	_ = b.f.SetColWidth(name, "A", "A", 34)
	return name
}

func (b *book) bold(sheet, cell string) {
	if style, err := b.f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}}); err == nil {
		_ = b.f.SetCellStyle(sheet, cell, cell, style)
	}
}

func (b *book) percent(sheet, cell string) {
	if style, err := b.f.NewStyle(&excelize.Style{NumFmt: 10}); err == nil { // 0.00%
		_ = b.f.SetCellStyle(sheet, cell, cell, style)
	}
}

// percentColumns formats columns from..to (1-based) of the data rows as percentages.
func (b *book) percentColumns(sheet string, from, to, rows int) {
	if rows == 0 || from > to {
		return
	}
	style, err := b.f.NewStyle(&excelize.Style{NumFmt: 10})
	if err != nil {
		return
	}
	first, _ := excelize.CoordinatesToCellName(from, 2)
	last, _ := excelize.CoordinatesToCellName(to, rows+1)
	_ = b.f.SetCellStyle(sheet, first, last, style)
}

// sheetName makes name a valid Excel sheet name (at most 31 characters, none
// of []:*?/\) that hasn't been used yet.
func sheetName(used map[string]bool, name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return ' '
		}
		return r
	}, name)
	base := []rune(strings.Join(strings.Fields(name), " "))
	for k := 1; ; k++ {
		suffix := ""
		if k > 1 {
			suffix = fmt.Sprintf(" (%d)", k)
		}
		limit := 31 - len(suffix)
		cut := base
		if len(cut) > limit {
			cut = cut[:limit]
		}
		candidate := strings.TrimSpace(string(cut)) + suffix
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

// names turns ids and contexts into the words the UI uses.
type names struct {
	p    *model.Project
	crit map[string]model.Criterion
	alt  map[string]string
}

func labeler(p *model.Project) names {
	n := names{p: p, crit: map[string]model.Criterion{}, alt: map[string]string{}}
	for _, c := range p.Criteria {
		n.crit[c.ID] = c
	}
	for _, a := range p.Alternatives {
		n.alt[a.ID] = a.Name
	}
	return n
}

// criterion is a criterion's path from the top: "Technical › Performance".
func (n names) criterion(id string) string {
	parts := []string{}
	for c, ok := n.crit[id]; ok; c, ok = n.crit[c.ParentID] {
		parts = append([]string{c.Name}, parts...)
		if len(parts) > len(n.crit) {
			break // a cycle; Prepare rejects these, but never loop forever
		}
	}
	return strings.Join(parts, " › ")
}

func (n names) item(id string) string {
	if a, ok := n.alt[id]; ok {
		return a
	}
	return n.crit[id].Name
}

func (n names) context(ctx string) string {
	kind, id, _ := model.ParseContext(ctx)
	switch {
	case kind == "alts":
		return "Options on " + n.criterion(id)
	case id == "":
		return "Top-level criteria"
	default:
		return "Criteria within " + n.criterion(id)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/xlsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
make lint && make test
git add internal/xlsx/export.go internal/xlsx/export_test.go
git commit -m "feat(xlsx): export results, weights, consistency and every matrix to Excel"
```

---

### Task 4: Packets and merging

**Files:**
- Modify: `internal/model/prepare.go` (validity checks extracted into `known`)
- Create: `internal/model/merge.go`
- Test: `internal/model/merge_test.go`

**Interfaces:**
- Consumes: `Prepare`, `fixture()` (Plan 1).
- Produces:
-   - `model.Packet(p, evaluatorID) (*Project, error)`
-   - `model.Merge(owner, packet, policy) (*MergeReport, error)`, where policy is `MergeAsk` (""), `MergeReplace` or `MergeSkip`
-   - `type MergeReport {Added, Replaced, Skipped []string; Judgments, Dropped int}`
-   - `ErrDifferentProject`, `*EvaluatorConflictError{Names}`
-   - `(p *Project).known()`, with `known.pair(j)` and `known.rating(r)`

- [ ] **Step 1: Write the failing test `internal/model/merge_test.go`** (complete file)

```go
package model

import (
	"errors"
	"testing"
)

func countFor(p *Project, ev string) (pairs, ratings int) {
	for _, j := range p.Pairwise {
		if j.EvaluatorID == ev {
			pairs++
		}
	}
	for _, r := range p.Ratings {
		if r.EvaluatorID == ev {
			ratings++
		}
	}
	return
}

func TestPacketHoldsOnlyThatPerson(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 2})
	pk, err := Packet(p, "e2")
	if err != nil {
		t.Fatal(err)
	}
	if pk.ID != p.ID || len(pk.Evaluators) != 1 || pk.Evaluators[0].ID != "e2" {
		t.Errorf("packet evaluators = %+v", pk.Evaluators)
	}
	if pairs, ratings := countFor(pk, "e1"); pairs+ratings != 0 {
		t.Error("packet carries someone else's judgments")
	}
	if pairs, _ := countFor(pk, "e2"); pairs != 1 {
		t.Errorf("packet has %d of Bob's judgments, want 1", pairs)
	}
	if len(pk.Values) != len(p.Values) || len(pk.Criteria) != len(p.Criteria) {
		t.Error("packet lost the shared structure or measured values")
	}
	if pk.Pairwise[0].Value = 9; p.Pairwise[len(p.Pairwise)-1].Value == 9 {
		t.Error("packet shares memory with the project")
	}
	if _, err := Packet(p, "nobody"); !errors.Is(err, ErrUnknownEvaluator) {
		t.Errorf("unknown evaluator: err = %v", err)
	}
}

// returned simulates Bob's packet coming back with his judgments filled in.
func returned(t *testing.T, owner *Project) *Project {
	t.Helper()
	pk, err := Packet(owner, "e2")
	if err != nil {
		t.Fatal(err)
	}
	pk.Pairwise = append(pk.Pairwise,
		PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c4", B: "c1", Value: 3},
		PairwiseJudgment{EvaluatorID: "e2", Context: AltsContext("c2"), A: "a1", B: "a2", Value: 5},
		PairwiseJudgment{EvaluatorID: "e2", Context: AltsContext("c2"), A: "a1", B: "zz", Value: 5}, // an option the owner deleted
	)
	pk.Ratings = append(pk.Ratings, Rating{EvaluatorID: "e2", CriterionID: "c3", AlternativeID: "a1", Value: 8})
	pk.Values[0].Value = 1 // the colleague edited a measured value: ignored
	pk.Name = "Renamed by Bob"
	return pk
}

func TestMergeAsksBeforeReplacingSomeone(t *testing.T) {
	owner := fixture()
	owner.Pairwise = append(owner.Pairwise, PairwiseJudgment{EvaluatorID: "e2", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 2})
	var ce *EvaluatorConflictError
	if _, err := Merge(owner, returned(t, owner), MergeAsk); !errors.As(err, &ce) || len(ce.Names) != 1 || ce.Names[0] != "Bob" {
		t.Fatalf("err = %v, want a conflict naming Bob", err)
	}
	if pairs, _ := countFor(owner, "e2"); pairs != 1 {
		t.Error("a refused merge still changed the project")
	}

	rep, err := Merge(owner, returned(t, owner), MergeReplace)
	if err != nil {
		t.Fatal(err)
	}
	if pairs, ratings := countFor(owner, "e2"); pairs != 2 || ratings != 1 {
		t.Errorf("after replace Bob has %d judgments and %d ratings, want 2 and 1", pairs, ratings)
	}
	if len(rep.Replaced) != 1 || rep.Replaced[0] != "Bob" || rep.Dropped != 1 || rep.Judgments != 3 {
		t.Errorf("report = %+v", rep)
	}
	if owner.Name != "Fixture" || owner.Values[0].Value != 100 {
		t.Error("the packet's structural edits leaked into the owner's project")
	}
	if pairs, _ := countFor(owner, "e1"); pairs != 5 {
		t.Error("someone else's judgments changed")
	}
}

func TestMergeSkipKeepsTheCurrentJudgments(t *testing.T) {
	owner := fixture()
	rep, err := Merge(owner, returned(t, owner), MergeSkip)
	if err != nil {
		t.Fatal(err)
	}
	if pairs, ratings := countFor(owner, "e2"); pairs+ratings != 0 || len(rep.Skipped) != 1 {
		t.Errorf("skip changed Bob's judgments (%d, %d), report %+v", pairs, ratings, rep)
	}
}

func TestMergeAddsNewPeople(t *testing.T) {
	owner := fixture()
	pk := returned(t, owner)
	pk.Evaluators[0] = Evaluator{ID: "e3", Name: "Cleo", Weight: 2}
	for i := range pk.Pairwise {
		pk.Pairwise[i].EvaluatorID = "e3"
	}
	for i := range pk.Ratings {
		pk.Ratings[i].EvaluatorID = "e3"
	}
	rep, err := Merge(owner, pk, MergeAsk)
	if err != nil {
		t.Fatal(err)
	}
	if len(owner.Evaluators) != 3 || owner.Evaluators[2].Name != "Cleo" || owner.Evaluators[2].Weight != 2 {
		t.Errorf("evaluators = %+v", owner.Evaluators)
	}
	if len(rep.Added) != 1 || rep.Added[0] != "Cleo" {
		t.Errorf("report = %+v", rep)
	}
}

func TestMergeRefusesAnotherProject(t *testing.T) {
	owner := fixture()
	other := New("Other")
	if _, err := Merge(owner, other, MergeReplace); !errors.Is(err, ErrDifferentProject) {
		t.Errorf("err = %v, want ErrDifferentProject", err)
	}
}

func TestAFailedMergeChangesNothing(t *testing.T) {
	owner := fixture()
	before, _ := clone(owner)
	pk := returned(t, owner)
	pk.Ratings[len(pk.Ratings)-1].Value = 99 // out of range: the merge must fail
	var ve *ValidationError
	if _, err := Merge(owner, pk, MergeReplace); !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a validation error", err)
	}
	if len(owner.Evaluators) != len(before.Evaluators) || len(owner.Pairwise) != len(before.Pairwise) || len(owner.Ratings) != len(before.Ratings) {
		t.Error("a failed merge changed the project")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/model`
Expected: FAIL — `undefined: Packet`.

- [ ] **Step 3: Write `internal/model/prepare.go`** (complete file)

```go
package model

import (
	"fmt"
	"math"
	"strings"

	"github.com/Xafloc/ahp-matrix/internal/ahp"
)

// Problem is one validation failure, tied to a field path such as
// "criteria[2].method".
type Problem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError lists every problem found in a project.
type ValidationError struct{ Problems []Problem }

func (e *ValidationError) Error() string {
	msg := fmt.Sprintf("invalid project: %s: %s", e.Problems[0].Field, e.Problems[0].Message)
	if n := len(e.Problems) - 1; n > 0 {
		msg += fmt.Sprintf(" (and %d more)", n)
	}
	return msg
}

// Prepare readies a project for saving: it fills in defaults, drops judgments
// that refer to deleted items (and duplicates), validates what is left, and
// snaps pairwise values to the project's scale. Prepare returns a
// *ValidationError when the project cannot be saved.
func (p *Project) Prepare() error {
	p.fillDefaults()
	p.prune()
	if err := p.Validate(); err != nil {
		return err
	}
	allowed := p.Scale.Allowed()
	for i := range p.Pairwise {
		p.Pairwise[i].Value = ahp.Snap(p.Pairwise[i].Value, allowed)
	}
	return nil
}

// childrenOf maps each parent ID ("" for the top level) to its children, in
// project order.
func (p *Project) childrenOf() map[string][]Criterion {
	kids := map[string][]Criterion{}
	for _, c := range p.Criteria {
		kids[c.ParentID] = append(kids[c.ParentID], c)
	}
	return kids
}

func (p *Project) fillDefaults() {
	p.SchemaVersion = SchemaVersion
	if p.ID == "" {
		p.ID = NewProjectID()
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Scale == "" {
		p.Scale = ScaleSaaty9
	}
	if p.Alternatives == nil {
		p.Alternatives = []Alternative{}
	}
	if p.Criteria == nil {
		p.Criteria = []Criterion{}
	}
	if p.Evaluators == nil {
		p.Evaluators = []Evaluator{}
	}
	if p.Pairwise == nil {
		p.Pairwise = []PairwiseJudgment{}
	}
	if p.Ratings == nil {
		p.Ratings = []Rating{}
	}
	if p.Values == nil {
		p.Values = []Value{}
	}
	for i := range p.Alternatives {
		a := &p.Alternatives[i]
		a.Name = strings.TrimSpace(a.Name)
		if a.ID == "" {
			a.ID = NewID()
		}
	}
	for i := range p.Evaluators {
		e := &p.Evaluators[i]
		e.Name = strings.TrimSpace(e.Name)
		if e.ID == "" {
			e.ID = NewID()
		}
		if e.Weight == 0 {
			e.Weight = 1
		}
	}
	for i := range p.Criteria {
		c := &p.Criteria[i]
		c.Name = strings.TrimSpace(c.Name)
		if c.ID == "" {
			c.ID = NewID()
		}
	}
	kids := p.childrenOf()
	for i := range p.Criteria {
		c := &p.Criteria[i]
		if len(kids[c.ID]) > 0 {
			continue
		}
		if c.Method == "" {
			c.Method = MethodPairwise
		}
		if c.Method == MethodValue && c.Direction == "" {
			c.Direction = DirHigher
		}
	}
}

// prune drops judgments that refer to missing evaluators, criteria or
// alternatives, or to a context they don't belong to. Judgments of
// alternatives are kept for any existing criterion, not just leaves, so
// nesting a criterion and un-nesting it again loses nothing; only leaves use
// them. When the same pair, rating or value appears twice, the later one wins.
func (p *Project) prune() {
	k := p.known()
	pairs := []PairwiseJudgment{}
	pairAt := map[string]int{}
	for _, j := range p.Pairwise {
		if !k.pair(j) {
			continue
		}
		lo, hi := j.A, j.B
		if lo > hi {
			lo, hi = hi, lo
		}
		key := j.EvaluatorID + "\x00" + j.Context + "\x00" + lo + "\x00" + hi
		if i, dup := pairAt[key]; dup {
			pairs[i] = j
			continue
		}
		pairAt[key] = len(pairs)
		pairs = append(pairs, j)
	}
	p.Pairwise = pairs

	ratings := []Rating{}
	ratingAt := map[string]int{}
	for _, r := range p.Ratings {
		if !k.rating(r) {
			continue
		}
		key := r.EvaluatorID + "\x00" + r.CriterionID + "\x00" + r.AlternativeID
		if i, dup := ratingAt[key]; dup {
			ratings[i] = r
			continue
		}
		ratingAt[key] = len(ratings)
		ratings = append(ratings, r)
	}
	p.Ratings = ratings

	values := []Value{}
	valueAt := map[string]int{}
	for _, v := range p.Values {
		if !k.crit[v.CriterionID] || !k.alts[v.AlternativeID] {
			continue
		}
		key := v.CriterionID + "\x00" + v.AlternativeID
		if i, dup := valueAt[key]; dup {
			values[i] = v
			continue
		}
		valueAt[key] = len(values)
		values = append(values, v)
	}
	p.Values = values
}

// known is a snapshot of a project's ids, for checking what judgments refer to.
type known struct {
	evals, alts, crit map[string]bool
	parent            map[string]string
}

func (p *Project) known() known {
	k := known{evals: map[string]bool{}, alts: map[string]bool{}, crit: map[string]bool{}, parent: map[string]string{}}
	for _, e := range p.Evaluators {
		k.evals[e.ID] = true
	}
	for _, a := range p.Alternatives {
		k.alts[a.ID] = true
	}
	for _, c := range p.Criteria {
		k.crit[c.ID] = true
		k.parent[c.ID] = c.ParentID
	}
	return k
}

// pair reports whether a pairwise judgment refers to an existing evaluator
// and to two different items of a context that exists.
func (k known) pair(j PairwiseJudgment) bool {
	kind, id, ok := ParseContext(j.Context)
	if !ok || !k.evals[j.EvaluatorID] || j.A == j.B {
		return false
	}
	if kind == "alts" {
		return k.crit[id] && k.alts[j.A] && k.alts[j.B]
	}
	if id != "" && !k.crit[id] {
		return false
	}
	return k.crit[j.A] && k.crit[j.B] && k.parent[j.A] == id && k.parent[j.B] == id
}

func (k known) rating(r Rating) bool {
	return k.evals[r.EvaluatorID] && k.crit[r.CriterionID] && k.alts[r.AlternativeID]
}

// Validate reports every problem that prevents the project from being saved.
// It returns nil or a *ValidationError.
func (p *Project) Validate() error {
	var probs []Problem
	add := func(field, format string, args ...any) {
		probs = append(probs, Problem{Field: field, Message: fmt.Sprintf(format, args...)})
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

	if !ValidID(p.ID) {
		add("id", "must be 1–64 letters, digits, '-' or '_'")
	}
	if strings.TrimSpace(p.Name) == "" {
		add("name", "is required")
	}
	if !p.Scale.Valid() {
		add("scale", "must be %q or %q", ScaleSaaty9, ScaleClassic5)
	}

	seen := map[string]bool{}
	for i, a := range p.Alternatives {
		f := fmt.Sprintf("alternatives[%d]", i)
		if seen[a.ID] {
			add(f+".id", "duplicate id %q", a.ID)
		}
		seen[a.ID] = true
		if a.Name == "" {
			add(f+".name", "is required")
		}
	}

	if len(p.Evaluators) == 0 {
		add("evaluators", "at least one evaluator is required")
	}
	seen = map[string]bool{}
	for i, e := range p.Evaluators {
		f := fmt.Sprintf("evaluators[%d]", i)
		if seen[e.ID] {
			add(f+".id", "duplicate id %q", e.ID)
		}
		seen[e.ID] = true
		if e.Name == "" {
			add(f+".name", "is required")
		}
		if !finite(e.Weight) || e.Weight <= 0 {
			add(f+".weight", "must be greater than 0")
		}
	}

	byID := map[string]Criterion{}
	for i, c := range p.Criteria {
		f := fmt.Sprintf("criteria[%d]", i)
		if _, dup := byID[c.ID]; dup {
			add(f+".id", "duplicate id %q", c.ID)
		}
		if c.ID == rootKey {
			add(f+".id", "%q is reserved", rootKey)
		}
		byID[c.ID] = c
		if c.Name == "" {
			add(f+".name", "is required")
		}
	}
	kids := p.childrenOf()
	for i, c := range p.Criteria {
		f := fmt.Sprintf("criteria[%d]", i)
		if c.ParentID != "" {
			if _, ok := byID[c.ParentID]; !ok || c.ParentID == c.ID {
				add(f+".parentId", "refers to a missing criterion %q", c.ParentID)
			} else if inCycle(c.ID, byID) {
				add(f+".parentId", "creates a cycle")
			}
		}
		if len(kids[c.ID]) > 0 {
			continue
		}
		switch c.Method {
		case MethodPairwise, MethodRating:
		case MethodValue:
			if c.Direction != DirHigher && c.Direction != DirLower {
				add(f+".direction", "must be %q or %q", DirHigher, DirLower)
			}
		default:
			add(f+".method", "must be %q, %q or %q", MethodPairwise, MethodRating, MethodValue)
		}
	}

	for i, j := range p.Pairwise {
		if !finite(j.Value) || j.Value <= 0 {
			add(fmt.Sprintf("pairwise[%d].value", i), "must be a positive number")
		}
	}
	for i, r := range p.Ratings {
		if !finite(r.Value) || r.Value < 1 || r.Value > 10 {
			add(fmt.Sprintf("ratings[%d].value", i), "must be between 1 and 10")
		}
	}
	for i, v := range p.Values {
		if !finite(v.Value) {
			add(fmt.Sprintf("values[%d].value", i), "must be a number")
		}
	}

	if len(probs) > 0 {
		return &ValidationError{Problems: probs}
	}
	return nil
}

// inCycle reports whether following parents up from id returns to id.
func inCycle(id string, byID map[string]Criterion) bool {
	cur := byID[id].ParentID
	for steps := 0; cur != "" && steps <= len(byID); steps++ {
		if cur == id {
			return true
		}
		cur = byID[cur].ParentID
	}
	return false
}
```

- [ ] **Step 4: Write `internal/model/merge.go`** (complete file)

```go
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrDifferentProject means a returned file belongs to another decision.
var ErrDifferentProject = errors.New("this file is a different decision; only files exported from this one can be merged")

// MergePolicy says what to do when a returned file holds judgments for
// someone who already has judgments in the project.
type MergePolicy string

const (
	MergeAsk     MergePolicy = ""        // refuse with *EvaluatorConflictError so the person can choose
	MergeReplace MergePolicy = "replace" // the file's judgments replace theirs
	MergeSkip    MergePolicy = "skip"    // keep theirs and ignore the file's
)

// EvaluatorConflictError lists the people already present when merging with MergeAsk.
type EvaluatorConflictError struct{ Names []string }

func (e *EvaluatorConflictError) Error() string {
	return fmt.Sprintf("%s already %s judgments in this decision", strings.Join(e.Names, ", "), map[bool]string{true: "has", false: "have"}[len(e.Names) == 1])
}

// MergeReport says what a merge did.
type MergeReport struct {
	Added     []string `json:"added"`     // people new to the project
	Replaced  []string `json:"replaced"`  // people whose judgments were replaced
	Skipped   []string `json:"skipped"`   // people left as they were
	Judgments int      `json:"judgments"` // judgments brought in
	Dropped   int      `json:"dropped"`   // judgments about options or criteria the project no longer has
}

// Packet is a copy of the project to send to one person: the full structure
// and measured values, but only that person's judgments.
func Packet(p *Project, evaluatorID string) (*Project, error) {
	var who *Evaluator
	for i := range p.Evaluators {
		if p.Evaluators[i].ID == evaluatorID {
			who = &p.Evaluators[i]
		}
	}
	if who == nil {
		return nil, fmt.Errorf("%w %q", ErrUnknownEvaluator, evaluatorID)
	}
	out, err := clone(p)
	if err != nil {
		return nil, err
	}
	out.Evaluators = []Evaluator{*who}
	out.Pairwise = keep(out.Pairwise, func(j PairwiseJudgment) bool { return j.EvaluatorID == evaluatorID })
	out.Ratings = keep(out.Ratings, func(r Rating) bool { return r.EvaluatorID == evaluatorID })
	return out, nil
}

// Merge brings the judgments in a returned packet (or any export of the same
// project) into owner. Only people's judgments come across; the owner's
// options, criteria, measured values and names always win. Judgments about
// items the owner no longer has are dropped and counted. On error, owner is
// unchanged.
func Merge(owner, packet *Project, policy MergePolicy) (*MergeReport, error) {
	if packet.ID != owner.ID {
		return nil, ErrDifferentProject
	}
	next, err := clone(owner)
	if err != nil {
		return nil, err
	}
	existing := map[string]string{}
	for _, e := range next.Evaluators {
		existing[e.ID] = e.Name
	}
	rep := &MergeReport{Added: []string{}, Replaced: []string{}, Skipped: []string{}}
	var conflicts []string
	incoming := map[string]bool{}
	for _, e := range packet.Evaluators {
		name, ok := existing[e.ID]
		switch {
		case !ok:
			next.Evaluators = append(next.Evaluators, e)
			rep.Added = append(rep.Added, e.Name)
			incoming[e.ID] = true
		case policy == MergeAsk:
			conflicts = append(conflicts, name)
		case policy == MergeReplace:
			rep.Replaced = append(rep.Replaced, name)
			incoming[e.ID] = true
		default:
			rep.Skipped = append(rep.Skipped, name)
		}
	}
	if len(conflicts) > 0 {
		return nil, &EvaluatorConflictError{Names: conflicts}
	}

	// Replace means replace: drop what the owner had for these people first.
	next.Pairwise = keep(next.Pairwise, func(j PairwiseJudgment) bool { return !incoming[j.EvaluatorID] })
	next.Ratings = keep(next.Ratings, func(r Rating) bool { return !incoming[r.EvaluatorID] })
	k := next.known()
	for _, j := range packet.Pairwise {
		if !incoming[j.EvaluatorID] {
			continue
		}
		if k.pair(j) {
			next.Pairwise = append(next.Pairwise, j)
		} else {
			rep.Dropped++
		}
	}
	for _, r := range packet.Ratings {
		if !incoming[r.EvaluatorID] {
			continue
		}
		if k.rating(r) {
			next.Ratings = append(next.Ratings, r)
		} else {
			rep.Dropped++
		}
	}
	if err := next.Prepare(); err != nil {
		return nil, err
	}
	for _, j := range next.Pairwise {
		if incoming[j.EvaluatorID] {
			rep.Judgments++
		}
	}
	for _, r := range next.Ratings {
		if incoming[r.EvaluatorID] {
			rep.Judgments++
		}
	}
	*owner = *next
	return rep, nil
}

func keep[T any](items []T, ok func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, it := range items {
		if ok(it) {
			out = append(out, it)
		}
	}
	return out
}

// clone deep-copies a project.
func clone(p *Project) (*Project, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var out Project
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/model`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
make lint && make test
git add internal/model/merge.go internal/model/merge_test.go internal/model/prepare.go
git commit -m "feat(model): per-person packets and merging returned judgments"
```

---

### Task 5: Sample decisions and templates

**Files:**
- Create: `internal/samples/samples/*.ahp.json`, `internal/samples/templates/*.ahp.json` (generated)
- Create: `internal/samples/samples.go`
- Test: `internal/samples/samples_test.go`

**Interfaces:**
- Consumes: `model.Decode`, `model.NewProjectID`, `model.Compute`.
- Produces: `samples.Samples()`, `samples.Templates()` (returning `[]Entry{ID, Name, Description, Alternatives, Criteria}`); `samples.Sample(id)`, `samples.Template(id, name)` (each a fresh project with a new id); `samples.ErrNotFound`. Ids containing `/`, `\` or `.` are rejected.

- [ ] **Step 1: Generate the sample and template files**

Save this script outside the repo, e.g. as `/tmp/gensamples.py`, and run it:

```python
"""Writes the bundled sample decisions and templates (run once; output is committed)."""
import json, math, sys, pathlib
out = pathlib.Path(sys.argv[1])
STAMP = "2026-10-15T12:00:00Z"
SAATY = [1/9,1/8,1/7,1/6,1/5,1/4,1/3,1/2,1,2,3,4,5,6,7,8,9]
def snap(v): return min(SAATY, key=lambda s: abs(math.log(s)-math.log(v)))

def project(pid, name, desc):
    return {"schemaVersion": 1, "id": pid, "version": 1, "name": name, "description": desc,
            "createdAt": STAMP, "updatedAt": STAMP, "scale": "saaty9", "alternatives": [], "criteria": [],
            "evaluators": [], "pairwise": [], "ratings": [], "values": []}

def alts(p, names):
    for i, n in enumerate(names):
        p["alternatives"].append({"id": f"o{i+1}", "name": n, "description": "", "color": f"series-{i+1}"})

def crit(p, cid, name, parent=None, method=None, direction=None, unit=None, notes=""):
    c = {"id": cid, "name": name, "description": "", "notes": notes}
    if parent: c["parentId"] = parent
    if method: c["method"] = method
    if direction: c["direction"] = direction
    if unit: c["unit"] = unit
    p["criteria"].append(c)

def weigh(p, ev, ctx, ids, weights, skew=None):
    """Judgments implied by weights (snapped to the scale), optionally nudged per pair."""
    for i in range(len(ids)):
        for j in range(i+1, len(ids)):
            v = weights[i] / weights[j]
            if skew and (i, j) in skew: v *= skew[(i, j)]
            p["pairwise"].append({"evaluatorId": ev, "context": ctx, "a": ids[i], "b": ids[j], "value": snap(v)})

def rate(p, ev, cid, values):
    for i, v in enumerate(values):
        p["ratings"].append({"evaluatorId": ev, "criterionId": cid, "alternativeId": f"o{i+1}", "value": v})

def measure(p, cid, values):
    for i, v in enumerate(values):
        p["values"].append({"criterionId": cid, "alternativeId": f"o{i+1}", "value": v})

def write(sub, slug, p):
    (out / sub / f"{slug}.ahp.json").write_text(json.dumps(p, indent=2, ensure_ascii=False) + "\n")

# 1. Cloud provider: hierarchy and all three scoring methods.
p = project("01SAMPLECLOUDPROVIDER00000", "Choosing a cloud provider",
            "Where to run the new customer platform for the next three years. A sample: explore it, change it, or start your own.")
p["evaluators"] = [{"id": "e1", "name": "Platform team", "weight": 1}]
alts(p, ["AWS", "Google Cloud", "Azure", "Hetzner"])
crit(p, "cost", "Monthly cost", method="value", direction="lower", unit="USD", notes="Quotes for the reference architecture, with committed-use discounts.")
crit(p, "tech", "Technical")
crit(p, "perf", "Performance", "tech", "pairwise", notes="Load-test results from the September trial.")
crit(p, "rel", "Reliability", "tech", "rating")
crit(p, "eco", "Ecosystem")
crit(p, "svc", "Managed services", "eco", "rating")
crit(p, "hire", "Hiring market", "eco", "pairwise", notes="How easy it is to hire engineers who already know the platform.")
crit(p, "support", "Support quality", method="rating")
weigh(p, "e1", "criteria:root", ["cost", "tech", "eco", "support"], [0.35, 0.35, 0.18, 0.12])
weigh(p, "e1", "criteria:tech", ["perf", "rel"], [0.4, 0.6])
weigh(p, "e1", "criteria:eco", ["svc", "hire"], [0.55, 0.45])
weigh(p, "e1", "alts:perf", ["o1", "o2", "o3", "o4"], [0.3, 0.32, 0.24, 0.14])
weigh(p, "e1", "alts:hire", ["o1", "o2", "o3", "o4"], [0.45, 0.2, 0.3, 0.05])
rate(p, "e1", "rel", [9, 9, 8, 7])
rate(p, "e1", "svc", [10, 8, 8, 3])
rate(p, "e1", "support", [7, 6, 7, 8])
measure(p, "cost", [18400, 16900, 17800, 6200])
write("samples", "cloud-provider", p)

# 2. Hiring: two people who disagree.
p = project("01SAMPLEHIRINGENGINEER0000", "Hiring a senior engineer",
            "Three finalists for the payments team. The engineering lead and the product manager judged separately; see where they disagree.")
p["evaluators"] = [{"id": "lead", "name": "Engineering lead", "weight": 1}, {"id": "pm", "name": "Product manager", "weight": 1}]
alts(p, ["Amara", "Ben", "Chen"])
for cid, name in [("depth", "Technical depth"), ("design", "System design"), ("comm", "Communication"), ("fit", "Team fit")]:
    crit(p, cid, name, method="pairwise")
ids = ["depth", "design", "comm", "fit"]
weigh(p, "lead", "criteria:root", ids, [0.4, 0.3, 0.15, 0.15])
weigh(p, "pm", "criteria:root", ids, [0.15, 0.2, 0.4, 0.25])
cands = ["o1", "o2", "o3"]
lead = {"depth": [0.5, 0.3, 0.2], "design": [0.4, 0.4, 0.2], "comm": [0.3, 0.2, 0.5], "fit": [0.35, 0.3, 0.35]}
pm = {"depth": [0.45, 0.3, 0.25], "design": [0.35, 0.35, 0.3], "comm": [0.2, 0.15, 0.65], "fit": [0.25, 0.25, 0.5]}
for cid in ids:
    weigh(p, "lead", f"alts:{cid}", cands, lead[cid])
    weigh(p, "pm", f"alts:{cid}", cands, pm[cid])
write("samples", "hiring", p)

# 3. Family car: small, mostly measured.
p = project("01SAMPLEFAMILYCAR000000000", "Buying a family car",
            "A quick one: three cars, four criteria, mostly facts from the brochures.")
p["evaluators"] = [{"id": "e1", "name": "Me", "weight": 1}]
alts(p, ["Toyota Corolla Touring", "Skoda Octavia Estate", "Kia Niro"])
crit(p, "price", "Price", method="value", direction="lower", unit="EUR")
crit(p, "safety", "Safety", method="rating", notes="Euro NCAP adult occupant scores, mapped to 1–10.")
crit(p, "boot", "Boot space", method="value", direction="higher", unit="litres")
crit(p, "running", "Running cost", method="value", direction="lower", unit="EUR a year")
weigh(p, "e1", "criteria:root", ["price", "safety", "boot", "running"], [0.3, 0.35, 0.15, 0.2])
rate(p, "e1", "safety", [9, 9, 8])
measure(p, "price", [31990, 32500, 34990])
measure(p, "boot", [596, 640, 451])
measure(p, "running", [1450, 1600, 1150])
write("samples", "family-car", p)

# Templates: structure only.
def template(slug, pid, name, desc, tree):
    t = project(pid, name, desc)
    t["evaluators"] = [{"id": "e1", "name": "Me", "weight": 1}]
    for k, (group, kids) in enumerate(tree):
        gid = f"g{k+1}"
        if kids:
            crit(t, gid, group)
            for m, kid in enumerate(kids):
                crit(t, f"{gid}c{m+1}", kid, gid, "pairwise")
        else:
            crit(t, gid, group, method="pairwise")
    write("templates", slug, t)

template("vendor-selection", "01TEMPLATEVENDORSELECTION0", "Vendor selection",
         "The 23 criteria of the original Priority Matrix spreadsheet, grouped so you only compare criteria that belong together.",
         [("Technical", ["Accessibility", "Availability", "Capacity", "Data accuracy", "Performance", "Reliability", "Scalability", "Security"]),
          ("Fit", ["Functionality", "Flexibility", "Extensibility", "Integration", "Useability", "Globalization", "Standardization", "Simplification"]),
          ("Delivery", ["Implementation", "Migration", "Development", "Manageability / Supportability"]),
          ("Commercial", ["Cost", "Economy", "Vendor"])])
template("software-tool", "01TEMPLATESOFTWARETOOL0000", "Software tool choice",
         "For choosing a library, SaaS product or internal tool.",
         [("Capabilities", ["Features we need", "Performance", "Integrations"]), ("Adoption", ["Learning curve", "Documentation", "Community"]),
          ("Cost", []), ("Risk", ["Vendor stability", "Security", "Lock-in"])])
template("job-offer", "01TEMPLATEJOBOFFER00000000", "Job offer comparison",
         "Weigh offers on more than salary.",
         [("Pay and benefits", ["Salary", "Equity", "Benefits"]), ("The work", ["Interesting problems", "Growth", "Team"]),
          ("Life", ["Commute or remote", "Hours", "Location"])])
print("ok")
```

```bash
mkdir -p internal/samples/samples internal/samples/templates
python3 /tmp/gensamples.py internal/samples
```
Expected: three files in `internal/samples/samples/`, `cloud-provider.ahp.json`, `family-car.ahp.json` and `hiring.ahp.json`, and three in `internal/samples/templates/`, `job-offer`, `software-tool` and `vendor-selection` (all `.ahp.json`).

- [ ] **Step 2: Write the failing test `internal/samples/samples_test.go`** (complete file)

```go
package samples

import (
	"errors"
	"testing"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

func TestSamplesAreCompleteAndConsistent(t *testing.T) {
	list := Samples()
	if len(list) != 3 {
		t.Fatalf("got %d samples, want 3", len(list))
	}
	for _, e := range list {
		p, err := Sample(e.ID)
		if err != nil {
			t.Fatal(err)
		}
		before := len(p.Pairwise) + len(p.Ratings) + len(p.Values)
		if err := p.Prepare(); err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		if after := len(p.Pairwise) + len(p.Ratings) + len(p.Values); after != before {
			t.Errorf("%s: Prepare dropped %d judgments", e.ID, before-after)
		}
		r, err := model.Compute(p, model.AllEvaluators)
		if err != nil {
			t.Fatal(err)
		}
		if r.Completeness != 1 || !r.Consistent || len(r.Issues) != 0 {
			t.Errorf("%s: completeness %v, consistent %v, issues %+v", e.ID, r.Completeness, r.Consistent, r.Issues)
		}
		if e.Alternatives != len(p.Alternatives) || e.Criteria != len(p.Criteria) || e.Name != p.Name {
			t.Errorf("%s: listing %+v doesn't match the file", e.ID, e)
		}
	}
}

func TestSampleCopiesAreNewProjects(t *testing.T) {
	a, _ := Sample("cloud-provider")
	b, _ := Sample("cloud-provider")
	if a.ID == b.ID || !model.ValidID(a.ID) || a.CreatedAt.IsZero() {
		t.Errorf("copies share an id or lack a creation time: %q %q", a.ID, b.ID)
	}
	if _, err := Sample("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown sample: err = %v", err)
	}
}

func TestTemplatesAreStructureOnly(t *testing.T) {
	list := Templates()
	if len(list) != 3 {
		t.Fatalf("got %d templates, want 3", len(list))
	}
	for _, e := range list {
		p, err := Template(e.ID, "My decision")
		if err != nil {
			t.Fatal(err)
		}
		if p.Name != "My decision" || len(p.Alternatives) != 0 || len(p.Pairwise) != 0 || len(p.Criteria) == 0 {
			t.Errorf("%s: %+v", e.ID, p)
		}
		if err := p.Prepare(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
	}
	vendor, _ := Template("vendor-selection", "")
	leaves := 0
	for _, c := range vendor.Criteria {
		if c.ParentID != "" {
			leaves++
		}
	}
	if leaves != 23 || vendor.Name != "Vendor selection" {
		t.Errorf("vendor template has %d criteria under groups and name %q; want the spreadsheet's 23", leaves, vendor.Name)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/samples`
Expected: FAIL — `undefined: Samples`.

- [ ] **Step 4: Write `internal/samples/samples.go`** (complete file)

```go
// Package samples bundles example decisions to explore and templates to
// start from.
package samples

import (
	"embed"
	"errors"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

//go:embed samples/*.ahp.json templates/*.ahp.json
var files embed.FS

// ErrNotFound means there is no sample or template with that id.
var ErrNotFound = errors.New("no such sample or template")

// Entry describes a sample or template for a picker.
type Entry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Alternatives int    `json:"alternatives"`
	Criteria     int    `json:"criteria"`
}

func Samples() []Entry   { return list("samples") }
func Templates() []Entry { return list("templates") }

// Sample returns a fresh copy of a sample decision, with its own id.
func Sample(id string) (*model.Project, error) { return load("samples", id) }

// Template returns a new decision with a template's criteria and no options.
// An empty name keeps the template's name.
func Template(id, name string) (*model.Project, error) {
	p, err := load("templates", id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) != "" {
		p.Name = name
		p.Description = ""
	}
	return p, nil
}

func list(dir string) []Entry {
	names, _ := files.ReadDir(dir)
	out := []Entry{}
	for _, n := range names {
		id := strings.TrimSuffix(n.Name(), ".ahp.json")
		if p, err := load(dir, id); err == nil {
			out = append(out, Entry{ID: id, Name: p.Name, Description: p.Description, Alternatives: len(p.Alternatives), Criteria: len(p.Criteria)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func load(dir, id string) (*model.Project, error) {
	if strings.ContainsAny(id, `/\.`) {
		return nil, ErrNotFound
	}
	data, err := files.ReadFile(path.Join(dir, id+".ahp.json"))
	if err != nil {
		return nil, ErrNotFound
	}
	p, err := model.Decode(data)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p.ID, p.CreatedAt, p.UpdatedAt, p.Version = model.NewProjectID(), now, now, 0
	return p, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/samples`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
make lint && make test
git add internal/samples/samples internal/samples/samples.go internal/samples/samples_test.go internal/samples/templates
git commit -m "feat(samples): bundled example decisions and templates"
```

---

### Task 6: HTTP endpoints for import, export, packets, merge, samples and templates

**Files:**
- Create: `internal/server/exchange.go`
- Modify: `internal/server/projects.go`, `internal/server/server.go`
- Test: `internal/server/exchange_test.go`

**Interfaces:**
- Consumes: Tasks 1–5.
- Produces:
-   - `GET /api/samples` and `GET /api/templates`
-   - `POST /api/projects {name?, fromSample?, fromTemplate?}`
-   - `POST /api/import/xlsx` (multipart), returning `{project, report}`; errors are 400 `bad-workbook`
-   - `GET /api/projects/{id}/export.xlsx` and `.csv`, with `?evaluator=`
-   - `GET /api/projects/{id}/export.ahp.json?evaluator=<id>`, returning that person's packet
-   - `POST /api/projects/{id}/merge[?existing=replace|skip]`, returning `{project, report}`; errors are 409 `evaluator-exists` with `details.evaluators`, 400 `different-project`, and 409 `conflict`
-   - helpers `create`, `readUploadNamed`, `loadResults` and `attachment`

- [ ] **Step 1: Write the failing test `internal/server/exchange_test.go`** (complete file)

```go
package server

import (
	"bytes"
	"encoding/csv"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// upload sends data as the multipart "file" field.
func upload(t *testing.T, h http.Handler, target, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = mw.Close()
	return request(t, h, "POST", target, &buf, "Content-Type", mw.FormDataContentType())
}

func TestSamplesAndTemplates(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "GET", "/api/samples", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":"cloud-provider"`) {
		t.Fatalf("samples: %d %s", rec.Code, rec.Body.String())
	}
	rec = request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"cloud-provider"}`))
	p := decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || p.Name != "Choosing a cloud provider" || len(p.Pairwise) == 0 {
		t.Errorf("from sample: %d %+v", rec.Code, p.Name)
	}

	rec = request(t, h, "GET", "/api/templates", nil)
	if !strings.Contains(rec.Body.String(), `"id":"vendor-selection"`) {
		t.Fatalf("templates: %s", rec.Body.String())
	}
	rec = request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromTemplate":"vendor-selection","name":"CRM vendors"}`))
	p = decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || p.Name != "CRM vendors" || len(p.Criteria) != 27 {
		t.Errorf("from template: %d %q with %d criteria", rec.Code, p.Name, len(p.Criteria))
	}

	rec = request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"../../etc/passwd"}`))
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not-found" {
		t.Errorf("unknown sample: %d %s", rec.Code, rec.Body.String())
	}
}

func legacyBytes(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "Summary")
	_, _ = f.NewSheet("Criteria Summary")
	_, _ = f.NewSheet("Criterion 1")
	_ = f.SetCellValue("Summary", "C19", "Alpha")
	_ = f.SetCellValue("Summary", "C20", "Beta")
	_ = f.SetCellValue("Summary", "D7", "Price")
	_ = f.SetCellValue("Criterion 1", "D6", 5)
	var buf bytes.Buffer
	_ = f.Write(&buf)
	return buf.Bytes()
}

func TestImportLegacyWorkbook(t *testing.T) {
	h, _ := newTestServer(t)
	rec := upload(t, h, "/api/import/xlsx", "Priority Matrix - CRM.xlsx", legacyBytes(t))
	body := decode[struct {
		Project *model.Project
		Report  struct{ Alternatives, Criteria, Judgments int }
	}](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || body.Project.Name != "Priority Matrix - CRM" || body.Report.Judgments != 1 {
		t.Errorf("import: %d %s", rec.Code, rec.Body.String())
	}
	rec = upload(t, h, "/api/import/xlsx", "notes.xlsx", []byte("hello"))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "bad-workbook" {
		t.Errorf("junk: %d %s", rec.Code, rec.Body.String())
	}
}

func TestExportWorkbookAndCSV(t *testing.T) {
	h, _ := newTestServer(t)
	p := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"family-car"}`)).Body.Bytes())

	rec := request(t, h, "GET", "/api/projects/"+p.ID+"/export.xlsx", nil)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "buying-a-family-car.xlsx") {
		t.Fatalf("xlsx: %d %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}
	if _, err := excelize.OpenReader(rec.Body); err != nil {
		t.Errorf("xlsx doesn't open: %v", err)
	}

	rec = request(t, h, "GET", "/api/projects/"+p.ID+"/export.csv", nil)
	rows, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil || len(rows) != 4 || rows[0][0] != "Rank" || rows[1][0] != "1" {
		t.Errorf("csv: %v %v", rows, err)
	}
	if rec.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Errorf("csv content type %q", rec.Header().Get("Content-Type"))
	}
	rec = request(t, h, "GET", "/api/projects/"+p.ID+"/export.csv?evaluator=nobody", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown evaluator: %d", rec.Code)
	}
}

func TestPacketAndMergeRoundTrip(t *testing.T) {
	h, _ := newTestServer(t)
	owner := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"hiring"}`)).Body.Bytes())

	rec := request(t, h, "GET", "/api/projects/"+owner.ID+"/export.ahp.json?evaluator=pm", nil)
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "hiring-a-senior-engineer-for-product-manager.ahp.json") {
		t.Errorf("packet filename: %q", cd)
	}
	packet := decode[*model.Project](t, rec.Body.Bytes())
	if len(packet.Evaluators) != 1 || packet.Evaluators[0].ID != "pm" {
		t.Fatalf("packet evaluators = %+v", packet.Evaluators)
	}
	// The product manager changes one answer and sends the file back.
	packet.Pairwise[0].Value = 1.0 / packet.Pairwise[0].Value
	data := mustJSON(t, packet)

	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge", "back.ahp.json", readAll(t, data))
	if rec.Code != http.StatusConflict || errorCode(t, rec) != "evaluator-exists" || !strings.Contains(rec.Body.String(), "Product manager") {
		t.Fatalf("merge without a choice: %d %s", rec.Code, rec.Body.String())
	}
	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=replace", "back.ahp.json", readAll(t, mustJSON(t, packet)))
	body := decode[struct {
		Project *model.Project
		Report  model.MergeReport
	}](t, rec.Body.Bytes())
	if rec.Code != 200 || len(body.Report.Replaced) != 1 || body.Project.Version != owner.Version+1 {
		t.Errorf("merge: %d %s", rec.Code, rec.Body.String())
	}

	other := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"family-car"}`)).Body.Bytes())
	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=replace", "car.ahp.json", readAll(t, mustJSON(t, other)))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "different-project" {
		t.Errorf("merging another decision: %d %s", rec.Code, rec.Body.String())
	}
	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=sometimes", "back.ahp.json", readAll(t, mustJSON(t, packet)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad policy: %d", rec.Code)
	}
}

func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/server`
Expected: FAIL — 404s and a `Method Not Allowed` for the new routes.

- [ ] **Step 3: Write `internal/server/exchange.go`** (complete file)

```go
package server

import (
	"bytes"
	"encoding/csv"
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Xafloc/ahp-matrix/internal/model"
	"github.com/Xafloc/ahp-matrix/internal/samples"
	"github.com/Xafloc/ahp-matrix/internal/store"
	"github.com/Xafloc/ahp-matrix/internal/xlsx"
)

func (s *server) exchangeRoutes() {
	s.mux.HandleFunc("GET /api/samples", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, samples.Samples()) })
	s.mux.HandleFunc("GET /api/templates", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, samples.Templates()) })
	s.mux.HandleFunc("POST /api/import/xlsx", s.importWorkbook)
	s.mux.HandleFunc("GET /api/projects/{id}/export.xlsx", s.exportWorkbook)
	s.mux.HandleFunc("GET /api/projects/{id}/export.csv", s.exportCSV)
	s.mux.HandleFunc("POST /api/projects/{id}/merge", s.mergeProject)
}

// importWorkbook creates a project from a workbook made with the original
// Priority Matrix template, named after the uploaded file.
func (s *server) importWorkbook(w http.ResponseWriter, r *http.Request) {
	data, name, ok := readUploadNamed(w, r)
	if !ok {
		return
	}
	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if name == "" || name == "." {
		name = "Imported decision"
	}
	p, rep, err := xlsx.ImportLegacy(bytes.NewReader(data), name)
	switch {
	case errors.Is(err, xlsx.ErrNotWorkbook), errors.Is(err, xlsx.ErrNotTemplate):
		writeError(w, http.StatusBadRequest, "bad-workbook", err.Error(), nil)
		return
	case err != nil:
		s.internalError(w, err)
		return
	}
	if !prepare(w, p) || !s.create(w, r, p) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": p, "report": rep})
}

func (s *server) exportWorkbook(w http.ResponseWriter, r *http.Request) {
	p, res, ok := s.loadResults(w, r)
	if !ok {
		return
	}
	var buf bytes.Buffer
	if err := xlsx.Export(&buf, p, res); err != nil {
		s.internalError(w, err)
		return
	}
	attachment(w, model.Slug(p.Name)+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	_, _ = w.Write(buf.Bytes())
}

func (s *server) exportCSV(w http.ResponseWriter, r *http.Request) {
	p, res, ok := s.loadResults(w, r)
	if !ok {
		return
	}
	header, rows := xlsx.ResultsTable(p, res)
	attachment(w, model.Slug(p.Name)+".csv", "text/csv; charset=utf-8")
	cw := csv.NewWriter(w)
	_ = cw.Write(header)
	for _, row := range rows {
		rec := make([]string, len(row))
		for i, v := range row {
			switch v := v.(type) {
			case float64:
				rec[i] = strconv.FormatFloat(v, 'f', 6, 64)
			case int:
				rec[i] = strconv.Itoa(v)
			default:
				rec[i] = v.(string)
			}
		}
		_ = cw.Write(rec)
	}
	cw.Flush()
}

// mergeProject brings the judgments from a returned file into the project.
// ?existing=replace|skip says what to do about people who already have
// judgments; without it, such a merge is refused with 409 evaluator-exists so
// the person can choose.
func (s *server) mergeProject(w http.ResponseWriter, r *http.Request) {
	policy := model.MergePolicy(r.URL.Query().Get("existing"))
	if policy != model.MergeAsk && policy != model.MergeReplace && policy != model.MergeSkip {
		writeError(w, http.StatusBadRequest, "bad-request", "existing must be replace or skip", nil)
		return
	}
	owner, ok := s.load(w, r)
	if !ok {
		return
	}
	data, ok := readUpload(w, r)
	if !ok {
		return
	}
	packet, ok := decodeProject(w, data)
	if !ok {
		return
	}
	expected := owner.Version
	rep, err := model.Merge(owner, packet, policy)
	var conflict *model.EvaluatorConflictError
	var invalid *model.ValidationError
	switch {
	case errors.Is(err, model.ErrDifferentProject):
		writeError(w, http.StatusBadRequest, "different-project", err.Error(), nil)
		return
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, "evaluator-exists", err.Error(), map[string][]string{"evaluators": conflict.Names})
		return
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid", invalid.Error(), invalid.Problems)
		return
	case err != nil:
		s.internalError(w, err)
		return
	}
	err = s.cfg.Store.Save(r.Context(), owner, expected)
	var ce *store.ConflictError
	switch {
	case errors.As(err, &ce):
		writeError(w, http.StatusConflict, "conflict", "this project was changed while merging; try again", map[string]int{"currentVersion": ce.Current})
	case err != nil:
		s.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"project": owner, "report": rep})
	}
}

// loadResults loads the project in the URL and computes results for ?evaluator= (default everyone).
func (s *server) loadResults(w http.ResponseWriter, r *http.Request) (*model.Project, *model.Results, bool) {
	p, ok := s.load(w, r)
	if !ok {
		return nil, nil, false
	}
	evaluator := r.URL.Query().Get("evaluator")
	if evaluator == "" {
		evaluator = model.AllEvaluators
	}
	res, err := model.Compute(p, evaluator)
	switch {
	case errors.Is(err, model.ErrUnknownEvaluator):
		writeError(w, http.StatusBadRequest, "unknown-evaluator", err.Error(), nil)
		return nil, nil, false
	case err != nil:
		s.internalError(w, err)
		return nil, nil, false
	}
	return p, res, true
}

func attachment(w http.ResponseWriter, filename, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
}
```

- [ ] **Step 4: Write `internal/server/projects.go`** (complete file)

```go
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/model"
	"github.com/Xafloc/ahp-matrix/internal/samples"
	"github.com/Xafloc/ahp-matrix/internal/store"
)

func (s *server) projectRoutes() {
	s.mux.HandleFunc("GET /api/projects", s.listProjects)
	s.mux.HandleFunc("POST /api/projects", s.createProject)
	s.mux.HandleFunc("GET /api/projects/{id}", s.getProject)
	s.mux.HandleFunc("PUT /api/projects/{id}", s.saveProject)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)
	s.mux.HandleFunc("POST /api/projects/{id}/duplicate", s.duplicateProject)
	s.mux.HandleFunc("GET /api/projects/{id}/results", s.projectResults)
	s.mux.HandleFunc("GET /api/projects/{id}/export.ahp.json", s.exportProject)
	s.mux.HandleFunc("POST /api/import/ahp", s.importProject)
}

// projectSummary is a library card: enough to list projects without loading them.
type projectSummary struct {
	ID           string       `json:"id,omitempty"`
	File         string       `json:"file"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	Alternatives int          `json:"alternatives"`
	Criteria     int          `json:"criteria"`
	Ranking      []rankedName `json:"ranking"` // best first
	Completeness float64      `json:"completeness"`
	Consistent   bool         `json:"consistent"`
	Error        string       `json:"error,omitempty"` // the file could not be read
}

type rankedName struct {
	Name  string  `json:"name"`
	Color string  `json:"color"`
	Score float64 `json:"score"`
}

func (s *server) listProjects(w http.ResponseWriter, r *http.Request) {
	entries, err := s.cfg.Store.List(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := []projectSummary{}
	for _, e := range entries {
		if e.Err != nil {
			out = append(out, projectSummary{File: e.File, Name: e.File, Ranking: []rankedName{}, Error: e.Err.Error()})
			continue
		}
		out = append(out, summarize(e))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Error == "") != (out[j].Error == "") {
			return out[i].Error == "" // readable projects first
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"projects": out})
}

func summarize(e store.Entry) projectSummary {
	p := e.Project
	sum := projectSummary{
		ID: p.ID, File: e.File, Name: p.Name, Description: p.Description, UpdatedAt: p.UpdatedAt,
		Alternatives: len(p.Alternatives), Criteria: len(p.Criteria), Ranking: []rankedName{},
	}
	res, err := model.Compute(p, model.AllEvaluators)
	if err != nil {
		sum.Error = err.Error()
		return sum
	}
	sum.Completeness, sum.Consistent = res.Completeness, res.Consistent
	for i, sc := range res.Scores {
		a := p.Alternatives[i]
		sum.Ranking = append(sum.Ranking, rankedName{Name: a.Name, Color: a.Color, Score: sc.Score})
	}
	sort.SliceStable(sum.Ranking, func(i, j int) bool { return sum.Ranking[i].Score > sum.Ranking[j].Score })
	return sum
}

func (s *server) createProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		FromSample   string `json:"fromSample"`
		FromTemplate string `json:"fromTemplate"`
	}
	if r.ContentLength != 0 {
		data, ok := readBody(w, r)
		if !ok {
			return
		}
		if err := json.Unmarshal(data, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad-json", err.Error(), nil)
			return
		}
	}
	var p *model.Project
	var err error
	switch {
	case req.FromSample != "":
		p, err = samples.Sample(req.FromSample)
	case req.FromTemplate != "":
		p, err = samples.Template(req.FromTemplate, strings.TrimSpace(req.Name))
	default:
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "Untitled decision"
		}
		p = model.New(name)
		p.Description = req.Description
	}
	if errors.Is(err, samples.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not-found", err.Error(), nil)
		return
	} else if err != nil {
		s.internalError(w, err)
		return
	}
	s.createNew(w, r, p, http.StatusCreated)
}

func (s *server) getProject(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.load(w, r); ok {
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *server) saveProject(w http.ResponseWriter, r *http.Request) {
	data, ok := readBody(w, r)
	if !ok {
		return
	}
	p, ok := decodeProject(w, data)
	if !ok {
		return
	}
	if p.ID != r.PathValue("id") {
		writeError(w, http.StatusBadRequest, "id-mismatch", "the project id in the body doesn't match the URL", nil)
		return
	}
	if !prepare(w, p) {
		return
	}
	err := s.cfg.Store.Save(r.Context(), p, p.Version)
	var ce *store.ConflictError
	switch {
	case errors.As(err, &ce):
		writeError(w, http.StatusConflict, "conflict", "this project was changed somewhere else; reload it or overwrite", map[string]int{"currentVersion": ce.Current})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not-found", "project not found", nil)
	case err != nil:
		s.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *server) deleteProject(w http.ResponseWriter, r *http.Request) {
	err := s.cfg.Store.Delete(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not-found", "project not found", nil)
	case err != nil:
		s.internalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *server) duplicateProject(w http.ResponseWriter, r *http.Request) {
	p, ok := s.load(w, r)
	if !ok {
		return
	}
	cp, err := clone(p)
	if err != nil {
		s.internalError(w, err)
		return
	}
	cp.ID = model.NewProjectID()
	cp.Name = p.Name + " (copy)"
	cp.CreatedAt = time.Time{}
	s.createNew(w, r, cp, http.StatusCreated)
}

func (s *server) projectResults(w http.ResponseWriter, r *http.Request) {
	p, ok := s.load(w, r)
	if !ok {
		return
	}
	evaluator := r.URL.Query().Get("evaluator")
	if evaluator == "" {
		evaluator = model.AllEvaluators
	}
	res, err := model.Compute(p, evaluator)
	switch {
	case errors.Is(err, model.ErrUnknownEvaluator):
		writeError(w, http.StatusBadRequest, "unknown-evaluator", err.Error(), nil)
	case err != nil:
		s.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}

func (s *server) exportProject(w http.ResponseWriter, r *http.Request) {
	p, ok := s.load(w, r)
	if !ok {
		return
	}
	filename := model.Slug(p.Name) + ".ahp.json"
	// ?evaluator= exports that person's packet: everything but other people's judgments.
	if ev := r.URL.Query().Get("evaluator"); ev != "" {
		packet, err := model.Packet(p, ev)
		if errors.Is(err, model.ErrUnknownEvaluator) {
			writeError(w, http.StatusBadRequest, "unknown-evaluator", err.Error(), nil)
			return
		} else if err != nil {
			s.internalError(w, err)
			return
		}
		filename = model.Slug(p.Name) + "-for-" + model.Slug(packet.Evaluators[0].Name) + ".ahp.json"
		p = packet
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	writeJSON(w, http.StatusOK, p)
}

// importProject accepts an .ahp.json file, as a multipart "file" field or as
// the raw body. It keeps the file's project id unless that id is taken or unsafe.
func (s *server) importProject(w http.ResponseWriter, r *http.Request) {
	data, ok := readUpload(w, r)
	if !ok {
		return
	}
	p, ok := decodeProject(w, data)
	if !ok {
		return
	}
	if !model.ValidID(p.ID) {
		p.ID = model.NewProjectID()
	} else if _, err := s.cfg.Store.Get(r.Context(), p.ID); err == nil {
		p.ID = model.NewProjectID()
	}
	p.CreatedAt = time.Time{}
	s.createNew(w, r, p, http.StatusCreated)
}

// load fetches the project named in the URL, writing a 404 if it is missing.
func (s *server) load(w http.ResponseWriter, r *http.Request) (*model.Project, bool) {
	p, err := s.cfg.Store.Get(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not-found", "project not found", nil)
		return nil, false
	case err != nil:
		s.internalError(w, err)
		return nil, false
	}
	return p, true
}

// createNew prepares and creates a new project, then writes it back with
// status. If the id turns out to be taken (or might belong to a file this
// build can't read), the project gets a fresh id instead.
func (s *server) createNew(w http.ResponseWriter, r *http.Request, p *model.Project, status int) {
	if prepare(w, p) && s.create(w, r, p) {
		writeJSON(w, status, p)
	}
}

// create stores a prepared new project, writing an error response and
// returning false if it can't.
func (s *server) create(w http.ResponseWriter, r *http.Request, p *model.Project) bool {
	err := s.cfg.Store.Create(r.Context(), p)
	if errors.Is(err, store.ErrExists) {
		p.ID = model.NewProjectID()
		err = s.cfg.Store.Create(r.Context(), p)
	}
	if err != nil {
		s.internalError(w, err)
		return false
	}
	return true
}

func prepare(w http.ResponseWriter, p *model.Project) bool {
	err := p.Prepare()
	var ve *model.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusUnprocessableEntity, "invalid", ve.Error(), ve.Problems)
		return false
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, "invalid", err.Error(), nil)
		return false
	}
	return true
}

func decodeProject(w http.ResponseWriter, data []byte) (*model.Project, bool) {
	p, err := model.Decode(data)
	var fv *model.FutureVersionError
	switch {
	case errors.As(err, &fv):
		writeError(w, http.StatusUnprocessableEntity, "unsupported-version", err.Error(), nil)
		return nil, false
	case err != nil:
		writeError(w, http.StatusBadRequest, "bad-project", err.Error(), nil)
		return nil, false
	}
	return p, true
}

// readBody reads the whole request body, answering 413 when it is too large.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		bodyError(w, err)
		return nil, false
	}
	return data, true
}

// readUpload returns the multipart "file" field, or the raw body otherwise.
func readUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	data, _, ok := readUploadNamed(w, r)
	return data, ok
}

// readUploadNamed is readUpload that also returns the uploaded file's name
// ("" for a raw body).
func readUploadNamed(w http.ResponseWriter, r *http.Request) ([]byte, string, bool) {
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct != "multipart/form-data" {
		data, ok := readBody(w, r)
		return data, "", ok
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		bodyError(w, err)
		return nil, "", false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		bodyError(w, err)
		return nil, "", false
	}
	return data, header.Filename, true
}

func bodyError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "too-large", fmt.Sprintf("uploads are limited to %d MB", maxBody>>20), nil)
		return
	}
	writeError(w, http.StatusBadRequest, "bad-request", err.Error(), nil)
}

func clone(p *model.Project) (*model.Project, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var cp model.Project
	err = json.Unmarshal(data, &cp)
	return &cp, err
}

func (s *server) internalError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, "internal", err.Error(), nil)
}
```

- [ ] **Step 5: Write `internal/server/server.go`** (complete file)

```go
// Package server exposes projects over a JSON HTTP API and serves the web UI.
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/Xafloc/ahp-matrix/internal/store"
)

// Config configures the HTTP handler.
type Config struct {
	Store   store.Store
	UI      fs.FS  // built web assets; a build without index.html serves a placeholder page
	Version string // shown by GET /api/version
	Host    string // the address the server listens on; also accepted in the Host header
}

type server struct {
	cfg Config
	mux *http.ServeMux
}

// New returns the handler for the API and the UI.
func New(cfg Config) http.Handler {
	s := &server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s.guard(s.mux)
}

func (s *server) routes() {
	s.mux.HandleFunc("GET /api/version", s.handleVersion)
	s.projectRoutes()
	s.exchangeRoutes()
	s.mux.Handle("GET /", s.static())
}

func (s *server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.cfg.Version})
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// writeJSON encodes v before sending any header, so a value that can't be
// encoded (such as NaN) becomes a 500 error rather than an empty success.
func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		status = http.StatusInternalServerError
		data, _ = json.Marshal(map[string]apiError{"error": {Code: "internal", Message: "could not encode the response: " + err.Error()}})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

// writeError sends {"error": {"code", "message", "details"}}.
func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: message, Details: details}})
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/server`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
make lint && make test
git add internal/server/exchange.go internal/server/exchange_test.go internal/server/projects.go internal/server/server.go
git commit -m "feat(server): import/export, packets, merge, samples and templates endpoints"
```

---

### Task 7: Lint: golangci-lint v2 and its findings

**Files:**
- Create: `.golangci.yml`
- Modify: `internal/model/decode.go`, `internal/model/scale.go`, `cmd/ahp-matrix/main.go`

**Interfaces:**
- Produces: `isIDRune(r)` in model; `main` stops signal handling before `os.Exit`; errors wrap both causes (`%w: %w`).

- [ ] **Step 1: Write the failing test `.golangci.yml`** (complete file)

```yaml
version: "2"

linters:
  default: standard
  enable:
    - bodyclose
    - errorlint
    - gocritic
    - misspell
    - nilerr
    - unconvert
  settings:
    errcheck:
      # Closing files we only read, or temporary files on an error path, can't
      # fail in a way that loses data.
      exclude-functions:
        - (*os.File).Close
        - (mime/multipart.File).Close
        - (*github.com/xuri/excelize/v2.File).Close
        - os.Remove
        # The startup banner goes to the terminal; there's nothing to do if that fails.
        - fmt.Fprintf
        - fmt.Fprintln
    gocritic:
      disabled-checks:
        - ifElseChain
  exclusions:
    rules:
      - path: _test\.go
        linters:
          - errcheck

formatters:
  enable:
    - gofmt
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...`
Expected: FAIL — the error-wrapping (`errorlint`), `exitAfterDefer` (`gocritic`) and De Morgan's law (`QF1001`) findings.

- [ ] **Step 3: Write `internal/model/decode.go`** (complete file)

```go
package model

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotProject means the data is not an AHP Matrix project document.
var ErrNotProject = errors.New("not an AHP Matrix project file")

// FutureVersionError means the file was written by a newer build.
type FutureVersionError struct{ Version int }

func (e *FutureVersionError) Error() string {
	return fmt.Sprintf("project file uses schema version %d, but this build supports up to %d; upgrade ahp-matrix to open it", e.Version, SchemaVersion)
}

// Decode parses a project document. Older schema versions are migrated to
// SchemaVersion; newer ones are rejected with *FutureVersionError.
func Decode(data []byte) (*Project, error) {
	var head struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotProject, err)
	}
	switch {
	case head.SchemaVersion <= 0:
		return nil, ErrNotProject
	case head.SchemaVersion > SchemaVersion:
		return nil, &FutureVersionError{Version: head.SchemaVersion}
	}
	// Version 1 is current. A future version 2 adds a case here that decodes
	// the version 1 shape and converts it.
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotProject, err)
	}
	return &p, nil
}
```

- [ ] **Step 4: Write `internal/model/scale.go`** (complete file)

```go
package model

import (
	"crypto/rand"
	"encoding/base32"
	"strings"

	"github.com/oklog/ulid/v2"
)

// Scale is the set of values a pairwise judgment may take.
type Scale string

const (
	ScaleSaaty9   Scale = "saaty9"   // 1–9 and reciprocals
	ScaleClassic5 Scale = "classic5" // 10, 5, 1, 1/5, 1/10 from the original spreadsheet
)

// Valid reports whether s is a known scale.
func (s Scale) Valid() bool { return s == ScaleSaaty9 || s == ScaleClassic5 }

// Allowed returns the values the scale permits, ascending.
func (s Scale) Allowed() []float64 {
	if s == ScaleClassic5 {
		return []float64{0.1, 0.2, 1, 5, 10}
	}
	out := make([]float64, 0, 17)
	for k := 9; k >= 2; k-- {
		out = append(out, 1/float64(k))
	}
	for k := 1; k <= 9; k++ {
		out = append(out, float64(k))
	}
	return out
}

const rootKey = "root"

// CriteriaContext names the comparison of the children of parentID (empty for
// the top-level criteria).
func CriteriaContext(parentID string) string {
	if parentID == "" {
		return "criteria:" + rootKey
	}
	return "criteria:" + parentID
}

// AltsContext names the comparison of alternatives on a leaf criterion.
func AltsContext(criterionID string) string { return "alts:" + criterionID }

// ParseContext splits a context into its kind ("criteria" or "alts") and its
// criterion ID, which is empty for the top-level criteria group.
func ParseContext(c string) (kind, id string, ok bool) {
	kind, id, found := strings.Cut(c, ":")
	if !found || id == "" {
		return "", "", false
	}
	switch kind {
	case "criteria":
		if id == rootKey {
			id = ""
		}
		return kind, id, true
	case "alts":
		return kind, id, true
	}
	return "", "", false
}

// NewProjectID returns a new, time-ordered ID for a project.
func NewProjectID() string { return ulid.Make().String() }

// ValidID reports whether id is safe to use as a project id: it ends up in
// file names and URLs, so only letters, digits, '-' and '_' are allowed.
func ValidID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !isIDRune(r) {
			return false
		}
	}
	return true
}

func isIDRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
}

// NewID returns a short random ID for an item inside a project.
func NewID() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return strings.ToLower(base32.StdEncoding.EncodeToString(b[:]))
}

// Slug turns a name into a short, file-name-safe string.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if b.Len() >= 40 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	if s := strings.TrimRight(b.String(), "-"); s != "" {
		return s
	}
	return "project"
}
```

- [ ] **Step 5: Write `cmd/ahp-matrix/main.go`** (complete file)

```go
// Command ahp-matrix starts a local web app for AHP decision making.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/server"
	"github.com/Xafloc/ahp-matrix/internal/store"
	"github.com/Xafloc/ahp-matrix/web"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ahp-matrix:", err)
		os.Exit(1)
	}
}

// run starts the server and blocks until ctx is cancelled.
func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("ahp-matrix", flag.ContinueOnError)
	flags.SetOutput(out)
	host := flags.String("host", "127.0.0.1", "address to listen on; there is no login, so keep it local")
	port := flags.Int("port", 7777, "port to listen on; a free port is used if it is busy")
	dataDir := flags.String("data-dir", "", "folder for project files (default: your user data folder)")
	noOpen := flags.Bool("no-open", false, "don't open a browser")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	dir := *dataDir
	if dir == "" {
		d, err := defaultDataDir()
		if err != nil {
			return err
		}
		dir = d
	}
	st, err := store.NewFileStore(dir)
	if err != nil {
		return err
	}
	ln, err := listen(*host, *port)
	if err != nil {
		return err
	}
	url := "http://" + net.JoinHostPort(browseHost(*host), strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))

	srv := &http.Server{
		Handler:           server.New(server.Config{Store: st, UI: web.FS(), Version: version, Host: *host}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(out, "AHP Matrix %s\n  projects: %s\n  open:     %s\n", version, dir, url)
	if ip := net.ParseIP(*host); *host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		fmt.Fprintf(out, "  warning:  listening on %s, and there is no login; anyone who can reach this address can edit your projects\n", *host)
	}
	if !*noOpen {
		_ = openBrowser(url)
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}

// listen opens host:port, falling back to any free port if that one is taken.
func listen(host string, port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err == nil || port == 0 {
		return ln, err
	}
	return net.Listen("tcp", net.JoinHostPort(host, "0"))
}

// browseHost is the host to put in the printed URL.
func browseHost(host string) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	return host
}

// defaultDataDir is ~/Library/Application Support/ahp-matrix on macOS,
// %AppData%\ahp-matrix on Windows, and $XDG_DATA_HOME/ahp-matrix (default
// ~/.local/share/ahp-matrix) elsewhere.
func defaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin", "windows":
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(d, "ahp-matrix"), nil
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "ahp-matrix"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "ahp-matrix"), nil
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
make lint && make test
git add .golangci.yml cmd/ahp-matrix/main.go internal/model/decode.go internal/model/scale.go
git commit -m "chore: golangci-lint v2 with its findings fixed"
```

---

### Task 8: UI: examples, templates, spreadsheet import, downloads, packets, merge and polish

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/format.ts`
- Modify: `web/src/lib/edits.ts`
- Modify: `web/src/lib/sensitivity.ts`
- Modify: `web/src/lib/session.svelte.ts`
- Modify: `web/src/components/Balance.svelte`
- Modify: `web/src/components/charts/LineChart.svelte`
- Modify: `web/src/components/Library.svelte`
- Modify: `web/src/components/Setup.svelte`
- Modify: `web/src/components/Results.svelte`
- Modify: `web/src/components/Sensitivity.svelte`
- Modify: `web/src/components/Workspace.svelte`
- Modify: `web/src/components/Weigh.svelte`
- Modify: `web/src/components/Report.svelte`
- Test: `web/src/lib/format.test.ts`
- Test: `web/src/lib/edits.test.ts`
- Test: `web/src/lib/sensitivity.test.ts`
- Test: `web/src/lib/session.test.ts`
- Test: `web/src/components/charts/charts.test.ts`
- Test: `web/e2e/exchange.spec.ts`
- Modify: `web/e2e/robustness.spec.ts`
- Create: `web/e2e/fixtures/Priority Matrix - Example.xlsx` (generated)

**Interfaces:**
- Consumes: the Task 6 endpoints and the Task 2 types.
- Produces:
-   - `Client` gains `createProject(name, {sample?, template?})`, `importFile(file)` (returns `{project, report?}`), `mergeFile(id, file, existing?)`, `samples()` and `templates()`
-   - `exportUrl(id, kind = 'ahp.json', evaluator?)`
-   - `count(n, one, many?)`, `setEvaluatorWeight(p, id, w)`, `shiftLabel(stability)`; `setJudgment` keeps notes
-   - `ProjectSession` ignores results responses that arrive out of order
-   - the balance's ↑/↓ act like →/←
-   - the line chart labels up to eight lines directly

- [ ] **Step 1: Write the failing test `web/src/lib/format.test.ts`** (complete file)

```ts
import { expect, test } from 'vitest';
import { colorOf, count, percent, relativeTime } from './format';

test('percent', () => {
  expect(percent(0.4567)).toBe('45.7%');
  expect(percent(0.05, 0)).toBe('5%');
});

test('colorOf maps series slots to tokens and passes hex through', () => {
  expect(colorOf('series-3')).toBe('var(--series-3)');
  expect(colorOf('#aabbcc')).toBe('#aabbcc');
  expect(colorOf('')).toBe('var(--series-none)');
  expect(colorOf('red; background:url(x)')).toBe('var(--series-none)');
});

test('relativeTime', () => {
  const now = Date.parse('2026-10-15T12:00:00Z');
  expect(relativeTime('2026-10-15T11:59:30Z', now)).toBe('just now');
  expect(relativeTime('2026-10-15T11:55:00Z', now)).toBe('5 minutes ago');
  expect(relativeTime('2026-10-14T11:00:00Z', now)).toBe('yesterday');
});

test('count pluralizes', () => {
  expect(count(1, 'criterion', 'criteria')).toBe('1 criterion');
  expect(count(6, 'criterion', 'criteria')).toBe('6 criteria');
  expect(count(0, 'option')).toBe('0 options');
});
```

- [ ] **Step 2: Write the failing test `web/src/lib/edits.test.ts`** (complete file)

```ts
import { expect, test } from 'vitest';
import {
  addAlternative,
  addCriterion,
  changeScale,
  getJudgment,
  indent,
  moveCriterion,
  outdent,
  removeAlternative,
  removeCriterion,
  removeEvaluator,
  setEvaluatorWeight,
  setJudgment,
  setRating,
  setValue,
  shift,
} from './edits';
import { fixture } from './fixture.test-helper';
import { childrenOf } from './tree';

const ids = (cs: { id: string }[]) => cs.map((c) => c.id);

test('new alternatives take the first free series color, then none', () => {
  const p = fixture();
  removeAlternative(p, 'a2');
  expect(addAlternative(p, ' Delta ').color).toBe('series-2');
  for (let i = 0; i < 5; i++) addAlternative(p, `X${i}`);
  expect(addAlternative(p, 'Ninth').color).toBe('');
  expect(p.alternatives.find((a) => a.name === 'Delta')).toBeTruthy();
});

test('removing an alternative removes its judgments', () => {
  const p = fixture();
  removeAlternative(p, 'a1');
  expect(p.pairwise.some((j) => j.a === 'a1' || j.b === 'a1')).toBe(false);
  expect(p.ratings).toHaveLength(0);
  expect(p.values).toHaveLength(0);
});

test('removing a criterion removes its subtree and every judgment about it', () => {
  const p = fixture();
  removeCriterion(p, 'c1');
  expect(ids(p.criteria)).toEqual(['c4']);
  expect(p.pairwise).toHaveLength(0);
  expect(p.ratings).toHaveLength(0);
});

test('judgments read back in either orientation and replace each other', () => {
  const p = fixture();
  expect(getJudgment(p, 'e1', 'alts:c2', 'a2', 'a1')).toBe(0.5);
  setJudgment(p, 'e1', 'alts:c2', 'a2', 'a1', 3);
  expect(getJudgment(p, 'e1', 'alts:c2', 'a1', 'a2')).toBeCloseTo(1 / 3);
  expect(p.pairwise.filter((j) => j.context === 'alts:c2')).toHaveLength(1);
  setJudgment(p, 'e1', 'alts:c2', 'a1', 'a2', undefined);
  expect(getJudgment(p, 'e1', 'alts:c2', 'a1', 'a2')).toBeUndefined();
  expect(getJudgment(p, 'e2', 'criteria:root', 'c1', 'c4')).toBeUndefined();
});

test('ratings and values replace earlier entries; non-numbers clear values', () => {
  const p = fixture();
  setRating(p, 'e1', 'c3', 'a1', 9);
  expect(p.ratings).toEqual([{ evaluatorId: 'e1', criterionId: 'c3', alternativeId: 'a1', value: 9 }]);
  setValue(p, 'c4', 'a1', Number.NaN);
  expect(p.values).toHaveLength(0);
});

test('moving a criterion inside another drops judgments that no longer apply', () => {
  const p = fixture();
  expect(moveCriterion(p, 'c4', 'c1', 'inside')).toBe(true);
  expect(ids(childrenOf(p, 'c1'))).toEqual(['c2', 'c3', 'c4']);
  expect(getJudgment(p, 'e1', 'criteria:root', 'c1', 'c4')).toBeUndefined(); // no longer siblings
  expect(p.values).toHaveLength(1); // alternatives' data survives
  expect(moveCriterion(p, 'c1', 'c2', 'inside')).toBe(false); // into its own subtree
});

test('indent, outdent and shift', () => {
  const p = fixture();
  expect(indent(p, 'c4')).toBe(true);
  expect(p.criteria.find((c) => c.id === 'c4')?.parentId).toBe('c1');
  expect(outdent(p, 'c4')).toBe(true);
  expect(ids(childrenOf(p))).toEqual(['c1', 'c4']);
  expect(indent(p, 'c1')).toBe(false); // no previous sibling
  expect(shift(p, 'c4', -1)).toBe(true);
  expect(ids(childrenOf(p))).toEqual(['c4', 'c1']);
  expect(shift(p, 'c4', -1)).toBe(false);
});

test('adding a child keeps the parent\'s alternative judgments (it just stops using them)', () => {
  const p = fixture();
  addCriterion(p, 'Latency', 'c2');
  expect(getJudgment(p, 'e1', 'alts:c2', 'a1', 'a2')).toBe(2);
});

test('the last evaluator stays; others go with their judgments', () => {
  const p = fixture();
  expect(removeEvaluator(p, 'e1')).toBe(true);
  expect(p.pairwise).toHaveLength(0);
  expect(removeEvaluator(p, 'e2')).toBe(false);
});

test('changing the scale snaps judgments', () => {
  const p = fixture();
  changeScale(p, 'classic5');
  expect(p.pairwise.map((j) => j.value)).toEqual([5, 1, 1]);
});

test('re-judging a pair keeps its note', () => {
  const p = fixture();
  p.pairwise[2].note = 'Benchmarks from March';
  setJudgment(p, 'e1', 'alts:c2', 'a2', 'a1', 4);
  expect(p.pairwise.find((j) => j.context === 'alts:c2')?.note).toBe('Benchmarks from March');
});

test('a blank or zero weight counts as 1, as the server treats it', () => {
  const p = fixture();
  setEvaluatorWeight(p, 'e2', Number.NaN);
  expect(p.evaluators[1].weight).toBe(1);
  setEvaluatorWeight(p, 'e2', 0);
  expect(p.evaluators[1].weight).toBe(1);
  setEvaluatorWeight(p, 'e2', 2.5);
  expect(p.evaluators[1].weight).toBe(2.5);
  setEvaluatorWeight(p, 'e2', -1); // kept, so the save explains it
  expect(p.evaluators[1].weight).toBe(-1);
});
```

- [ ] **Step 3: Write the failing test `web/src/lib/sensitivity.test.ts`** (complete file)

```ts
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, test } from 'vitest';
import { at, crossovers, leaderAt, lineFor, shiftLabel, stability } from './sensitivity';
import type { Project, Results } from './types';

// Golden files are written by `go test ./internal/model -run Golden -update`.
const dir = fileURLToPath(new URL('../../../internal/model/testdata/golden', import.meta.url));
const golden = readdirSync(dir)
  .filter((f) => f.endsWith('.json'))
  .map((f) => ({ name: f, ...(JSON.parse(readFileSync(join(dir, f), 'utf8')) as { project: Project; results: Results }) }));
type Golden = (typeof golden)[number];
type CriterionResult = Results['criteria'][number];

describe.each(golden)('$name matches Go', ({ results }: Golden) => {
  test.each(results.criteria.map((c): [string, CriterionResult] => [c.id, c]))('criterion %s', (_id: string, c: CriterionResult) => {
    const line = lineFor(results, c.id);
    const siblings = results.criteria.filter((x) => x.id !== c.id && (x.parentId ?? '') === (c.parentId ?? ''));
    if (siblings.length === 0) {
      expect(line).toBeNull();
      expect(c.stability).toBeNull();
      return;
    }
    expect(line).not.toBeNull();
    // At the current weight the line reproduces Go's scores...
    at(line!, c.local).forEach((s, a) => expect(s).toBeCloseTo(results.scores[a].score, 12));
    // ...and its stability matches Go's.
    const st = stability(line!, c.local);
    if (c.stability === null) expect(st).toBeNull();
    else expect(st).toBeCloseTo(c.stability, 9);
  });
});

test('crossovers are where the leader changes', () => {
  const line = { intercept: [0.2, 0.8], slope: [0.7, -0.7] }; // meet at 3/7
  expect(crossovers(line)).toEqual([{ x: expect.closeTo(3 / 7, 12), a: 0, b: 1 }]);
  expect(leaderAt(line, 0.4)).toBe(1);
  expect(leaderAt(line, 0.5)).toBe(0);
  expect(stability(line, 0.5)).toBeCloseTo(0.5 - 3 / 7, 12);
});

test('stability reads in whole points, never "0 points"', () => {
  expect(shiftLabel(null)).toBe('Holds at any weight');
  expect(shiftLabel(0.004)).toBe('Flips at less than 1 point');
  expect(shiftLabel(0.124)).toBe('Flips at ±12 points');
  expect(shiftLabel(0.01)).toBe('Flips at ±1 point');
});
```

- [ ] **Step 4: Write the failing test `web/src/lib/session.test.ts`** (complete file)

```ts
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { ApiError, type Client } from './api';
import { fixture } from './fixture.test-helper';
import { ProjectSession } from './session.svelte';
import type { Project, Results } from './types';

function fakeClient() {
  let stored = fixture();
  const saves: Project[] = [];
  const client: Client = {
    listProjects: async () => [],
    createProject: async () => fixture(),
    getProject: async () => structuredClone(stored),
    saveProject: async (p) => {
      saves.push(structuredClone(p));
      if (p.version !== stored.version) throw new ApiError(409, 'conflict', 'changed', { currentVersion: stored.version });
      stored = { ...structuredClone(p), version: p.version + 1, updatedAt: '2026-10-15T12:01:00Z' };
      return structuredClone(stored);
    },
    deleteProject: async () => {},
    duplicateProject: async () => fixture(),
    results: async (_id, ev) => ({ evaluator: ev }) as Results,
    importFile: async () => ({ project: fixture() }),
    mergeFile: async () => ({ project: fixture(), report: { added: [], replaced: [], skipped: [], judgments: 0, dropped: 0 } }),
    samples: async () => [],
    templates: async () => [],
  };
  return { client, saves, setStored: (p: Project) => (stored = p), stored: () => stored };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

test('edits within the delay are saved once, then results refresh', async () => {
  const f = fakeClient();
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'One';
  s.changed();
  s.project.name = 'Two';
  s.changed();
  expect(s.saveState).toBe('pending');
  await vi.advanceTimersByTimeAsync(600);
  expect(f.saves.map((p) => p.name)).toEqual(['Two']);
  expect(s.saveState).toBe('saved');
  expect(s.project.version).toBe(2);
  expect(s.results.all?.evaluator).toBe('all');
});

test('an edit made while saving is saved afterwards', async () => {
  const f = fakeClient();
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'First';
  s.changed();
  const flushing = s.flush();
  s.project.name = 'Second'; // lands while the first save is in flight
  s.changed();
  await flushing;
  await vi.advanceTimersByTimeAsync(600);
  expect(f.saves.map((p) => p.name)).toEqual(['First', 'Second']);
  expect(s.project.version).toBe(3);
});

test('a conflict stops autosave until the person chooses', async () => {
  const f = fakeClient();
  f.setStored({ ...fixture(), version: 5, name: 'Elsewhere' });
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'Mine';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  expect(s.saveState).toBe('conflict');
  s.changed(); // further edits don't retry on their own
  await vi.advanceTimersByTimeAsync(600);
  expect(f.saves).toHaveLength(1);

  await s.overwrite();
  expect(s.saveState).toBe('saved');
  expect(f.stored().name).toBe('Mine');
});

test('reload discards local edits after a conflict', async () => {
  const f = fakeClient();
  f.setStored({ ...fixture(), version: 5, name: 'Elsewhere' });
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'Mine';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  await s.reload();
  expect(s.project.name).toBe('Elsewhere');
  expect(s.saveState).toBe('saved');
});

test('when the server is unreachable, edits wait for a retry', async () => {
  const f = fakeClient();
  const real = f.client.saveProject;
  let down = true;
  f.client.saveProject = async (p) => {
    if (down) throw new ApiError(0, 'offline', "Can't reach AHP Matrix. Is it still running?");
    return real(p);
  };
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'Unsaved';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  expect(s.saveState).toBe('error');
  expect(s.error).toContain("Can't reach AHP Matrix");

  down = false;
  await s.retry();
  expect(s.saveState).toBe('saved');
  expect(f.stored().name).toBe('Unsaved');
});

test('a rejected save explains what to fix in plain words', async () => {
  const f = fakeClient();
  f.client.saveProject = async () => {
    throw new ApiError(422, 'invalid', 'invalid project: alternatives[0].name: is required', [{ field: 'alternatives[0].name', message: 'is required' }]);
  };
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.alternatives[0].name = '';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  expect(s.saveState).toBe('error');
  expect(s.error).toBe('Option 1 needs a name. Your changes will save as soon as that’s fixed.');
});

test('a slow, older results response never replaces a newer one', async () => {
  const f = fakeClient();
  let calls = 0;
  const pending: ((r: Results) => void)[] = [];
  f.client.results = (_id, ev) =>
    new Promise((resolve) => {
      const n = ++calls;
      pending.push(() => resolve({ evaluator: ev, completeness: n } as Results));
    });
  const s = new ProjectSession(fixture(), f.client, 600);
  const first = s.refreshResults(['all']);
  const second = s.refreshResults(['all']);
  pending[1]({} as Results); // the newer request answers first
  pending[0]({} as Results);
  await Promise.all([first, second]);
  expect(s.results.all.completeness).toBe(2);
});
```

- [ ] **Step 5: Write the failing test `web/src/components/charts/charts.test.ts`** (complete file)

```ts
import { render } from 'svelte/server';
import { expect, test } from 'vitest';
import { fixture } from '../../lib/fixture.test-helper';
import { colorOf } from '../../lib/format';
import type { Results } from '../../lib/types';
import Contributions from './Contributions.svelte';
import LineChart from './LineChart.svelte';
import PriorityBars from './PriorityBars.svelte';
import WeightTree from './WeightTree.svelte';

const text = (html: string) => html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');

const results: Results = {
  evaluator: 'all',
  scores: [
    { id: 'a1', score: 0.5, rank: 1 },
    { id: 'a2', score: 0.2, rank: 3 },
    { id: 'a3', score: 0.3, rank: 2 },
  ],
  criteria: [
    { id: 'c1', local: 0.75, global: 0.75, subtree: [0.5, 0.2, 0.3], stability: null },
    { id: 'c2', parentId: 'c1', local: 0.5, global: 0.375, subtree: [0.6, 0.2, 0.2], stability: null },
    { id: 'c3', parentId: 'c1', local: 0.5, global: 0.375, subtree: [0.4, 0.2, 0.4], stability: null },
    { id: 'c4', local: 0.25, global: 0.25, subtree: [0.5, 0.2, 0.3], stability: null },
  ],
  matrices: [],
  issues: [],
  disagreements: [],
  completeness: 1,
  consistent: true,
};

test('priority bars label every bar and can rank them', () => {
  const { body } = render(PriorityBars, { props: { names: ['A', 'B', 'C'], colors: ['red', 'green', 'blue'], values: [0.2, 0.5, 0.3], sorted: true } });
  expect(text(body)).toMatch(/1 B .*50\.0%.*2 C .*30\.0%.*3 A .*20\.0%/);
});

test('the weight tree shows local and global shares, indented', () => {
  const { body } = render(WeightTree, { props: { project: fixture(), results } });
  expect(text(body)).toMatch(/Performance 50% .*37\.5%/);
  expect(body).toMatch(/padding-left: 1\.7rem/); // depth 1
});

test('contributions are weight × priority, rows best first, with totals', () => {
  const { body } = render(Contributions, { props: { project: fixture(), results } });
  const t = text(body);
  expect(t.indexOf('Alpha')).toBeLessThan(t.indexOf('Gamma'));
  expect(t.indexOf('Gamma')).toBeLessThan(t.indexOf('Beta'));
  expect(t).toContain('22.5%'); // Alpha on Performance: 0.375 × 0.6
  expect(t).toContain('50.0%'); // Alpha's total
});

test('the line chart marks crossovers and labels lines directly', () => {
  const { body } = render(LineChart, {
    props: { line: { intercept: [0.2, 0.8], slope: [0.7, -0.7] }, names: ['Up', 'Down'], colors: ['red', 'blue'], current: 0.5, x: 0.3 },
  });
  expect(body.match(/class="cross/g)).toHaveLength(1);
  expect(body.match(/class="label/g)).toHaveLength(2);
  expect(text(body)).toContain('Ranking changes');
});

test('past eight options the colors run out, but every bar keeps its name', () => {
  const names = Array.from({ length: 9 }, (_, i) => `Option ${i + 1}`);
  const colors = names.map((_, i) => colorOf(i < 8 ? `series-${i + 1}` : ''));
  const { body } = render(PriorityBars, { props: { names, colors, values: names.map(() => 1 / 9) } });
  for (const n of names) expect(text(body)).toContain(n);
  expect(body).toContain('var(--series-none)');
});

test('the line chart labels up to eight lines directly', () => {
  const names = Array.from({ length: 8 }, (_, i) => `Option ${i + 1}`);
  const line = { intercept: names.map((_, i) => i / 10), slope: names.map(() => 0) };
  const { body } = render(LineChart, { props: { line, names, colors: names.map(() => 'red'), current: 0.5, x: 0.5 } });
  expect(body.match(/class="label/g)).toHaveLength(8);
});
```

- [ ] **Step 6: Run the tests to verify they fail**

Run: `cd web && npx vitest run`
Expected: FAIL — `count is not a function`, `setEvaluatorWeight is not a function`, `shiftLabel is not a function`, plus the note and the out-of-order assertions, and (for charts) 2 labels where 8 were expected.

- [ ] **Step 7: Write `web/src/lib/api.ts`** (complete file)

```ts
import type { ImportReport, MergeReport, Project, ProjectSummary, Results, StarterEntry } from './types';

/** An error response from the API: {"error": {"code", "message", "details"}}. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly details?: unknown,
  ) {
    super(message);
  }
}

async function call<T>(method: string, path: string, body?: BodyInit, json = true): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined && json) headers['Content-Type'] = 'application/json';
  let res: Response;
  try {
    res = await fetch(path, { method, headers, body });
  } catch {
    throw new ApiError(0, 'offline', "Can't reach AHP Matrix. Is it still running?");
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    const e = data?.error ?? {};
    throw new ApiError(res.status, e.code ?? 'error', e.message ?? res.statusText, e.details);
  }
  return data as T;
}

/** Where a new project starts: blank, a copy of a sample, or a template's criteria. */
export interface Starter {
  sample?: string;
  template?: string;
}

export type MergePolicy = 'replace' | 'skip';

export interface Client {
  listProjects(): Promise<ProjectSummary[]>;
  createProject(name: string, from?: Starter): Promise<Project>;
  getProject(id: string): Promise<Project>;
  saveProject(p: Project): Promise<Project>;
  deleteProject(id: string): Promise<void>;
  duplicateProject(id: string): Promise<Project>;
  results(id: string, evaluator: string): Promise<Results>;
  /** Imports an .ahp.json file, or a workbook made with the Priority Matrix template (.xlsx). */
  importFile(file: File): Promise<{ project: Project; report?: ImportReport }>;
  /** Merges the judgments in a returned file. Without a policy, people already present cause an ApiError 'evaluator-exists'. */
  mergeFile(id: string, file: File, existing?: MergePolicy): Promise<{ project: Project; report: MergeReport }>;
  samples(): Promise<StarterEntry[]>;
  templates(): Promise<StarterEntry[]>;
}

const upload = (file: File) => {
  const form = new FormData();
  form.append('file', file);
  return form;
};

export const api: Client = {
  async listProjects() {
    return (await call<{ projects: ProjectSummary[] }>('GET', '/api/projects')).projects;
  },
  createProject: (name, from = {}) =>
    call('POST', '/api/projects', JSON.stringify({ name, fromSample: from.sample, fromTemplate: from.template })),
  getProject: (id) => call('GET', `/api/projects/${encodeURIComponent(id)}`),
  saveProject: (p) => call('PUT', `/api/projects/${encodeURIComponent(p.id)}`, JSON.stringify(p)),
  deleteProject: (id) => call('DELETE', `/api/projects/${encodeURIComponent(id)}`),
  duplicateProject: (id) => call('POST', `/api/projects/${encodeURIComponent(id)}/duplicate`),
  results: (id, evaluator) =>
    call('GET', `/api/projects/${encodeURIComponent(id)}/results?evaluator=${encodeURIComponent(evaluator)}`),
  async importFile(file) {
    if (/\.xlsx$/i.test(file.name)) return call('POST', '/api/import/xlsx', upload(file), false);
    return { project: await call<Project>('POST', '/api/import/ahp', upload(file), false) };
  },
  mergeFile: (id, file, existing) =>
    call('POST', `/api/projects/${encodeURIComponent(id)}/merge${existing ? `?existing=${existing}` : ''}`, upload(file), false),
  samples: () => call('GET', '/api/samples'),
  templates: () => call('GET', '/api/templates'),
};

export type ExportKind = 'ahp.json' | 'xlsx' | 'csv';

/**
 * A download link. For ahp.json, an evaluator id gives that person's copy to
 * send them; for xlsx and csv it picks whose judgments the results use.
 */
export function exportUrl(id: string, kind: ExportKind = 'ahp.json', evaluator?: string): string {
  const q = evaluator ? `?evaluator=${encodeURIComponent(evaluator)}` : '';
  return `/api/projects/${encodeURIComponent(id)}/export.${kind}${q}`;
}
```

- [ ] **Step 8: Write `web/src/lib/format.ts`** (complete file)

```ts
/** 0.4567 → "45.7%"; values under 10% keep one decimal, larger ones too, for comparability. */
export const percent = (x: number, digits = 1) => `${(x * 100).toFixed(digits)}%`;

/** "1 criterion", "6 criteria". */
export const count = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

/** A CSS color for an alternative's stored color: a series slot ("series-3") or a hex value. */
export function colorOf(color: string): string {
  if (/^series-[1-8]$/.test(color)) return `var(--${color})`;
  if (/^#[0-9a-f]{3,8}$/i.test(color)) return color;
  return 'var(--series-none)';
}

/** "just now", "5 minutes ago", "yesterday", "3 Mar". */
export function relativeTime(iso: string, now = Date.now()): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return '';
  const s = Math.round((now - t) / 1000);
  if (s < 60) return 'just now';
  const m = Math.round(s / 60);
  if (m < 60) return `${m} minute${m === 1 ? '' : 's'} ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h} hour${h === 1 ? '' : 's'} ago`;
  const d = Math.round(h / 24);
  if (d === 1) return 'yesterday';
  if (d < 7) return `${d} days ago`;
  return new Date(t).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: d > 300 ? 'numeric' : undefined });
}
```

- [ ] **Step 9: Write `web/src/lib/edits.ts`** (complete file)

```ts
// Pure edits to a project document. Each function changes p in place, the
// same way the server's Prepare would leave it, so the UI never shows data the
// server is about to drop.
import { snap } from './scale';
import { childrenOf, descendants } from './tree';
import type { Alternative, Criterion, Direction, Evaluator, Method, PairwiseJudgment, Project, Scale } from './types';

export const SERIES_SLOTS = 8;

/** A short random id for an item inside a project (like model.NewID). */
export function newId(): string {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
  const bytes = crypto.getRandomValues(new Uint8Array(8));
  return Array.from(bytes, (b) => alphabet[b % 32]).join('');
}

export const criteriaContext = (parentId = '') => `criteria:${parentId || 'root'}`;
export const altsContext = (criterionId: string) => `alts:${criterionId}`;

// Alternatives

/** Adds an alternative with the first unused series color ('' after eight). */
export function addAlternative(p: Project, name: string): Alternative {
  const used = new Set(p.alternatives.map((a) => a.color));
  let color = '';
  for (let k = 1; k <= SERIES_SLOTS && !color; k++) if (!used.has(`series-${k}`)) color = `series-${k}`;
  const a: Alternative = { id: newId(), name: name.trim(), description: '', color };
  p.alternatives.push(a);
  return a;
}

export function removeAlternative(p: Project, id: string): void {
  p.alternatives = p.alternatives.filter((a) => a.id !== id);
  p.pairwise = p.pairwise.filter((j) => !(j.context.startsWith('alts:') && (j.a === id || j.b === id)));
  p.ratings = p.ratings.filter((r) => r.alternativeId !== id);
  p.values = p.values.filter((v) => v.alternativeId !== id);
}

// Criteria

export function addCriterion(p: Project, name: string, parentId = ''): Criterion {
  const c: Criterion = { id: newId(), name: name.trim(), description: '', notes: '', method: 'pairwise' };
  if (parentId) c.parentId = parentId;
  p.criteria.push(c);
  pruneJudgments(p);
  return c;
}

/** Removes a criterion, everything below it, and every judgment that refers to them. */
export function removeCriterion(p: Project, id: string): void {
  const gone = new Set(descendants(p, id));
  p.criteria = p.criteria.filter((c) => !gone.has(c.id));
  p.ratings = p.ratings.filter((r) => !gone.has(r.criterionId));
  p.values = p.values.filter((v) => !gone.has(v.criterionId));
  pruneJudgments(p);
}

export type Placement = 'before' | 'after' | 'inside';

/**
 * Moves criterion id relative to target: as its previous or next sibling, or
 * as its last child. Returns false (and changes nothing) for a move into its
 * own subtree.
 */
export function moveCriterion(p: Project, id: string, targetId: string, where: Placement): boolean {
  if (descendants(p, id).includes(targetId)) return false;
  const moving = p.criteria.find((c) => c.id === id);
  const target = p.criteria.find((c) => c.id === targetId);
  if (!moving || !target) return false;
  p.criteria = p.criteria.filter((c) => c.id !== id);
  setParent(moving, where === 'inside' ? target.id : (target.parentId ?? ''));
  let at = p.criteria.indexOf(target);
  if (where !== 'before') at += 1;
  if (where === 'inside') {
    // after the target's current last child, so it lands at the end
    const kids = childrenOf(p, target.id);
    if (kids.length) at = p.criteria.indexOf(kids[kids.length - 1]) + 1;
  }
  p.criteria.splice(at, 0, moving);
  pruneJudgments(p);
  return true;
}

/** Makes a criterion the last child of its previous sibling. */
export function indent(p: Project, id: string): boolean {
  const c = p.criteria.find((x) => x.id === id);
  if (!c) return false;
  const sibs = childrenOf(p, c.parentId ?? '');
  const i = sibs.indexOf(c);
  return i > 0 && moveCriterion(p, id, sibs[i - 1].id, 'inside');
}

/** Makes a criterion the next sibling of its parent. */
export function outdent(p: Project, id: string): boolean {
  const c = p.criteria.find((x) => x.id === id);
  return !!c?.parentId && moveCriterion(p, id, c.parentId, 'after');
}

/** Swaps a criterion with its previous (-1) or next (+1) sibling. */
export function shift(p: Project, id: string, dir: -1 | 1): boolean {
  const c = p.criteria.find((x) => x.id === id);
  if (!c) return false;
  const sibs = childrenOf(p, c.parentId ?? '');
  const other = sibs[sibs.indexOf(c) + dir];
  if (!other) return false;
  const i = p.criteria.indexOf(c);
  const j = p.criteria.indexOf(other);
  [p.criteria[i], p.criteria[j]] = [p.criteria[j], p.criteria[i]];
  return true;
}

function setParent(c: Criterion, parentId: string) {
  if (parentId) c.parentId = parentId;
  else delete c.parentId;
}

export function setMethod(p: Project, id: string, method: Method): void {
  const c = p.criteria.find((x) => x.id === id);
  if (!c) return;
  c.method = method;
  if (method === 'value') c.direction ??= 'higher';
}

export function setDirection(p: Project, id: string, direction: Direction): void {
  const c = p.criteria.find((x) => x.id === id);
  if (c) c.direction = direction;
}

/**
 * Drops pairwise judgments whose context no longer exists or whose items are
 * no longer siblings in it (mirrors the server's prune). Judgments of
 * alternatives survive while their criterion exists, even if it gains children.
 */
export function pruneJudgments(p: Project): void {
  const crit = new Map(p.criteria.map((c) => [c.id, c]));
  const alts = new Set(p.alternatives.map((a) => a.id));
  const evals = new Set(p.evaluators.map((e) => e.id));
  p.pairwise = p.pairwise.filter((j) => {
    if (!evals.has(j.evaluatorId) || j.a === j.b) return false;
    const [kind, id] = j.context.split(':');
    if (kind === 'alts') return crit.has(id) && alts.has(j.a) && alts.has(j.b);
    const parent = id === 'root' ? '' : id;
    if (parent && !crit.has(parent)) return false;
    return (crit.get(j.a)?.parentId ?? '') === parent && (crit.get(j.b)?.parentId ?? '') === parent && crit.has(j.a) && crit.has(j.b);
  });
}

// Judgments

/** How strongly a is preferred over b, or undefined if the pair isn't judged. */
export function getJudgment(p: Project, evaluatorId: string, context: string, a: string, b: string): number | undefined {
  for (const j of p.pairwise) {
    if (j.evaluatorId !== evaluatorId || j.context !== context) continue;
    if (j.a === a && j.b === b) return j.value;
    if (j.a === b && j.b === a) return 1 / j.value;
  }
  return undefined;
}

/** Records (or with undefined, clears) how strongly a is preferred over b. */
export function setJudgment(p: Project, evaluatorId: string, context: string, a: string, b: string, value: number | undefined): void {
  const same = (j: PairwiseJudgment) =>
    j.evaluatorId === evaluatorId && j.context === context && ((j.a === a && j.b === b) || (j.a === b && j.b === a));
  const note = p.pairwise.find(same)?.note; // a new value keeps the reasoning behind the old one
  p.pairwise = p.pairwise.filter((j) => !same(j));
  if (value !== undefined) p.pairwise.push(note ? { evaluatorId, context, a, b, value, note } : { evaluatorId, context, a, b, value });
}

export function getRating(p: Project, evaluatorId: string, criterionId: string, alternativeId: string): number | undefined {
  return p.ratings.find((r) => r.evaluatorId === evaluatorId && r.criterionId === criterionId && r.alternativeId === alternativeId)?.value;
}

export function setRating(p: Project, evaluatorId: string, criterionId: string, alternativeId: string, value: number | undefined): void {
  p.ratings = p.ratings.filter((r) => !(r.evaluatorId === evaluatorId && r.criterionId === criterionId && r.alternativeId === alternativeId));
  if (value !== undefined) p.ratings.push({ evaluatorId, criterionId, alternativeId, value });
}

export function getValue(p: Project, criterionId: string, alternativeId: string): number | undefined {
  return p.values.find((v) => v.criterionId === criterionId && v.alternativeId === alternativeId)?.value;
}

export function setValue(p: Project, criterionId: string, alternativeId: string, value: number | undefined): void {
  p.values = p.values.filter((v) => !(v.criterionId === criterionId && v.alternativeId === alternativeId));
  if (value !== undefined && Number.isFinite(value)) p.values.push({ criterionId, alternativeId, value });
}

// Evaluators and scale

export function addEvaluator(p: Project, name: string): Evaluator {
  const e: Evaluator = { id: newId(), name: name.trim(), weight: 1 };
  p.evaluators.push(e);
  return e;
}

/**
 * Sets a person's weight. Blank or zero counts as 1, as the server treats it;
 * a negative weight is kept so the save can explain what's wrong.
 */
export function setEvaluatorWeight(p: Project, id: string, weight: number): void {
  const e = p.evaluators.find((x) => x.id === id);
  if (e) e.weight = Number.isFinite(weight) && weight !== 0 ? weight : 1;
}

/** Removes an evaluator and their judgments. The last evaluator can't be removed. */
export function removeEvaluator(p: Project, id: string): boolean {
  if (p.evaluators.length <= 1) return false;
  p.evaluators = p.evaluators.filter((e) => e.id !== id);
  p.pairwise = p.pairwise.filter((j) => j.evaluatorId !== id);
  p.ratings = p.ratings.filter((r) => r.evaluatorId !== id);
  return true;
}

/** Switches scale, rounding every judgment to the nearest value on the new one. */
export function changeScale(p: Project, scale: Scale): void {
  p.scale = scale;
  for (const j of p.pairwise) j.value = snap(j.value, scale);
}
```

- [ ] **Step 10: Write `web/src/lib/sensitivity.ts`** (complete file)

```ts
// Mirrors internal/ahp/sensitivity.go so the sensitivity slider can redraw
// without a round trip. Parity with Go is checked against the golden files in
// internal/model/testdata/golden (see sensitivity.test.ts).
import type { Results } from './types';

/** Each alternative's overall score as intercept + slope·x, where x is one criterion's local weight. */
export interface Line {
  intercept: number[];
  slope: number[];
}

export const at = (l: Line, x: number) => l.intercept.map((c, a) => c + l.slope[a] * x);

/**
 * The scores as a function of criterion id's local weight, with its siblings
 * rescaled to share the rest (equally, if they currently have no weight).
 * Null when the criterion has no siblings.
 */
export function lineFor(r: Results, id: string): Line | null {
  const node = r.criteria.find((c) => c.id === id);
  if (!node) return null;
  const parentId = node.parentId ?? '';
  const sibs = r.criteria.filter((c) => c.id !== id && (c.parentId ?? '') === parentId);
  if (sibs.length === 0) return null;
  const gp = parentId ? (r.criteria.find((c) => c.id === parentId)?.global ?? 0) : 1;
  const w = node.local;
  const line: Line = { intercept: [], slope: [] };
  r.scores.forEach((s, a) => {
    let others = 0;
    for (const sb of sibs) others += sb.local * sb.subtree[a];
    const group = w * node.subtree[a] + others;
    let q: number;
    if (1 - w > 1e-12) q = others / (1 - w);
    else q = sibs.reduce((t, sb) => t + sb.subtree[a], 0) / sibs.length;
    const rest = s.score - gp * group;
    line.intercept.push(rest + gp * q);
    line.slope.push(gp * (node.subtree[a] - q));
  });
  return line;
}

/** The weight in [0, 1] where alternatives a and b score equally, if any. */
function meet(l: Line, a: number, b: number): number | null {
  const ds = l.slope[a] - l.slope[b];
  if (Math.abs(ds) < 1e-15) return null;
  const x = (l.intercept[b] - l.intercept[a]) / ds;
  return x >= 0 && x <= 1 ? x : null;
}

export interface Crossover {
  x: number;
  a: number;
  b: number;
}

/** Every weight in [0, 1] where two alternatives swap places, sorted by weight. */
export function crossovers(l: Line): Crossover[] {
  const out: Crossover[] = [];
  for (let a = 0; a < l.slope.length; a++)
    for (let b = a + 1; b < l.slope.length; b++) {
      const x = meet(l, a, b);
      if (x !== null) out.push({ x, a, b });
    }
  return out.sort((p, q) => p.x - q.x);
}

const argmax = (v: number[]) => v.reduce((best, x, i) => (x > v[best] ? i : best), 0);

/** The smallest change from weight w at which the leader changes, or null if it never does. */
export function stability(l: Line, w: number): number | null {
  const top = argmax(at(l, w));
  let best: number | null = null;
  for (let b = 0; b < l.slope.length; b++) {
    if (b === top) continue;
    const x = meet(l, top, b);
    if (x !== null && (best === null || Math.abs(x - w) < best)) best = Math.abs(x - w);
  }
  return best;
}

/** How far a criterion's weight can move before the leader changes, in words. */
export function shiftLabel(stability: number | null): string {
  if (stability === null) return 'Holds at any weight';
  const points = Math.round(stability * 100);
  if (points < 1) return 'Flips at less than 1 point';
  return `Flips at ±${points} point${points === 1 ? '' : 's'}`;
}

/** The index of the leading alternative at weight x. */
export const leaderAt = (l: Line, x: number) => argmax(at(l, x));
```

- [ ] **Step 11: Write `web/src/lib/session.svelte.ts`** (complete file)

```ts
import { ApiError, api, type Client } from './api';
import { explain, type Problem } from './problems';
import { ALL_EVALUATORS, type Project, type Results } from './types';

export type SaveState = 'saved' | 'pending' | 'saving' | 'conflict' | 'error';

/**
 * One open project. Edits change `project` in place and call `changed()`;
 * the session saves 600 ms after the last edit, keeps the local copy as the
 * source of truth (only the version comes back from the server), and fetches
 * fresh results after each save.
 */
export class ProjectSession {
  project: Project = $state()!;
  saveState: SaveState = $state('saved');
  error = $state('');
  /** Evaluator whose judgments the compare screens edit. */
  scoringAs = $state('');
  /** Latest results, keyed by evaluator id or ALL_EVALUATORS. */
  results: Record<string, Results> = $state({});

  private timer: ReturnType<typeof setTimeout> | undefined;
  private inFlight: Promise<void> | null = null;
  private watched = new Set<string>([ALL_EVALUATORS]);
  private requests = new Map<string, number>(); // latest results request per evaluator

  constructor(
    project: Project,
    private client: Client = api,
    private delayMs = 600,
  ) {
    this.project = project;
    this.scoringAs = project.evaluators[0]?.id ?? '';
  }

  static async open(id: string, client: Client = api): Promise<ProjectSession> {
    const s = new ProjectSession(await client.getProject(id), client);
    await s.refreshResults();
    return s;
  }

  /** True while some edit hasn't reached the server: waiting, saving, rejected or in conflict. */
  get unsaved(): boolean {
    return this.saveState !== 'saved';
  }

  /** Call after every edit to project. */
  changed(): void {
    if (this.saveState === 'conflict') return; // wait for the person to choose
    this.saveState = 'pending';
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.flush(), this.delayMs);
  }

  /** Saves now if there are unsaved edits; resolves when saving is done. */
  async flush(): Promise<void> {
    clearTimeout(this.timer);
    if (this.inFlight) await this.inFlight;
    if (this.saveState !== 'pending') return;
    this.inFlight = this.save();
    try {
      await this.inFlight;
    } finally {
      this.inFlight = null;
    }
  }

  private async save(): Promise<void> {
    this.saveState = 'saving';
    const sent = $state.snapshot(this.project) as Project;
    try {
      const saved = await this.client.saveProject(sent);
      this.project.version = saved.version;
      this.project.updatedAt = saved.updatedAt;
      // Edits made while saving left the state 'pending'; save those too.
      if (this.saveState === 'saving') this.saveState = 'saved';
      else this.timer = setTimeout(() => void this.flush(), this.delayMs);
      this.error = '';
      await this.refreshResults();
    } catch (e) {
      if (e instanceof ApiError && e.code === 'conflict') {
        this.saveState = 'conflict';
        this.error = 'This project was changed in another window.';
      } else if (e instanceof ApiError && e.code === 'invalid' && Array.isArray(e.details)) {
        this.saveState = 'error';
        this.error = `${explain(e.details as Problem[], this.project).join(' ')} Your changes will save as soon as that’s fixed.`;
      } else {
        this.saveState = 'error';
        this.error = e instanceof Error ? e.message : String(e);
      }
    }
  }

  /** Also keep results for this evaluator fresh after each save. */
  watch(evaluator: string): void {
    if (this.watched.has(evaluator)) return;
    this.watched.add(evaluator);
    void this.refreshResults([evaluator]);
  }

  async refreshResults(evaluators: string[] = [...this.watched]): Promise<void> {
    const known = new Set([ALL_EVALUATORS, ...this.project.evaluators.map((e) => e.id)]);
    await Promise.all(
      evaluators
        .filter((ev) => known.has(ev))
        .map(async (ev) => {
          // Responses can arrive out of order; only the latest request may write.
          const ticket = (this.requests.get(ev) ?? 0) + 1;
          this.requests.set(ev, ticket);
          try {
            const r = await this.client.results(this.project.id, ev);
            if (this.requests.get(ev) === ticket) this.results[ev] = r;
          } catch {
            // Results are advisory; the next save tries again.
          }
        }),
    );
  }

  /** Conflict resolution: discard local edits and load the stored project. */
  async reload(): Promise<void> {
    clearTimeout(this.timer);
    this.project = await this.client.getProject(this.project.id);
    if (!this.project.evaluators.some((e) => e.id === this.scoringAs)) this.scoringAs = this.project.evaluators[0]?.id ?? '';
    this.saveState = 'saved';
    this.error = '';
    await this.refreshResults();
  }

  /** Conflict resolution: keep local edits and replace the stored project. */
  async overwrite(): Promise<void> {
    const stored = await this.client.getProject(this.project.id);
    this.project.version = stored.version;
    this.saveState = 'pending';
    await this.flush();
  }

  /** Retry after an error. */
  retry(): Promise<void> {
    this.saveState = 'pending';
    return this.flush();
  }
}
```

- [ ] **Step 12: Write `web/src/components/Balance.svelte`** (complete file)

```svelte
<script lang="ts">
  // A balance beam for one pairwise judgment: the preferred side sinks. The
  // row of stops underneath is the control; arrow keys move along it.
  import { describe, fraction, stops, type CompareKind } from '../lib/scale';
  import type { Scale } from '../lib/types';

  interface Side {
    name: string;
    color?: string;
  }
  let {
    a,
    b,
    value,
    scale,
    kind,
    on = '',
    onchange,
  }: {
    a: Side;
    b: Side;
    value: number | undefined;
    scale: Scale;
    kind: CompareKind;
    on?: string;
    onchange: (v: number | undefined) => void;
  } = $props();

  // Positions run left (a strongly preferred) to right (b strongly preferred).
  const positions = $derived([...stops(scale)].reverse());
  const max = $derived(positions[0]);
  const middle = $derived((positions.length - 1) / 2);
  const index = $derived(value === undefined ? -1 : positions.findIndex((v) => Math.abs(Math.log(v / value)) < 1e-6));
  const tilt = $derived(value === undefined ? 0 : (-14 * Math.log(value)) / Math.log(max));
  const sentence = $derived(value === undefined ? 'Not compared yet' : describe(value, scale, kind, a.name, b.name, on));

  function set(i: number) {
    onchange(positions[Math.max(0, Math.min(positions.length - 1, i))]);
  }

  function onKey(e: KeyboardEvent) {
    const from = index < 0 ? middle : index;
    const moves: Record<string, () => void> = {
      ArrowLeft: () => set(index < 0 ? middle - 1 : from - 1),
      ArrowRight: () => set(index < 0 ? middle + 1 : from + 1),
      // A slider's ↑/↓ raise and lower its value, here toward the right-hand side.
      ArrowUp: () => set(index < 0 ? middle + 1 : from + 1),
      ArrowDown: () => set(index < 0 ? middle - 1 : from - 1),
      Home: () => set(0),
      End: () => set(positions.length - 1),
      '=': () => set(middle),
      Backspace: () => onchange(undefined),
      Delete: () => onchange(undefined),
    };
    if (moves[e.key]) {
      e.preventDefault();
      moves[e.key]();
    }
  }

  // Labels under the stops: strength toward the nearer side.
  const label = (v: number) => fraction(v >= 1 ? v : 1 / v);
</script>

<div class="balance">
  <svg viewBox="0 0 640 230" class="scene" aria-hidden="true">
    <line x1="320" y1="70" x2="320" y2="206" class="post" />
    <path d="M276 208h88" class="foot" />
    <g class="beam" class:unset={value === undefined} style:transform="rotate({tilt}deg)">
      <line x1="70" y1="70" x2="570" y2="70" class="bar" />
      {#each [{ x: 70, side: a }, { x: 570, side: b }] as end}
        <g class="pan" style:transform="rotate({-tilt}deg)" style:transform-origin="{end.x}px 70px">
          <line x1={end.x} y1="70" x2={end.x - 46} y2="132" />
          <line x1={end.x} y1="70" x2={end.x + 46} y2="132" />
          <path d="M{end.x - 62} 132h124a62 16 0 0 1-124 0Z" class="dish" style:--c={end.side.color ?? 'var(--ink-3)'} />
        </g>
      {/each}
    </g>
    <circle cx="320" cy="70" r="7" class="pivot" />
  </svg>

  <div class="names">
    <span class="side left"><span class="dot" style:background={a.color ?? 'var(--ink-3)'}></span>{a.name}</span>
    <span class="side right">{b.name}<span class="dot" style:background={b.color ?? 'var(--ink-3)'}></span></span>
  </div>

  <div
    class="stops"
    style:--n={positions.length}
    role="slider"
    tabindex="0"
    aria-label="Compare {a.name} with {b.name}"
    aria-valuemin={0}
    aria-valuemax={positions.length - 1}
    aria-valuenow={index < 0 ? undefined : index}
    aria-valuetext={sentence}
    onkeydown={onKey}
  >
    {#each positions as v, i}
      <button
        type="button"
        tabindex="-1"
        class="stop"
        class:mid={i === middle}
        class:on={i === index}
        class:toward-a={i < middle}
        class:toward-b={i > middle}
        aria-label={describe(v, scale, kind, a.name, b.name, on)}
        onclick={() => set(i)}
      >
        <span>{label(v)}</span>
      </button>
    {/each}
  </div>

  <p class="sentence" class:unset={value === undefined} aria-live="polite">{sentence}</p>
</div>

<style>
  .balance {
    display: grid;
    gap: 0.6rem;
    justify-items: center;
  }
  .scene {
    width: min(100%, 40rem);
    height: auto;
    overflow: visible;
  }
  .post,
  .foot {
    stroke: var(--ink);
    stroke-width: 6;
    stroke-linecap: round;
    fill: none;
  }
  .beam {
    transform-origin: 320px 70px;
    transition: transform 420ms cubic-bezier(0.34, 1.4, 0.5, 1);
  }
  .bar {
    stroke: var(--brass);
    stroke-width: 7;
    stroke-linecap: round;
  }
  .beam.unset .bar {
    stroke-dasharray: 2 12;
    stroke: var(--line-strong);
  }
  .pan {
    transition: transform 420ms cubic-bezier(0.34, 1.4, 0.5, 1);
  }
  .pan line {
    stroke: var(--ink-3);
    stroke-width: 1.5;
  }
  .dish {
    fill: color-mix(in oklab, var(--c) 22%, var(--paper));
    stroke: var(--c);
    stroke-width: 2.5;
  }
  .pivot {
    fill: var(--paper);
    stroke: var(--ink);
    stroke-width: 4;
  }
  .names {
    width: min(100%, 40rem);
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    font-family: var(--font-display);
    font-size: 1.25rem;
    font-weight: 650;
    margin-top: -0.5rem;
  }
  .side {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    max-width: 48%;
  }
  .right {
    text-align: right;
  }
  .dot {
    flex: none;
    width: 0.7rem;
    height: 0.7rem;
    border-radius: 50%;
  }
  .stops {
    display: grid;
    grid-template-columns: repeat(var(--n, 17), minmax(0, 1fr));
    grid-auto-flow: column;
    width: min(100%, 40rem);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-s);
    background: var(--paper);
    overflow: hidden;
  }
  .stops:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 3px;
  }
  .stop {
    border: 0;
    border-right: 1px solid var(--line);
    background: transparent;
    padding: 0.55rem 0;
    cursor: pointer;
    font-weight: 600;
    font-size: 0.85rem;
    color: var(--ink-2);
  }
  .stop:last-child {
    border-right: 0;
  }
  .stop:hover {
    background: var(--brass-wash);
  }
  .stop.mid {
    color: var(--ink);
  }
  .stop.on {
    background: var(--ink);
    color: var(--paper);
  }
  .sentence {
    font-size: 1.05rem;
    font-weight: 550;
    min-height: 1.6em;
    text-align: center;
  }
  .sentence.unset {
    color: var(--ink-3);
    font-weight: 400;
  }
</style>
```

- [ ] **Step 13: Write `web/src/components/charts/LineChart.svelte`** (complete file)

```svelte
<script lang="ts">
  // Each option's overall score as one criterion's weight goes from 0 to 100%.
  // Lines are straight (the scores are linear in the weight). A dashed marker
  // shows the current weight, a solid one the what-if weight, and dots mark
  // where the ranking changes. Hover shows every score at that weight.
  import { percent } from '../../lib/format';
  import { at, crossovers, type Line } from '../../lib/sensitivity';

  let {
    line,
    names,
    colors,
    current,
    x,
  }: { line: Line; names: string[]; colors: string[]; current: number; x: number } = $props();

  const W = 640;
  const H = 300;
  const pad = { l: 48, r: 150, t: 16, b: 34 };
  const iw = W - pad.l - pad.r;
  const ih = H - pad.t - pad.b;

  const ends = $derived([at(line, 0), at(line, 1)]);
  const lo = $derived(Math.max(0, Math.min(...ends[0], ...ends[1]) - 0.02));
  const hi = $derived(Math.min(1, Math.max(...ends[0], ...ends[1]) + 0.02));
  const sx = (v: number) => pad.l + v * iw;
  const sy = $derived((v: number) => pad.t + (1 - (v - lo) / (hi - lo || 1)) * ih);
  const ticks = $derived(Array.from({ length: 5 }, (_, k) => lo + ((hi - lo) * k) / 4));
  const cross = $derived(crossovers(line));
  // Direct labels while they fit (up to eight lines); the legend is always there too.
  const direct = $derived(names.length <= 8);

  // Direct labels at the right end, nudged apart so they don't collide.
  const labels = $derived.by(() => {
    const ys = ends[1].map((v, a) => ({ a, y: sy(v) })).sort((p, q) => p.y - q.y);
    for (let k = 1; k < ys.length; k++) ys[k].y = Math.max(ys[k].y, ys[k - 1].y + 15);
    return ys;
  });

  let hover = $state<number | null>(null);
  let svg = $state<SVGSVGElement | undefined>();
  function move(e: PointerEvent) {
    if (!svg) return;
    const r = svg.getBoundingClientRect();
    const v = ((e.clientX - r.left) * (W / r.width) - pad.l) / iw;
    hover = v >= 0 && v <= 1 ? v : null;
  }
  const hoverRows = $derived(
    hover === null ? [] : at(line, hover).map((s, a) => ({ a, s })).sort((p, q) => q.s - p.s),
  );
</script>

<div class="chart">
  <svg bind:this={svg} viewBox="0 0 {W} {H}" role="img" aria-label="Scores as the weight changes" onpointermove={move} onpointerleave={() => (hover = null)}>
    {#each ticks as t}
      <line class="grid" x1={pad.l} x2={pad.l + iw} y1={sy(t)} y2={sy(t)} />
      <text class="axis" x={pad.l - 8} y={sy(t) + 4} text-anchor="end">{percent(t, 0)}</text>
    {/each}
    {#each [0, 0.25, 0.5, 0.75, 1] as t}
      <text class="axis" x={sx(t)} y={H - 10} text-anchor="middle">{percent(t, 0)}</text>
    {/each}
    <line class="now" x1={sx(current)} x2={sx(current)} y1={pad.t} y2={pad.t + ih} />
    <line class="whatif" x1={sx(x)} x2={sx(x)} y1={pad.t} y2={pad.t + ih} />
    {#each names as _, a}
      <line class="series" x1={sx(0)} y1={sy(ends[0][a])} x2={sx(1)} y2={sy(ends[1][a])} style:stroke={colors[a]} />
    {/each}
    {#each cross as c}
      <circle class="cross" cx={sx(c.x)} cy={sy(at(line, c.x)[c.a])} r="5" />
    {/each}
    {#each names as _, a}
      <circle class="dot" cx={sx(x)} cy={sy(at(line, x)[a])} r="4.5" style:fill={colors[a]} />
    {/each}
    {#if direct}
      {#each labels as l}
        <text class="label" x={sx(1) + 8} y={l.y + 4}>{names[l.a]}</text>
      {/each}
    {/if}
    {#if hover !== null}
      <line class="hair" x1={sx(hover)} x2={sx(hover)} y1={pad.t} y2={pad.t + ih} />
    {/if}
  </svg>
  {#if hover !== null}
    <div class="tip" style:left="{(sx(hover) / W) * 100}%">
      <strong>At {percent(hover, 0)}</strong>
      {#each hoverRows as r}
        <span><i style:background={colors[r.a]}></i>{names[r.a]} {percent(r.s)}</span>
      {/each}
    </div>
  {/if}
  <ul class="legend">
    {#each names as n, a}<li><i style:background={colors[a]}></i>{n}</li>{/each}
    <li><i class="key-now"></i>Current weight</li>
    <li><i class="key-cross"></i>Ranking changes</li>
  </ul>
</div>

<style>
  .chart {
    position: relative;
  }
  svg {
    width: 100%;
    height: auto;
    display: block;
    touch-action: none;
  }
  .grid {
    stroke: var(--line);
    stroke-width: 1;
  }
  .axis {
    fill: var(--ink-3);
    font-size: 11px;
  }
  .series {
    stroke-width: 2;
  }
  .now {
    stroke: var(--ink-3);
    stroke-dasharray: 4 4;
  }
  .whatif {
    stroke: var(--brass);
    stroke-width: 2;
  }
  .hair {
    stroke: var(--ink-3);
    stroke-width: 1;
  }
  .cross {
    fill: var(--paper);
    stroke: var(--ink);
    stroke-width: 2;
  }
  .dot {
    stroke: var(--paper);
    stroke-width: 2;
  }
  .label {
    fill: var(--ink);
    font-size: 12px;
    font-weight: 600;
  }
  .tip {
    position: absolute;
    top: 0.5rem;
    transform: translateX(-50%);
    display: grid;
    gap: 0.1rem;
    padding: 0.45rem 0.6rem;
    background: var(--paper);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-s);
    box-shadow: var(--shadow-lift);
    font-size: 0.82rem;
    pointer-events: none;
    white-space: nowrap;
  }
  i {
    display: inline-block;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    margin-right: 0.35rem;
  }
  .legend {
    list-style: none;
    margin: 0.4rem 0 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    font-size: 0.85rem;
    color: var(--ink-2);
  }
  .key-now {
    width: 1rem;
    height: 0;
    border-radius: 0;
    border-top: 1.5px dashed var(--ink-3);
    vertical-align: middle;
  }
  .key-cross {
    background: var(--paper);
    border: 2px solid var(--ink);
  }
</style>
```

- [ ] **Step 14: Write `web/src/components/Library.svelte`** (complete file)

```svelte
<script lang="ts">
  import { api, exportUrl } from '../lib/api';
  import { colorOf, percent, relativeTime } from '../lib/format';
  import { href, router } from '../lib/router.svelte';
  import { count } from '../lib/format';
  import type { ImportReport, ProjectSummary, StarterEntry } from '../lib/types';
  import Mark from './Mark.svelte';
  import ThemeToggle from './ThemeToggle.svelte';

  let projects = $state<ProjectSummary[] | null>(null);
  let error = $state('');
  let creating = $state(false);
  let newName = $state('');
  let confirmDelete = $state('');
  let query = $state('');
  let fileInput = $state<HTMLInputElement | undefined>();
  let startFrom = $state('');
  let templates = $state<StarterEntry[]>([]);
  let examples = $state<StarterEntry[]>([]);
  let imported = $state<{ id: string; name: string; report: ImportReport } | null>(null);

  api.templates().then((t) => (templates = t), () => {});
  api.samples().then((s) => (examples = s), () => {});

  const shown = $derived(
    (projects ?? []).filter((p) => !query.trim() || p.name.toLowerCase().includes(query.trim().toLowerCase())),
  );

  async function load() {
    try {
      projects = await api.listProjects();
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
  load();

  async function create(e: SubmitEvent) {
    e.preventDefault();
    try {
      const p = await api.createProject(newName.trim(), startFrom ? { template: startFrom } : {});
      router.go(href(p.id, 'setup'));
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    }
  }

  async function act(fn: () => Promise<unknown>) {
    try {
      await fn();
      await load();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function openExample(id: string) {
    try {
      const p = await api.createProject('', { sample: id });
      router.go(href(p.id, 'results'));
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function importChosen() {
    const file = fileInput?.files?.[0];
    if (!file) return;
    try {
      const { project, report } = await api.importFile(file);
      if (report) {
        // A spreadsheet: say what came across before opening it.
        imported = { id: project.id, name: project.name, report };
        await load();
      } else {
        router.go(href(project.id, 'results'));
      }
    } catch (e) {
      error = `${file.name} couldn't be imported: ${e instanceof Error ? e.message : e}`;
    }
    if (fileInput) fileInput.value = '';
  }

  // Ring geometry for the completeness indicator.
  const R = 9;
  const C = 2 * Math.PI * R;
</script>

<header class="top">
  <a class="brand" href="#/"><Mark /> <span>AHP Matrix</span></a>
  <ThemeToggle />
</header>

<main class="library">
  <div class="intro">
    <h1>Decisions</h1>
    <div class="actions">
      <button class="btn" onclick={() => fileInput?.click()}>Import file</button>
      <input
        bind:this={fileInput}
        type="file"
        accept=".json,application/json,.xlsx"
        class="sr-only"
        onchange={importChosen}
        tabindex="-1"
        aria-label="Import a decision file or Priority Matrix spreadsheet"
      />
      <button class="btn primary" onclick={() => (creating = true)}>New decision</button>
    </div>
  </div>

  {#if creating}
    <form class="new panel" onsubmit={create}>
      <label for="new-name">What are you deciding?</label>
      <!-- svelte-ignore a11y_autofocus -->
      <input id="new-name" class="field" bind:value={newName} placeholder="Choose a CRM vendor" autofocus />
      <select class="field start" bind:value={startFrom} aria-label="Start from">
        <option value="">Start blank</option>
        {#each templates as t (t.id)}<option value={t.id}>Template: {t.name}</option>{/each}
      </select>
      <button class="btn primary" type="submit">Create</button>
      <button class="btn quiet" type="button" onclick={() => (creating = false)}>Cancel</button>
      {#if startFrom}
        <p class="small muted hint">{templates.find((t) => t.id === startFrom)?.description}</p>
      {/if}
    </form>
  {/if}

  {#if imported}
    <section class="notice panel" role="status">
      <p>
        Imported <strong>{imported.name}</strong>: {count(imported.report.alternatives, 'option')},
        {count(imported.report.criteria, 'criterion', 'criteria')} and {count(imported.report.judgments, 'comparison')}.
      </p>
      {#if imported.report.skipped.length}
        <p class="small">Some cells couldn't be used and were left out:</p>
        <ul class="small">{#each imported.report.skipped as s}<li>{s}</li>{/each}</ul>
      {/if}
      <div class="actions">
        <a class="btn primary" href={href(imported.id, 'results')}>Open it</a>
        <button class="btn quiet" onclick={() => (imported = null)}>Dismiss</button>
      </div>
    </section>
  {/if}

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if projects === null}
    {#if !error}<p class="muted">Loading…</p>{/if}
  {:else if projects.length === 0}
    <section class="empty">
      <Mark size={88} tilt={0} />
      <h2>Weigh your first decision</h2>
      <p class="muted">
        List the options, decide what matters, and compare them two at a time. AHP Matrix turns those comparisons into a
        ranking, and tells you when your answers contradict each other.
      </p>
      <div class="actions">
        <button class="btn primary" onclick={() => (creating = true)}>New decision</button>
        <button class="btn" onclick={() => fileInput?.click()}>Import a Priority Matrix spreadsheet</button>
      </div>
      {#if examples.length}
        <h3>Or explore an example</h3>
        <ul class="examples">
          {#each examples as ex (ex.id)}
            <li>
              <button class="example" onclick={() => openExample(ex.id)}>
                <strong>{ex.name}</strong>
                <span class="small muted">{ex.description}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {:else}
    {#if projects.length > 6}
      <input class="field search" type="search" placeholder="Find a decision" bind:value={query} aria-label="Find a decision" />
    {/if}
    <ul class="rows">
      {#each shown as p (p.file)}
        <li class="row" class:broken={!!p.error}>
          {#if p.error}
            <div class="name">
              <span>{p.file}</span>
              <span class="small rust">{p.error}</span>
            </div>
          {:else}
            <a class="name" href={href(p.id!, p.alternatives && p.criteria ? 'results' : 'setup')}>
              <span class="title">{p.name}</span>
              <span class="small muted">
                {#if p.ranking.length && p.alternatives >= 2 && p.criteria > 0}
                  {p.ranking[0].name} leads with {percent(p.ranking[0].score, 0)}
                {:else}
                  {count(p.alternatives, 'option')}, {count(p.criteria, 'criterion', 'criteria')}
                {/if}
              </span>
            </a>
            <div class="strip" aria-hidden="true">
              {#each p.ranking as r}
                <span style:flex-grow={Math.max(r.score, 0.0001)} style:background={colorOf(r.color)} title="{r.name} {percent(r.score)}"></span>
              {/each}
            </div>
            <div class="status small">
              {#if p.alternatives < 2 || p.criteria === 0}
                <span class="muted">Not set up yet</span>
              {:else}
              <svg width="24" height="24" viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="12" r={R} fill="none" stroke="var(--line)" stroke-width="3" />
                <circle
                  cx="12"
                  cy="12"
                  r={R}
                  fill="none"
                  stroke="var(--brass)"
                  stroke-width="3"
                  stroke-dasharray="{C * p.completeness} {C}"
                  transform="rotate(-90 12 12)"
                  stroke-linecap="round"
                />
              </svg>
              <span>{percent(p.completeness, 0)} judged</span>
              {#if !p.consistent}<span class="rust">Contradictions to review</span>{/if}
              {/if}
            </div>
            <span class="small muted when">{relativeTime(p.updatedAt)}</span>
            <div class="more">
              {#if confirmDelete === p.id}
                <span class="small">Move to trash?</span>
                <button class="btn quiet danger" onclick={() => act(() => api.deleteProject(p.id!))}>Delete</button>
                <button class="btn quiet" onclick={() => (confirmDelete = '')}>Keep</button>
              {:else}
                <button class="btn quiet" onclick={() => act(() => api.duplicateProject(p.id!))}>Duplicate</button>
                <a class="btn quiet" href={exportUrl(p.id!)} download>Export</a>
                <button class="btn quiet" onclick={() => (confirmDelete = p.id!)}>Delete</button>
              {/if}
            </div>
          {/if}
        </li>
      {/each}
    </ul>
    {#if shown.length === 0}<p class="muted">No decision matches “{query}”.</p>{/if}
    {#if examples.length}
      <p class="small muted more-examples">
        Examples:
        {#each examples as ex, i (ex.id)}{#if i > 0}, {/if}<button class="link" onclick={() => openExample(ex.id)}>{ex.name}</button>{/each}
      </p>
    {/if}
  {/if}
</main>

<style>
  .top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.9rem clamp(1rem, 4vw, 2.5rem);
  }
  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    text-decoration: none;
    font-family: var(--font-display);
    font-weight: 700;
    font-size: 1.1rem;
    letter-spacing: -0.01em;
  }
  .library {
    max-width: 72rem;
    margin: 0 auto;
    padding: 1.5rem clamp(1rem, 4vw, 2.5rem) 4rem;
    display: grid;
    gap: 1.25rem;
  }
  .intro {
    display: flex;
    justify-content: space-between;
    align-items: end;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .intro h1 {
    font-size: clamp(2.2rem, 5vw, 3.4rem);
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .new {
    display: grid;
    grid-template-columns: auto 1fr auto auto auto;
    align-items: center;
    gap: 0.6rem;
    padding: 0.9rem 1rem;
  }
  .new label {
    font-weight: 600;
  }
  .start {
    width: auto;
  }
  .hint {
    grid-column: 1 / -1;
  }
  .notice {
    display: grid;
    gap: 0.5rem;
    padding: 1rem 1.2rem;
  }
  .notice ul {
    margin: 0;
    padding-left: 1.2rem;
  }
  .examples {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.5rem;
    width: 100%;
  }
  .example {
    display: grid;
    gap: 0.15rem;
    width: 100%;
    text-align: left;
    padding: 0.75rem 1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    background: var(--paper);
    cursor: pointer;
  }
  .example:hover {
    border-color: var(--brass);
  }
  .link {
    border: 0;
    background: none;
    padding: 0;
    color: var(--ink-2);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
    font: inherit;
  }
  .more-examples {
    margin-top: 0.5rem;
  }
  .error {
    color: var(--rust);
  }
  .empty {
    display: grid;
    justify-items: start;
    gap: 0.9rem;
    padding: 3rem 0;
    max-width: 36rem;
  }
  .search {
    max-width: 22rem;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  .row {
    display: grid;
    grid-template-columns: minmax(12rem, 2fr) minmax(8rem, 1.4fr) minmax(9rem, 1fr) 7rem auto;
    align-items: center;
    gap: 1.25rem;
    padding: 0.9rem 0.25rem;
    border-bottom: 1px solid var(--line);
  }
  .row:hover {
    background: color-mix(in oklab, var(--paper) 55%, var(--fog));
  }
  .name {
    display: grid;
    text-decoration: none;
    min-width: 0;
  }
  .title {
    font-weight: 650;
    font-size: 1.05rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  a.name:hover .title {
    text-decoration: underline;
    text-decoration-color: var(--brass);
    text-underline-offset: 3px;
  }
  .strip {
    display: flex;
    gap: 2px;
    height: 10px;
    border-radius: 3px;
    overflow: hidden;
    background: var(--line);
  }
  .strip span {
    flex-basis: 0;
    min-width: 2px;
  }
  .status {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
  }
  .rust {
    color: var(--rust);
  }
  .more {
    display: flex;
    gap: 0.15rem;
    align-items: center;
    justify-content: end;
    opacity: 0.55;
  }
  .row:hover .more,
  .more:focus-within {
    opacity: 1;
  }
  .broken {
    grid-template-columns: 1fr;
  }
  @media (max-width: 860px) {
    .row {
      grid-template-columns: 1fr auto;
    }
    .strip {
      grid-column: 1 / -1;
      order: 3;
    }
    .status,
    .when {
      display: none;
    }
    .new {
      grid-template-columns: 1fr;
    }
  }
</style>
```

- [ ] **Step 15: Write `web/src/components/Setup.svelte`** (complete file)

```svelte
<script lang="ts">
  import { ApiError, api, exportUrl, type MergePolicy } from '../lib/api';
  import { addAlternative, addEvaluator, changeScale, removeAlternative, removeEvaluator, setEvaluatorWeight } from '../lib/edits';
  import { colorOf, count } from '../lib/format';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import type { Scale } from '../lib/types';
  import CriteriaTree from './CriteriaTree.svelte';

  let { session }: { session: ProjectSession } = $props();
  const p = $derived(session.project);

  let newAlt = $state('');
  let newEvaluator = $state('');
  let pendingScale = $state<Scale | null>(null);
  let mergeInput = $state<HTMLInputElement | undefined>();
  let mergeFile = $state<File | null>(null);
  let mergeConflict = $state<string[]>([]);
  let mergeNote = $state('');
  let mergeError = $state('');

  // Brings a returned file's judgments in. Unsaved edits are saved first; if
  // the file holds judgments for people already here, ask before replacing.
  async function merge(existing?: MergePolicy) {
    const file = mergeFile;
    if (!file) return;
    mergeError = mergeNote = '';
    await session.flush();
    if (session.unsaved) {
      mergeError = 'Fix the unsaved changes first; then merge.';
      return;
    }
    try {
      const { report } = await api.mergeFile(p.id, file, existing);
      await session.reload();
      mergeConflict = [];
      const who = [...report.added, ...report.replaced];
      mergeNote =
        who.length === 0
          ? 'Nothing merged: the current judgments were kept.'
          : `Merged ${count(report.judgments, 'judgment')} from ${who.join(', ')}.` +
            (report.dropped ? ` ${count(report.dropped, 'judgment')} about options or criteria no longer in this decision ${report.dropped === 1 ? 'was' : 'were'} left out.` : '');
      mergeFile = null;
    } catch (e) {
      if (e instanceof ApiError && e.code === 'evaluator-exists') {
        mergeConflict = (e.details as { evaluators: string[] }).evaluators;
      } else {
        mergeError = `${file.name} couldn't be merged: ${e instanceof Error ? e.message : e}`;
        mergeFile = null;
      }
    }
    if (mergeInput) mergeInput.value = '';
  }

  function edit(fn: () => void) {
    fn();
    session.changed();
  }

  function submitAlt(e: SubmitEvent) {
    e.preventDefault();
    if (!newAlt.trim()) return;
    edit(() => addAlternative(session.project, newAlt));
    newAlt = '';
  }

  function submitEvaluator(e: SubmitEvent) {
    e.preventDefault();
    if (!newEvaluator.trim()) return;
    edit(() => addEvaluator(session.project, newEvaluator));
    newEvaluator = '';
  }

  function pickScale(s: Scale) {
    if (s === p.scale) return;
    if (p.pairwise.length === 0) edit(() => changeScale(session.project, s));
    else pendingScale = s;
  }

  const scales: { id: Scale; name: string; detail: string }[] = [
    { id: 'saaty9', name: '1 to 9', detail: 'The standard AHP scale: equal, moderately, strongly, very strongly, extremely.' },
    { id: 'classic5', name: '10, 5, 1', detail: 'The coarser scale from the original Priority Matrix spreadsheet.' },
  ];
</script>

<div class="setup">
  <section class="about">
    <label class="small muted" for="desc">What is this decision about?</label>
    <textarea
      id="desc"
      class="field"
      rows="2"
      placeholder="Context for anyone who opens this later: the goal, deadlines, constraints."
      bind:value={session.project.description}
      oninput={() => session.changed()}
    ></textarea>
  </section>

  <section>
    <header>
      <h2>Options</h2>
      <p class="muted">The things you're choosing between.</p>
    </header>
    <ul class="list">
      {#each p.alternatives as a (a.id)}
        <li class="item">
          <span class="chip" style:background={colorOf(a.color)} aria-hidden="true"></span>
          <input class="field inline name" aria-label="Option name" bind:value={a.name} oninput={() => session.changed()} />
          <input
            class="field inline desc"
            aria-label="Notes about {a.name}"
            placeholder="Notes"
            bind:value={a.description}
            oninput={() => session.changed()}
          />
          <button class="btn quiet" aria-label="Remove {a.name}" title="Remove" onclick={() => edit(() => removeAlternative(session.project, a.id))}>✕</button>
        </li>
      {/each}
    </ul>
    <form class="add" onsubmit={submitAlt}>
      <input class="field" placeholder="Add an option, then press Enter" aria-label="New option" bind:value={newAlt} />
      <button class="btn" type="submit" disabled={!newAlt.trim()}>Add</button>
    </form>
  </section>

  <section>
    <header>
      <h2>Criteria</h2>
      <p class="muted">What matters. Group related criteria by dragging one onto another; you'll only compare criteria within the same group.</p>
    </header>
    <CriteriaTree {session} />
  </section>

  <section>
    <header>
      <h2>People</h2>
      <p class="muted">Everyone whose judgments count. Each person compares on their own; results combine them, weighted.</p>
    </header>
    <ul class="list">
      {#each p.evaluators as e (e.id)}
        <li class="item">
          <input class="field inline name" aria-label="Person's name" bind:value={e.name} oninput={() => session.changed()} />
          <label class="small muted weight">
            Weight
            <input
              class="field"
              type="number"
              min="0.1"
              step="0.1"
              value={e.weight}
              onchange={(ev) => edit(() => setEvaluatorWeight(session.project, e.id, ev.currentTarget.valueAsNumber))}
            />
          </label>
          <a class="btn quiet small" href={exportUrl(p.id, 'ahp.json', e.id)} download title="A copy with only {e.name}'s judgments, for them to fill in">
            Send their copy
          </a>
          <button
            class="btn quiet"
            aria-label="Remove {e.name}"
            title={p.evaluators.length === 1 ? 'A decision needs at least one person' : 'Remove, with their judgments'}
            disabled={p.evaluators.length === 1}
            onclick={() =>
              edit(() => {
                removeEvaluator(session.project, e.id);
                if (session.scoringAs === e.id) session.scoringAs = session.project.evaluators[0].id;
              })}>✕</button
          >
        </li>
      {/each}
    </ul>
    <form class="add" onsubmit={submitEvaluator}>
      <input class="field" placeholder="Add a person" aria-label="New person" bind:value={newEvaluator} />
      <button class="btn" type="submit" disabled={!newEvaluator.trim()}>Add</button>
    </form>
    <div class="remote">
      <p class="small muted">
        Someone judging remotely? Send them their copy. They open it with Import file in their own AHP Matrix, make their comparisons,
        export it from the library, and send it back for you to merge.
      </p>
      <button class="btn" onclick={() => mergeInput?.click()}>Merge a returned file</button>
      <input
        bind:this={mergeInput}
        type="file"
        accept=".json,application/json"
        class="sr-only"
        tabindex="-1"
        aria-label="Returned decision file to merge"
        onchange={() => {
          mergeFile = mergeInput?.files?.[0] ?? null;
          merge();
        }}
      />
      {#if mergeConflict.length}
        <div class="confirm" role="alert">
          <span>
            {mergeConflict.join(', ')} already {mergeConflict.length === 1 ? 'has' : 'have'} judgments here. Replace them with the ones in
            {mergeFile?.name}, or keep the current ones?
          </span>
          <button class="btn" onclick={() => merge('replace')}>Replace</button>
          <button class="btn quiet" onclick={() => merge('skip')}>Keep current</button>
        </div>
      {/if}
      {#if mergeNote}<p class="small" role="status">{mergeNote}</p>{/if}
      {#if mergeError}<p class="small rust" role="alert">{mergeError}</p>{/if}
    </div>
  </section>

  <section>
    <header>
      <h2>Comparison scale</h2>
    </header>
    <div class="scales" role="radiogroup" aria-label="Comparison scale">
      {#each scales as s}
        <button class="scale" role="radio" aria-checked={p.scale === s.id} onclick={() => pickScale(s.id)}>
          <strong>{s.name}</strong>
          <span class="small muted">{s.detail}</span>
        </button>
      {/each}
    </div>
    {#if pendingScale}
      <div class="confirm" role="alert">
        <span>Existing comparisons will be rounded to the nearest value on the new scale.</span>
        <button
          class="btn"
          onclick={() => {
            edit(() => changeScale(session.project, pendingScale!));
            pendingScale = null;
          }}>Change scale</button
        >
        <button class="btn quiet" onclick={() => (pendingScale = null)}>Cancel</button>
      </div>
    {/if}
  </section>

  {#if p.alternatives.length >= 2 && p.criteria.length >= 1}
    <a class="btn primary next" href={href(p.id, 'weigh')}>Next: weigh the criteria</a>
  {/if}
</div>

<style>
  .setup {
    display: grid;
    gap: 2.5rem;
    max-width: 52rem;
  }
  section {
    display: grid;
    gap: 0.75rem;
  }
  header {
    display: grid;
    gap: 0.25rem;
  }
  .about textarea {
    resize: vertical;
    min-height: 3.5rem;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    border-top: 1px solid var(--line);
  }
  .item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.3rem 0;
    border-bottom: 1px solid var(--line);
  }
  .chip {
    flex: none;
    width: 0.8rem;
    height: 0.8rem;
    border-radius: 3px;
  }
  .name {
    flex: 1 1 14rem;
    font-weight: 600;
  }
  .desc {
    flex: 2 1 14rem;
    color: var(--ink-2);
  }
  .weight {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }
  .weight input {
    width: 5rem;
  }
  .add {
    display: flex;
    gap: 0.5rem;
    max-width: 30rem;
  }
  .remote {
    display: grid;
    gap: 0.5rem;
    justify-items: start;
  }
  .rust {
    color: var(--rust);
  }
  .scales {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: 0.75rem;
  }
  .scale {
    display: grid;
    gap: 0.25rem;
    text-align: left;
    padding: 0.8rem 1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    background: var(--paper);
    cursor: pointer;
  }
  .scale[aria-checked='true'] {
    border-color: var(--brass);
    box-shadow: inset 0 0 0 1px var(--brass);
    background: var(--brass-wash);
  }
  .confirm {
    display: flex;
    gap: 0.6rem;
    align-items: center;
    flex-wrap: wrap;
  }
  .next {
    justify-self: start;
  }
</style>
```

- [ ] **Step 16: Write `web/src/components/Results.svelte`** (complete file)

```svelte
<script lang="ts">
  import { exportUrl } from '../lib/api';
  import { colorOf, percent } from '../lib/format';
  import { href } from '../lib/router.svelte';
  import { fraction } from '../lib/scale';
  import type { ProjectSession } from '../lib/session.svelte';
  import { path } from '../lib/tree';
  import { ALL_EVALUATORS } from '../lib/types';
  import Contributions from './charts/Contributions.svelte';
  import PriorityBars from './charts/PriorityBars.svelte';
  import WeightTree from './charts/WeightTree.svelte';

  let { session }: { session: ProjectSession } = $props();
  const p = $derived(session.project);
  let viewing = $state(ALL_EVALUATORS);
  $effect(() => session.watch(viewing));
  const r = $derived(session.results[viewing]);

  const ranked = $derived(r ? [...r.scores].sort((a, b) => b.score - a.score) : []);
  const altName = (id: string) => p.alternatives.find((a) => a.id === id)?.name ?? id;
  const crit = (id: string) => (id ? path(p, id).join(' › ') : '');
  const itemName = (id: string) => p.alternatives.find((a) => a.id === id)?.name ?? p.criteria.find((c) => c.id === id)?.name ?? id;
  const evName = (id: string) => p.evaluators.find((e) => e.id === id)?.name ?? id;

  // Where to go to review a matrix: weigh/<group> or score/<leaf>.
  function reviewHref(context: string) {
    const [kind, id] = context.split(':');
    return kind === 'alts' ? href(p.id, 'score', id) : href(p.id, 'weigh', id);
  }
  function matrixName(context: string) {
    const [kind, id] = context.split(':');
    if (kind === 'alts') return `Options on ${crit(id)}`;
    return id === 'root' ? 'Top-level criteria' : `Criteria within ${crit(id)}`;
  }

  // Only differences of opinion worth discussing: roughly a factor of two apart or more.
  const disagreements = $derived((r?.disagreements ?? []).filter((d) => d.spread >= 1.4).slice(0, 6));
  const lead = $derived(ranked.length >= 2 ? ranked[1].score > 0 ? (ranked[0].score - ranked[1].score) / ranked[0].score : 1 : 0);
</script>

<div class="results">
  {#if p.evaluators.length > 1}
    <div class="who" role="tablist" aria-label="Whose judgments">
      <button role="tab" class="btn quiet" aria-selected={viewing === ALL_EVALUATORS} onclick={() => (viewing = ALL_EVALUATORS)}>Everyone combined</button>
      {#each p.evaluators as e (e.id)}
        <button role="tab" class="btn quiet" aria-selected={viewing === e.id} onclick={() => (viewing = e.id)}>{e.name}</button>
      {/each}
    </div>
  {/if}

  {#if !r}
    <p class="muted">Working out the results…</p>
  {:else if p.alternatives.length < 2 || p.criteria.length === 0}
    <div class="empty">
      <h1>No results yet</h1>
      <p class="muted">Add at least two options and one criterion, then compare them.</p>
      <a class="btn primary" href={href(p.id, 'setup')}>Go to setup</a>
    </div>
  {:else}
    <section class="headline">
      <h1>
        {altName(ranked[0].id)} comes out ahead{ranked.length > 1 ? `, ${lead < 0.06 ? 'only just' : lead < 0.3 ? 'clearly' : 'by a wide margin'}` : ''}.
      </h1>
      <p class="muted">
        {percent(ranked[0].score)} of the overall weight, against {percent(ranked[1].score)} for {altName(ranked[1].id)}.
        {#if r.completeness < 1}
          These results are provisional: {percent(r.completeness, 0)} of the judgments are in.
        {/if}
      </p>
    </section>

    <PriorityBars
      big
      sorted
      names={p.alternatives.map((a) => a.name)}
      colors={p.alternatives.map((a) => colorOf(a.color))}
      values={r.scores.map((s) => s.score)}
    />

    {#if r.issues.length}
      <section class="issues">
        <h2>Worth fixing first</h2>
        <ul>
          {#each r.issues as i}
            <li>
              {#if i.criterionId}<strong>{crit(i.criterionId)}:</strong>{/if}
              {i.message}
              {#if i.context}<a href={reviewHref(i.context)}>Open</a>{:else if i.criterionId}<a href={href(p.id, 'score', i.criterionId)}>Open</a>{/if}
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    <section>
      <h2>Where each score comes from</h2>
      <p class="muted small">Each cell is a criterion's weight times how well the option does on it. The cells in a row add up to its total.</p>
      <Contributions project={p} results={r} />
    </section>

    <section>
      <h2>How much each criterion counts</h2>
      <WeightTree project={p} results={r} />
    </section>

    <section>
      <h2>Consistency of the comparisons</h2>
      <table class="matrices">
        <tbody>
          {#each r.matrices as m (m.context)}
            <tr>
              <th scope="row">{matrixName(m.context)}</th>
              <td>
                {#if !m.connected}<span class="muted">Not enough comparisons yet</span>
                {:else if m.items.length < 3}<span class="muted">Two items: always consistent</span>
                {:else}
                  <span class:bad={!m.consistent} class:ok={m.consistent}>{m.consistent ? 'Consistent' : 'Contradictions'}</span>
                  <span class="muted small">ratio {m.cr.toFixed(2)} of {m.threshold.toFixed(2)}{m.complete ? '' : ', provisional'}</span>
                {/if}
              </td>
              <td><a href={reviewHref(m.context)}>Review</a></td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>

    {#if disagreements.length}
      <section>
        <h2>Where people disagree most</h2>
        <p class="muted small">Each number is how strongly the first is preferred over the second: 1 is equal, 9 is extreme, 1/9 is the reverse.</p>
        <ul class="disagree">
          {#each disagreements as d}
            <li>
              <span class="pair">{itemName(d.a)} vs {itemName(d.b)} <span class="muted small">({matrixName(d.context)})</span></span>
              <span class="small">
                {#each Object.entries(d.values) as [ev, v], k}{k ? ', ' : ''}{evName(ev)} said {fraction(v)}{/each}
              </span>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    <div class="foot no-print">
      <a class="btn" href={href(p.id, 'sensitivity')}>Test how robust this is</a>
      <a class="btn quiet" href={href(p.id, 'report')}>Printable report</a>
      <a class="btn quiet" href={exportUrl(p.id, 'xlsx', viewing)} download>Download Excel</a>
      <a class="btn quiet" href={exportUrl(p.id, 'csv', viewing)} download>Download CSV</a>
    </div>
  {/if}
</div>

<style>
  .results {
    display: grid;
    gap: 2.25rem;
    max-width: 58rem;
  }
  section {
    display: grid;
    gap: 0.75rem;
  }
  .headline h1 {
    font-size: clamp(1.9rem, 4vw, 2.8rem);
    max-width: 22ch;
  }
  .headline {
    gap: 0.5rem;
  }
  .who {
    display: flex;
    gap: 0.25rem;
    flex-wrap: wrap;
  }
  .who [aria-selected='true'] {
    background: var(--paper);
    color: var(--ink);
    box-shadow: inset 0 -2px 0 var(--brass);
  }
  .empty {
    display: grid;
    gap: 0.9rem;
    justify-items: start;
  }
  .issues {
    padding: 1rem 1.2rem;
    background: var(--rust-wash);
    border-radius: var(--radius-m);
  }
  .issues ul {
    margin: 0;
    padding-left: 1.1rem;
    display: grid;
    gap: 0.3rem;
  }
  .matrices {
    border-collapse: collapse;
    width: 100%;
  }
  .matrices th,
  .matrices td {
    text-align: left;
    padding: 0.45rem 0.5rem;
    border-bottom: 1px solid var(--line);
    font-weight: 400;
  }
  .matrices th {
    font-weight: 550;
  }
  .ok {
    color: var(--good);
    font-weight: 600;
  }
  .bad {
    color: var(--rust);
    font-weight: 600;
  }
  .disagree {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.4rem;
  }
  .disagree li {
    display: grid;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--line);
  }
  .pair {
    font-weight: 600;
  }
  .foot {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
</style>
```

- [ ] **Step 17: Write `web/src/components/Sensitivity.svelte`** (complete file)

```svelte
<script lang="ts">
  import { colorOf, percent } from '../lib/format';
  import { href } from '../lib/router.svelte';
  import { at, leaderAt, lineFor, shiftLabel } from '../lib/sensitivity';
  import type { ProjectSession } from '../lib/session.svelte';
  import { path } from '../lib/tree';
  import { ALL_EVALUATORS } from '../lib/types';
  import LineChart from './charts/LineChart.svelte';

  let { session, sub }: { session: ProjectSession; sub: string } = $props();
  const p = $derived(session.project);
  const r = $derived(session.results[ALL_EVALUATORS]);

  // Criteria that can be varied (they have siblings), most fragile first.
  const options = $derived(
    (r?.criteria ?? [])
      .filter((c) => lineFor(r!, c.id) !== null)
      .sort((x, y) => (x.stability ?? Infinity) - (y.stability ?? Infinity)),
  );
  const chosen = $derived(options.find((c) => c.id === sub) ?? options[0]);
  const line = $derived(chosen && r ? lineFor(r, chosen.id) : null);

  let x = $state(0);
  let lastChosen = '';
  $effect(() => {
    if (chosen && chosen.id !== lastChosen) {
      lastChosen = chosen.id;
      x = chosen.local;
    }
  });

  const names = $derived(p.alternatives.map((a) => a.name));
  const colors = $derived(p.alternatives.map((a) => colorOf(a.color)));
  const ranking = $derived(line ? at(line, x).map((s, a) => ({ a, s })).sort((m, n) => n.s - m.s) : []);
  const leaderNow = $derived(line && chosen ? leaderAt(line, chosen.local) : 0);
  const changed = $derived(line ? leaderAt(line, x) !== leaderNow : false);

  const verdict = (stability: number | null) => {
    if (stability === null) return 'The leader holds at any weight';
    const points = Math.round(stability * 100);
    return `The leader changes if this weight moves ${points < 1 ? 'less than 1 point' : `${points} point${points === 1 ? '' : 's'}`}`;
  };
</script>

{#if !r}
  <p class="muted">Working out the results…</p>
{:else if !chosen || !line}
  <div class="empty">
    <h1>Nothing to vary yet</h1>
    <p class="muted">What-if analysis changes the weight of one criterion against the others in its group, so you need at least two criteria and two options.</p>
    <a class="btn primary" href={href(p.id, 'setup')}>Go to setup</a>
  </div>
{:else}
  <div class="layout">
    <nav aria-label="Criteria">
      <p class="small muted pick">Most fragile first</p>
      <ul>
        {#each options as c (c.id)}
          <li>
            <a href={href(p.id, 'sensitivity', c.id)} class:current={c.id === chosen.id} aria-current={c.id === chosen.id ? 'true' : undefined}>
              <span class="name">{path(p, c.id).join(' › ')}</span>
              <span class="small" class:fragile={c.stability !== null && c.stability < 0.1}>
                {shiftLabel(c.stability)}
              </span>
            </a>
          </li>
        {/each}
      </ul>
    </nav>

    <section class="main">
      <h1>What if {chosen ? p.criteria.find((c) => c.id === chosen.id)?.name : ''} counted more, or less?</h1>
      <p class="muted">{verdict(chosen.stability)}. Drag to see how the ranking responds; the other criteria in its group share the rest.</p>

      <label class="slider">
        <span class="small">Weight within its group: <strong>{percent(x, 0)}</strong> <span class="muted">(now {percent(chosen.local, 0)})</span></span>
        <input type="range" min="0" max="1" step="0.005" bind:value={x} />
      </label>

      <LineChart {line} {names} {colors} current={chosen.local} {x} />

      <div class="readout" class:changed aria-live="polite">
        {#if changed}
          <strong>{names[ranking[0].a]} would lead instead of {names[leaderNow]}.</strong>
        {:else}
          <strong>{names[ranking[0].a]} still leads.</strong>
        {/if}
        <ol>
          {#each ranking as row (row.a)}
            <li><i style:background={colors[row.a]}></i>{names[row.a]} <span class="muted">{percent(row.s)}</span></li>
          {/each}
        </ol>
        <button class="btn quiet" onclick={() => (x = chosen.local)}>Reset</button>
      </div>
    </section>
  </div>
{/if}

<style>
  .layout {
    display: grid;
    grid-template-columns: 16rem minmax(0, 1fr);
    gap: 2rem;
    align-items: start;
  }
  nav ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 2px;
  }
  .pick {
    margin: 0 0 0.4rem 0.65rem;
  }
  nav a {
    display: grid;
    padding: 0.5rem 0.65rem;
    border-radius: var(--radius-s);
    text-decoration: none;
  }
  nav a:hover {
    background: color-mix(in oklab, var(--paper) 60%, var(--fog));
  }
  nav a.current {
    background: var(--paper);
    box-shadow: inset 3px 0 0 var(--brass);
  }
  .name {
    font-weight: 550;
  }
  .fragile {
    color: var(--rust);
    font-weight: 600;
  }
  .main {
    display: grid;
    gap: 1rem;
    max-width: 48rem;
  }
  .slider {
    display: grid;
    gap: 0.4rem;
  }
  .slider input {
    width: 100%;
    accent-color: var(--brass);
  }
  .readout {
    display: grid;
    gap: 0.4rem;
    justify-items: start;
    padding: 0.9rem 1.1rem;
    border-radius: var(--radius-m);
    background: var(--paper);
    border: 1px solid var(--line);
  }
  .readout.changed {
    border-color: var(--brass);
    background: var(--brass-wash);
  }
  .readout ol {
    margin: 0;
    padding-left: 1.2rem;
  }
  i {
    display: inline-block;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    margin-right: 0.4rem;
  }
  .empty {
    display: grid;
    gap: 0.9rem;
    justify-items: start;
    max-width: 36rem;
  }
  @media (max-width: 1000px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
</style>
```

- [ ] **Step 18: Write `web/src/components/Workspace.svelte`** (complete file)

```svelte
<script lang="ts">
  import { count } from '../lib/format';
  import { scoreProgress, weighProgress } from '../lib/progress';
  import { href, type Step } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import Mark from './Mark.svelte';
  import Results from './Results.svelte';
  import Score from './Score.svelte';
  import Sensitivity from './Sensitivity.svelte';
  import Setup from './Setup.svelte';
  import ThemeToggle from './ThemeToggle.svelte';
  import Weigh from './Weigh.svelte';

  let { session, step, sub }: { session: ProjectSession; step: Step; sub: string } = $props();
  const p = $derived(session.project);

  const weigh = $derived(weighProgress(p, session.scoringAs));
  const score = $derived(scoreProgress(p, session.scoringAs));
  const ready = $derived(p.alternatives.length >= 2 && p.criteria.length >= 1);

  const steps = $derived([
    { id: 'setup' as Step, label: 'Set up', detail: `${count(p.alternatives.length, 'option')}, ${count(p.criteria.length, 'criterion', 'criteria')}`, done: ready },
    { id: 'weigh' as Step, label: 'Weigh criteria', detail: weigh.total ? `${weigh.done} of ${weigh.total} compared` : 'Nothing to compare yet', done: weigh.total > 0 && weigh.done === weigh.total },
    { id: 'score' as Step, label: 'Score options', detail: score.total ? `${score.done} of ${score.total} judged` : 'Nothing to score yet', done: score.total > 0 && score.done === score.total },
    { id: 'results' as Step, label: 'Results', detail: '', done: false },
    { id: 'sensitivity' as Step, label: 'What if', detail: 'How robust is the winner?', done: false },
  ]);

  const saveLabel = $derived(
    {
      saved: 'All changes saved',
      pending: 'Saving…',
      saving: 'Saving…',
      conflict: 'Not saved',
      error: 'Not saved',
    }[session.saveState],
  );
</script>

<div class="shell">
  <header class="bar no-print">
    <a class="home" href="#/" title="All decisions" aria-label="All decisions"><Mark size={26} /></a>
    <input
      class="field inline title"
      aria-label="Decision name"
      bind:value={session.project.name}
      oninput={() => session.changed()}
    />
    <span class="save small" class:warn={session.saveState === 'error' || session.saveState === 'conflict'} aria-live="polite">{saveLabel}</span>
    <span class="spacer"></span>
    {#if p.evaluators.length > 1}
      <label class="small who">
        Judging as
        <select class="field" bind:value={session.scoringAs}>
          {#each p.evaluators as e (e.id)}<option value={e.id}>{e.name}</option>{/each}
        </select>
      </label>
    {/if}
    <ThemeToggle />
  </header>

  {#if session.saveState === 'conflict'}
    <div class="banner no-print" role="alert">
      <span>{session.error} Keep your version, or load the other one and lose your recent edits?</span>
      <button class="btn" onclick={() => session.overwrite()}>Keep mine</button>
      <button class="btn quiet" onclick={() => session.reload()}>Load the other version</button>
    </div>
  {:else if session.saveState === 'error'}
    <div class="banner no-print" role="alert">
      <span><strong>Not saved.</strong> {session.error}</span>
      <button class="btn" onclick={() => session.retry()}>Try again</button>
    </div>
  {/if}

  <nav class="rail no-print" aria-label="Steps">
    <ol>
      {#each steps as s, i (s.id)}
        <li>
          <a href={href(p.id, s.id)} class:current={s.id === step} aria-current={s.id === step ? 'page' : undefined}>
            <span class="num" class:done={s.done}>{#if s.done}✓{:else}{i + 1}{/if}</span>
            <span class="label">
              <span>{s.label}</span>
              {#if s.detail}<span class="small muted">{s.detail}</span>{/if}
            </span>
          </a>
        </li>
      {/each}
    </ol>
    <a class="btn quiet small report" href={href(p.id, 'report')}>Printable report</a>
  </nav>

  <main class="content">
    {#if step === 'setup'}
      <Setup {session} />
    {:else if step === 'weigh'}
      <Weigh {session} {sub} />
    {:else if step === 'score'}
      <Score {session} {sub} />
    {:else if step === 'results'}
      <Results {session} />
    {:else}
      <Sensitivity {session} {sub} />
    {/if}
  </main>
</div>

<style>
  .shell {
    display: grid;
    grid-template-columns: 15.5rem 1fr;
    grid-template-rows: auto auto 1fr;
    grid-template-areas: 'bar bar' 'banner banner' 'rail content';
    min-height: 100vh;
  }
  .bar {
    grid-area: bar;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem clamp(0.75rem, 2vw, 1.5rem);
    border-bottom: 1px solid var(--line);
    background: var(--paper);
    position: sticky;
    top: 0;
    z-index: 5;
  }
  .home {
    display: inline-flex;
  }
  .title {
    font-family: var(--font-display);
    font-weight: 650;
    font-size: 1.15rem;
    max-width: 28rem;
  }
  .save {
    color: var(--ink-3);
    white-space: nowrap;
  }
  .save.warn {
    color: var(--rust);
  }
  .spacer {
    flex: 1;
  }
  .who {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--ink-2);
  }
  .who select {
    width: auto;
  }
  .banner {
    grid-area: banner;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 1.5rem;
    background: var(--rust-wash);
    border-bottom: 1px solid var(--line);
    flex-wrap: wrap;
  }
  .rail {
    grid-area: rail;
    position: sticky;
    top: 3.3rem;
    align-self: start;
    max-height: calc(100vh - 3.3rem);
    overflow-y: auto;
    padding: 1.5rem 0.75rem;
    border-right: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .rail ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.15rem;
  }
  .rail a {
    display: flex;
    gap: 0.7rem;
    align-items: flex-start;
    padding: 0.55rem 0.6rem;
    border-radius: var(--radius-s);
    text-decoration: none;
  }
  .rail a:hover {
    background: color-mix(in oklab, var(--paper) 60%, var(--fog));
  }
  .rail a.current {
    background: var(--paper);
    box-shadow: inset 3px 0 0 var(--brass);
  }
  .label {
    display: grid;
    line-height: 1.3;
    font-weight: 550;
  }
  .num {
    flex: none;
    width: 1.5rem;
    height: 1.5rem;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-size: 0.78rem;
    font-weight: 700;
    border: 1.5px solid var(--line-strong);
    color: var(--ink-2);
  }
  .num.done {
    background: var(--brass-ink);
    border-color: var(--brass-ink);
    color: var(--paper);
  }
  .report {
    align-self: flex-start;
  }
  .content {
    grid-area: content;
    padding: 2rem clamp(1rem, 3vw, 3rem) 4rem;
    min-width: 0;
  }
  @media (max-width: 820px) {
    .shell {
      grid-template-columns: 1fr;
      grid-template-areas: 'bar' 'banner' 'rail' 'content';
    }
    .rail {
      position: static;
      max-height: none;
      border-right: 0;
      border-bottom: 1px solid var(--line);
      padding: 0.5rem;
      overflow-x: auto;
    }
    .rail ol {
      grid-auto-flow: column;
    }
    .label .small,
    .report {
      display: none;
    }
    .bar {
      flex-wrap: wrap;
    }
    .title {
      flex: 1 1 10rem;
      min-width: 0;
    }
  }
  @media print {
    .shell {
      display: block;
    }
  }
</style>
```

- [ ] **Step 19: Write `web/src/components/Weigh.svelte`** (complete file)

```svelte
<script lang="ts">
  import { criteriaContext } from '../lib/edits';
  import { count } from '../lib/format';
  import { pairProgress } from '../lib/progress';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { groups, path } from '../lib/tree';
  import PairCompare from './PairCompare.svelte';
  import SubNav from './SubNav.svelte';

  let { session, sub }: { session: ProjectSession; sub: string } = $props();
  const p = $derived(session.project);
  const all = $derived(groups(p));
  const key = (parentId: string) => parentId || 'root';
  const group = $derived(all.find((g) => key(g.parentId) === sub) ?? all[0]);
  const results = $derived(session.results[session.scoringAs]);

  const entries = $derived(
    all.map((g) => {
      const ctx = criteriaContext(g.parentId);
      const prog = pairProgress(p, session.scoringAs, ctx, g.items.map((c) => c.id));
      const report = results?.matrices.find((m) => m.context === ctx);
      return {
        key: key(g.parentId),
        label: g.parentId ? path(p, g.parentId).join(' › ') : 'Top-level criteria',
        href: href(p.id, 'weigh', key(g.parentId)),
        done: prog.done,
        total: prog.total,
        flag: !!report && report.connected && !report.consistent,
        detail: count(g.items.length, 'criterion', 'criteria'),
      };
    }),
  );
</script>

{#if !group}
  <div class="empty">
    <h1>Nothing to weigh yet</h1>
    <p class="muted">Weighing compares criteria with each other, so you need at least two. Add them in setup.</p>
    <a class="btn primary" href={href(p.id, 'setup')}>Go to setup</a>
  </div>
{:else}
  <div class="layout" class:single={all.length === 1}>
    {#if all.length > 1}
      <SubNav label="Criteria groups" {entries} current={key(group.parentId)} />
    {/if}
    <section class="main">
      <h1>
        {group.parentId ? `Within ${p.criteria.find((c) => c.id === group.parentId)?.name}, which matters more?` : 'Which matters more?'}
      </h1>
      {#key group.parentId}
        <PairCompare
          {session}
          context={criteriaContext(group.parentId)}
          items={group.items.map((c) => ({ id: c.id, name: c.name }))}
          kind="criteria"
        />
      {/key}
    </section>
  </div>
{/if}

<style>
  .layout {
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    gap: 2rem;
    align-items: start;
  }
  .layout.single {
    grid-template-columns: minmax(0, 1fr);
  }
  .main {
    display: grid;
    gap: 1.25rem;
    max-width: 46rem;
  }
  .empty {
    display: grid;
    gap: 0.9rem;
    justify-items: start;
    max-width: 34rem;
  }
  @media (max-width: 1000px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
</style>
```

- [ ] **Step 20: Write `web/src/components/Report.svelte`** (complete file)

```svelte
<script lang="ts">
  // A one-page summary for printing or saving as PDF from the browser.
  import { colorOf, count } from '../lib/format';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { flatten } from '../lib/tree';
  import { ALL_EVALUATORS } from '../lib/types';
  import Contributions from './charts/Contributions.svelte';
  import PriorityBars from './charts/PriorityBars.svelte';
  import WeightTree from './charts/WeightTree.svelte';

  let { session }: { session: ProjectSession } = $props();
  const p = $derived(session.project);
  const r = $derived(session.results[ALL_EVALUATORS]);
  const notes = $derived(flatten(p).filter((f) => f.criterion.notes.trim()));
  const today = new Date().toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' });
</script>

<div class="bar no-print">
  <a class="btn quiet" href={href(p.id, 'results')}>Back to results</a>
  <button class="btn primary" onclick={() => window.print()}>Print or save as PDF</button>
</div>

<article class="report">
  <header>
    <h1>{p.name}</h1>
    {#if p.description}<p>{p.description}</p>{/if}
    <p class="muted small">
      {count(p.alternatives.length, 'option')}, {count(p.criteria.length, 'criterion', 'criteria')}, judged by {p.evaluators.map((e) => e.name).join(', ')}. Prepared {today}.
    </p>
  </header>

  {#if r}
    <section>
      <h2>Ranking</h2>
      <PriorityBars sorted names={p.alternatives.map((a) => a.name)} colors={p.alternatives.map((a) => colorOf(a.color))} values={r.scores.map((s) => s.score)} />
      {#if r.completeness < 1}<p class="small">Provisional: {Math.round(r.completeness * 100)}% of the judgments were made.</p>{/if}
      {#if !r.consistent}<p class="small">Some comparisons contradict each other; see the consistency review in the app.</p>{/if}
    </section>
    <section>
      <h2>Criteria weights</h2>
      <WeightTree project={p} results={r} />
    </section>
    <section>
      <h2>Score breakdown</h2>
      <Contributions project={p} results={r} />
    </section>
  {/if}

  {#if notes.length}
    <section>
      <h2>Notes</h2>
      <dl>
        {#each notes as { criterion: c } (c.id)}
          <dt>{c.name}</dt>
          <dd>{c.notes}</dd>
        {/each}
      </dl>
    </section>
  {/if}
</article>

<style>
  .bar {
    display: flex;
    justify-content: space-between;
    padding: 0.75rem 1.5rem;
  }
  .report {
    max-width: 50rem;
    margin: 0 auto 4rem;
    padding: 2.5rem 3rem;
    background: var(--paper);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    display: grid;
    gap: 2rem;
  }
  header {
    display: grid;
    gap: 0.5rem;
  }
  section {
    display: grid;
    gap: 0.75rem;
    break-inside: avoid;
  }
  dt {
    font-weight: 650;
  }
  dd {
    margin: 0 0 0.75rem;
    white-space: pre-wrap;
  }
  @media print {
    .report {
      border: 0;
      padding: 0;
      max-width: none;
    }
  }
</style>
```

- [ ] **Step 21: Run the tests to verify they pass**

Run: `cd web && npx vitest run`
Expected: PASS

- [ ] **Step 22: Generate the spreadsheet fixture for the end-to-end test**

Add a throwaway test in `internal/xlsx` that writes `legacyWorkbook(t).Bytes()` to the path in `$FIXTURE_OUT`, then run:

```bash
FIXTURE_OUT="$PWD/web/e2e/fixtures/Priority Matrix - Example.xlsx" go test ./internal/xlsx -run <throwaway test name>
```
Delete the throwaway test afterwards. The fixture only contains made-up data.

- [ ] **Step 23: Write the end-to-end tests `web/e2e/exchange.spec.ts`, then update `web/e2e/robustness.spec.ts` (weights apply on blur)**

```ts
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';
import { addAll, createDecision, saved } from './helpers';

const fixture = fileURLToPath(new URL('./fixtures/Priority Matrix - Example.xlsx', import.meta.url));

test('an example opens straight into its results', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Buying a family car/ }).first().click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
  await expect(page.getByLabel('Decision name')).toHaveValue('Buying a family car');
});

test('a template starts with its criteria and no options', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'New decision' }).first().click();
  await page.getByLabel('What are you deciding?').fill('CRM vendors');
  await page.getByLabel('Start from').selectOption({ label: 'Template: Vendor selection' });
  await expect(page.getByText('The 23 criteria of the original Priority Matrix spreadsheet')).toBeVisible();
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page.getByLabel('Decision name')).toHaveValue('CRM vendors');
  await expect(page.getByLabel('Criterion name')).toHaveCount(27);
  await expect(page.getByLabel('Option name')).toHaveCount(0);
});

test('a Priority Matrix spreadsheet imports with a summary of what came across', async ({ page }) => {
  await page.goto('/');
  await page.locator('input[type=file]').setInputFiles(fixture);
  await expect(page.getByRole('status')).toContainText('Imported Priority Matrix - Example: 3 options, 3 criteria and 7 comparisons.');
  await expect(page.getByRole('status')).toContainText('Criterion 3!E7: “n/a” is not a number');
  await page.getByRole('link', { name: 'Open it' }).click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
});

test('results download as Excel and CSV', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Choosing a cloud provider/ }).first().click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
  for (const [link, file] of [['Download Excel', 'choosing-a-cloud-provider.xlsx'], ['Download CSV', 'choosing-a-cloud-provider.csv']]) {
    const download = page.waitForEvent('download');
    await page.getByRole('link', { name: link }).click();
    expect((await download).suggestedFilename()).toBe(file);
  }
});

test("a colleague's returned copy merges back, after asking", async ({ page }, info) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  await page.getByRole('link', { name: /Set up/ }).click();

  const download = page.waitForEvent('download');
  await page.getByRole('listitem').filter({ has: page.getByRole('button', { name: 'Remove Product manager' }) }).getByRole('link', { name: 'Send their copy' }).click();
  const copy = await download;
  expect(copy.suggestedFilename()).toBe('hiring-a-senior-engineer-for-product-manager.ahp.json');
  const path = info.outputPath(copy.suggestedFilename());
  await copy.saveAs(path);

  await page.getByLabel('Returned decision file to merge').setInputFiles(path);
  await expect(page.getByRole('alert')).toContainText('Product manager already has judgments here.');
  await page.getByRole('button', { name: 'Replace' }).click();
  // 6 criteria pairs + 4 × 3 option pairs
  await expect(page.getByRole('status')).toContainText('Merged 18 judgments from Product manager.');
});

test('↑ and ↓ move the balance like → and ←', async ({ page }) => {
  await createDecision(page, 'Arrows');
  await addAll(page, 'New criterion', ['Left', 'Right']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  const slider = page.getByRole('slider');
  await slider.press('ArrowUp');
  await expect(page.getByText('Right matters slightly more than Left')).toBeVisible();
  await slider.press('ArrowDown');
  await slider.press('ArrowDown');
  await expect(page.getByText('Left matters slightly more than Right')).toBeVisible();
});
```

```ts
import { expect, test } from '@playwright/test';
import { addAll, createDecision, saved } from './helpers';

test('leaving with changes that could not be saved asks first, and can go back to them', async ({ page }) => {
  await createDecision(page, 'Risky');
  await addAll(page, 'New person', ['Priya']);
  await saved(page);
  await page.getByLabel('Weight').nth(1).fill('-1'); // the server rejects this
  await page.getByLabel('Weight').nth(1).blur(); // weights apply when you finish editing them
  await expect(page.getByText("Priya's weight must be more than 0.")).toBeVisible();
  await page.getByLabel('What is this decision about?').fill('Notes I care about');
  await page.getByRole('link', { name: 'All decisions' }).click();

  await expect(page.getByRole('heading', { name: 'Your changes to Risky aren’t saved' })).toBeVisible();
  await page.getByRole('button', { name: 'Go back to Risky' }).click();
  await expect(page.getByLabel('What is this decision about?')).toHaveValue('Notes I care about');

  await page.getByLabel('Weight').nth(1).fill('1');
  await page.getByLabel('Weight').nth(1).blur();
  await saved(page);
  await page.getByRole('link', { name: 'All decisions' }).click();
  await expect(page.getByRole('heading', { name: 'Decisions' })).toBeVisible();
});

test('closing the tab with unsaved changes asks for confirmation', async ({ page }) => {
  await createDecision(page, 'Closing');
  await page.getByLabel('Weight').first().fill('-1');
  await page.getByLabel('Weight').first().blur();
  await expect(page.getByText('Not saved.')).toBeVisible();
  const prompted = page.waitForEvent('dialog').then((d) => (d.dismiss(), d.type()));
  await page.close({ runBeforeUnload: true });
  expect(await prompted).toBe('beforeunload');
});

test('Enter on a focused link or button does what it says, not "next pair"', async ({ page }) => {
  await createDecision(page, 'Keys');
  await addAll(page, 'New option', ['A', 'B']);
  await addAll(page, 'New criterion', ['One', 'Two']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('button', { name: /One compared with Two/ })).toBeVisible();
  await page.getByRole('link', { name: /Results/ }).focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/results$/);
});

test('nesting criteria with Alt+arrow keys keeps the focus on the criterion', async ({ page }) => {
  await createDecision(page, 'Tree keys');
  await addAll(page, 'New criterion', ['Cost', 'Licence', 'Support']);
  await saved(page);
  const licence = page.getByLabel('Criterion name').nth(1);
  await licence.focus();
  await page.keyboard.press('Alt+ArrowRight'); // into Cost
  await expect(page.getByLabel('Criterion name').nth(1)).toBeFocused();
  await page.keyboard.press('Alt+ArrowLeft'); // back out
  await page.keyboard.press('Alt+ArrowDown'); // below Support
  await expect(page.getByLabel('Criterion name').nth(2)).toBeFocused();
  await expect(page.getByLabel('Criterion name').nth(2)).toHaveValue('Licence');
});

test('the printed report is dark ink on white, with its bars, even in dark mode', async ({ page }) => {
  await createDecision(page, 'Printed');
  await addAll(page, 'New option', ['A', 'B']);
  await addAll(page, 'New criterion', ['Only']);
  await saved(page);
  await page.goto(page.url().replace(/\/setup$/, '/report'));
  await page.emulateMedia({ media: 'print', colorScheme: 'dark' });
  const heading = await page.getByRole('heading', { name: 'Printed' }).evaluate((el) => getComputedStyle(el).color);
  const [r, g, b] = heading.match(/\d+/g)!.map(Number);
  expect(r + g + b, `heading color ${heading}`).toBeLessThan(200);
  const adjust = await page.locator('.bar').first().evaluate((el) => getComputedStyle(el).printColorAdjust);
  expect(adjust).toBe('exact');
});

test('people can be switched on a phone', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createDecision(page, 'Phone judges');
  await addAll(page, 'New person', ['Priya']);
  await saved(page);
  await expect(page.getByLabel('Judging as')).toBeVisible();
});

test('pages load without security-policy errors', async ({ page }) => {
  const violations: string[] = [];
  page.on('console', (m) => m.type() === 'error' && /Content Security Policy/i.test(m.text()) && violations.push(m.text()));
  await createDecision(page, 'Clean console');
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  expect(violations).toEqual([]);
});
```

- [ ] **Step 24: Run everything**

Run: `cd web && npm run check && npx vitest run && npx playwright test`
Expected: 0 errors and 0 warnings, 61 unit tests and 21 end-to-end tests pass.

- [ ] **Step 25: Commit**

```bash
make lint && (cd web && npm run check)
git add web/e2e/exchange.spec.ts web/e2e/fixtures web/e2e/robustness.spec.ts web/src/components/Balance.svelte web/src/components/Library.svelte web/src/components/Report.svelte web/src/components/Results.svelte web/src/components/Sensitivity.svelte web/src/components/Setup.svelte web/src/components/Weigh.svelte web/src/components/Workspace.svelte web/src/components/charts/LineChart.svelte web/src/components/charts/charts.test.ts web/src/lib/api.ts web/src/lib/edits.test.ts web/src/lib/edits.ts web/src/lib/format.test.ts web/src/lib/format.ts web/src/lib/sensitivity.test.ts web/src/lib/sensitivity.ts web/src/lib/session.svelte.ts web/src/lib/session.test.ts
git commit -m "feat(web): examples, templates, spreadsheet import, Excel/CSV downloads, remote judging; polish"
```

---

### Task 9: Releases, CI and README

**Files:**
- Create: `.goreleaser.yaml`, `.github/workflows/release.yml`
- Modify: `.github/workflows/ci.yml`, `Makefile`, `README.md`

**Interfaces:**
- Produces:
-   - pushing a `v*` tag runs GoReleaser: archives for linux, darwin and windows on amd64 and arm64, checksums, a GitHub release, and a Homebrew cask in `Xafloc/homebrew-tap` when `HOMEBREW_TAP_GITHUB_TOKEN` is set
-   - CI gains golangci-lint and moves to v7 actions
-   - `make snapshot`

- [ ] **Step 1: Run the tests to verify they fail**

Run: `go run github.com/goreleaser/goreleaser/v2@v2.18.3 check`
Expected: FAIL — `open .goreleaser.yaml: no such file or directory`.

- [ ] **Step 2: Write `.goreleaser.yaml`** (complete file)

```yaml
# Release builds: `git tag vX.Y.Z && git push --tags` runs .github/workflows/release.yml.
# Try it locally with: go run github.com/goreleaser/goreleaser/v2@v2.18.3 release --snapshot --clean
version: 2
project_name: ahp-matrix

before:
  hooks:
    - make web # the UI is embedded in the binary

builds:
  - main: ./cmd/ahp-matrix
    binary: ahp-matrix
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{ .Version }}

archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE
      - README.md

checksum:
  name_template: checksums.txt

release:
  github:
    owner: Xafloc
    name: ahp-matrix

changelog:
  use: github
  sort: asc
  filters:
    exclude:
      - "^docs"
      - "^test"
      - "^Merge"

# Homebrew: `brew install xafloc/tap/ahp-matrix`. Publishing needs a token with
# write access to github.com/Xafloc/homebrew-tap in HOMEBREW_TAP_GITHUB_TOKEN;
# without it, the release still happens and the cask upload is skipped.
homebrew_casks:
  - name: ahp-matrix
    repository:
      owner: Xafloc
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: https://github.com/Xafloc/ahp-matrix
    description: Make better decisions with the Analytic Hierarchy Process, in a local web app.
    license: MIT
    skip_upload: '{{ if isEnvSet "HOMEBREW_TAP_GITHUB_TOKEN" }}false{{ else }}true{{ end }}'
    hooks:
      post:
        # The binaries aren't notarized; let macOS run the one Homebrew installed.
        install: |
          if OS.mac?
            system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/ahp-matrix"]
          end
```

- [ ] **Step 3: Write `.github/workflows/release.yml`** (complete file)

```yaml
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0 # goreleaser builds the changelog from history
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: actions/setup-node@v7
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: web/package-lock.json
      - uses: goreleaser/goreleaser-action@v7
        with:
          version: v2.18.3
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          # Optional: a token that can push to github.com/Xafloc/homebrew-tap.
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
```

- [ ] **Step 4: Write `.github/workflows/ci.yml`** (complete file)

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: make lint
      - uses: golangci/golangci-lint-action@v9
        with:
          version: v2.14.0
      - run: make test
      - run: make build

  web:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-node@v7
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: make web-check
      - run: make web

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: actions/setup-node@v7
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: cd web && npm ci && npx playwright install --with-deps chromium
      - run: make e2e
      - if: failure()
        uses: actions/upload-artifact@v7
        with:
          name: playwright-traces
          path: web/test-results/
```

- [ ] **Step 5: Write `Makefile`** (complete file)

```make
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: test lint build run web web-check e2e snapshot

test:
	go test -race ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	go vet ./...

# The UI is embedded at build time: run `make web` first (or `make run`).
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/ahp-matrix ./cmd/ahp-matrix

run: web build
	./bin/ahp-matrix

web: web/node_modules
	cd web && npm run build

web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

web-check: web/node_modules
	cd web && npm run check && npm test

e2e: web/node_modules
	cd web && npm run e2e

snapshot:
	go run github.com/goreleaser/goreleaser/v2@v2.18.3 release --snapshot --clean
```

- [ ] **Step 6: Write `README.md`** (complete file)

````markdown
# AHP Matrix

Make better decisions with the [Analytic Hierarchy Process](https://en.wikipedia.org/wiki/Analytic_hierarchy_process):
compare your options two at a time, check that your judgments agree with each other, and see how robust the winner is.
AHP Matrix is a single binary that runs a local web app. Your projects stay on your machine as `.ahp.json` files.

![Comparing two criteria on the balance](docs/images/compare.jpg)

- **Compare in pairs on a balance.** Arrow keys tip it, Enter moves to the next pair, and you get useful results long before every pair is answered.
- **Rate or measure where pairs are overkill.** Each criterion can be compared in pairs, rated 1–10, or given measured values such as price ("lower is better").
- **Group criteria** into a hierarchy, so you only compare criteria that belong together.
- **Catch contradictions.** Every set of comparisons gets a consistency ratio, and the app points out the answers that disagree with the rest, with a one-click fix.
- **Several people.** Everyone judges separately, the results combine them (weighted), and the app shows where people disagree most. Send someone their own copy, then merge it back when they return it.
- **What if?** Drag a criterion's weight and watch the ranking change. Each criterion shows how far it can move before the winner changes.
- **Bring your spreadsheet.** Import workbooks made with the classic "Priority Matrix" Excel template, and export results to Excel or CSV.
- **Start from an example or a template**, including vendor selection with the template's 23 criteria, grouped.

![Results for a laptop decision](docs/images/results.jpg)

## Install

Download the archive for your system from [Releases](https://github.com/Xafloc/ahp-matrix/releases), unpack it, and run `ahp-matrix`.
It starts on http://127.0.0.1:7777 and opens your browser. On macOS, Homebrew works too:

```sh
brew install xafloc/tap/ahp-matrix
```

`ahp-matrix --help` lists the flags (`--port`, `--host`, `--data-dir`, `--no-open`).
Projects are stored in `~/Library/Application Support/ahp-matrix` (macOS),
`%AppData%\ahp-matrix` (Windows) or `~/.local/share/ahp-matrix` (Linux) unless `--data-dir` is given.
Each decision is one `.ahp.json` file; deleted ones move to a `trash` folder there.

### Build from source

Requires Go 1.24+ and Node 20.19+.

```sh
git clone https://github.com/Xafloc/ahp-matrix && cd ahp-matrix
make run    # builds the UI and the binary, then starts it
```

## Development

```sh
make test        # Go tests (race detector)
make lint        # gofmt + go vet
make web-check   # type-check and unit-test the UI
make e2e         # end-to-end browser tests against the real binary
make snapshot    # build release archives for every platform into dist/
```

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` runs GoReleaser.

For live-reloading UI work, run the API in one terminal (`make build && bin/ahp-matrix --no-open`)
and `cd web && npm run dev` in another, then open http://localhost:5173.

## License

MIT
````

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go run github.com/goreleaser/goreleaser/v2@v2.18.3 check`
Expected: PASS

- [ ] **Step 8: Build a snapshot release and try it**

```bash
make snapshot
tar tzf dist/ahp-matrix_*_darwin_arm64.tar.gz      # LICENSE, README.md, ahp-matrix
./dist/ahp-matrix_darwin_arm64*/ahp-matrix version   # 0.0.0-SNAPSHOT-<commit>
```
Start the binary with `--no-open --port 7820 --data-dir "$(mktemp -d)"`. Check that `GET /` serves the app (title "AHP Matrix") and that `GET /api/samples` lists three samples. Then stop it and `rm -rf dist`.

- [ ] **Step 9: Verify everything**

Run: `make lint && make test && make web-check && make web && make build && make e2e`, plus golangci-lint as in Task 7.
Expected: all green.

- [ ] **Step 10: Commit**

```bash
make lint && make test
git add .github/workflows/ci.yml .github/workflows/release.yml .goreleaser.yaml Makefile README.md
git commit -m "build: GoReleaser releases, golangci-lint in CI, install docs"
```

---

## After Plan 3

These publish things on the owner's GitHub account, so the owner decides when:
- Create `github.com/Xafloc/homebrew-tap` and add a token with write access to it as the `HOMEBREW_TAP_GITHUB_TOKEN` secret in `Xafloc/ahp-matrix`.
- Tag the first release (`git tag v0.1.0 && git push origin v0.1.0`), then check the release page and `brew install xafloc/tap/ahp-matrix`.
