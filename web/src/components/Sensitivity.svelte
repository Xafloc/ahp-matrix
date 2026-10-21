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
      {#if r.method === 'priorities'}
        <p class="small muted">
          What if works from everyone's answers combined, so its ranking can differ from Results, where each person's own ranking is combined.
        </p>
      {/if}

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
