<script lang="ts">
  // Who judges the decision, what each person is asked to judge, how far
  // they've got, where their copy is, and how their answers are combined.
  import { exportUrl } from '../lib/api';
  import { addEvaluator, removeEvaluator, setEvaluatorWeight } from '../lib/edits';
  import { relativeTime } from '../lib/format';
  import { reminder, ROLES, roleOf, statusOf, tasksFor, type Role, type Status } from '../lib/people';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { leaves } from '../lib/tree';
  import { ALL_EVALUATORS, type Aggregation, type Evaluator } from '../lib/types';

  let { session, onmerge }: { session: ProjectSession; onmerge: (file: File) => void } = $props();
  let mergeInput = $state<HTMLInputElement | undefined>();
  const p = $derived(session.project);
  const everyone = $derived(session.results[ALL_EVALUATORS]);
  // Leaves people score on; measured values are entered once, by the owner.
  const judged = $derived(leaves(p).filter((c) => c.method !== 'value'));

  let newPerson = $state('');
  let notice = $state<{ id: string; text: string } | null>(null);
  let busy = $state('');

  const progressOf = (id: string) => everyone?.people?.find((x) => x.id === id);

  function edit(fn: () => void) {
    fn();
    session.changed();
  }

  function submitPerson(e: SubmitEvent) {
    e.preventDefault();
    if (!newPerson.trim()) return;
    edit(() => addEvaluator(session.project, newPerson));
    newPerson = '';
  }

  function setRole(e: Evaluator, role: Role) {
    edit(() => {
      const tasks = tasksFor(role, judged.map((c) => c.id), e.tasks);
      if (tasks) e.tasks = tasks;
      else delete e.tasks;
    });
  }

  function toggleWeigh(e: Evaluator, on: boolean) {
    if (e.tasks) edit(() => (e.tasks!.weigh = on));
  }

  function toggleLeaf(e: Evaluator, id: string, on: boolean) {
    const t = e.tasks;
    if (!t) return;
    edit(() => {
      const rest = (t.score ?? []).filter((x) => x !== id);
      t.score = on ? judged.map((c) => c.id).filter((x) => x === id || rest.includes(x)) : rest;
    });
  }

  // Records the invitation (with the structure the copy is made from), saves
  // it, then downloads the copy, so the copy and the record always agree.
  async function invite(e: Evaluator) {
    busy = e.id;
    notice = null;
    try {
      await session.flush();
      if (session.unsaved) {
        notice = { id: e.id, text: 'Fix the unsaved changes first; then invite.' };
        return;
      }
      await session.refreshResults([ALL_EVALUATORS]);
      const structure = progressOf(e.id)?.structure;
      const person = session.project.evaluators.find((x) => x.id === e.id);
      if (!structure || !person) return;
      person.invite = { sentAt: new Date().toISOString(), structure };
      session.changed();
      await session.flush();
      if (session.unsaved) {
        notice = { id: e.id, text: `The invitation couldn't be saved: ${session.error}` };
        return;
      }
      const a = document.createElement('a');
      a.href = exportUrl(p.id, 'ahp.json', e.id);
      a.download = '';
      a.click();
      notice = { id: e.id, text: `Send the downloaded file to ${e.name}. They open it with Import file in AHP Matrix.` };
    } finally {
      busy = '';
    }
  }

  async function copyReminder(e: Evaluator) {
    const prog = progressOf(e.id) ?? { done: 0, total: 0 };
    const text = reminder(p.name, e.name, prog, p.evaluators[0].name);
    try {
      await navigator.clipboard.writeText(text);
      notice = { id: e.id, text: 'Reminder copied. Paste it into an email or chat.' };
    } catch {
      notice = { id: e.id, text };
    }
  }

  function statusText(s: Status): string {
    switch (s.kind) {
      case 'you':
        return 'You';
      case 'not-invited':
        return 'Not invited';
      case 'invited':
        return `Invited ${relativeTime(s.at)}`;
      case 'returned':
        return `Returned ${relativeTime(s.at)}`;
      case 'out-of-date':
        return 'Out of date';
    }
  }

  const methods: { id: Aggregation; name: string; detail: string }[] = [
    {
      id: 'judgments',
      name: 'Combine answers',
      detail:
        "Each comparison is combined across people first, with a weighted geometric mean, then one ranking is worked out. The AHP standard: if one person says 3 and another 1/3, they cancel to equal. 1–10 ratings use a weighted average.",
    },
    {
      id: 'priorities',
      name: 'Combine rankings',
      detail:
        "Each person's answers make their own ranking, and the rankings are combined with a weighted geometric mean. Suits people who judge different parts, or who should count as separate voices. Criteria weights still show the combined answers.",
    },
  ];
  const method = $derived(p.aggregation ?? 'judgments');
