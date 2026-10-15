# AHP Matrix — Design Spec

**Date:** 2026-10-15
**Status:** Approved
**Repo:** `github.com/Xafloc/ahp-matrix` (public, MIT)

## 1. Intent

**Goal.** A modern, open-source decision-making tool that replaces the
"Priority Matrix – 25.xlsx" spreadsheet. You run one binary; it starts a local
HTTP server and opens a polished browser UI where you create, load, and edit
decision projects (alternatives scored against weighted criteria).

**Audience.** The public. A stranger should be able to install it in one step and
reach a meaningful result within a minute (bundled samples help here).

**Success criteria.**
- Install with a single binary download or `brew install`; run `ahp-matrix`; the browser opens.
- Everything the spreadsheet does, with correct AHP math and a consistency check.
- Far less work to score than the spreadsheet's 1,125 comparisons, via faster scoring methods, hierarchy, and a one-pair-at-a-time flow.
- Supports multiple evaluators locally, and is designed so a hosted, multi-user mode can come later without a rewrite.

**Decided in brainstorming:**
- Local-first and single-user at runtime, but every judgment records which evaluator made it.
- v1 includes faster scoring methods, sensitivity analysis, hierarchical criteria, and Excel import/export.
- Stack: Go backend; Svelte 5 + Vite + TypeScript SPA compiled into the binary; one JSON file per project.

## 2. Review of the source spreadsheet

The workbook implements a simplified AHP:

- `Criteria Summary`: you fill in the lower triangle of a 25×25 criteria matrix (row-vs-column, scale 10/5/1/⅕/⅒); the upper triangle fills in as reciprocals. Weights are row sums divided by the grand total.
- `Criterion N` (×25): a 10×10 matrix comparing alternatives for one criterion, with the same row-sum weighting and a notes cell (`R5`).
- `Summary`: an alternative's score is Σ (criterion weight × alternative's local weight), plus its relative value.
- Scale values live in `Summary!D33:D37` (10, 5, 1, 0.2, 0.1).

Problems this design fixes:
1. **Approximate weights.** Normalizing row sums is only exact when judgments are perfectly consistent. → Use the principal eigenvector.
2. **No consistency check.** → Show the Consistency Ratio (CR) for every matrix and point to the judgments that contradict each other most.
3. **Silent failure on blanks.** A missing judgment is treated as zero and still yields scores. → Show completeness explicitly; a mathematically sound method handles incomplete matrices.
4. **Broken Notes sheet.** It links to an external workbook (`[1]`). → Notes are stored with the project.
5. **Comparison overload and hard limits** (25 criteria, 10 alternatives). → No hard limits; hierarchy plus alternative scoring methods.
6. **Coarse 5-step scale.** → Saaty's 1–9 scale by default; the legacy scale is kept for imports.

## 3. Architecture

```
cmd/ahp-matrix/        main: flags, start server, open browser
internal/ahp/          pure math (no I/O): eigenvector, consistency, incomplete matrices,
                       aggregation, synthesis, sensitivity
internal/model/        Project types, validation, schema migration
internal/store/        Store interface + FileStore (JSON files)
internal/xlsx/         legacy template import; results export
internal/server/       HTTP API, middleware, static UI
internal/samples/      embedded sample projects / templates (JSON)
web/                   Svelte app; web/embed.go embeds web/dist via go:embed
```

Each unit has one job:
- `ahp` takes plain matrices and vectors and returns numbers. It knows nothing about projects.
- `model` turns a Project into matrices and calls `ahp`.
- `store` only persists data.
- `server` only translates HTTP to and from model and store calls.

This keeps the math exhaustively testable, and lets a future hosted mode swap in a different `Store` implementation (for example SQLite or Postgres).

**Libraries:**
- Go standard library `net/http` (1.22+ routing patterns)
- `github.com/xuri/excelize/v2`
- `github.com/pkg/browser` (or equivalent) to open the browser

No CGO, so builds cross-compile cleanly. Releases use `goreleaser` (macOS, Linux and Windows on amd64/arm64, plus a Homebrew tap under Xafloc).

`go install` isn't a supported install path because the UI has to be built first. `web/dist` holds a committed placeholder `index.html` so the Go code always compiles.

## 4. Data model (`*.ahp.json`)

