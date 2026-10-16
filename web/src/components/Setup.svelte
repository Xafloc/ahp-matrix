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
        mergeConflict = []; // the question no longer applies
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
