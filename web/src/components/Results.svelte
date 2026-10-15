<script lang="ts">
  import { colorOf, percent } from '../lib/format';
  import { href } from '../lib/router.svelte';
  import { fraction } from '../lib/scale';
  import type { ProjectSession } from '../lib/session.svelte';
  import { path } from '../lib/tree';
  import { ALL_EVALUATORS } from '../lib/types';
  import Contributions from './charts/Contributions.svelte';
  import PriorityBars from './charts/PriorityBars.svelte';
  import WeightTree from './charts/WeightTree.svelte';

  let { session }: { session: ProjectSession } = $props();
  const p = $derived(session.project);
  let viewing = $state(ALL_EVALUATORS);
  $effect(() => session.watch(viewing));
  const r = $derived(session.results[viewing]);

  const ranked = $derived(r ? [...r.scores].sort((a, b) => b.score - a.score) : []);
  const altName = (id: string) => p.alternatives.find((a) => a.id === id)?.name ?? id;
  const crit = (id: string) => (id ? path(p, id).join(' › ') : '');
  const itemName = (id: string) => p.alternatives.find((a) => a.id === id)?.name ?? p.criteria.find((c) => c.id === id)?.name ?? id;
  const evName = (id: string) => p.evaluators.find((e) => e.id === id)?.name ?? id;

  // Where to go to review a matrix: weigh/<group> or score/<leaf>.
  function reviewHref(context: string) {
    const [kind, id] = context.split(':');
    return kind === 'alts' ? href(p.id, 'score', id) : href(p.id, 'weigh', id);
  }
  function matrixName(context: string) {
    const [kind, id] = context.split(':');
    if (kind === 'alts') return `Options on ${crit(id)}`;
    return id === 'root' ? 'Top-level criteria' : `Criteria within ${crit(id)}`;
  }

  // Only differences of opinion worth discussing: roughly a factor of two apart or more.
  const disagreements = $derived((r?.disagreements ?? []).filter((d) => d.spread >= 1.4).slice(0, 6));
  const lead = $derived(ranked.length >= 2 ? ranked[1].score > 0 ? (ranked[0].score - ranked[1].score) / ranked[0].score : 1 : 0);
</script>