```jsonc
{
  "schemaVersion": 1,
  "id": "01J…",                 // ULID
  "version": 7,                 // incremented on every save (optimistic concurrency)
  "name": "Pick an IAM vendor",
  "description": "",
  "createdAt": "…", "updatedAt": "…",
  "scale": "saaty9",            // "saaty9" | "classic5"
  "alternatives": [ { "id": "a1", "name": "Courion", "description": "", "color": "#…" } ],
  "criteria": [
    { "id": "c1", "parentId": null, "name": "Technical", "description": "", "notes": "" },
    { "id": "c2", "parentId": "c1", "name": "Performance",
      "method": "pairwise",     // leaves only: "pairwise" | "rating" | "value"
      "direction": "higher",    // "value" only: "higher" | "lower"
      "unit": "",               // "value" only, display only
      "notes": "" }
  ],
  "evaluators": [ { "id": "e1", "name": "Darren", "weight": 1 } ],
  "pairwise": [
    // context: "criteria:<parentId|root>" compares sibling criteria;
    //          "alts:<leafCriterionId>" compares alternatives
    { "evaluatorId": "e1", "context": "criteria:root", "a": "c1", "b": "c3",
      "value": 3, "note": "" }  // value = how strongly a is preferred over b (>0)
  ],
  "ratings": [ { "evaluatorId": "e1", "criterionId": "c4", "alternativeId": "a1", "value": 7 } ],
  "values":  [ { "criterionId": "c5", "alternativeId": "a1", "value": 125000 } ]
}
```

**Rules:**
- **Criteria form a tree** (`parentId`). Only leaves have a `method`, and only leaves score alternatives. Sibling groups are always compared pairwise.
- **Pairwise judgments** are stored once per unordered pair, with the reciprocal implied. Writing (b, a) replaces (a, b) with the inverted value.
- **Each pairwise value must be in the scale's allowed set**: Saaty is 1–9 and their reciprocals; classic5 is {10, 5, 1, ⅕, ⅒}. Changing the scale snaps existing values to the nearest allowed value, and the UI asks for confirmation first.
- **Ratings** are 1–10, recorded per evaluator.
- **Measured values** are objective data (no evaluator). `higher` requires values ≥ 0 that sum to more than 0; `lower` requires values > 0.
- **Deleting an alternative, criterion or evaluator** cascades to the judgments that reference it.
- **On load**, `schemaVersion` is migrated forward. Unknown future versions are rejected with a clear error.

## 5. Math (`internal/ahp`)

**Priority vector.** The principal eigenvector, computed by power iteration and normalized to sum to 1. It converges in a few dozen iterations for n ≤ 50 (tolerance 1e-10, iteration cap 1000).

**Incomplete matrices.** Uses Harker's method:
- missing entries are set to 0
- each diagonal entry becomes 1 + (number of missing entries in that row)
- the eigenvector is then computed as usual.

This needs the comparison graph to be connected. If it isn't, the result is "incomplete" and lists the groups that aren't linked to each other. This method is what makes the one-pair-at-a-time flow show useful results before every pair has been answered.

**Consistency.**
- CI = (λmax − n)/(n − 1), and CR = CI/RI.
- RI comes from Saaty's table for n ≤ 15. Above that it uses Alonso and Lamata's regression, RI = (1.7699·n − 4.3513)/(n − 1).
- n ≤ 2 gives CR = 0.
- The threshold is 0.10 (0.05 for n = 3, 0.08 for n = 4).
- For incomplete matrices, CR is computed on Harker's modified matrix and labeled "provisional" until every pair has been answered.

**Fix suggestions.**
- Each judgment is ranked by its deviation |ln(aᵢⱼ · wⱼ / wᵢ)|.
- The top 3 are shown with a suggested value: wᵢ/wⱼ snapped to the nearest allowed scale value.

**Local priorities by method:**
- **pairwise:** the eigenvector of the alternatives matrix.
- **rating:** rᵢ / Σr.
- **value, `higher`:** vᵢ / Σv.
- **value, `lower`:** (1/vᵢ) / Σ(1/v).

**Hierarchy synthesis.**
- A leaf's global weight is the product of the local weights along its path from the root.
- An alternative's score is Σ over leaves of (global weight × local priority).
- Scores sum to 1 (distributive mode, matching the spreadsheet).

**Group aggregation (AIJ).**
- For each pair, combine the evaluators who answered it with an evaluator-weighted geometric mean of their judgments, then compute as normal.
- Ratings are combined with an evaluator-weighted arithmetic mean.
- Results can be computed for "All evaluators" or for any single evaluator.
- Disagreement per pair is the geometric standard deviation of the judgments. The most contested pairs are surfaced.

**Sensitivity.**
- For a criterion c with local weight w among its siblings, set its weight to x ∈ [0, 1] and rescale the siblings by (1 − x)/(1 − w).
- Every alternative's score is then linear in x: Sₐ(x) = restₐ + Gₚ·[x·Tₐ(c) + (1 − x)/(1 − w)·Σ_{s≠c} wₛ·Tₐ(s)], where
  - Tₐ(n) is the alternative's score within subtree n, normalized by n's global weight
  - Gₚ is the parent's global weight
  - restₐ is the alternative's score contribution from outside c's sibling group
