# Group judging, Phase C (v0.4.0): independence and discussion

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Each task is red, then green, then commit. Every task's tests are written first and watched failing.

**Goal:** people's answers stay independent until they've finished, and then disagreements become an agenda for discussion: one scale showing everyone's answer, a way to revise your own, and the rankings side by side.

**Spec:** `docs/superpowers/specs/2026-10-21-group-judging-design.md` §9 and §10. Builds on Phases A and B (v0.3.0).

## Global constraints

- **No schema change:** everything here is computed from version 2 data. Go stays 1.24, with no new dependencies.
- **"Show anyway" lasts for the session only:** it isn't saved in the project.
- **Copy:** sentence case and plain words. Every screen works at 390 px and in both themes.
- **Charts** follow the project's dataviz rules:
  - identity is never shown by colour alone: markers are labelled with names, or in a legend with direct labels
  - hover/focus shows values
  - the text uses ink colours, not series colours

## Review focus

1. **One person:** a decision with only one person shows none of this (no banner, nothing hidden).
2. **Hidden results:** results are hidden on Results, What if and the printable report alike, and don't leak through the library's ranking strip. The strip is owner-wide, so it stays.
3. **Revise my answer** must change only the judging person's answer, and land on the right pair in the right view.
4. **Long names:** the people scale must stay readable with long names and with up to 8 people on one pair. Markers that collide stack.
5. **Equal winners:** an agreement headline with ties ("Darren picks A and B equally") must not claim agreement wrongly.

## Tasks

### C1. "Now judging as" (Workspace bar)

- **When:** more than one person is set up.
- **Bar:** the "Judging as" picker becomes a banner-style chip in the bar, showing the person's name and their colour (series slot by position, through the person's initial in a dot). The label reads "Now judging as".
- **Weigh/Score:** the page heading area repeats it once: "Answering as Priya".
- **Tests:** an e2e check that the chip shows the name and that switching updates the compare screen's answers; an SSR check that it's absent for one person.

### C2. Results hidden until you finish

- **`lib/independence.ts`:** `hiddenFor(results, evaluatorId, people)` is true when there's more than one person and that person's `PersonProgress.finished` is false and their total is above 0.
- **Results, What if and Report:** when hidden, they show "Results are hidden until Priya finishes, so others' answers don't sway hers." with how many answers are left, a **Go to the next comparison** link (the first unfinished task, `firstTask`-style but for unanswered work), and **Show anyway**. Show anyway is a session flag (`session.revealed: Set<string>`).
- **Received copies:** these have only one person, so nothing is hidden.
- **Tests:**
  - **unit:** `hiddenFor` and the next-unfinished-task helper
  - **e2e:** with two people and Priya unfinished, Results is hidden for Priya; Show anyway reveals it; switching to a finished person shows results straight away

### C3. The disagreement view

- **`PeopleScale.svelte`:** one 9…1…9 scale per contested pair, with a labelled marker per person at their answer, the combined answer as a distinct brass marker, and markers that collide stacked vertically. Hovering or focusing a marker shows "Priya: Speed matters strongly more than Cost".
- **Results "Where people disagree most"** becomes a list of these, sorted by spread:
  - each with the pair's sentence for the combined answer
  - each with **Revise my answer**, which goes to that pair for the person being judged as: `href(id, 'weigh' | 'score', sub)` plus a pair hint in the URL (`?pair=a|b`). `PairCompare` opens on that pair in One pair mode
- **Tests:**
  - **unit:** a marker-position and stacking helper (`placeMarkers(values, scale) → {x, row}`)
  - **SSR:** a pair with three people renders three named markers and a combined marker
  - **e2e:** Revise my answer lands on the pair, shows the person's current answer, and changing it updates the view

### C4. Side-by-side rankings and agreement

- **Results (everyone combined):** a table with one row per option, the combined rank and score, and a column per person with their rank and score. The session watches every person's results.
- **Headline (`lib/agreement.ts`, `agreement(people, rankings)`):**
  - "All 3 people rank MacBook Pro 14 first"
  - "People disagree on the winner: Darren and Priya pick MacBook Pro 14; Sam picks ThinkPad X1"
  - ties: "Darren puts MacBook Pro 14 and ThinkPad X1 level"
  - people who haven't judged anything are left out and noted ("Sam hasn't answered yet")
- **Tests:**
  - **unit:** every headline shape, including ties and people with no answers
  - **e2e:** the hiring example shows the table and headline

### C5. Verify and release

- Full suite, browser review (both themes, phone), independent review, fix pass.
- README: a sentence or two on independence and discussion.
- Merge, CI, tag v0.4.0, check the Homebrew update.
