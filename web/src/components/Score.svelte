<script lang="ts">
  import { altsContext, getRating, getValue, setDirection, setRating, setValue } from '../lib/edits';
  import { colorOf } from '../lib/format';
  import { leafProgress } from '../lib/progress';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { leaves, path } from '../lib/tree';
  import type { Direction } from '../lib/types';
  import PriorityBars from './charts/PriorityBars.svelte';
  import PairCompare from './PairCompare.svelte';
  import SubNav from './SubNav.svelte';

  let { session, sub }: { session: ProjectSession; sub: string } = $props();
  const p = $derived(session.project);
  const all = $derived(leaves(p));
  const leaf = $derived(all.find((c) => c.id === sub) ?? all[0]);
  const ev = $derived(session.scoringAs);
  $effect(() => session.watch(ev));
  const results = $derived(session.results[ev]);
  const priorities = $derived(results?.criteria.find((c) => c.id === leaf?.id)?.subtree);

  const methodLabel = { pairwise: 'In pairs', rating: 'Rated 1–10', value: 'Measured' };
  const entries = $derived(
    all.map((c) => {
      const prog = leafProgress(p, ev, c);
      const report = results?.matrices.find((m) => m.context === altsContext(c.id));
      return {
        key: c.id,
        label: path(p, c.id).join(' › '),
        href: href(p.id, 'score', c.id),
        done: prog.done,
        total: prog.total,
        flag: !!report && report.connected && !report.consistent,
        detail: methodLabel[c.method ?? 'pairwise'],
      };
    }),
  );
  const items = $derived(p.alternatives.map((a) => ({ id: a.id, name: a.name, color: colorOf(a.color) })));

  function rate(alt: string, v: number | undefined) {
    setRating(session.project, ev, leaf!.id, alt, v);
    session.changed();
  }
  function measure(alt: string, raw: string) {
    setValue(session.project, leaf!.id, alt, raw.trim() === '' ? undefined : Number(raw));
    session.changed();
  }
</script>

{#if !leaf || p.alternatives.length < 2}
  <div class="empty">
    <h1>Nothing to score yet</h1>
    <p class="muted">Scoring needs at least two options and one criterion. Add them in setup.</p>
    <a class="btn primary" href={href(p.id, 'setup')}>Go to setup</a>
  </div>
{:else}
  <div class="layout">
    <SubNav label="Criteria to score" {entries} current={leaf.id} />
    <section class="main">
      {#if leaf.method === 'rating'}
        <h1>Rate each option on {leaf.name}</h1>
        <p class="muted">1 is poor, 10 is excellent.</p>
        <ul class="rows">
          {#each p.alternatives as a (a.id)}
            {@const r = getRating(p, ev, leaf.id, a.id)}
            <li>
              <span class="who"><span class="dot" style:background={colorOf(a.color)}></span>{a.name}</span>
              <div class="seg" role="radiogroup" aria-label="Rating for {a.name}">
                {#each [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] as n}
                  <button role="radio" aria-checked={r === n} class:on={r === n} onclick={() => rate(a.id, r === n ? undefined : n)}>{n}</button>
                {/each}
              </div>
            </li>
          {/each}
        </ul>
      {:else if leaf.method === 'value'}
        <h1>Enter each option's {leaf.name.toLowerCase()}</h1>
        <div class="direction" role="radiogroup" aria-label="Which values are better">
          {#each [['higher', 'Higher is better'], ['lower', 'Lower is better']] as [d, text]}
            <button
              role="radio"
              aria-checked={(leaf.direction ?? 'higher') === d}
              class="btn quiet"
              onclick={() => {
                setDirection(session.project, leaf.id, d as Direction);
                session.changed();
              }}>{text}</button
            >
          {/each}
        </div>
        <ul class="rows">
          {#each p.alternatives as a (a.id)}
            {@const v = getValue(p, leaf.id, a.id)}
            <li>
              <label class="who" for="v-{a.id}"><span class="dot" style:background={colorOf(a.color)}></span>{a.name}</label>
              <span class="measure">
                <input
                  id="v-{a.id}"
                  class="field"
                  type="number"
                  step="any"
                  value={v ?? ''}
                  oninput={(e) => measure(a.id, e.currentTarget.value)}
                />
                {#if leaf.unit}<span class="muted small">{leaf.unit}</span>{/if}
              </span>
            </li>
          {/each}
        </ul>
        <p class="small muted">Measured values are shared by everyone; they aren't anyone's opinion.</p>
      {:else}
        <h1>Which is better on {leaf.name}, and by how much?</h1>
        {#key leaf.id}
          <PairCompare {session} context={altsContext(leaf.id)} {items} kind="alternatives" on={leaf.name} />
        {/key}
      {/if}

      {#if priorities && p.alternatives.length}
        <div class="result">
          <h3>What this gives</h3>
          <PriorityBars names={p.alternatives.map((a) => a.name)} colors={p.alternatives.map((a) => colorOf(a.color))} values={priorities} />
        </div>
      {/if}

      <label class="notes">
        <span class="small muted">Notes on {leaf.name}</span>
        <textarea class="field" rows="2" bind:value={leaf.notes} oninput={() => session.changed()} placeholder="Sources, assumptions, open questions"></textarea>
      </label>
    </section>
  </div>
{/if}

<style>
  .layout {
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    gap: 2rem;
    align-items: start;
  }
  .main {
    display: grid;
    gap: 1.25rem;
    max-width: 46rem;
  }
  .empty {
    display: grid;
    gap: 0.9rem;
    justify-items: start;
    max-width: 34rem;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  .rows li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--line);
  }
  .who {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 600;
  }
  .dot {
    width: 0.65rem;
    height: 0.65rem;
    border-radius: 50%;
  }
  .seg {
    display: grid;
    grid-template-columns: repeat(10, 2.1rem);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-s);
    overflow: hidden;
  }
  .seg button {
    border: 0;
    border-right: 1px solid var(--line);
    padding: 0.4rem 0;
    background: var(--paper);
    cursor: pointer;
    font-weight: 600;
    color: var(--ink-2);
  }
  .seg button:last-child {
    border-right: 0;
  }
  .seg button:hover {
    background: var(--brass-wash);
  }
  .seg button.on {
    background: var(--ink);
    color: var(--paper);
  }
  .direction {
    display: flex;
    gap: 0.25rem;
  }
  .direction [aria-checked='true'] {
    background: var(--paper);
    color: var(--ink);
    box-shadow: inset 0 -2px 0 var(--brass);
  }
  .measure {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
  }
  .measure input {
    width: 9rem;
    text-align: right;
  }
  .result {
    display: grid;
    gap: 0.6rem;
    padding-top: 1rem;
    border-top: 1px solid var(--line);
  }
  .notes {
    display: grid;
    gap: 0.3rem;
  }
  @media (max-width: 1000px) {
    .layout {
      grid-template-columns: 1fr;
    }
    .seg {
      grid-template-columns: repeat(10, 1.8rem);
    }
  }
</style>
