# Group judging, Phase A (v0.2.0): collect

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Each task is red, then green, then commit. Every task's tests are written first and watched failing.

**Goal:**
- A People step for roles, progress, status and invitations.
- Merging returned answers with a preview: by drop, by import, or with the button, with undo.
- "Out of date" warnings.
- Received-copy mode.
- Comparison screens filtered by role.
- The "combine rankings" method.

**Spec:** `docs/superpowers/specs/2026-10-21-group-judging-design.md` §4–7. Builds on everything up to v0.1.4.

**Format note:** this plan lists each task's interfaces and the tests to write first, rather than every implementation line. Execution is still strictly test-first, with a browser review and an independent review before release.

## Global constraints

- Schema version 2; version 1 files migrate on load without changes; files newer than this build are rejected.
- Nothing is silently discarded: everything a merge can't apply is listed in the preview and the report.
- The Go floor stays at 1.24. No new dependencies in Phase A.
- UI copy follows the earlier plans: sentence case, plain words, errors that say what to do next.
- Every new screen works at 390 px wide, in both themes.

## Tasks

### A1. Schema version 2 and fingerprints (`internal/model`)

- **Types:** `Tasks{Weigh, ScoreAll bool; Score []string}`, `Invite{SentAt, ReturnedAt *time.Time; Structure string}`, `PacketInfo{For, From, Structure string; SentAt time.Time}`.
- **New fields:** `Evaluator.Tasks *Tasks` (nil means everything), `Evaluator.Invite *Invite`, `Project.Aggregation`, `Project.Packet *PacketInfo`.
- **Version:** `SchemaVersion = 2`; `Decode` migrates version 1.
- **Prepare** drops task leaves that no longer exist or aren't leaves, defaults `Aggregation` to `"judgments"`, and rejects any other value.
- **`Fingerprint(p, evaluatorID) string`:** a SHA-256 of the canonical structure plus that person's tasks, first 12 hex characters.
- **`Packet(p, id)`:** now sets `Packet{For, From (the first evaluator's name), SentAt, Structure}`.
- **Tests:**
  - version 1 decodes and migrates
  - version 3 is rejected
  - the fingerprint is stable across renames and notes, and changes when a criterion is added, moved or switched to a different method, and when tasks change
  - stale task leaves are pruned
  - an invalid aggregation fails validation
  - a packet carries its packet info

### A2. Progress, completeness and combining rankings (`model.Compute`)

- **Per person:** `Results.People []PersonProgress{ID, Done, Total, Finished, Consistent, Structure, OutOfDate}`.
- **Measured values:** `Results.Values Progress`.
- **Completeness and method:** completeness as in spec §5; `Results.Method`.
- **Assigned work:** `assigned(p, ev)` gives the weigh groups and the score leaves.
- **The "combine rankings" method** (`aggregation: priorities`): a synthesis per person where their tasks cover a part, otherwise the combined answers; the weighted geometric mean of their scores, renormalized.
- **Tests:**
  - totals for Everything / weigh only / score only / custom leaves
  - "all" completeness from per-person sums
  - `outOfDate` after a structural change
  - per-person `consistent`
  - the "combine rankings" method on a hand-checked two-person case (two options, one criterion: person A 3:1, person B 1:1, equal weights → geometric mean of the [.75, .25] and [.5, .5] scores, renormalized)
  - the default method leaves scores unchanged

### A3. Merge preview, dropped lines and `returnedAt`

- **`model.PreviewMerge(owner, packet) (*MergePreview, error)`:** `People[{Name, Judgments, ReplacesExisting, PreviouslyReturnedAt}]`, `Dropped []string`, `OutOfDate`.
- **`MergeReport.Dropped`:** readable lines in place of the bare count.
- **`Merge`** sets `Invite.ReturnedAt` for the people merged.
- **Server:** `POST /api/projects/{id}/merge/preview`.
- **Tests:**
  - preview lists a dropped comparison about a deleted criterion, in words
  - `replacesExisting` is true only when the person has judgments
  - `outOfDate` when the copy's packet fingerprint differs
  - the preview doesn't change the project
  - merge sets `returnedAt`
  - the endpoint returns the preview and 400 for another decision

### A4. The People step (UI)

- **Rail:** route step `people`, between setup and weigh. The People section moves out of Set up.
- **`PeopleStep.svelte`:**
  - a row per person: name, weight, role chooser (Everything / Weigh criteria / Score options / Custom with a checklist), a progress bar from `results.people`, a consistency mark, status
  - the "how people are combined" choice
- **Invite:** records `invite{sentAt, structure}` and saves first, then downloads the copy. **Copy reminder** copies a short message. Out-of-date rows show **Send updated copy**.
- **Tests:**
  - **unit:** a role ↔ tasks mapping helper (`roleOf(tasks)`, `tasksFor(role, leaves)`), and a status helper (Not invited / Invited / Returned / Out of date)
  - **e2e:** add a person, set "Weigh criteria", check their progress total equals the number of criteria pairs, invite and check the status shows Invited, change structure and check it shows Out of date

### A5. Merging in the UI

- **`MergeDialog.svelte`:** shows the preview, with Merge / Replace / Keep current / Cancel; an Undo toast afterwards.
- **Ways in:** dropping a file on the workspace (with a "Drop to merge" overlay) or on a library row; Import file reads the file's ID and origin and, if it matches a decision in the library, offers "Merge into *X*" or "Import as a separate copy".
- **e2e:**
  - drop a returned file, see the preview, merge, and the status becomes Returned
  - Undo restores
  - Import of a returned file offers merge
  - the dropped lines appear for a deleted criterion

### A6. Received-copy mode and role filtering

- **When the project has `packet`:**
  - a banner with **Send your answers back** (downloads the export, then shows next steps)
  - Set up's structure controls disabled, with an explanation
  - the People step shows only that person, with no invite or merge controls
  - importing a copy opens the first assigned task
- **Weigh / Score:** groups and leaves not assigned to the person being judged as are greyed out ("Not assigned to Priya"), and the first assigned one is selected.
- **e2e:**
  - import a copy: the banner, read-only structure, first task opened
  - send back: the download is named for the person
  - role filtering in Weigh/Score

### A7. Verify and release

- Full suite.
- Browser review (desktop in both themes, phone).
- Independent review, then the fix pass.
- Merge, CI, tag v0.2.0, check the Homebrew update.
