# Group judging: collecting and combining several people's judgments

**Date:** 2026-10-21
**Status:** Approved ("Build it all", after the review in conversation)
**Builds on:** `2026-10-15-ahp-matrix-design.md` §4–5 and §9

## 1. Intent

Several people should be able to judge one decision, easily and correctly.
- The owner can see who still has work to do.
- People who don't have AHP Matrix can still take part.
- Returned answers come back in one obvious step, never silently lost.
- Judgments stay independent until everyone is done; then disagreements become an agenda for discussion.

**Success looks like:**
- An owner invites a colleague in under a minute.
- The colleague answers without installing anything.
- The owner merges the returned file by dropping it in, and the People step shows that person as done.

## 2. Problems with v0.1 (from the review)

1. **Installing is required** to answer at all.
2. **A received copy has no guidance:** it opens on an empty Results page, with "Send their copy" and "Merge" buttons that make no sense to the recipient.
3. **Silent discards:**
   - the recipient's structural edits are thrown away on merge
   - judgments about deleted items are only counted
   - nobody is told when a copy goes out of date
4. **Import file makes a duplicate** where the person meant to merge.
5. **No overview of who's done.** "Everyone combined" completeness counts a pair as answered if anyone answered it.
6. **Everyone must judge everything.** There's no way to say "only weigh the criteria" or "only score Security".
7. **Answers aren't independent:** others' answers and the combined results are visible while you judge.

## 3. Phases (each one a release)

- **A (v0.2.0), collect:**
  - schema version 2 (roles, invitations, received-copy info, combining method)
  - the People step
  - progress, roles and per-person completeness
  - invitations as `.ahp.json`
  - received-copy mode
  - merging with a preview, by drop or import, with undo
  - "out of date" warnings
  - comparison screens filtered by role
  - the "combine rankings" method
- **B (v0.3.0), no install:**
  - a single self-contained `.html` judging page per invitation
  - a TypeScript port of the consistency math, checked against the golden files
  - a ready-to-send invitation message
- **C (v0.4.0), independence and discussion:**
  - results hidden from a person until they finish (with "Show anyway")
  - a "Now judging as" banner
  - a disagreement view built for discussion, with each person's answers on one scale and "Revise my answer"
  - side-by-side rankings and an agreement summary

## 4. Data model (schema version 2)

Bumping the version means older builds refuse new files ("upgrade AHP Matrix") instead of silently dropping the new fields. Migrating from version 1 needs no data changes.

```jsonc
"evaluators": [{
  "id", "name", "weight",
  "tasks": {                    // omitted means everything
    "weigh": true,              // compares the criteria in every sibling group
    "scoreAll": false,          // scores the options on every leaf...
    "score": ["c4", "c7"]       // ...or only on these leaves
  },
  "invite": {                   // set when a copy was sent; omitted before then
    "sentAt": "…",
    "structure": "a1b2…",       // fingerprint of what the copy contained
    "returnedAt": "…"           // set by a merge
  }
}],
"aggregation": "judgments",     // "judgments" (combine answers, the default) | "priorities" (combine rankings)
"packet": {                     // only in a copy sent to someone
  "for": "e2", "from": "Darren", "sentAt": "…", "structure": "a1b2…"
}
```

**The structure fingerprint** is a short hash of the options' IDs, the scale, the criteria tree (IDs, parent IDs, methods and directions), and that person's tasks. It doesn't cover names or notes: renaming something doesn't invalidate a copy.

**A person's assigned work:**
- **Weighing:** every comparison in every sibling group of two or more criteria.
- **Scoring:** for each assigned leaf, the option pairs (pairwise), one rating per option (rating), or nothing for measured values, which are shared facts the owner enters.

## 5. Computing (Go, `model.Compute`)

**`Results.people[]`**, one entry per person:
- id
- `done` and `total`, counting only the work assigned to them
- `finished` (`done == total` and `total > 0`)
- `consistent` (all of their own comparison sets are within their thresholds)
- `structure` (their current fingerprint)
- `outOfDate` (they have an invite whose fingerprint no longer matches)

**`Results.values`:** progress on measured values, `{done, total}`.

**Completeness:**
- for one person: their own `done/total`
- for "all": the sum of everyone's `done` over the sum of their `total`, so it reflects every person rather than "anyone answered"
- measured values are counted once in both

**The "combine rankings" method** (`aggregation: "priorities"`):
- Each person who has judged something gets their own synthesis. Where their role doesn't cover part of the tree, it uses the group's combined answers for that part.
- Their overall scores are combined with a weighted geometric mean of person weights and renormalized.
- Only `scores` change. Criteria weights, the score breakdown and what-if keep describing the combined answers, and the UI labels them that way.
- `Results.method` reports which method was used.