- The points where the ranking changes are solved exactly.
- **Stability** of a criterion is the smallest |x − w| at which the top alternative changes ("none" if that never happens in [0, 1]).

## 6. Persistence (`internal/store`)

```go
type Store interface {
    List(ctx) ([]ProjectSummary, error)
    Get(ctx, id) (*model.Project, error)
    Create(ctx, *model.Project) error
    Save(ctx, p *model.Project, expectedVersion int) error // ErrConflict on mismatch
    Delete(ctx, id) error
}
```

`FileStore` details:
- **Location:** `--data-dir`, defaulting to the OS user data directory:
  - macOS: `~/Library/Application Support/ahp-matrix`
  - Linux: `$XDG_DATA_HOME/ahp-matrix`
  - Windows: `%AppData%`
- **Filenames:** `<slug>-<id-suffix>.ahp.json`.
- **Writes are atomic** (write a temp file, fsync, rename). The previous version is kept as `.ahp.json.bak`.
- **Deleting** moves the file to `trash/` instead of removing it.
- **Invalid files** in the folder are listed with an error badge rather than crashing the app.

## 7. HTTP API (`internal/server`)

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/projects` | list summaries (name, counts, leader, completeness, CR status, updatedAt) |
| POST | `/api/projects` | create: blank, `fromTemplate`, or `fromSample` |
| GET / PUT / DELETE | `/api/projects/{id}` | read, save whole document (`version` → 409 on conflict), delete |
| POST | `/api/projects/{id}/duplicate` | copy |
| GET | `/api/projects/{id}/results?evaluator=all\|<id>` | weights, scores, per-matrix CR and completeness, fix suggestions, disagreement, sensitivity inputs |
| POST | `/api/import/xlsx` | multipart upload of a legacy template → new project |
| POST | `/api/import/ahp` | upload an `.ahp.json` → new project |
| POST | `/api/projects/{id}/merge` | upload an `.ahp.json` and merge its evaluators' judgments (see §9) |
| GET | `/api/projects/{id}/export.{xlsx,csv,ahp.json}` | downloads (`?evaluator=<id>` on `.ahp.json` exports a single evaluator's packet) |
| GET | `/api/templates`, `/api/samples` | built-in starters |

**Saving.** The UI saves the whole document, autosaving 600 ms after the last edit. That's simple and fine for files this size. A version conflict (for example, the same project open in two tabs) shows a "reload / overwrite" prompt.

**Errors** use the format `{ "error": { "code", "message", "details" } }`. Validation errors point to the offending field.

**Local-server safety:**
- Binds to `127.0.0.1` by default. `--host` can change that, but logs a warning, because there's no authentication.
- Rejects requests whose `Host` header isn't localhost or the configured host, which stops DNS-rebinding attacks.
- Requests that change data must have JSON or multipart content and a same-origin `Origin` header, which stops other websites from triggering changes (CSRF).
- Uploads are capped at 20 MB.

**CLI:** `ahp-matrix [--port 7777] [--host 127.0.0.1] [--data-dir DIR] [--no-open]`, plus `ahp-matrix version`. If the port is busy, it falls back to a free port and prints the URL.

## 8. UI (`web/`)

Svelte 5, Vite, TypeScript and Tailwind v4, with a small hash router. Charts are hand-written SVG components (bars, stacked bars, line chart, donut and sunburst), with no heavy chart library. Light and dark themes follow the OS setting, with a manual toggle. The look is clean, with generous spacing, color-coded alternatives and smooth transitions.

**Screens:**
1. **Library:** project cards showing name, current leader, a mini ranking bar, completeness ring, CR badge and last edit. Plus "New", "Import .xlsx / .ahp.json", templates and samples. Has search and sort.
2. **Project workspace:** a step rail along the left, an evaluator switcher in the top bar ("Scoring as: …"), and overall progress.
   - **Setup:** add, edit and reorder alternatives. A criteria tree editor with drag-and-drop to nest and reorder, and a per-leaf choice of method, direction and unit. Evaluators and scale. A warning when a sibling group has more than 9 items, suggesting a split or a rating method.
   - **Weigh criteria:** one comparison per sibling group (from root downward).
   - **Score alternatives:** one panel per leaf criterion, laid out according to its method:
     - **pairwise:** compare alternatives two at a time
     - **rating:** a row of 1–10 controls
     - **value:** number inputs with a unit and a "higher/lower is better" chip
   - **Results:** ranked bars with scores; stacked bars showing each criterion's contribution; a sunburst of the weight hierarchy; per-evaluator rankings side by side with the most-contested pairs; and a consistency report with "fix" links that jump straight to the pair.
   - **Sensitivity:** pick a criterion, drag its weight, and the score lines update live, with crossover points marked. Also a stability table ranking criteria by how fragile the winner is.
3. **Pairwise comparison**, with two modes you can switch between:
   - **Focus:** two big cards and a 17-stop slider (9 … 1 … 9) with spoken-style labels ("A is strongly preferred"), an optional note, and a "Skip". Keyboard: ←/→ to move, 1–9 to set, Enter for next, S to skip. The order is smart: it asks first the pairs that most improve how well connected and stable the results are.
   - **Matrix:** a heat-mapped grid, where the reciprocal fills in automatically and inconsistent cells are outlined.
   - Both modes show a live consistency meter.
4. **Print report:** a print stylesheet so the browser's "Save as PDF" produces a clean summary.

**The live slider math**, linear recombination only (§5 Sensitivity), is mirrored in TypeScript. All other math runs on the server. Parity is guaranteed by shared golden fixtures (§10).

## 9. Import / export / merge

**Legacy `.xlsx` import (`internal/xlsx`).**
- Detects the template by its sheet names (`Summary`, `Criteria Summary`, `Criterion 1…`).
- Reads:
  - alternatives from `Summary!C19:C28`
  - criteria from `Summary!D7:AB7`
  - criteria judgments from the lower triangle of `Criteria Summary!D5:AB29`, where cell (row i, column j<i) means "i compared to j"
  - alternative judgments from the lower triangle of each `Criterion N!D5:M14`
  - notes from `Criterion N!R5`
- Blank names are skipped. Cells that contain formulas use their cached values.
- The scale is `classic5`, everything goes under a single evaluator named "Imported", and criteria are a flat list.
- The import ends with a report of what was imported and what was skipped.

**Export.**
- **`.xlsx`:** a new workbook with Results, Weights, Consistency, Judgments, and one sheet per pairwise matrix. It doesn't recreate the legacy template.
- **`.csv`:** the results table.
- **`.ahp.json`:** the full project, or a single evaluator's packet.

**Multi-evaluator round trip.**
1. The owner exports an evaluator packet: the project structure plus only that evaluator.
2. The colleague runs the app, imports the packet, scores it, and sends it back.
3. The owner chooses "Merge" and uploads it.

Merge rules:
- The packet must have the same project `id`.
- Judgments are merged per evaluator. If an evaluator already exists, the owner chooses "replace" or "skip".
- Judgments that reference alternatives or criteria the owner's project doesn't have are dropped and listed.
- Structural edits in the packet are ignored. The owner's structure always wins.

## 10. Testing

- **`internal/ahp`** (table-driven):
  - published worked examples (Saaty's textbook examples with known weights, λmax and CR)
  - perfectly consistent matrices give CR ≈ 0 and exact ratio weights
  - weights always sum to 1
  - Harker incomplete results equal the complete result when nothing is missing
  - a disconnected comparison graph is detected
  - AIJ aggregation of identical evaluators equals a single evaluator
  - sensitivity crossovers checked against brute-force sampling
- **`internal/model`:** validation, cascade deletes, scale snapping, schema migration.
- **`internal/store`:** atomic writes, `.bak` copies, conflict detection, tolerance of corrupt files (using a temporary directory).
- **`internal/xlsx`:**
  - import of a fixture made by filling the blank template programmatically with made-up data (the real 2006 data isn't committed)
  - round trip: export, then reading back the values
- **`internal/server`:** `httptest` tests for each endpoint, including the 409 conflict, Host and Origin rejection, and the upload size limit.
- **Golden fixtures:** Go writes the results and sensitivity inputs for each sample project to `testdata/golden/*.json`. Vitest checks that the TypeScript slider math matches.
- **Frontend:**
  - Vitest for stores and utilities
  - one Playwright smoke test: create a project → add 3 alternatives and 2 criteria → compare → see results → drag sensitivity
- **CI (GitHub Actions):** `go vet`, `go test -race`, `golangci-lint`, `npm run check && npm test && npm run build`, and Playwright on Linux. `goreleaser` runs on `v*` tags.

## 11. Bundled content

- **Samples** (ready to explore):
  - "Choosing a cloud provider" (hierarchical, with all three methods)
  - "Hiring a senior engineer" (two evaluators who disagree)
  - "Buying a car" (small and quick)
- **Templates** (structure only): Vendor selection (based on the spreadsheet's 23 criteria, grouped into a hierarchy), Software tool choice, and Job offer comparison.

## 12. Non-goals for v1

- Accounts, a hosted mode, or real-time collaboration. The data model and `Store` interface are ready for them.
- Ideal-mode synthesis, absolute-measurement rating intensities, and ANP (the network variant of AHP).
- Writing back to the legacy `.xlsx` template format.
- Undo/redo. Autosave and `.bak` files cover recovery.
- Internationalization.

## 13. Resolved decisions

- **Binary name:** `ahp-matrix`.
- **License:** MIT.
