<script lang="ts">
  // Compares a set of items two at a time, for the evaluator the session is
  // judging as: one pair on the balance (Focus) or all pairs at once (Grid).
  import { getJudgment, setJudgment } from '../lib/edits';
  import { focusOrder, type Pair } from '../lib/pairs';
  import { pairProgress } from '../lib/progress';
  import type { CompareKind } from '../lib/scale';
  import type { CompareSession } from '../lib/local.svelte';
  import Balance from './Balance.svelte';
  import ConsistencyMeter from './ConsistencyMeter.svelte';
  import PairList from './PairList.svelte';

  export interface Item {
    id: string;
    name: string;
    color?: string;
  }

  let {
    session,
    context,
    items,
    kind,
    on = '',
    start = '',
  }: { session: CompareSession; context: string; items: Item[]; kind: CompareKind; on?: string; start?: string } = $props();

  const p = $derived(session.project);
  const ev = $derived(session.scoringAs);
  $effect(() => session.watch(ev));

  const report = $derived(session.results[ev]?.matrices.find((m) => m.context === context));
  const ids = $derived(items.map((i) => i.id));
  const progress = $derived(pairProgress(p, ev, context, ids));

  let mode = $state<'focus' | 'grid'>('focus');
  // The order is fixed when the view opens (re-sorting while answering would be disorienting).
  let order = $state<Pair[]>([]);
  let at = $state(0);
  let orderKey = '';

  $effect(() => {
    const key = `${context}|${ids.join(',')}|${ev}`;
    if (key === orderKey) return;
    orderKey = key;
    order = focusOrder(items.length, report?.weights);
    // start ("a|b") opens a particular pair, say from Revise my answer.
    const [sa, sb] = start.split('|').map((id) => ids.indexOf(id));
    const asked = order.findIndex(([i, j]) => (i === sa && j === sb) || (i === sb && j === sa));
    const firstOpen = order.findIndex(([i, j]) => value(i, j) === undefined);
    at = asked >= 0 ? asked : firstOpen < 0 ? 0 : firstOpen;
    if (asked >= 0) mode = 'focus';
  });

  const value = (i: number, j: number) => getJudgment(p, ev, context, ids[i], ids[j]);
  const current = $derived(order[at]);

  function set(i: number, j: number, v: number | undefined) {
    setJudgment(session.project, ev, context, ids[i], ids[j], v);
    session.changed();
  }

  function next() {
    const after = order.findIndex(([i, j], k) => k > at && value(i, j) === undefined);
    at = after >= 0 ? after : Math.min(at + 1, order.length - 1);
  }

  function onKey(e: KeyboardEvent) {
    // Shortcuts apply on the balance itself (or with nothing focused); a
    // focused link or button keeps its own Enter.
    const t = e.target as HTMLElement;
    if (mode !== 'focus' || (t !== document.body && !t.closest('[role=slider]'))) return;
    if (e.key === 'Enter') {
      e.preventDefault();
      next();
    } else if (e.key === 's' || e.key === 'S') {
      at = Math.min(at + 1, order.length - 1);
    }
  }

  function fix(a: string, b: string, v: number) {
    setJudgment(session.project, ev, context, a, b, v);
    session.changed();
  }
</script>

<svelte:window onkeydown={onKey} />

{#if items.length < 2}
  <p class="muted">Add at least two to compare.</p>
{:else}
  <div class="compare">
    <div class="head">
      <span class="small muted">{progress.done} of {progress.total} pairs compared</span>
      <div class="modes" role="tablist" aria-label="View">
        <button role="tab" aria-selected={mode === 'focus'} class="btn quiet" onclick={() => (mode = 'focus')}>One pair</button>
        <button role="tab" aria-selected={mode === 'grid'} class="btn quiet" onclick={() => (mode = 'grid')}>All pairs</button>
      </div>
    </div>

    {#if mode === 'focus' && current}
      {@const [i, j] = current}
      <Balance
        a={items[i]}
        b={items[j]}
        value={value(i, j)}
        scale={p.scale}
        {kind}
        {on}
        onchange={(v) => set(i, j, v)}
      />
      <div class="nav">
        <button class="btn quiet" disabled={at === 0} onclick={() => (at -= 1)}>Previous</button>
        <span class="small muted">Pair {at + 1} of {order.length}</span>
        {#if progress.done === progress.total}
          <span class="small done">All compared</span>
        {/if}
        <button class="btn primary" disabled={at === order.length - 1 && progress.done === progress.total} onclick={next}>
          {value(i, j) === undefined ? 'Skip' : 'Next'}
        </button>
      </div>
      <p class="hint small muted">Use ← and → to tip the balance, Enter for the next pair.</p>
    {:else}
      <PairList {items} get={value} set={set} scale={p.scale} {kind} {on} {report} onfix={fix} />
    {/if}

    <ConsistencyMeter {report} {items} {kind} scale={p.scale} onfix={fix} fixes={mode === 'focus'} />
  </div>
{/if}

<style>
  .compare {
    display: grid;
    gap: 1.25rem;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .modes {
    display: flex;
    gap: 0.25rem;
  }
  .modes [aria-selected='true'] {
    background: var(--paper);
    color: var(--ink);
    box-shadow: inset 0 -2px 0 var(--brass);
  }
  .nav {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
  }
  .done {
    color: var(--good);
    font-weight: 600;
  }
  .hint {
    text-align: center;
    max-width: none;
  }
</style>
