# Group judging, Phase B (v0.3.0): no install

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Each task is red, then green, then commit. Every task's tests are written first and watched failing.

**Goal:** someone who doesn't have AHP Matrix can still judge. Each invitation can be a single self-contained `.html` page that works offline from a double-click, runs the same consistency check as the app, and saves a file to send back that merges like any returned copy.

**Spec:** `docs/superpowers/specs/2026-10-21-group-judging-design.md` §8 and §10. Builds on Phase A (v0.2.0).

**Format note:** like Phase A, this plan lists each task's interfaces and the tests to write first.

## Global constraints

- **Nothing new to install:** the page needs no server, no network and no install. It opens from `file://` in current Chrome, Safari, Firefox and Edge.
- **One file:** scripts, styles and the Latin subsets of the two fonts are inlined. No external `src` or `href`.
- **Same math:** the page's consistency results match Go's to 1e-9 on the golden files.
- **Same answers:** what the page sends back is an ordinary `.ahp.json` copy (the packet plus that person's answers), so Phase A's merge, preview and Undo apply unchanged.
- **Dependencies:** the Go floor stays 1.24; no new Go or npm runtime dependencies. The inlining is a small build script of our own.
- **Copy:** sentence case, plain words, errors that say what to do next. Every screen works at 390 px, in both themes.

## Review focus

1. **Escaping:** a decision or person named `</script><script>alert(1)</script>` must not break out of the injected JSON. Go's `json.Marshal` escapes `<`, `>` and `&`; the test checks it.
2. **Storage:** saving progress in local storage can throw (Safari private mode, `file://` in some browsers). The page must still work, just without resuming.
3. **Strange packets:** the page opened without a packet (a raw `judge.html`), or with a packet from a newer schema, shows a clear message rather than a blank page.
4. **Packet contents:** the returned file keeps the packet's `id`, `origin` and `packet` fields, so Import recognition and merge matching work.
5. **Size:** the page should be under 600 KB, so it can be emailed.

## Tasks

### B1. Port the consistency math to TypeScript (`web/src/lib/ahp.ts`)

- **Port** of `internal/ahp`:
  - `Matrix`, `newMatrix(n)`, `setPair(m, i, j, v)`
  - `analyze(m) → {weights | null, lambdaMax, cr, threshold, consistent, missing, complete, connected, components}`: Harker's method, bisection on λmax with the M-matrix pivot test, then inverse iteration
  - `randomIndex(n)`, `crThreshold(n)`, `snap(v, allowed)`, `allowed(scale)`
  - `suggestions(m, w, allowed, k)`
  - `crWith(m, i, j, v)`
- **`matrixReport(p, evaluatorId, context, items) → MatrixReport`:** builds one person's matrix from their judgments, exactly as `model.compute` does for one evaluator; three suggestions with `crAfter`.
- **Tests:**
  - **golden parity:** for every matrix in `internal/model/testdata/golden/*.json`, `analyze` + `suggestions` on its `matrix` reproduce `weights`, `cr`, `consistent`, `connected`, `missing` and the suggestions (pair, current, suggested, `crAfter`) to 1e-9 relative
  - a hand-checked 3×3
  - an incomplete but connected 4×4 (Harker)
  - a disconnected matrix gives components and no weights
  - `matrixReport` on the fixture for e1 matches the golden e1-only values, computed by Go as a new golden `fixture-e1.json`

### B2. A session that works without a server (`web/src/lib/local.svelte.ts`)

- **`interface CompareSession`:** `project`, `scoringAs`, `results: Record<string, {matrices: MatrixReport[]}>`, `changed()`, `watch(ev)`. Both `ProjectSession` and the new `LocalSession` implement it; `PairCompare` and `ConsistencyMeter` accept the interface.
- **`LocalSession`:**
  - `changed()` recomputes the person's matrices with `matrixReport`, synchronously
  - it saves the project to local storage under `ahp-judge:<id>:<evaluator>`, guarded by try/catch
  - `LocalSession.restore(packet)` resumes from storage when the saved copy has the same structure fingerprint string, and otherwise starts from the packet
- **Tests:**
  - `changed()` updates the results
  - resume and fresh start
  - storage that throws on read or write still works
  - a stored copy with a different `packet.structure` is ignored

### B3. The judging page (`web/judge.html`, `web/src/judge/`)

- **Input:** reads the packet from `<script type="application/json" id="ahp-packet">`. With none, or one whose `schemaVersion` is newer than 2, it says so: "This page is missing its decision. Ask the sender for a new invitation."
- **Welcome:**
  - who asked and the decision's name and description
  - what they're asked to do (`describeTasks`) and roughly how long (about 10 seconds a comparison, rounded to minutes)
  - **Start**
- **Tasks:** one step per assigned sibling group (weigh) and per assigned leaf (score), in order, with a step list and progress:
  - pairwise steps use `PairCompare` with a `LocalSession`
  - rating steps use a new `RatingRows.svelte`, extracted from `Score.svelte` and reused there
  - measured-value leaves are skipped, since they're the sender's
- **Consistency:** the existing meter, flags and **Why?** dialog, driven by `matrixReport`.
- **Finish and send back:**
  - downloads `<slug(decision)>-answers-from-<slug(person)>.ahp.json` through a Blob
  - the done screen says: "Email this file to Darren. That's all; you can close this page."
  - unfinished steps are listed with **Finish anyway**
- **Tests:**
  - **unit:** the step list from tasks (`judgeSteps(p, e)`), the time estimate, the missing-packet message
  - **SSR:** the welcome screen text for a weigh-only person

### B4. Building it as one file

- **Vite:** a second input, `judge.html`.
- **`web/scripts/inline-judge.mjs`, run after `vite build`:**
  - replaces the page's `<script src>` and `<link rel=stylesheet>` with inline contents
  - inlines `url(...)` font files in the CSS as base64 `data:` URIs, Latin subsets only (the judge entry imports a small `judge-fonts.css` declaring the Latin woff2s)
  - drops the now-unused `dist/assets/judge-*` files
  - fails the build if any external reference remains
- **The app build is unchanged:** the main app keeps its CSP-safe, non-inlined fonts.
- **Tests:** a vitest check over `dist/judge.html` (skipped when dist isn't built) that finds no `src=`/`href=` to files, one `#ahp-packet` placeholder, and a size under 600 KB. The e2e build runs it.

### B5. Serving invitations (`internal/server`)

- **`GET /api/projects/{id}/invite.html?evaluator=<id>`:**
  - makes the packet (`model.Packet`) and reads the embedded `judge.html`
  - replaces the placeholder `<script type="application/json" id="ahp-packet"></script>` with the packet JSON (Go's HTML-safe escaping)
  - **filename:** `<slug>-for-<person>.html`
  - **errors:** 400 `unknown-evaluator`; 503 `judge-page-missing` when the UI wasn't built (development)
- **Tests:**
  - the page contains the packet and decodes back to it
  - a `</script>` in a name stays escaped
  - unknown evaluator
  - missing page
  - the filename

### B6. The invitation dialog (People step)

- **Invite** opens `InviteDialog.svelte`. It records the invitation as in Phase A, then offers:
  - **Download web page** (recommended): "They open it in any browser; nothing to install"
  - **Download app file**: "For people who use AHP Matrix"
  - an optional **Answer by** date
  - a ready-to-paste message that adapts to the role and the date, with **Copy message**
- **`invitation(p, e, {from, deadline, kind})` → string:** "Hi Priya, could you help me decide <decision>? I'd like you to <describeTasks>. It takes about N minutes. Open the attached page in your browser, answer, then press Finish and send back the file it saves. [Please answer by Fri 24 Oct.] Thanks, Darren"
- **Copy reminder** mentions the page or the app, whichever was sent last (`invite.kind`, a new optional field: `"page" | "app"`).
- **Tests:**
  - **unit:** `invitation()` for each role, with and without a date, page and app
  - **e2e:** Invite → dialog → Download web page gives `<slug>-for-priya.html` and status Invited

### B7. Round trip, verify and release

- **e2e round trip:**
  - Invite → download the page → open it from `file://` in Playwright → Start → answer every comparison → Finish and send back → the saved file → drop on the decision → the preview lists Priya → Merge → status Returned and progress full
  - the page at 390 px wide with no horizontal scroll
  - a contradiction on the page shows **Why?**
- Full suite, browser review (both themes, phone), independent review, fix pass.
- README section "Inviting people who don't have AHP Matrix".
- Merge, CI, tag v0.3.0, check the Homebrew update.
