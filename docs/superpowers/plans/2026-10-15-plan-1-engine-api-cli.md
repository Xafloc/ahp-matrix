# AHP Matrix — Plan 1: Engine, API and CLI

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A tested Go backend for AHP Matrix: the AHP math, the project document model, JSON file storage, the HTTP API, and the `ahp-matrix` command, which serves a placeholder page until the web UI lands in Plan 2.

**Architecture:** `internal/ahp` is pure math on matrices and vectors. `internal/model` owns the `.ahp.json` document: it validates projects and turns them into `ahp` inputs and results. `internal/store` persists projects behind a `Store` interface (`FileStore` = one JSON file per project). `internal/server` translates HTTP to model and store calls and serves the embedded UI from `web/`. `cmd/ahp-matrix` wires everything together.

**Tech Stack:** Go 1.24+ standard library (`net/http` routing patterns, `embed`, `testing`), `github.com/oklog/ulid/v2`. No CGO.

**Spec:** `docs/superpowers/specs/2026-10-15-ahp-matrix-design.md`

**Roadmap:** this is Plan 1 of 3.
- **Plan 2, Web UI:** the Svelte app, every screen including sensitivity, TypeScript–Go parity fixtures, and Playwright.
- **Plan 3, Import/export and distribution:** legacy `.xlsx` import, `.xlsx`/`.csv` export, evaluator packets and merging, samples and templates, goreleaser, the Homebrew tap, and golangci-lint.

All code in this plan was compiled and its tests run (`go vet`, `go test -race`) before the plan was written, so a step that says "Expected: PASS" should pass exactly as written.

## Global Constraints

- Module path `github.com/Xafloc/ahp-matrix`; binary `ahp-matrix`; MIT license.
- `go 1.24` in `go.mod`. Builds must work with `CGO_ENABLED=0`. The only third-party dependency in this plan is `github.com/oklog/ulid/v2` (v2.1.2).
- Every change leaves `make lint` (gofmt + `go vet`) and `make test` (`go test -race ./...`) passing.
- Listen on `127.0.0.1:7777` by default; fall back to a free port if it is busy. Default data folders: `~/Library/Application Support/ahp-matrix` (macOS), `%AppData%\ahp-matrix` (Windows), `$XDG_DATA_HOME/ahp-matrix` or `~/.local/share/ahp-matrix` (others).
- Request bodies and uploads are capped at 20 MB.
- Project files are named `<slug>-<last 6 chars of id, lowercased>.ahp.json`, use `schemaVersion` 1, and are written atomically with the previous version kept as `.ahp.json.bak`. Deleted projects move to `trash/`.
- Consistency: Saaty's RI table for n ≤ 15, Alonso & Lamata `RI = (1.7699n − 4.3513)/(n − 1)` above. CR thresholds are 0.05 (n = 3), 0.08 (n = 4) and 0.10 otherwise.
- API errors are always `{"error": {"code", "message", "details"}}`.
- Comments explain *why* or describe contracts; match the density of the code shown here.

## Spec refinements made while planning

These are small, deliberate departures from the spec's wording. Each one is covered by tests.
- **`Store.List` returns `[]store.Entry`** (project, file name, read error) rather than summaries. The server builds the library-card summaries, so the store stays a pure persistence layer.
- **Judgments survive structural edits.** Judgments of alternatives (pairwise, ratings, values) are kept for any criterion that still exists, not only leaves, so nesting a criterion and un-nesting it loses nothing. Only judgments that reference deleted items are dropped ("cascade").
- **The browser is opened with the OS command** (`open`, `xdg-open`, `rundll32`) instead of `github.com/pkg/browser`, whose indirect `golang.org/x/sys` pin would otherwise force contributors onto the newest Go.
- **CI lint is gofmt + `go vet` for now.** golangci-lint arrives with the release tooling in Plan 3.
- **Missing ratings or values get the average** of the alternatives that have data (flagged with an issue), and unresolvable groups get equal weights (flagged). This keeps results available while a project is still being filled in.

## Review Focus

Inputs that the spec implies but doesn't spell out, most likely to bite first. Each has a test in the task that owns the code.
1. **A project file copied by hand** in Finder or Explorer (same id, two files). Expected: the two must not silently edit each other. The copy is flagged in the library and the original keeps working. *(Task 9: `TestHandCopiedFileIsFlaggedNotAliased`)*
2. **A leaf criterion that gains children and later loses them.** Expected: its alternatives' judgments come back. *(Task 8: `TestPrepareKeepsJudgmentsWhenALeafGainsChildren`)*
3. **Legacy-sized matrices** (25 criteria, past the n > 15 RI branch). Expected: positive weights summing to 1 and a finite CR. *(Task 2: `TestAnalyzeWeightsAlwaysSumToOne`, which goes up to n = 30)*
4. **Project names with no Latin letters** (e.g. 評価). Expected: distinct, readable files rather than collisions or errors. *(Task 9: `TestNonLatinNamesGetDistinctFiles`)*
5. **A project file deleted on disk while it is open in the browser.** Expected: autosave gets a clean 404 that the UI can act on, not a 500. *(Task 11: `TestSaveAfterFileDeletedElsewhere`)*

## File Structure

```
go.mod, go.sum, Makefile, .gitignore, README.md, .github/workflows/ci.yml
cmd/ahp-matrix/main.go        flags, data dir, listener, startup banner, graceful shutdown
cmd/ahp-matrix/browser.go     open the default browser
internal/ahp/matrix.go        Matrix type, Snap
internal/ahp/analyze.go       eigenvector (power iteration), Harker, connectivity, RI, CR
internal/ahp/suggest.go       fix suggestions for contradictory judgments
internal/ahp/aggregate.go     GeoMean, ArithMean, GeoStdDev (multi-evaluator)
internal/ahp/synth.go         Tree, Synthesize (hierarchy → global weights, scores)
internal/ahp/sensitivity.go   Line, SensitivityLine, Crossovers, Stability
internal/model/project.go     document types, New
internal/model/scale.go       Scale, contexts, IDs, Slug
internal/model/decode.go      Decode + schema version gate
internal/model/prepare.go     defaults, pruning, validation, snapping
internal/model/compute.go     Compute → Results
internal/store/store.go       Store interface, errors, Entry
internal/store/file.go        FileStore
internal/server/server.go     Config, New, routes, JSON helpers
internal/server/guard.go      Host/Origin/Content-Type checks, body cap
internal/server/static.go     embedded UI + SPA fallback (+ placeholder.html)
internal/server/projects.go   project endpoints
web/embed.go, web/dist/.gitkeep   embedded UI (filled by Plan 2)
```

---