## 6. Merging

**`POST /api/projects/{id}/merge/preview`** (multipart) returns, without saving:
- `people`: for each person, name, `judgments` (count), `replacesExisting` (bool) and `previouslyReturnedAt`
- `dropped`: one readable line per judgment that can't apply, e.g. "Comparison of Warranty with Price: Warranty is no longer in this decision"
- `outOfDate` (the copy was made from an older structure)
- the decision's name and ID

**`POST …/merge`** is unchanged, except that it now also sets `invite.returnedAt` for the people merged and fills `MergeReport.dropped` with the same readable lines.

**Undo:** the UI keeps the project as it was before the merge, and **Undo** saves that copy back.

## 7. UI

**The People step**, between Set up and Weigh. The rail becomes Set up, People, Weigh criteria, Score options, Results, What if. The People section moves here from Set up.
- **A table, one row per person:**
  - name and weight
  - a role chooser: Everything / Weigh criteria / Score options / Custom (Weigh, plus a checklist of leaves)
  - a progress bar (`done/total`) and a consistency mark
  - status: You, Not invited, Invited on date, Returned on date, or **Out of date**
  - actions: **Invite** (Phase A downloads the `.ahp.json` copy; Phase B adds the web page and message), **Copy reminder**, and **Send updated copy** when out of date
- **Collecting answers:** a **Merge returned answers** button, a drop hint, and the "how people are combined" choice with a one-line explanation of each method.

**Merging:**
- **Ways in:** dropping a file anywhere in the workspace or on a library row, the merge button, or Import file when the file's ID (or origin) matches a decision you already have.
- **Preview dialog:** "Merge Priya's answers into *Laptops*?", with each person's count, replace or new, and the dropped lines, plus a warning if the copy is out of date.
- **Actions:** **Merge** / **Cancel**. If the person already has answers, the actions read **Replace their answers** / **Keep current**.
- **After merging:** a toast with **Undo**.

**Received-copy mode** (the project has `packet`):
- A banner: "Darren asked you to judge this decision. You're judging as Priya." with **Send your answers back**, which downloads the file and says to email it to Darren.
- Structure is read-only: options, criteria and scale controls are disabled, with a line explaining why.
- Importing a copy opens on the first assigned task, not Results.

**Role filtering:** Weigh and Score list only the groups and leaves assigned to the person being judged as. Everything else is listed but greyed out: "Not assigned to Priya".

## 8. Phase B: the no-install judging page

- **The build:** a second Vite entry produces `web/dist/judge.html`, a single file with its scripts, styles and Latin font subsets inlined. Go embeds it as before.
- **`GET /api/projects/{id}/invite.html?evaluator=<id>`** copies the page and injects the person's copy, escaped as JSON in a `<script type="application/json" id="ahp-packet">` element.
- **The page works with no server:**
  - a welcome screen (who asked, the decision, roughly how long, the steps)
  - the assigned tasks, using the same balance and pair-list components
  - a consistency check with **Why?**, using the TypeScript port of the math, checked against the golden files
  - **Finish and send back**, which downloads `<decision> - <person>'s answers.ahp.json` and shows what to do next
  - progress saved in the browser's local storage, where available
- **The invitation dialog** in the app gives both files (web page, app copy) and a ready-to-paste message that adapts to the role and the deadline field.

## 9. Phase C: independence and discussion

- **"Now judging as" banner:** when more than one person is set up, it sits in the bar with the person's name and colour.
- **Hidden results:** if that person hasn't finished their tasks, Results shows "Results are hidden until Priya finishes, so others' answers don't sway hers" with **Show anyway** (this session only).
- **The disagreement view:** each contested pair shows every person's answer as a marker on one 9…1…9 scale, plus the combined answer. **Revise my answer** opens that pair for the person being judged as.
- **Side-by-side rankings:** a table of each person's ranking against the combined one, with a headline such as "All 3 people rank MacBook Pro 14 first", or "People disagree on the winner: Darren and Priya pick…".

## 10. Testing

- **Go:**
  - fingerprints are stable, and change only with structural edits
  - per-person totals for every role shape
  - completeness for "all"
  - the "combine rankings" method on a hand-computed two-person case
  - the merge preview's dropped lines and its `outOfDate` flag
  - `returnedAt` is set
  - the version 1 → 2 migration
- **TypeScript:** the consistency port against the golden files (CR, weights, suggestions).
- **End-to-end:**
  - the invite → judge → return → merge round trip, through both the app copy and the HTML page
  - drop-to-merge
  - Import recognizing a returned file
  - Undo
  - out-of-date → updated copy
  - role filtering
  - hidden results and **Show anyway**
  - phone layout for the People step and the judging page