<div class="results">
  {#if p.evaluators.length > 1}
    <div class="who" role="tablist" aria-label="Whose judgments">
      <button role="tab" class="btn quiet" aria-selected={viewing === ALL_EVALUATORS} onclick={() => (viewing = ALL_EVALUATORS)}>Everyone combined</button>
      {#each p.evaluators as e (e.id)}
        <button role="tab" class="btn quiet" aria-selected={viewing === e.id} onclick={() => (viewing = e.id)}>{e.name}</button>
      {/each}
    </div>
  {/if}

  {#if !r}
    <p class="muted">Working out the results…</p>
  {:else if p.alternatives.length < 2 || p.criteria.length === 0}
    <div class="empty">
      <h1>No results yet</h1>
      <p class="muted">Add at least two options and one criterion, then compare them.</p>
      <a class="btn primary" href={href(p.id, 'setup')}>Go to setup</a>
    </div>
  {:else}
    <section class="headline">
      <h1>
        {altName(ranked[0].id)} comes out ahead{ranked.length > 1 ? `, ${lead < 0.06 ? 'only just' : lead < 0.3 ? 'clearly' : 'by a wide margin'}` : ''}.
      </h1>
      <p class="muted">
        {percent(ranked[0].score)} of the overall weight, against {percent(ranked[1].score)} for {altName(ranked[1].id)}.
        {#if r.completeness < 1}
          These results are provisional: {percent(r.completeness, 0)} of the judgments are in.
        {/if}
      </p>
    </section>

    <PriorityBars
      big
      sorted
      names={p.alternatives.map((a) => a.name)}
      colors={p.alternatives.map((a) => colorOf(a.color))}
      values={r.scores.map((s) => s.score)}
    />

    {#if r.issues.length}
      <section class="issues">
        <h2>Worth fixing first</h2>
        <ul>
          {#each r.issues as i}
            <li>
              {#if i.criterionId}<strong>{crit(i.criterionId)}:</strong>{/if}
              {i.message}
              {#if i.context}<a href={reviewHref(i.context)}>Open</a>{:else if i.criterionId}<a href={href(p.id, 'score', i.criterionId)}>Open</a>{/if}
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    <section>
      <h2>Where each score comes from</h2>
      <p class="muted small">Each cell is a criterion's weight times how well the option does on it. The cells in a row add up to its total.</p>
      <Contributions project={p} results={r} />
    </section>

    <section>
      <h2>How much each criterion counts</h2>
      <WeightTree project={p} results={r} />
    </section>

    <section>
      <h2>Consistency of the comparisons</h2>
      <table class="matrices">
        <tbody>
          {#each r.matrices as m (m.context)}
            <tr>
              <th scope="row">{matrixName(m.context)}</th>
              <td>
                {#if !m.connected}<span class="muted">Not enough comparisons yet</span>
                {:else if m.items.length < 3}<span class="muted">Two items: always consistent</span>
                {:else}
                  <span class:bad={!m.consistent} class:ok={m.consistent}>{m.consistent ? 'Consistent' : 'Contradictions'}</span>
                  <span class="muted small">ratio {m.cr.toFixed(2)} of {m.threshold.toFixed(2)}{m.complete ? '' : ', provisional'}</span>
                {/if}
              </td>
              <td><a href={reviewHref(m.context)}>Review</a></td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>

    {#if disagreements.length}
      <section>
        <h2>Where people disagree most</h2>
        <p class="muted small">Each number is how strongly the first is preferred over the second: 1 is equal, 9 is extreme, 1/9 is the reverse.</p>
        <ul class="disagree">
          {#each disagreements as d}
            <li>
              <span class="pair">{itemName(d.a)} vs {itemName(d.b)} <span class="muted small">({matrixName(d.context)})</span></span>
              <span class="small">
                {#each Object.entries(d.values) as [ev, v], k}{k ? ', ' : ''}{evName(ev)} said {fraction(v)}{/each}
              </span>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    <div class="foot no-print">
      <a class="btn" href={href(p.id, 'sensitivity')}>Test how robust this is</a>
      <a class="btn quiet" href={href(p.id, 'report')}>Printable report</a>
    </div>
  {/if}
</div>

<style>
  .results {
    display: grid;
    gap: 2.25rem;
    max-width: 58rem;
  }
  section {
    display: grid;
    gap: 0.75rem;
  }
  .headline h1 {
    font-size: clamp(1.9rem, 4vw, 2.8rem);
    max-width: 22ch;
  }
  .headline {
    gap: 0.5rem;
  }
  .who {
    display: flex;
    gap: 0.25rem;
    flex-wrap: wrap;
  }
  .who [aria-selected='true'] {
    background: var(--paper);
    color: var(--ink);
    box-shadow: inset 0 -2px 0 var(--brass);
  }
  .empty {
    display: grid;
    gap: 0.9rem;
    justify-items: start;
  }
  .issues {
    padding: 1rem 1.2rem;
    background: var(--rust-wash);
    border-radius: var(--radius-m);
  }
  .issues ul {
    margin: 0;
    padding-left: 1.1rem;
    display: grid;
    gap: 0.3rem;
  }
  .matrices {
    border-collapse: collapse;
    width: 100%;
  }
  .matrices th,
  .matrices td {
    text-align: left;
    padding: 0.45rem 0.5rem;
    border-bottom: 1px solid var(--line);
    font-weight: 400;
  }
  .matrices th {
    font-weight: 550;
  }
  .ok {
    color: var(--good);
    font-weight: 600;
  }
  .bad {
    color: var(--rust);
    font-weight: 600;
  }
  .disagree {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.4rem;
  }
  .disagree li {
    display: grid;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--line);
  }
  .pair {
    font-weight: 600;
  }
  .foot {
    display: flex;
    gap: 0.5rem;
  }
</style>