### Task 1: Scaffolding and the comparison matrix

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `README.md`
- Create: `.github/workflows/ci.yml`
- Create: `internal/ahp/matrix.go`
- Test: `internal/ahp/matrix_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `type ahp.Matrix [][]float64` (0 = unjudged); `ahp.NewMatrix(n int) Matrix`; `(Matrix).Set(i, j int, v float64)` (fills the reciprocal); `(Matrix).N() int`; `(Matrix).Judged(i, j int) bool`; `(Matrix).Missing() int`; `ahp.Snap(v float64, allowed []float64) float64` (nearest on a log scale). Test helpers in package `ahp` that later tasks' tests reuse: `saaty9`, `approx`, `approxSlice`, `consistent(w []float64) Matrix`.

- [ ] **Step 1: Create the module and project scaffolding**

From the repo root (it already contains `LICENSE` and `docs/`):

```bash
mkdir -p internal/ahp .github/workflows
printf 'module github.com/Xafloc/ahp-matrix\n\ngo 1.24\n' > go.mod
```

Create `.gitignore`:

```gitignore
/bin/
/dist/
web/node_modules/
web/dist/*
!web/dist/.gitkeep
coverage.out
.DS_Store
```

Create `Makefile` (recipe lines are indented with a **tab**):

```make
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: test lint build run

test:
	go test -race ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	go vet ./...

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/ahp-matrix ./cmd/ahp-matrix

run: build
	./bin/ahp-matrix
```

Create `.github/workflows/ci.yml`:

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
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make lint
      - run: make test
      - run: make build
```

Create `README.md`:

````markdown
# AHP Matrix

Make better decisions with the [Analytic Hierarchy Process](https://en.wikipedia.org/wiki/Analytic_hierarchy_process):
compare options two at a time, check that your judgments are consistent, and see how robust the winner is.
AHP Matrix is a single binary that runs a local web app; your projects stay on your machine as `.ahp.json` files.

> **Status:** early development. The engine and API work; the web UI is coming next.

## Development

Requires Go 1.24 or newer.

```sh
make test   # run all tests with the race detector
make lint   # gofmt + go vet
make run    # build and start on http://127.0.0.1:7777
```

`ahp-matrix --help` lists the flags (`--port`, `--host`, `--data-dir`, `--no-open`).
Projects are stored in `~/Library/Application Support/ahp-matrix` (macOS),
`%AppData%\ahp-matrix` (Windows) or `~/.local/share/ahp-matrix` (Linux) unless `--data-dir` is given.

## License

MIT
````

- [ ] **Step 2: Write the failing test `internal/ahp/matrix_test.go`**

```go
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
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/ahp/`
Expected: FAIL — build fails with `undefined: NewMatrix` and `undefined: Snap`.

- [ ] **Step 4: Write `internal/ahp/matrix.go`**

```go
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/ahp/`
Expected: PASS (`ok`)

- [ ] **Step 6: Commit**

```bash
make lint
git add .github/workflows/ci.yml .gitignore Makefile README.md go.mod internal/ahp/matrix.go internal/ahp/matrix_test.go
git commit -m "feat(ahp): add comparison matrix and scale snapping"
```

---

### Task 2: Priority vector, consistency and incomplete matrices

**Files:**
- Create: `internal/ahp/analyze.go`
- Test: `internal/ahp/analyze_test.go`

**Interfaces:**
- Consumes: `Matrix`, `NewMatrix`, `Set`, `Judged`, `Missing` (Task 1); test helpers `approx`, `approxSlice`, `consistent`, `saaty9`.
- Produces: `ahp.Analyze(m Matrix) Analysis` where `type Analysis struct { Weights []float64; LambdaMax, CI, CR, Threshold float64; Consistent bool; Missing int; Complete, Connected bool; Components [][]int }`. `Weights` is nil when the comparisons don't connect every item. Also `ahp.RandomIndex(n int) float64` and `ahp.CRThreshold(n int) float64`.

- [ ] **Step 1: Write the failing test `internal/ahp/analyze_test.go`**

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ahp/`
Expected: FAIL — build fails with `undefined: Analyze` and `undefined: RandomIndex`.

- [ ] **Step 3: Write `internal/ahp/analyze.go`**

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ahp/`
Expected: PASS (`ok`)

- [ ] **Step 5: Commit**

```bash
make lint
git add internal/ahp/analyze.go internal/ahp/analyze_test.go
git commit -m "feat(ahp): eigenvector priorities, consistency ratio, Harker incomplete matrices"
```

---

### Task 3: Fix suggestions and multi-evaluator aggregation

**Files:**
- Create: `internal/ahp/suggest.go`
- Create: `internal/ahp/aggregate.go`
- Test: `internal/ahp/suggest_test.go`

**Interfaces:**
- Consumes: `Matrix`, `Snap` (Task 1); `Analyze` (Task 2).
- Produces: `ahp.Suggestions(m Matrix, w []float64, allowed []float64, k int) []Suggestion` where `type Suggestion struct { I, J int; Current, Suggested, Deviation float64 }`, most deviant first; `type ahp.Weighted struct{ Value, Weight float64 }`; `ahp.GeoMean([]Weighted) float64`, `ahp.ArithMean([]Weighted) float64`, `ahp.GeoStdDev([]Weighted) float64`.

- [ ] **Step 1: Write the failing test `internal/ahp/suggest_test.go`**

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ahp/`
Expected: FAIL — build fails with `undefined: Suggestions` and `undefined: GeoMean`.

- [ ] **Step 3: Write `internal/ahp/suggest.go`**

```go
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
// the weights w, most deviant first. Pairs with negligible deviation are left out.
func Suggestions(m Matrix, w []float64, allowed []float64, k int) []Suggestion {
	var out []Suggestion
	for i := 0; i < m.N(); i++ {
		for j := i + 1; j < m.N(); j++ {
			if !m.Judged(i, j) {
				continue
			}
			dev := math.Abs(math.Log(m[i][j] * w[j] / w[i]))
			if dev < 1e-9 {
				continue
			}
			out = append(out, Suggestion{I: i, J: j, Current: m[i][j], Suggested: Snap(w[i]/w[j], allowed), Deviation: dev})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Deviation > out[b].Deviation })
	if len(out) > k {
		out = out[:k]
	}
	return out
}
```

- [ ] **Step 4: Write `internal/ahp/aggregate.go`**

```go
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/ahp/`
Expected: PASS (`ok`)

- [ ] **Step 6: Commit**

```bash
make lint
git add internal/ahp/aggregate.go internal/ahp/suggest.go internal/ahp/suggest_test.go
git commit -m "feat(ahp): suggest fixes for inconsistent judgments; aggregate evaluators"
```

---

### Task 4: Hierarchy synthesis

**Files:**
- Create: `internal/ahp/synth.go`
- Test: `internal/ahp/synth_test.go`

**Interfaces:**
- Consumes: test helpers `approxSlice` (Task 1).
- Produces: `type ahp.Tree struct { Parent []int; Local []float64; Leaf [][]float64 }` (parents before children, -1 = top level, `Leaf[i]` nil for internal nodes); `type ahp.Synthesis struct { Global []float64; Subtree [][]float64; Scores []float64 }`; `ahp.Synthesize(t Tree, nAlt int) (Synthesis, error)`. Test helper `sampleTree()`, used by Task 5's tests.

- [ ] **Step 1: Write the failing test `internal/ahp/synth_test.go`**

```go
package ahp

import "testing"

// sampleTree is Technical (0.6) with children Performance and Security
// (0.5 each), plus Cost (0.4), scoring two alternatives.
func sampleTree() Tree {
	return Tree{
		Parent: []int{-1, -1, 0, 0},
		Local:  []float64{0.6, 0.4, 0.5, 0.5},
		Leaf:   [][]float64{nil, {0.2, 0.8}, {0.7, 0.3}, {0.4, 0.6}},
	}
}

func TestSynthesize(t *testing.T) {
	s, err := Synthesize(sampleTree(), 2)
	if err != nil {
		t.Fatal(err)
	}
	approxSlice(t, "Global", s.Global, []float64{0.6, 0.4, 0.3, 0.3}, 1e-12)
	approxSlice(t, "Subtree[0]", s.Subtree[0], []float64{0.55, 0.45}, 1e-12)
	approxSlice(t, "Subtree[1]", s.Subtree[1], []float64{0.2, 0.8}, 1e-12)
	approxSlice(t, "Scores", s.Scores, []float64{0.41, 0.59}, 1e-12)
}

func TestSynthesizeRejectsBadTrees(t *testing.T) {
	bad := Tree{Parent: []int{1, -1}, Local: []float64{1, 1}, Leaf: [][]float64{{1}, nil}}
	if _, err := Synthesize(bad, 1); err == nil {
		t.Error("child before parent: want error")
	}
	short := sampleTree()
	short.Leaf[1] = []float64{1}
	if _, err := Synthesize(short, 2); err == nil {
		t.Error("leaf with wrong length: want error")
	}
	if _, err := Synthesize(Tree{Parent: []int{-1}}, 1); err == nil {
		t.Error("mismatched slices: want error")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ahp/`
Expected: FAIL — build fails with `undefined: Tree` and `undefined: Synthesize`.

- [ ] **Step 3: Write `internal/ahp/synth.go`**

```go
package ahp

import (
	"errors"
	"fmt"
)

// Tree is a criteria hierarchy flattened into parallel slices, ordered so that
// every parent comes before its children.
type Tree struct {
	Parent []int       // parent index, or -1 for a top-level criterion
	Local  []float64   // weight among siblings; each sibling group sums to 1
	Leaf   [][]float64 // per node: alternatives' local priorities for a leaf, nil otherwise
}

// Synthesis is the result of combining a Tree.
type Synthesis struct {
	Global  []float64   // per node: product of local weights from the top down
	Subtree [][]float64 // per node: alternatives' scores within that subtree; each sums to 1
	Scores  []float64   // per alternative: overall score; sums to 1
}

// Synthesize combines local weights and leaf priorities for nAlt alternatives.
func Synthesize(t Tree, nAlt int) (Synthesis, error) {
	n := len(t.Parent)
	if len(t.Local) != n || len(t.Leaf) != n {
		return Synthesis{}, errors.New("ahp: tree slices differ in length")
	}
	s := Synthesis{Global: make([]float64, n), Subtree: make([][]float64, n), Scores: make([]float64, nAlt)}
	for i := 0; i < n; i++ {
		p := t.Parent[i]
		switch {
		case p >= i:
			return Synthesis{}, fmt.Errorf("ahp: node %d has parent %d; parents must come first", i, p)
		case p < 0:
			s.Global[i] = t.Local[i]
		default:
			s.Global[i] = s.Global[p] * t.Local[i]
		}
		s.Subtree[i] = make([]float64, nAlt)
	}
	// Bottom-up: children come after parents, so walking backwards finishes
	// every subtree before it is added to its parent.
	for i := n - 1; i >= 0; i-- {
		if t.Leaf[i] != nil {
			if len(t.Leaf[i]) != nAlt {
				return Synthesis{}, fmt.Errorf("ahp: leaf %d has %d priorities, want %d", i, len(t.Leaf[i]), nAlt)
			}
			copy(s.Subtree[i], t.Leaf[i])
		}
		target := s.Scores
		if p := t.Parent[i]; p >= 0 {
			target = s.Subtree[p]
		}
		for a := 0; a < nAlt; a++ {
			target[a] += t.Local[i] * s.Subtree[i][a]
		}
	}
	return s, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ahp/`
Expected: PASS (`ok`)

- [ ] **Step 5: Commit**

```bash
make lint
git add internal/ahp/synth.go internal/ahp/synth_test.go
git commit -m "feat(ahp): synthesize hierarchical criteria into global weights and scores"
```

---

### Task 5: Sensitivity analysis

**Files:**
- Create: `internal/ahp/sensitivity.go`
- Test: `internal/ahp/sensitivity_test.go`

**Interfaces:**
- Consumes: `Tree`, `Synthesis`, `Synthesize`, test helper `sampleTree` (Task 4).
- Produces: `type ahp.Line struct{ Intercept, Slope []float64 }` with `(Line).At(x float64) []float64`, `(Line).Crossovers() []Crossover` (`type Crossover struct { X float64; A, B int }`) and `(Line).Stability(w float64) (delta float64, ok bool)`; `ahp.SensitivityLine(t Tree, s Synthesis, node int) (Line, bool)`, where ok is false for an only child. Plan 2 mirrors `SensitivityLine` and `At` in TypeScript.

- [ ] **Step 1: Write the failing test `internal/ahp/sensitivity_test.go`**

```go
package ahp

import (
	"math"
	"testing"
)

// withWeight returns a copy of t where node's local weight is x and its
// siblings share 1−x in proportion to their current weights.
func withWeight(t Tree, node int, x float64) Tree {
	out := Tree{Parent: t.Parent, Leaf: t.Leaf, Local: append([]float64(nil), t.Local...)}
	w := t.Local[node]
	for i := range t.Parent {
		if i != node && t.Parent[i] == t.Parent[node] {
			out.Local[i] = t.Local[i] * (1 - x) / (1 - w)
		}
	}
	out.Local[node] = x
	return out
}

func TestSensitivityLineMatchesResynthesis(t *testing.T) {
	tree := sampleTree()
	s, _ := Synthesize(tree, 2)
	for node := range tree.Parent {
		l, ok := SensitivityLine(tree, s, node)
		if !ok {
			t.Fatalf("node %d: ok = false, want true", node)
		}
		for x := 0.0; x <= 1.0; x += 0.05 {
			want, err := Synthesize(withWeight(tree, node, x), 2)
			if err != nil {
				t.Fatal(err)
			}
			approxSlice(t, "score", l.At(x), want.Scores, 1e-12)
		}
	}
}

func TestSensitivityLineZeroWeightSiblings(t *testing.T) {
	tree := Tree{Parent: []int{-1, -1}, Local: []float64{1, 0}, Leaf: [][]float64{{0.9, 0.1}, {0.2, 0.8}}}
	s, _ := Synthesize(tree, 2)
	l, _ := SensitivityLine(tree, s, 0)
	approxSlice(t, "score at 0.25", l.At(0.25), []float64{0.375, 0.625}, 1e-12)
}

func TestSensitivityLineOnlyChild(t *testing.T) {
	tree := Tree{Parent: []int{-1}, Local: []float64{1}, Leaf: [][]float64{{0.5, 0.5}}}
	s, _ := Synthesize(tree, 2)
	if _, ok := SensitivityLine(tree, s, 0); ok {
		t.Error("only child: ok = true, want false")
	}
}

func TestCrossoversAndStability(t *testing.T) {
	// Scores are 0.2+0.7x and 0.8−0.7x, which meet at x = 3/7.
	tree := Tree{Parent: []int{-1, -1}, Local: []float64{0.5, 0.5}, Leaf: [][]float64{{0.9, 0.1}, {0.2, 0.8}}}
	s, _ := Synthesize(tree, 2)
	l, _ := SensitivityLine(tree, s, 0)
	cs := l.Crossovers()
	if len(cs) != 1 || cs[0].A != 0 || cs[0].B != 1 {
		t.Fatalf("Crossovers = %+v, want one crossing of 0 and 1", cs)
	}
	approx(t, "crossover", cs[0].X, 3.0/7, 1e-12)
	d, ok := l.Stability(0.5)
	if !ok {
		t.Fatal("Stability ok = false, want true")
	}
	approx(t, "stability", d, 0.5-3.0/7, 1e-12)
}

func TestStabilityMatchesBruteForce(t *testing.T) {
	tree := sampleTree()
	s, _ := Synthesize(tree, 2)
	for node := range tree.Parent {
		l, _ := SensitivityLine(tree, s, node)
		w := tree.Local[node]
		leader := argmax(l.At(w))
		want := math.Inf(1)
		for x := 0.0; x <= 1.0; x += 1e-5 {
			if argmax(l.At(x)) != leader {
				want = math.Min(want, math.Abs(x-w))
			}
		}
		got, ok := l.Stability(w)
		if math.IsInf(want, 1) {
			if ok {
				t.Errorf("node %d: Stability = %v, want none", node, got)
			}
			continue
		}
		approx(t, "stability", got, want, 2e-5)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ahp/`
Expected: FAIL — build fails with `undefined: SensitivityLine` and `undefined: Line`.

- [ ] **Step 3: Write `internal/ahp/sensitivity.go`**

```go
package ahp

import (
	"math"
	"sort"
)

// Line gives each alternative's overall score as a linear function of one
// criterion's local weight x: score[a](x) = Intercept[a] + Slope[a]·x.
type Line struct{ Intercept, Slope []float64 }

// At returns every alternative's score at weight x.
func (l Line) At(x float64) []float64 {
	out := make([]float64, len(l.Slope))
	for a := range out {
		out[a] = l.Intercept[a] + l.Slope[a]*x
	}
	return out
}

// SensitivityLine describes how overall scores change when node's local
// weight is set to x and its siblings are rescaled proportionally to share
// 1−x. If the siblings currently have no weight at all, they share 1−x
// equally. ok is false when node has no siblings, so its weight cannot change.
func SensitivityLine(t Tree, s Synthesis, node int) (l Line, ok bool) {
	p := t.Parent[node]
	gp := 1.0
	if p >= 0 {
		gp = s.Global[p]
	}
	var sibs []int
	for i := range t.Parent {
		if i != node && t.Parent[i] == p {
			sibs = append(sibs, i)
		}
	}
	if len(sibs) == 0 {
		return Line{}, false
	}
	nAlt := len(s.Scores)
	w := t.Local[node]
	l = Line{Intercept: make([]float64, nAlt), Slope: make([]float64, nAlt)}
	for a := 0; a < nAlt; a++ {
		others := 0.0
		for _, sb := range sibs {
			others += t.Local[sb] * s.Subtree[sb][a]
		}
		group := w*s.Subtree[node][a] + others // the sibling group's current share, within the parent
		var q float64                          // the siblings' score per unit of weight they share
		if 1-w > 1e-12 {
			q = others / (1 - w)
		} else {
			for _, sb := range sibs {
				q += s.Subtree[sb][a]
			}
			q /= float64(len(sibs))
		}
		rest := s.Scores[a] - gp*group
		l.Intercept[a] = rest + gp*q
		l.Slope[a] = gp * (s.Subtree[node][a] - q)
	}
	return l, true
}

// Crossover is a weight at which alternatives A and B score equally.
type Crossover struct {
	X    float64
	A, B int
}

// Crossovers returns every weight in [0, 1] where two alternatives' scores
// cross, sorted by weight.
func (l Line) Crossovers() []Crossover {
	var out []Crossover
	for a := range l.Slope {
		for b := a + 1; b < len(l.Slope); b++ {
			if x, ok := l.meet(a, b); ok {
				out = append(out, Crossover{X: x, A: a, B: b})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].X < out[j].X })
	return out
}

// Stability returns the smallest change from weight w at which another
// alternative overtakes the leader at w. ok is false when the leader never
// changes anywhere in [0, 1].
func (l Line) Stability(w float64) (delta float64, ok bool) {
	top := argmax(l.At(w))
	delta = math.Inf(1)
	for b := range l.Slope {
		if b == top {
			continue
		}
		if x, met := l.meet(top, b); met && math.Abs(x-w) < delta {
			delta, ok = math.Abs(x-w), true
		}
	}
	if !ok {
		return 0, false
	}
	return delta, true
}

// meet returns the weight in [0, 1] where alternatives a and b score equally.
func (l Line) meet(a, b int) (float64, bool) {
	ds := l.Slope[a] - l.Slope[b]
	if math.Abs(ds) < 1e-15 {
		return 0, false
	}
	x := (l.Intercept[b] - l.Intercept[a]) / ds
	return x, x >= 0 && x <= 1
}

func argmax(v []float64) int {
	best := 0
	for i := range v {
		if v[i] > v[best] {
			best = i
		}
	}
	return best
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ahp/`
Expected: PASS (`ok`)

- [ ] **Step 5: Commit**

```bash
make lint
git add internal/ahp/sensitivity.go internal/ahp/sensitivity_test.go
git commit -m "feat(ahp): linear sensitivity lines, crossovers and winner stability"
```

---

### Task 6: Project document types, scales, IDs and decoding

**Files:**
- Create: `internal/model/project.go`
- Create: `internal/model/scale.go`
- Create: `internal/model/decode.go`
- Test: `internal/model/model_test.go`
- Modify: `go.mod`, `go.sum` (add ulid)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: types `Project`, `Alternative`, `Criterion`, `Evaluator`, `PairwiseJudgment`, `Rating`, `Value` (JSON field names exactly as in spec §4); `Method` (`MethodPairwise`/`MethodRating`/`MethodValue`), `Direction` (`DirHigher`/`DirLower`), `SchemaVersion = 1`; `model.New(name string) *Project` (one evaluator named "Me"); `type Scale string` with `ScaleSaaty9`, `ScaleClassic5`, `(Scale).Valid() bool`, `(Scale).Allowed() []float64`; `model.CriteriaContext(parentID string) string` ("criteria:root" for ""), `model.AltsContext(criterionID string) string`, `model.ParseContext(c string) (kind, id string, ok bool)`; `model.NewProjectID() string` (ULID), `model.NewID() string` (8 chars), `model.Slug(name string) string`; `model.Decode(data []byte) (*Project, error)` with `ErrNotProject` and `*FutureVersionError{Version int}`.

- [ ] **Step 1: Add the ULID dependency**

```bash
mkdir -p internal/model
go get github.com/oklog/ulid/v2@v2.1.2
```

- [ ] **Step 2: Write the failing test `internal/model/model_test.go`**

```go
package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestScaleAllowed(t *testing.T) {
	s := ScaleSaaty9.Allowed()
	if len(s) != 17 || s[0] != 1.0/9 || s[8] != 1 || s[16] != 9 {
		t.Errorf("saaty9 = %v", s)
	}
	if c := ScaleClassic5.Allowed(); len(c) != 5 || c[4] != 10 {
		t.Errorf("classic5 = %v", c)
	}
	if Scale("bogus").Valid() || !ScaleClassic5.Valid() {
		t.Error("Valid is wrong")
	}
}

func TestContexts(t *testing.T) {
	cases := []struct {
		in       string
		kind, id string
		ok       bool
	}{
		{CriteriaContext(""), "criteria", "", true},
		{CriteriaContext("c1"), "criteria", "c1", true},
		{AltsContext("c2"), "alts", "c2", true},
		{"alts:", "", "", false},
		{"other:x", "", "", false},
		{"nocolon", "", "", false},
	}
	for _, c := range cases {
		kind, id, ok := ParseContext(c.in)
		if kind != c.kind || id != c.id || ok != c.ok {
			t.Errorf("ParseContext(%q) = %q, %q, %v; want %q, %q, %v", c.in, kind, id, ok, c.kind, c.id, c.ok)
		}
	}
}

func TestIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewID()
		if len(id) != 8 || seen[id] {
			t.Fatalf("NewID() = %q (len %d, duplicate %v)", id, len(id), seen[id])
		}
		seen[id] = true
	}
	if len(NewProjectID()) != 26 {
		t.Error("NewProjectID is not a ULID")
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Pick an IAM vendor!": "pick-an-iam-vendor",
		"  ---  ":             "project",
		"Café & Co. 2026":     "caf-co-2026",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecode(t *testing.T) {
	p := New("Test")
	data, _ := json.Marshal(p)
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID || got.Name != "Test" || len(got.Evaluators) != 1 {
		t.Errorf("round trip lost data: %+v", got)
	}

	if _, err := Decode([]byte(`{"name":"x"}`)); !errors.Is(err, ErrNotProject) {
		t.Errorf("missing schemaVersion: err = %v, want ErrNotProject", err)
	}
	if _, err := Decode([]byte(`not json`)); !errors.Is(err, ErrNotProject) {
		t.Errorf("garbage: err = %v, want ErrNotProject", err)
	}
	var fv *FutureVersionError
	if _, err := Decode([]byte(`{"schemaVersion":99}`)); !errors.As(err, &fv) || fv.Version != 99 {
		t.Errorf("future version: err = %v, want *FutureVersionError{99}", err)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/model/`
Expected: FAIL — build fails with `undefined: New`, `undefined: ScaleSaaty9` and others.

- [ ] **Step 4: Write `internal/model/project.go`**

```go
// Package model defines the project document stored in .ahp.json files,
// keeps it valid, and turns it into AHP results.
package model

import "time"

// SchemaVersion is the newest project file version this build reads and writes.
const SchemaVersion = 1

// Method is how a leaf criterion scores the alternatives.
type Method string

const (
	MethodPairwise Method = "pairwise" // compare alternatives two at a time
	MethodRating   Method = "rating"   // rate each alternative 1–10
	MethodValue    Method = "value"    // enter a measured value per alternative
)

// Direction says whether larger measured values are better.
type Direction string

const (
	DirHigher Direction = "higher"
	DirLower  Direction = "lower"
)

// Project is one decision: alternatives scored against weighted criteria.
type Project struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Version       int                `json:"version"` // incremented on every save
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
	Scale         Scale              `json:"scale"`
	Alternatives  []Alternative      `json:"alternatives"`
	Criteria      []Criterion        `json:"criteria"`
	Evaluators    []Evaluator        `json:"evaluators"`
	Pairwise      []PairwiseJudgment `json:"pairwise"`
	Ratings       []Rating           `json:"ratings"`
	Values        []Value            `json:"values"`
}

// Alternative is one option being chosen between.
type Alternative struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// Criterion is a node in the criteria tree. Only leaves (criteria without
// children) have a Method and score alternatives.
type Criterion struct {
	ID          string    `json:"id"`
	ParentID    string    `json:"parentId,omitempty"` // empty for a top-level criterion
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Notes       string    `json:"notes"`
	Method      Method    `json:"method,omitempty"`
	Direction   Direction `json:"direction,omitempty"` // MethodValue only
	Unit        string    `json:"unit,omitempty"`      // MethodValue only, for display
}

// Evaluator is a person whose judgments count, with a relative weight.
type Evaluator struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// PairwiseJudgment says how strongly A is preferred over B within Context
// (see CriteriaContext and AltsContext). Each unordered pair is stored once
// per evaluator and context; the reciprocal is implied.
type PairwiseJudgment struct {
	EvaluatorID string  `json:"evaluatorId"`
	Context     string  `json:"context"`
	A           string  `json:"a"`
	B           string  `json:"b"`
	Value       float64 `json:"value"`
	Note        string  `json:"note,omitempty"`
}

// Rating is one evaluator's 1–10 score for an alternative on a leaf criterion.
type Rating struct {
	EvaluatorID   string  `json:"evaluatorId"`
	CriterionID   string  `json:"criterionId"`
	AlternativeID string  `json:"alternativeId"`
	Value         float64 `json:"value"`
}

// Value is an objective measurement of an alternative on a leaf criterion.
type Value struct {
	CriterionID   string  `json:"criterionId"`
	AlternativeID string  `json:"alternativeId"`
	Value         float64 `json:"value"`
}

// New returns an empty project with a single evaluator, ready to save.
func New(name string) *Project {
	now := time.Now().UTC()
	return &Project{
		SchemaVersion: SchemaVersion,
		ID:            NewProjectID(),
		Name:          name,
		CreatedAt:     now,
		UpdatedAt:     now,
		Scale:         ScaleSaaty9,
		Alternatives:  []Alternative{},
		Criteria:      []Criterion{},
		Evaluators:    []Evaluator{{ID: NewID(), Name: "Me", Weight: 1}},
		Pairwise:      []PairwiseJudgment{},
		Ratings:       []Rating{},
		Values:        []Value{},
	}
}
```

- [ ] **Step 5: Write `internal/model/scale.go`**

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

- [ ] **Step 6: Write `internal/model/decode.go`**

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
		return nil, fmt.Errorf("%w: %v", ErrNotProject, err)
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
		return nil, fmt.Errorf("%w: %v", ErrNotProject, err)
	}
	return &p, nil
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/model/`
Expected: PASS (`ok`)

- [ ] **Step 8: Commit**

```bash
make lint
git add go.mod go.sum internal/model/decode.go internal/model/model_test.go internal/model/project.go internal/model/scale.go
git commit -m "feat(model): project document, scales, contexts, ids and decoding"
```

---

### Task 7: Defaults, pruning, validation and scale snapping

**Files:**
- Create: `internal/model/prepare.go`
- Test: `internal/model/fixture_test.go`
- Test: `internal/model/prepare_test.go`

**Interfaces:**
- Consumes: everything from Task 6; `ahp.Snap` (Task 1).
- Produces: `(*Project).Prepare() error`, which fills defaults, prunes orphaned or duplicate judgments, validates, then snaps pairwise values to the scale; `(*Project).Validate() error`; `type ValidationError struct{ Problems []Problem }`, `type Problem struct{ Field, Message string }` (JSON `field`, `message`); the unexported `(*Project).childrenOf() map[string][]Criterion`, used by Task 8. Test helper `fixture() *Project` (shown in Step 1), used by Task 8's tests.

- [ ] **Step 1: Write the failing test `internal/model/fixture_test.go`**

```go
package model

// fixture returns a small valid project:
//
//	Technical (c1) ─┬─ Performance (c2, pairwise)
//	                └─ Security    (c3, rating)
//	Cost (c4, value, lower is better)
//
// with alternatives a1–a3 and evaluators e1 and e2. Only e1 has judged.
// e1's judgments are fully consistent, so the expected results are exact:
// root weights 0.75/0.25, Technical's children 0.5/0.5, and Performance,
// Security and Cost priorities of [4,2,1]/7, [0.25,0.25,0.5] and [4,2,1]/7.
func fixture() *Project {
	p := New("Fixture")
	p.Evaluators = []Evaluator{{ID: "e1", Name: "Ann", Weight: 1}, {ID: "e2", Name: "Bob", Weight: 1}}
	p.Alternatives = []Alternative{{ID: "a1", Name: "Alpha"}, {ID: "a2", Name: "Beta"}, {ID: "a3", Name: "Gamma"}}
	p.Criteria = []Criterion{
		{ID: "c1", Name: "Technical"},
		{ID: "c2", ParentID: "c1", Name: "Performance", Method: MethodPairwise},
		{ID: "c3", ParentID: "c1", Name: "Security", Method: MethodRating},
		{ID: "c4", Name: "Cost", Method: MethodValue, Direction: DirLower, Unit: "USD"},
	}
	p.Pairwise = []PairwiseJudgment{
		{EvaluatorID: "e1", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 3},
		{EvaluatorID: "e1", Context: CriteriaContext("c1"), A: "c2", B: "c3", Value: 1},
		{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a1", B: "a2", Value: 2},
		{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a1", B: "a3", Value: 4},
		{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a2", B: "a3", Value: 2},
	}
	p.Ratings = []Rating{
		{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "a1", Value: 5},
		{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "a2", Value: 5},
		{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "a3", Value: 10},
	}
	p.Values = []Value{
		{CriterionID: "c4", AlternativeID: "a1", Value: 100},
		{CriterionID: "c4", AlternativeID: "a2", Value: 200},
		{CriterionID: "c4", AlternativeID: "a3", Value: 400},
	}
	return p
}
```

- [ ] **Step 2: Write the failing test `internal/model/prepare_test.go`**

```go
package model

import (
	"errors"
	"math"
	"testing"
)

func TestPrepareAcceptsFixture(t *testing.T) {
	p := fixture()
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if len(p.Pairwise) != 5 || len(p.Ratings) != 3 || len(p.Values) != 3 {
		t.Errorf("Prepare dropped valid judgments: %d pairwise, %d ratings, %d values", len(p.Pairwise), len(p.Ratings), len(p.Values))
	}
}

func TestPrepareFillsDefaults(t *testing.T) {
	p := &Project{
		Name:         "  Spaced  ",
		Evaluators:   []Evaluator{{Name: "Me"}},
		Alternatives: []Alternative{{Name: "A"}},
		Criteria:     []Criterion{{ID: "c1", Name: "Leaf"}, {ID: "c2", Name: "Measured", Method: MethodValue}},
	}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if p.Name != "Spaced" || p.Scale != ScaleSaaty9 || p.SchemaVersion != SchemaVersion {
		t.Errorf("defaults not filled: name %q, scale %q, schema %d", p.Name, p.Scale, p.SchemaVersion)
	}
	if p.Evaluators[0].ID == "" || p.Evaluators[0].Weight != 1 || p.Alternatives[0].ID == "" {
		t.Error("missing IDs or evaluator weight were not filled")
	}
	if p.Criteria[0].Method != MethodPairwise || p.Criteria[1].Direction != DirHigher {
		t.Errorf("leaf defaults: method %q, direction %q", p.Criteria[0].Method, p.Criteria[1].Direction)
	}
	if p.Pairwise == nil || p.Ratings == nil || p.Values == nil {
		t.Error("nil slices would encode as JSON null")
	}
}

func TestPrepareDropsOrphansAndDuplicates(t *testing.T) {
	p := fixture()
	p.Pairwise = append(p.Pairwise,
		PairwiseJudgment{EvaluatorID: "ghost", Context: CriteriaContext(""), A: "c1", B: "c4", Value: 2}, // unknown evaluator
		PairwiseJudgment{EvaluatorID: "e1", Context: AltsContext("gone"), A: "a1", B: "a2", Value: 2},    // criterion deleted
		PairwiseJudgment{EvaluatorID: "e1", Context: CriteriaContext(""), A: "c1", B: "c2", Value: 2},    // c2 is not top-level
		PairwiseJudgment{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a3", B: "a1", Value: 0.5},    // duplicate of a1/a3, reversed
		PairwiseJudgment{EvaluatorID: "e1", Context: AltsContext("c2"), A: "a1", B: "a1", Value: 1},      // self-comparison
		PairwiseJudgment{EvaluatorID: "e1", Context: "nonsense", A: "a1", B: "a2", Value: 1},             // bad context
	)
	p.Ratings = append(p.Ratings, Rating{EvaluatorID: "e1", CriterionID: "c3", AlternativeID: "gone", Value: 3})
	p.Values = append(p.Values, Value{CriterionID: "c4", AlternativeID: "a1", Value: 150})
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if len(p.Pairwise) != 5 || len(p.Ratings) != 3 || len(p.Values) != 3 {
		t.Fatalf("got %d pairwise, %d ratings, %d values; want 5, 3, 3", len(p.Pairwise), len(p.Ratings), len(p.Values))
	}
	if j := p.Pairwise[3]; j.A != "a3" || j.B != "a1" || j.Value != 0.5 {
		t.Errorf("later duplicate should replace the earlier one in place, got %+v", j)
	}
	if p.Values[0].Value != 150 {
		t.Errorf("later value should win, got %v", p.Values[0].Value)
	}
}

func TestPrepareSnapsToScale(t *testing.T) {
	p := fixture()
	p.Pairwise[0].Value = 2.6
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if p.Pairwise[0].Value != 3 {
		t.Errorf("saaty9 snap: got %v, want 3", p.Pairwise[0].Value)
	}
	p.Scale = ScaleClassic5
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if p.Pairwise[0].Value != 5 {
		t.Errorf("classic5 snap: got %v, want 5", p.Pairwise[0].Value)
	}
}

func TestValidateReportsProblems(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(p *Project)
		field  string
	}{
		{"empty name", func(p *Project) { p.Name = " " }, "name"},
		{"bad scale", func(p *Project) { p.Scale = "huge" }, "scale"},
		{"no evaluators", func(p *Project) { p.Evaluators = nil; p.Pairwise = nil; p.Ratings = nil }, "evaluators"},
		{"negative evaluator weight", func(p *Project) { p.Evaluators[0].Weight = -1 }, "evaluators[0].weight"},
		{"duplicate alternative", func(p *Project) { p.Alternatives[1].ID = "a1" }, "alternatives[1].id"},
		{"unnamed alternative", func(p *Project) { p.Alternatives[2].Name = "" }, "alternatives[2].name"},
		{"reserved criterion id", func(p *Project) { p.Criteria[3].ID = "root" }, "criteria[3].id"},
		{"missing parent", func(p *Project) { p.Criteria[3].ParentID = "nope" }, "criteria[3].parentId"},
		{"cycle", func(p *Project) { p.Criteria[0].ParentID = "c2" }, "criteria[0].parentId"},
		{"bad method", func(p *Project) { p.Criteria[1].Method = "vibes" }, "criteria[1].method"},
		{"bad direction", func(p *Project) { p.Criteria[3].Direction = "sideways" }, "criteria[3].direction"},
		{"negative judgment", func(p *Project) { p.Pairwise[0].Value = -2 }, "pairwise[0].value"},
		{"NaN judgment", func(p *Project) { p.Pairwise[0].Value = math.NaN() }, "pairwise[0].value"},
		{"rating too high", func(p *Project) { p.Ratings[0].Value = 11 }, "ratings[0].value"},
		{"infinite value", func(p *Project) { p.Values[0].Value = math.Inf(1) }, "values[0].value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := fixture()
			c.mutate(p)
			var ve *ValidationError
			if err := p.Prepare(); !errors.As(err, &ve) {
				t.Fatalf("Prepare() = %v, want *ValidationError", err)
			}
			for _, pr := range ve.Problems {
				if pr.Field == c.field {
					return
				}
			}
			t.Errorf("no problem for field %q; got %+v", c.field, ve.Problems)
		})
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/model/`
Expected: FAIL — build fails with `p.Prepare undefined` and `undefined: ValidationError`.

- [ ] **Step 4: Write `internal/model/prepare.go`**

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
	evals := map[string]bool{}
	for _, e := range p.Evaluators {
		evals[e.ID] = true
	}
	alts := map[string]bool{}
	for _, a := range p.Alternatives {
		alts[a.ID] = true
	}
	crit := map[string]Criterion{}
	for _, c := range p.Criteria {
		crit[c.ID] = c
	}
	exists := func(id string) bool { _, ok := crit[id]; return ok }

	belongs := func(j PairwiseJudgment) bool {
		kind, id, ok := ParseContext(j.Context)
		if !ok || !evals[j.EvaluatorID] || j.A == j.B {
			return false
		}
		if kind == "alts" {
			return exists(id) && alts[j.A] && alts[j.B]
		}
		if id != "" && !exists(id) {
			return false
		}
		a, okA := crit[j.A]
		b, okB := crit[j.B]
		return okA && okB && a.ParentID == id && b.ParentID == id
	}
	pairs := []PairwiseJudgment{}
	pairAt := map[string]int{}
	for _, j := range p.Pairwise {
		if !belongs(j) {
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
		if !evals[r.EvaluatorID] || !exists(r.CriterionID) || !alts[r.AlternativeID] {
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
		if !exists(v.CriterionID) || !alts[v.AlternativeID] {
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

// Validate reports every problem that prevents the project from being saved.
// It returns nil or a *ValidationError.
func (p *Project) Validate() error {
	var probs []Problem
	add := func(field, format string, args ...any) {
		probs = append(probs, Problem{Field: field, Message: fmt.Sprintf(format, args...)})
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

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

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/model/`
Expected: PASS (`ok`)

- [ ] **Step 6: Commit**

```bash
make lint
git add internal/model/fixture_test.go internal/model/prepare.go internal/model/prepare_test.go
git commit -m "feat(model): prepare projects for saving: defaults, pruning, validation, snapping"
```

---

### Task 8: Computing results

**Files:**
- Create: `internal/model/compute.go`
- Test: `internal/model/compute_test.go`

**Interfaces:**
- Consumes: `Prepare`, `childrenOf`, `fixture` (Task 7); `ahp.Analyze`, `Suggestions`, `GeoMean`, `ArithMean`, `GeoStdDev`, `Synthesize`, `SensitivityLine`, `(Line).Stability` (Tasks 2–5).
- Produces: `model.Compute(p *Project, evaluator string) (*Results, error)`, with `model.AllEvaluators = "all"` and `ErrUnknownEvaluator`; result types `Results`, `AltScore`, `CriterionResult` (`Local`, `Global`, `Subtree`, `Stability *float64`), `MatrixReport`, `PairSuggestion`, `Issue`, `Disagreement`; issue codes `IssueNoCriteria`, `IssueGroupDisconnected`, `IssueAltsDisconnected`, `IssueRatingsMissing`, `IssueValuesMissing`, `IssueValuesInvalid`. The JSON shape of `Results` is the contract Plan 2's UI reads.

- [ ] **Step 1: Write the failing test `internal/model/compute_test.go`**

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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/model/`
Expected: FAIL — build fails with `undefined: Compute` and `undefined: Results`.

- [ ] **Step 3: Write `internal/model/compute.go`**

```go
package model

import (
	"errors"
	"fmt"
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
		Missing: a.Missing, Pairs: pairs, Suggestions: []PairSuggestion{},
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
	return normalize(raw)
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

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/model/`
Expected: PASS (`ok`)

- [ ] **Step 5: Commit**

```bash
make lint
git add internal/model/compute.go internal/model/compute_test.go
git commit -m "feat(model): compute scores, consistency reports, disagreements and stability"
```

---

### Task 9: File store

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/file.go`
- Test: `internal/store/file_test.go`

**Interfaces:**
- Consumes: `model.Project`, `model.New`, `model.Decode`, `model.Slug` (Task 6).
- Produces: `type store.Store interface { List(ctx) ([]Entry, error); Get(ctx, id string) (*model.Project, error); Create(ctx, p *model.Project) error; Save(ctx, p *model.Project, expectedVersion int) error; Delete(ctx, id string) error }`; `type Entry struct { Project *model.Project; File string; Err error }`; errors `ErrNotFound`, `ErrExists`, `ErrConflict`, `ErrDuplicateID`, and `*ConflictError{Current int}`; `store.NewFileStore(dir string) (*FileStore, error)`; `(*FileStore).Dir() string`. `Create` sets Version to 1, and `Save` sets it to expectedVersion+1, both updating the caller's struct in place.

- [ ] **Step 1: Create the package folder**

```bash
mkdir -p internal/store
```

- [ ] **Step 2: Write the failing test `internal/store/file_test.go`**

```go
package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

var ctx = context.Background()

func newStore(t *testing.T) *FileStore {
	t.Helper()
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func create(t *testing.T, s *FileStore, name string) *model.Project {
	t.Helper()
	p := model.New(name)
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCreateAndGet(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Pick a vendor")
	if p.Version != 1 {
		t.Errorf("Version = %d, want 1", p.Version)
	}
	got, err := s.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Pick a vendor" || got.Version != 1 {
		t.Errorf("Get = %+v", got)
	}
	entries, _ := s.List(ctx)
	if len(entries) != 1 || !regexp.MustCompile(`^pick-a-vendor-[a-z0-9]{6}\.ahp\.json$`).MatchString(entries[0].File) {
		t.Errorf("List = %+v", entries)
	}
	if err := s.Create(ctx, got); !errors.Is(err, ErrExists) {
		t.Errorf("second Create: err = %v, want ErrExists", err)
	}
}

func TestSaveIncrementsVersionAndKeepsBackup(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Backup")
	p.Description = "changed"
	if err := s.Save(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	if p.Version != 2 {
		t.Errorf("Version = %d, want 2", p.Version)
	}
	entries, _ := s.List(ctx)
	data, err := os.ReadFile(filepath.Join(s.Dir(), entries[0].File+".bak"))
	if err != nil {
		t.Fatal(err)
	}
	old, _ := model.Decode(data)
	if old.Version != 1 || old.Description != "" {
		t.Errorf("backup holds %+v, want version 1", old)
	}
}

func TestSaveRejectsStaleVersion(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Conflict")
	if err := s.Save(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	err := s.Save(ctx, p, 1)
	var ce *ConflictError
	if !errors.Is(err, ErrConflict) || !errors.As(err, &ce) || ce.Current != 2 {
		t.Errorf("err = %v, want *ConflictError{Current: 2}", err)
	}
}

func TestConcurrentSavesOnlyOneWins(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Race")
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp := *p
			if s.Save(ctx, &cp, 1) == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Errorf("%d saves succeeded, want 1", wins)
	}
}

func TestGetAndSaveUnknown(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: err = %v, want ErrNotFound", err)
	}
	if err := s.Save(ctx, model.New("x"), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Save: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteMovesToTrash(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Trash me")
	_ = s.Save(ctx, p, 1) // creates a .bak too
	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if entries, _ := s.List(ctx); len(entries) != 0 {
		t.Errorf("List after delete = %+v", entries)
	}
	trashed, _ := os.ReadDir(filepath.Join(s.Dir(), "trash"))
	if len(trashed) != 2 {
		t.Errorf("trash holds %d files, want the project and its backup", len(trashed))
	}
	if err := s.Delete(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete: err = %v, want ErrNotFound", err)
	}
}

func TestListToleratesCorruptFiles(t *testing.T) {
	s := newStore(t)
	create(t, s, "Good")
	if err := os.WriteFile(filepath.Join(s.Dir(), "broken.ahp.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var good, bad int
	for _, e := range entries {
		if e.Err != nil {
			bad++
		} else {
			good++
		}
	}
	if good != 1 || bad != 1 {
		t.Errorf("got %d good and %d bad entries, want 1 and 1", good, bad)
	}
}

func TestHandCopiedFileIsFlaggedNotAliased(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Original")
	entries, _ := s.List(ctx)
	orig := filepath.Join(s.Dir(), entries[0].File)
	data, _ := os.ReadFile(orig)
	// Finder names a copy "<name> copy.ahp.json", which sorts before the original.
	copyPath := strings.TrimSuffix(orig, ".ahp.json") + " copy.ahp.json"
	if err := os.WriteFile(copyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	entries, _ = s.List(ctx)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		isCopy := strings.HasSuffix(e.File, " copy.ahp.json")
		if isCopy != errors.Is(e.Err, ErrDuplicateID) {
			t.Errorf("%s: err = %v; only the copy should be flagged", e.File, e.Err)
		}
	}

	p.Description = "edited"
	if err := s.Save(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(copyPath); string(after) != string(data) {
		t.Error("saving the original changed the hand-made copy")
	}
}

func TestNonLatinNamesGetDistinctFiles(t *testing.T) {
	s := newStore(t)
	create(t, s, "評価")
	create(t, s, "選択")
	entries, _ := s.List(ctx)
	if len(entries) != 2 || entries[0].File == entries[1].File || entries[0].Err != nil || entries[1].Err != nil {
		t.Errorf("entries = %+v, want two readable, distinct files", entries)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test -race ./internal/store/`
Expected: FAIL — build fails with `undefined: NewFileStore` and `undefined: ErrConflict`.

- [ ] **Step 4: Write `internal/store/store.go`**

```go
// Package store persists projects. FileStore keeps one JSON file per project;
// other implementations (for a hosted mode) can satisfy the same interface.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

var (
	ErrNotFound = errors.New("project not found")
	ErrExists   = errors.New("a project with this id already exists")
	ErrConflict = errors.New("project was changed elsewhere")
	// ErrDuplicateID marks a project file that shares its id with another
	// file, usually because it was copied by hand.
	ErrDuplicateID = errors.New("this file has the same project id as another file")
)

// ConflictError is returned by Save when the stored version differs from the
// expected one. errors.Is(err, ErrConflict) reports true for it.
type ConflictError struct{ Current int }

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%v (stored version is %d)", ErrConflict, e.Current)
}

func (e *ConflictError) Is(target error) bool { return target == ErrConflict }

// Entry is one project found by List. Err is set, and Project is nil, when
// the file could not be read.
type Entry struct {
	Project *model.Project
	File    string
	Err     error
}

// Store persists projects.
type Store interface {
	List(ctx context.Context) ([]Entry, error)
	Get(ctx context.Context, id string) (*model.Project, error)
	// Create stores a new project, setting Version to 1.
	Create(ctx context.Context, p *model.Project) error
	// Save replaces a project if its stored version equals expectedVersion,
	// setting p.Version to expectedVersion+1. Otherwise it returns *ConflictError.
	Save(ctx context.Context, p *model.Project, expectedVersion int) error
	Delete(ctx context.Context, id string) error
}
```

- [ ] **Step 5: Write `internal/store/file.go`**

```go
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

const (
	ext      = ".ahp.json"
	trashDir = "trash"
)

// FileStore keeps each project in <dir>/<slug>-<id suffix>.ahp.json. Writes
// are atomic, the previous version is kept as a .bak file, and deleted
// projects move to <dir>/trash.
type FileStore struct {
	dir string
	mu  sync.Mutex
	now func() time.Time
}

// NewFileStore opens (creating if needed) a project folder.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(filepath.Join(dir, trashDir), 0o755); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Dir returns the folder the store reads and writes.
func (s *FileStore) Dir() string { return s.dir }

func (s *FileStore) List(ctx context.Context) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scan()
}

func (s *FileStore) Get(ctx context.Context, id string) (*model.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, p, err := s.find(id)
	return p, err
}

func (s *FileStore) Create(ctx context.Context, p *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, _, err := s.find(p.ID); err == nil {
		return ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	next := *p
	next.Version = 1
	next.UpdatedAt = s.now()
	if next.CreatedAt.IsZero() {
		next.CreatedAt = next.UpdatedAt
	}
	name := model.Slug(p.Name) + "-" + idSuffix(p.ID) + ext
	if err := writeAtomic(filepath.Join(s.dir, name), &next); err != nil {
		return err
	}
	*p = next
	return nil
}

func (s *FileStore) Save(ctx context.Context, p *model.Project, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, cur, err := s.find(p.ID)
	if err != nil {
		return err
	}
	if cur.Version != expectedVersion {
		return &ConflictError{Current: cur.Version}
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".bak", old, 0o644); err != nil {
		return err
	}
	next := *p
	next.Version = expectedVersion + 1
	next.UpdatedAt = s.now()
	if err := writeAtomic(path, &next); err != nil {
		return err
	}
	*p = next
	return nil
}

func (s *FileStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, _, err := s.find(id)
	if err != nil {
		return err
	}
	dest := filepath.Join(s.dir, trashDir, s.now().Format("20060102T150405Z")+"-"+filepath.Base(path))
	if err := os.Rename(path, dest); err != nil {
		return err
	}
	if err := os.Rename(path+".bak", dest+".bak"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// scan reads every project file in the folder, sorted by file name.
func (s *FileStore) scan() ([]Entry, error) {
	names, err := filepath.Glob(filepath.Join(s.dir, "*"+ext))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		e := Entry{File: filepath.Base(name)}
		data, err := os.ReadFile(name)
		if err == nil {
			e.Project, err = model.Decode(data)
		}
		e.Err = err
		entries = append(entries, e)
	}
	// A file copied by hand keeps its project id, and two files with one id
	// would edit each other. Only one file per id is used: the one carrying
	// the store's own name for it, else the first. The others are flagged.
	owner := map[string]int{}
	for i, e := range entries {
		if e.Project == nil {
			continue
		}
		j, dup := owner[e.Project.ID]
		switch {
		case !dup:
			owner[e.Project.ID] = i
		case !ownName(entries[j]) && ownName(e):
			flagDuplicate(entries, j, i)
			owner[e.Project.ID] = i
		default:
			flagDuplicate(entries, i, j)
		}
	}
	return entries, nil
}

// ownName reports whether an entry's file name is the one Create would give it.
func ownName(e Entry) bool { return strings.HasSuffix(e.File, "-"+idSuffix(e.Project.ID)+ext) }

func flagDuplicate(entries []Entry, loser, winner int) {
	entries[loser].Project = nil
	entries[loser].Err = fmt.Errorf("%w (%s); import it to make an independent copy", ErrDuplicateID, entries[winner].File)
}

func idSuffix(id string) string { return strings.ToLower(id[max(0, len(id)-6):]) }

// find returns the path and contents of the file holding project id.
func (s *FileStore) find(id string) (string, *model.Project, error) {
	entries, err := s.scan()
	if err != nil {
		return "", nil, err
	}
	for _, e := range entries {
		if e.Project != nil && e.Project.ID == id {
			return filepath.Join(s.dir, e.File), e.Project, nil
		}
	}
	return "", nil, ErrNotFound
}

// writeAtomic writes v as indented JSON to path via a synced temp file and a
// rename, so a crash never leaves a half-written project.
func writeAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./internal/store/`
Expected: PASS (`ok`)

- [ ] **Step 7: Commit**

```bash
make lint
git add internal/store/file.go internal/store/file_test.go internal/store/store.go
git commit -m "feat(store): atomic JSON file store with backups, trash and conflict detection"
```

---

### Task 10: HTTP server foundation: guard, errors and static UI

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/guard.go`
- Create: `internal/server/static.go`
- Create: `internal/server/placeholder.html`
- Test: `internal/server/server_test.go`

**Interfaces:**
- Consumes: `store.Store`, `store.NewFileStore` (Task 9).
- Produces: `type server.Config struct { Store store.Store; UI fs.FS; Version string; Host string }`; `server.New(cfg Config) http.Handler`. Unexported pieces that Task 11 uses: `type server struct{ cfg Config; mux *http.ServeMux }`, `(*server).routes()`, `writeJSON(w, status int, v any)`, `writeError(w, status int, code, message string, details any)`, `type apiError`, `maxBody`. Test helpers: `newTestServer(t) (http.Handler, *store.FileStore)`, `request(t, h, method, target string, body io.Reader, headers ...string)` (sets Host to localhost), `errorCode(t, rec) string`.

- [ ] **Step 1: Create the package folder**

```bash
mkdir -p internal/server
```

- [ ] **Step 2: Write the failing test `internal/server/server_test.go`**

```go
package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Xafloc/ahp-matrix/internal/store"
)

var testUI = fstest.MapFS{
	"index.html":    {Data: []byte("<html>app</html>")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

func newTestServer(t *testing.T) (http.Handler, *store.FileStore) {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{Store: st, UI: testUI, Version: "test", Host: "127.0.0.1"}), st
}

// request sends a request addressed to localhost. Extra headers come in
// name/value pairs.
func request(t *testing.T, h http.Handler, method, target string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	req.Host = "localhost:7777"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON error: %q", rec.Body.String())
	}
	return body.Error.Code
}

func TestVersion(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "GET", "/api/version", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"version":"test"`) {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestGuardRejectsForeignHost(t *testing.T) {
	h, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/version", nil)
	req.Host = "evil.example:7777"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "bad-host" {
		t.Errorf("got %d %q, want 403 bad-host", rec.Code, rec.Body.String())
	}
}

func TestGuardAllowsAnyHostWhenListeningEverywhere(t *testing.T) {
	st, _ := store.NewFileStore(t.TempDir())
	h := New(Config{Store: st, UI: testUI, Host: "0.0.0.0"})
	req := httptest.NewRequest("GET", "/api/version", nil)
	req.Host = "192.168.1.20:7777"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("got %d, want 200", rec.Code)
	}
}

func TestGuardRejectsCrossOriginWrites(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "POST", "/api/version", strings.NewReader("{}"), "Origin", "https://evil.example")
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "bad-origin" {
		t.Errorf("got %d %q, want 403 bad-origin", rec.Code, rec.Body.String())
	}
	rec = request(t, h, "POST", "/api/version", strings.NewReader("{}"), "Origin", "http://localhost:7777")
	if rec.Code == http.StatusForbidden {
		t.Error("same-origin write was rejected")
	}
}

func TestGuardRejectsFormEncodedWrites(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "POST", "/api/version", strings.NewReader("a=b"), "Content-Type", "text/plain")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("got %d, want 415", rec.Code)
	}
}

func TestStaticServesAssetsAndFallsBackToIndex(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "GET", "/assets/app.js", nil)
	if rec.Code != 200 || rec.Body.String() != "console.log(1)" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
	}
	rec = request(t, h, "GET", "/projects/123", nil)
	if rec.Code != 200 || rec.Body.String() != "<html>app</html>" {
		t.Errorf("SPA fallback: %d %q", rec.Code, rec.Body.String())
	}
	rec = request(t, h, "GET", "/api/nope", nil)
	if rec.Code != 404 || errorCode(t, rec) != "not-found" {
		t.Errorf("unknown API path: %d %q", rec.Code, rec.Body.String())
	}
}

func TestStaticPlaceholderWithoutUI(t *testing.T) {
	st, _ := store.NewFileStore(t.TempDir())
	h := New(Config{Store: st, UI: fstest.MapFS{}})
	rec := request(t, h, "GET", "/", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "doesn't include the web UI") {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/server/`
Expected: FAIL — build fails with `undefined: New` and `undefined: Config`.

- [ ] **Step 4: Write `internal/server/server.go`**

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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError sends {"error": {"code", "message", "details"}}.
func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: message, Details: details}})
}
```

- [ ] **Step 5: Write `internal/server/guard.go`**

```go
package server

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// maxBody caps request bodies, including uploads.
const maxBody = 20 << 20

// guard protects a server that has no authentication and listens locally:
//   - the Host header must name this machine, which defeats DNS rebinding;
//   - requests that change data must come from our own origin (when the
//     browser says where they come from) and carry JSON or multipart bodies,
//     which together defeat cross-site request forgery;
//   - bodies are capped at maxBody.
func (s *server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedHost(r.Host) {
			writeError(w, http.StatusForbidden, "bad-host", "requests must be addressed to this machine", nil)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && !sameOrigin(o, r.Host) {
				writeError(w, http.StatusForbidden, "bad-origin", "cross-origin requests are not allowed", nil)
				return
			}
			ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if r.ContentLength != 0 && ct != "application/json" && ct != "multipart/form-data" {
				writeError(w, http.StatusUnsupportedMediaType, "unsupported-media-type", "send application/json or multipart/form-data", nil)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *server) allowedHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	switch {
	case host == "localhost" || host == "127.0.0.1" || host == "::1":
		return true
	case s.cfg.Host == "0.0.0.0" || s.cfg.Host == "::":
		return true // the user chose to listen on every interface
	default:
		return s.cfg.Host != "" && host == s.cfg.Host
	}
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Host == host
}
```

- [ ] **Step 6: Write `internal/server/static.go`**

```go
package server

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed placeholder.html
var placeholder []byte

// static serves the built UI. Files that exist are served directly (hashed
// assets are cached forever); any other path gets index.html so the
// single-page app can route it. Unknown /api/ paths get a JSON 404.
func (s *server) static() http.Handler {
	var files http.Handler
	if s.cfg.UI != nil {
		files = http.FileServerFS(s.cfg.UI)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not-found", "no such endpoint", nil)
			return
		}
		var index []byte
		if s.cfg.UI != nil {
			index, _ = fs.ReadFile(s.cfg.UI, "index.html")
		}
		if index == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(placeholder)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(s.cfg.UI, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
```

- [ ] **Step 7: Write `internal/server/placeholder.html`**

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>AHP Matrix</title>
<style>
  body { font: 16px/1.5 system-ui, sans-serif; max-width: 36rem; margin: 4rem auto; padding: 0 1rem; color: #1f2328; background: #fff; }
  code { background: #f2f3f5; padding: .1rem .3rem; border-radius: 4px; }
  @media (prefers-color-scheme: dark) { body { color: #e6e6e6; background: #15171a; } code { background: #262a30; } }
</style>
</head>
<body>
<h1>AHP Matrix</h1>
<p>The server is running, but this build doesn't include the web UI yet.</p>
<p>To include it, build the UI (see <em>Development</em> in the README) and rebuild the binary. Meanwhile the API is available at <a href="/api/projects"><code>/api/projects</code></a>.</p>
</body>
</html>
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/server/`
Expected: PASS (`ok`)

- [ ] **Step 9: Commit**

```bash
make lint
git add internal/server/guard.go internal/server/placeholder.html internal/server/server.go internal/server/server_test.go internal/server/static.go
git commit -m "feat(server): request guard, JSON errors, embedded UI with SPA fallback"
```

---

### Task 11: Project API

**Files:**
- Create: `internal/server/projects.go`
- Modify: `internal/server/server.go` (`routes`)
- Test: `internal/server/projects_test.go`

**Interfaces:**
- Consumes: everything from Task 10; `store.Store` and its errors (Task 9); `model.New`, `Decode`, `Prepare`, `Compute`, `Slug`, `NewProjectID`, `AllEvaluators`, `ErrUnknownEvaluator`, `ValidationError`, `FutureVersionError` (Tasks 6–8).
- Produces the HTTP contract Plan 2 builds on:
-   - `GET /api/projects` returns `{"projects":[summary]}`, where each summary has `id, file, name, description, updatedAt, alternatives, criteria, ranking[{name,color,score}], completeness, consistent, error?`.
-   - `POST /api/projects` takes `{name?, description?}` and returns 201 with the project.
-   - `GET /api/projects/{id}` returns the project.
-   - `PUT /api/projects/{id}` takes the whole project with its current `version` and returns 200 with the saved project, 409 `conflict` with `details.currentVersion`, 422 `invalid` with `details` set to the list of problems, 422 `unsupported-version`, 400 `id-mismatch` or `bad-project`, 404, or 413 `too-large`.
-   - `DELETE /api/projects/{id}` returns 204.
-   - `POST /api/projects/{id}/duplicate` returns 201.
-   - `GET /api/projects/{id}/results?evaluator=all|<id>` returns `model.Results`, or 400 `unknown-evaluator`.
-   - `GET /api/projects/{id}/export.ahp.json` returns the file as an attachment.
-   - `POST /api/import/ahp` takes a multipart `file` field or a raw body, and returns 201 (with a new id if the original id is already taken).

- [ ] **Step 1: Write the failing test `internal/server/projects_test.go`**

```go
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

func decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return v
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

// createProject makes a project through the API and returns it.
func createProject(t *testing.T, h http.Handler, name string) *model.Project {
	t.Helper()
	rec := request(t, h, "POST", "/api/projects", strings.NewReader(`{"name":"`+name+`"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	return decode[*model.Project](t, rec.Body.Bytes())
}

// scored adds two alternatives, two criteria and a full set of judgments.
func scored(p *model.Project) {
	e := p.Evaluators[0].ID
	p.Alternatives = []model.Alternative{{ID: "a1", Name: "Alpha"}, {ID: "a2", Name: "Beta"}}
	p.Criteria = []model.Criterion{{ID: "c1", Name: "Speed"}, {ID: "c2", Name: "Price", Method: model.MethodRating}}
	p.Pairwise = []model.PairwiseJudgment{
		{EvaluatorID: e, Context: model.CriteriaContext(""), A: "c1", B: "c2", Value: 3},
		{EvaluatorID: e, Context: model.AltsContext("c1"), A: "a1", B: "a2", Value: 5},
	}
	p.Ratings = []model.Rating{
		{EvaluatorID: e, CriterionID: "c2", AlternativeID: "a1", Value: 2},
		{EvaluatorID: e, CriterionID: "c2", AlternativeID: "a2", Value: 8},
	}
}

func TestCreateAndList(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Pick a laptop")
	if p.Version != 1 || len(p.Evaluators) != 1 || p.ID == "" {
		t.Errorf("created %+v", p)
	}
	rec := request(t, h, "POST", "/api/projects", nil)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "Untitled decision") {
		t.Errorf("create without body: %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "GET", "/api/projects", nil)
	list := decode[struct{ Projects []projectSummary }](t, rec.Body.Bytes())
	if len(list.Projects) != 2 {
		t.Fatalf("list has %d projects, want 2", len(list.Projects))
	}
}

func TestSaveComputeAndConflict(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Flow")
	scored(p)

	rec := request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	saved := decode[*model.Project](t, rec.Body.Bytes())
	if saved.Version != 2 {
		t.Errorf("Version = %d, want 2", saved.Version)
	}

	rec = request(t, h, "GET", "/api/projects/"+p.ID+"/results", nil)
	res := decode[model.Results](t, rec.Body.Bytes())
	if rec.Code != 200 || len(res.Scores) != 2 || res.Completeness != 1 {
		t.Errorf("results: %d %+v", rec.Code, res)
	}

	// p still says version 1, so saving it again conflicts.
	rec = request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" || !strings.Contains(rec.Body.String(), `"currentVersion":2`) {
		t.Errorf("stale save: %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "GET", "/api/projects", nil)
	list := decode[struct{ Projects []projectSummary }](t, rec.Body.Bytes())
	if top := list.Projects[0].Ranking; len(top) != 2 || top[0].Score < top[1].Score {
		t.Errorf("summary ranking = %+v, want best first", top)
	}
}

func TestSaveRejectsInvalidProjects(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Invalid")
	p.Criteria = []model.Criterion{{ID: "c1", Name: "Odd", Method: "vibes"}}
	rec := request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"criteria[0].method"`) {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "PUT", "/api/projects/someone-else", mustJSON(t, p))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "id-mismatch" {
		t.Errorf("id mismatch: %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "PUT", "/api/projects/"+p.ID, strings.NewReader(`{"schemaVersion":7,"id":"`+p.ID+`"}`))
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "unsupported-version" {
		t.Errorf("future schema: %d %s", rec.Code, rec.Body.String())
	}

	big := bytes.Repeat([]byte(" "), maxBody+1)
	rec = request(t, h, "PUT", "/api/projects/"+p.ID, bytes.NewReader(big))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body: %d", rec.Code)
	}
}

func TestGetDeleteAndDuplicate(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Original")

	rec := request(t, h, "POST", "/api/projects/"+p.ID+"/duplicate", nil)
	cp := decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || cp.ID == p.ID || cp.Name != "Original (copy)" {
		t.Errorf("duplicate: %d %+v", rec.Code, cp)
	}

	if rec := request(t, h, "DELETE", "/api/projects/"+p.ID, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec := request(t, h, "GET", "/api/projects/"+p.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("get deleted: %d", rec.Code)
	}
	if rec := request(t, h, "DELETE", "/api/projects/"+p.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete twice: %d", rec.Code)
	}
}

func TestResultsForUnknownEvaluator(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Who")
	rec := request(t, h, "GET", "/api/projects/"+p.ID+"/results?evaluator=zzz", nil)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "unknown-evaluator" {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestExportAndImport(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Round Trip")
	rec := request(t, h, "GET", "/api/projects/"+p.ID+"/export.ahp.json", nil)
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename=round-trip.ahp.json` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	exported := rec.Body.Bytes()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "round-trip.ahp.json")
	_, _ = fw.Write(exported)
	_ = mw.Close()
	rec = request(t, h, "POST", "/api/import/ahp", &buf, "Content-Type", mw.FormDataContentType())
	imported := decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || imported.ID == p.ID || imported.Name != "Round Trip" {
		t.Errorf("import over an existing id should get a new id: %d %+v", rec.Code, imported)
	}

	rec = request(t, h, "POST", "/api/import/ahp", strings.NewReader(`{"hello":"world"}`))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "bad-project" {
		t.Errorf("import junk: %d %s", rec.Code, rec.Body.String())
	}
}

func TestListShowsUnreadableFiles(t *testing.T) {
	h, st := newTestServer(t)
	createProject(t, h, "Fine")
	if err := os.WriteFile(filepath.Join(st.Dir(), "mangled.ahp.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := request(t, h, "GET", "/api/projects", nil)
	list := decode[struct{ Projects []projectSummary }](t, rec.Body.Bytes())
	if len(list.Projects) != 2 || list.Projects[0].Error != "" || list.Projects[1].Error == "" {
		t.Errorf("list = %+v, want the readable project first and the broken one flagged", list.Projects)
	}
}

func TestSaveAfterFileDeletedElsewhere(t *testing.T) {
	h, st := newTestServer(t)
	p := createProject(t, h, "Vanishing")
	entries, _ := st.List(context.Background())
	if err := os.Remove(filepath.Join(st.Dir(), entries[0].File)); err != nil {
		t.Fatal(err)
	}
	rec := request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not-found" {
		t.Errorf("got %d %s, want 404 not-found", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -race ./internal/server/`
Expected: FAIL — build fails with `s.projectRoutes undefined`, then, until Step 4 registers the routes, the new tests fail with 404s.

- [ ] **Step 3: Write `internal/server/projects.go`**

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
	"time"

	"github.com/Xafloc/ahp-matrix/internal/model"
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
		Name        string `json:"name"`
		Description string `json:"description"`
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
	if req.Name == "" {
		req.Name = "Untitled decision"
	}
	p := model.New(req.Name)
	p.Description = req.Description
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
	name := mime.FormatMediaType("attachment", map[string]string{"filename": model.Slug(p.Name) + ".ahp.json"})
	w.Header().Set("Content-Disposition", name)
	writeJSON(w, http.StatusOK, p)
}

// importProject accepts an .ahp.json file, as a multipart "file" field or as
// the raw body. It keeps the file's project id unless that id is taken.
func (s *server) importProject(w http.ResponseWriter, r *http.Request) {
	data, ok := readUpload(w, r)
	if !ok {
		return
	}
	p, ok := decodeProject(w, data)
	if !ok {
		return
	}
	if p.ID == "" {
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

// createNew prepares and creates a new project, then writes it back with status.
func (s *server) createNew(w http.ResponseWriter, r *http.Request, p *model.Project, status int) {
	if !prepare(w, p) {
		return
	}
	if err := s.cfg.Store.Create(r.Context(), p); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, status, p)
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
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct != "multipart/form-data" {
		return readBody(w, r)
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		bodyError(w, err)
		return nil, false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		bodyError(w, err)
		return nil, false
	}
	return data, true
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

- [ ] **Step 4: Register the project routes in `internal/server/server.go`**

Change `routes` to:

```go
func (s *server) routes() {
	s.mux.HandleFunc("GET /api/version", s.handleVersion)
	s.projectRoutes()
	s.mux.Handle("GET /", s.static())
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./internal/server/`
Expected: PASS (`ok`)

- [ ] **Step 6: Check the whole module still passes**

Run: `make lint && make test`
Expected: no gofmt output, vet clean, every package `ok`.

- [ ] **Step 7: Commit**

```bash
make lint
git add internal/server/projects.go internal/server/projects_test.go internal/server/server.go
git commit -m "feat(server): project CRUD, results, duplicate, import and export endpoints"
```

---

### Task 12: The ahp-matrix command

**Files:**
- Create: `web/embed.go`
- Create: `web/dist/.gitkeep`
- Create: `cmd/ahp-matrix/main.go`
- Create: `cmd/ahp-matrix/browser.go`
- Test: `cmd/ahp-matrix/main_test.go`

**Interfaces:**
- Consumes: `server.New`, `server.Config` (Tasks 10–11); `store.NewFileStore` (Task 9).
- Produces: `web.FS() fs.FS` (Plan 2's Vite build writes into `web/dist`); the `ahp-matrix` binary with `version` subcommand and flags `--host`, `--port`, `--data-dir`, `--no-open`; the testable `run(ctx context.Context, args []string, out io.Writer) error`; `listen(host string, port int) (net.Listener, error)`; `defaultDataDir() (string, error)`; `openBrowser(url string) error`. `main.version` is set by `-ldflags -X` (the Makefile already does this).

- [ ] **Step 1: Create the UI embed package**

```bash
mkdir -p web/dist cmd/ahp-matrix
touch web/dist/.gitkeep
```

Create `web/embed.go`:

```go
// Package web holds the compiled single-page app, which Vite builds into
// web/dist. Until the UI is built, dist holds only .gitkeep and the server
// shows a placeholder page.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built UI files, rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is embedded above, so this cannot happen
	}
	return sub
}
```

- [ ] **Step 2: Write the failing test `cmd/ahp-matrix/main_test.go`**

```go
package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer that is safe to write and read concurrently.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunServesTheAPIUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() { done <- run(ctx, []string{"--port", "0", "--no-open", "--data-dir", t.TempDir()}, out) }()

	urlRe := regexp.MustCompile(`open:\s+(http://\S+)`)
	var url string
	for deadline := time.Now().Add(5 * time.Second); url == "" && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if m := urlRe.FindStringSubmatch(out.String()); m != nil {
			url = m[1]
		}
	}
	if url == "" {
		t.Fatalf("server never printed its URL; output: %q", out.String())
	}

	resp, err := http.Get(url + "/api/version")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/version = %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run returned %v, want nil after shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

func TestListenFallsBackToAFreePort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	ln, err := listen("127.0.0.1", busyPort)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if got := ln.Addr().(*net.TCPAddr).Port; got == busyPort {
		t.Errorf("listen reused busy port %s", strconv.Itoa(got))
	}
}

func TestWarnsWhenNotLoopback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"--host", "0.0.0.0", "--port", "0", "--no-open", "--data-dir", t.TempDir()}, out)
	}()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if regexp.MustCompile(`open:`).MatchString(out.String()) {
			break
		}
	}
	cancel()
	<-done
	if !regexp.MustCompile(`warning:.*no login`).MatchString(out.String()) {
		t.Errorf("no warning in output: %q", out.String())
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test -race ./cmd/ahp-matrix/`
Expected: FAIL — build fails with `undefined: run` and `undefined: listen`.

- [ ] **Step 4: Write `cmd/ahp-matrix/main.go`**

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
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
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

- [ ] **Step 5: Write `cmd/ahp-matrix/browser.go`**

```go
package main

import (
	"os/exec"
	"runtime"
)

// openBrowser opens url in the user's default browser without waiting for it.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./cmd/ahp-matrix/`
Expected: PASS (`ok`)

- [ ] **Step 7: Build and smoke-test the real binary**

Terminal 1:

```bash
make build
./bin/ahp-matrix version          # prints a git describe string, or "dev"
./bin/ahp-matrix --no-open --port 7791 --data-dir /tmp/ahp-smoke
```
Expected banner:
```
AHP Matrix <version>
  projects: /tmp/ahp-smoke
  open:     http://127.0.0.1:7791
```

Terminal 2:

```bash
B=127.0.0.1:7791
curl -s $B/api/version                                    # {"version":"..."}
ID=$(curl -s -X POST -H 'Content-Type: application/json' -d '{"name":"Smoke"}' $B/api/projects | sed -E 's/^\{"schemaVersion":[0-9]+,"id":"([^"]+)".*/\1/')   # the project id comes first; later "id"s are evaluators
curl -s $B/api/projects/$ID/results                       # {"evaluator":"all","scores":[],...,"completeness":1,"consistent":true}
curl -s $B/ | grep -o "doesn't include the web UI"        # placeholder page
curl -s -H 'Host: evil.com' $B/api/version                # {"error":{"code":"bad-host",...}}
curl -s -X POST -H 'Origin: http://evil.com' -H 'Content-Type: application/json' -d '{}' $B/api/projects   # bad-origin
ls /tmp/ahp-smoke                                         # smoke-xxxxxx.ahp.json  trash
```
Then press Ctrl-C in terminal 1. It should exit cleanly with status 0. Finally, `rm -rf /tmp/ahp-smoke`.

- [ ] **Step 8: Run the full suite one last time**

Run: `make lint && make test`
Expected: every package `ok`, and `web` reports `[no test files]`.

- [ ] **Step 9: Commit**

```bash
make lint
git add cmd/ahp-matrix/browser.go cmd/ahp-matrix/main.go cmd/ahp-matrix/main_test.go web/dist/.gitkeep web/embed.go
git commit -m "feat: ahp-matrix command with local server, browser launch and graceful shutdown"
```

---

## After Plan 1

- Push `main` to `Xafloc/ahp-matrix` (switch the gh account first: `gh auth switch -u Xafloc`) and confirm the `ci` workflow is green.
- Next: write **Plan 2 (Web UI)** against the API contract in Task 11 and the `Results` JSON shape in Task 8.