</script>

<div class="people">
  <header>
    <h2>Who is judging</h2>
    <p class="muted">
      Everyone whose judgments count. Each person compares on their own, and the results combine them by weight. Ask someone to judge only part of
      the decision, like weighing the criteria or scoring the options, if that's where their expertise is.
    </p>
  </header>

  <ul class="list">
    {#each p.evaluators as e, i (e.id)}
      {@const prog = progressOf(e.id)}
      {@const status = statusOf(e, prog, i === 0)}
      {@const role = roleOf(e.tasks)}
      <li class="person">
        <div class="top">
          <input class="field inline name" aria-label="Person's name" bind:value={e.name} oninput={() => session.changed()} />
          <span class="status small" class:late={status.kind === 'out-of-date'} class:back={status.kind === 'returned'}>{statusText(status)}</span>
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
        </div>

        <div class="work">
          <label class="small muted field-label">
            Judges
            <select class="field" aria-label="What {e.name} judges" value={role} onchange={(ev) => setRole(e, ev.currentTarget.value as Role)}>
              {#each ROLES as r (r.id)}<option value={r.id}>{r.label}</option>{/each}
            </select>
          </label>
          <label class="small muted field-label">
            Weight
            <input
              class="field weight"
              type="number"
              min="0.1"
              step="0.1"
              value={e.weight}
              onchange={(ev) => edit(() => setEvaluatorWeight(session.project, e.id, ev.currentTarget.valueAsNumber))}
            />
          </label>
          <div class="progress small">
            {#if prog && prog.total > 0}
              <span class="bar" aria-hidden="true"><span style:width="{(prog.done / prog.total) * 100}%" class:full={prog.finished}></span></span>
              <span>{prog.done} of {prog.total} done</span>
              {#if prog.done > 1}
                <span class:good={prog.consistent} class:rust={!prog.consistent}>{prog.consistent ? 'Consistent' : 'Has contradictions'}</span>
              {/if}
            {:else if prog}
              <span class="muted">Nothing to judge yet</span>
            {/if}
          </div>
        </div>

        {#if role === 'custom' && e.tasks}
          <fieldset class="tasks small">
            <legend class="sr-only">What {e.name} judges</legend>
            <label><input type="checkbox" checked={e.tasks.weigh} onchange={(ev) => toggleWeigh(e, ev.currentTarget.checked)} /> Weigh the criteria</label>
            {#each judged as c (c.id)}
              <label>
                <input type="checkbox" checked={e.tasks.score?.includes(c.id) ?? false} onchange={(ev) => toggleLeaf(e, c.id, ev.currentTarget.checked)} />
                Score on {c.name}
              </label>
            {/each}
          </fieldset>
        {/if}

        {#if i > 0}
          <div class="actions">
            {#if status.kind === 'out-of-date'}
              <button class="btn small" disabled={busy === e.id} onclick={() => invite(e)}>Send updated copy</button>
            {:else}
              <button class="btn small" class:quiet={status.kind !== 'not-invited'} disabled={busy === e.id} onclick={() => invite(e)}>
                {status.kind === 'not-invited' ? 'Invite' : 'Invite again'}
              </button>
            {/if}
            {#if status.kind !== 'not-invited' && !prog?.finished}
              <button class="btn quiet small" onclick={() => copyReminder(e)}>Copy reminder</button>
            {/if}
          </div>
        {/if}
        {#if notice?.id === e.id}<p class="small notice" role="status">{notice.text}</p>{/if}
      </li>
    {/each}
  </ul>
  <form class="add" onsubmit={submitPerson}>
    <input class="field" placeholder="Add a person" aria-label="New person" bind:value={newPerson} />
    <button class="btn" type="submit" disabled={!newPerson.trim()}>Add</button>
  </form>

  {#if p.evaluators.length > 1}
    <section>
      <header>
        <h3>Collecting answers</h3>
        <p class="muted small">When someone sends their file back, merge it here. You'll see whose answers it holds before anything changes.</p>
      </header>
      <div class="remote">
        <button class="btn" onclick={() => mergeInput?.click()}>Merge returned answers</button>
        <span class="small muted">or drop the file anywhere on this page</span>
        <input
          bind:this={mergeInput}
          type="file"
          accept=".json,application/json"
          class="sr-only"
          tabindex="-1"
          aria-label="Returned decision file to merge"
          onchange={() => {
            const file = mergeInput?.files?.[0];
            if (mergeInput) mergeInput.value = '';
            if (file) onmerge(file);
          }}
        />
      </div>
    </section>
  {/if}

  <section>
    <header>
      <h3>How people are combined</h3>
    </header>
    <div class="methods" role="radiogroup" aria-label="How people are combined">
      {#each methods as m (m.id)}
        <button
          class="method"
          role="radio"
          aria-checked={method === m.id}
          onclick={() =>
            edit(() => (session.project.aggregation = m.id))}
        >
          <strong>{m.name}</strong>
          <span class="small muted">{m.detail}</span>
        </button>
      {/each}
    </div>
  </section>

  {#if p.alternatives.length >= 2 && p.criteria.length >= 1}
    <a class="btn primary next" href={href(p.id, 'weigh')}>Next: weigh the criteria</a>
  {/if}
</div>

<style>
  .people {
    display: grid;
    gap: 2rem;
    max-width: 52rem;
  }
  section,
  header {
    display: grid;
    gap: 0.4rem;
  }
  h3 {
    margin: 0;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    border-top: 1px solid var(--line);
  }
  .person {
    display: grid;
    gap: 0.5rem;
    padding: 0.75rem 0;
    border-bottom: 1px solid var(--line);
  }
  .top {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .name {
    flex: 1 1 10rem;
    min-width: 0;
    font-weight: 600;
  }
  .status {
    flex: none;
    padding: 0.15rem 0.55rem;
    border-radius: 999px;
    border: 1px solid var(--line);
    color: var(--ink-2);
    white-space: nowrap;
  }
  .status.back {
    border-color: color-mix(in oklab, var(--good) 45%, var(--line));
    color: var(--good);
  }
  .status.late {
    border-color: var(--rust);
    background: var(--rust-wash);
    color: var(--rust);
    font-weight: 600;
  }
  .work {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem 1.25rem;
  }
  .field-label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }
  .field-label select {
    width: auto;
  }
  .weight {
    width: 4.5rem;
  }
  .progress {
    display: inline-flex;
    align-items: center;
    gap: 0.6rem;
    color: var(--ink-2);
    flex-wrap: wrap;
  }
  .bar {
    width: 7rem;
    height: 0.4rem;
    border-radius: 999px;
    background: var(--line);
    overflow: hidden;
  }
  .bar span {
    display: block;
    height: 100%;
    background: var(--brass);
    border-radius: 999px;
  }
  .bar span.full {
    background: var(--good);
  }
  .good {
    color: var(--good);
  }
  .rust {
    color: var(--rust);
  }
  .tasks {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem 1rem;
    margin: 0;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-s);
    background: var(--paper);
  }
  .tasks label {
    display: inline-flex;
    gap: 0.35rem;
    align-items: center;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .notice {
    margin: 0;
    color: var(--ink-2);
  }
  .add {
    display: flex;
    gap: 0.5rem;
    max-width: 30rem;
    margin-top: -1.25rem;
  }
  .remote {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem 0.9rem;
  }
  .methods {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
    gap: 0.75rem;
  }
  .method {
    display: grid;
    gap: 0.3rem;
    align-content: start;
    text-align: left;
    padding: 0.8rem 1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    background: var(--paper);
    cursor: pointer;
    color: inherit;
    font: inherit;
  }
  .method[aria-checked='true'] {
    border-color: var(--brass);
    box-shadow: inset 0 0 0 1px var(--brass);
    background: var(--brass-wash);
  }
  .next {
    justify-self: start;
  }
</style>
