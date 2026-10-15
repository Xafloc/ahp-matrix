<script lang="ts">
  // A one-page summary for printing or saving as PDF from the browser.
  import { colorOf } from '../lib/format';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { flatten } from '../lib/tree';
  import { ALL_EVALUATORS } from '../lib/types';
  import Contributions from './charts/Contributions.svelte';
  import PriorityBars from './charts/PriorityBars.svelte';
  import WeightTree from './charts/WeightTree.svelte';

  let { session }: { session: ProjectSession } = $props();
  const p = $derived(session.project);
  const r = $derived(session.results[ALL_EVALUATORS]);
  const notes = $derived(flatten(p).filter((f) => f.criterion.notes.trim()));
  const today = new Date().toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' });
</script>

<div class="bar no-print">
  <a class="btn quiet" href={href(p.id, 'results')}>Back to results</a>
  <button class="btn primary" onclick={() => window.print()}>Print or save as PDF</button>
</div>

<article class="report">
  <header>
    <h1>{p.name}</h1>
    {#if p.description}<p>{p.description}</p>{/if}
    <p class="muted small">
      {p.alternatives.length} options, {p.criteria.length} criteria, judged by {p.evaluators.map((e) => e.name).join(', ')}. Prepared {today}.
    </p>
  </header>

  {#if r}
    <section>
      <h2>Ranking</h2>
      <PriorityBars sorted names={p.alternatives.map((a) => a.name)} colors={p.alternatives.map((a) => colorOf(a.color))} values={r.scores.map((s) => s.score)} />
      {#if r.completeness < 1}<p class="small">Provisional: {Math.round(r.completeness * 100)}% of the judgments were made.</p>{/if}
      {#if !r.consistent}<p class="small">Some comparisons contradict each other; see the consistency review in the app.</p>{/if}
    </section>
    <section>
      <h2>Criteria weights</h2>
      <WeightTree project={p} results={r} />
    </section>
    <section>
      <h2>Score breakdown</h2>
      <Contributions project={p} results={r} />
    </section>
  {/if}

  {#if notes.length}
    <section>
      <h2>Notes</h2>
      <dl>
        {#each notes as { criterion: c } (c.id)}
          <dt>{c.name}</dt>
          <dd>{c.notes}</dd>
        {/each}
      </dl>
    </section>
  {/if}
</article>

<style>
  .bar {
    display: flex;
    justify-content: space-between;
    padding: 0.75rem 1.5rem;
  }
  .report {
    max-width: 50rem;
    margin: 0 auto 4rem;
    padding: 2.5rem 3rem;
    background: var(--paper);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    display: grid;
    gap: 2rem;
  }
  header {
    display: grid;
    gap: 0.5rem;
  }
  section {
    display: grid;
    gap: 0.75rem;
    break-inside: avoid;
  }
  dt {
    font-weight: 650;
  }
  dd {
    margin: 0 0 0.75rem;
    white-space: pre-wrap;
  }
  @media print {
    .report {
      border: 0;
      padding: 0;
      max-width: none;
    }
  }
</style>
