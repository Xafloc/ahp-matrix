<script lang="ts">
    import { addAlternative, changeScale, removeAlternative } from '../lib/edits';
  import { colorOf } from '../lib/format';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import type { Scale } from '../lib/types';
  import CriteriaTree from './CriteriaTree.svelte';

  let { session }: { session: ProjectSession } = $props();
  const p = $derived(session.project);

  let newAlt = $state('');
  let pendingScale = $state<Scale | null>(null);
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
    <a class="btn primary next" href={href(p.id, p.evaluators.length > 1 ? 'people' : 'weigh')}>
      {p.evaluators.length > 1 ? 'Next: check who is judging' : 'Next: weigh the criteria'}
    </a>
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
  .add {
    display: flex;
    gap: 0.5rem;
    max-width: 30rem;
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
