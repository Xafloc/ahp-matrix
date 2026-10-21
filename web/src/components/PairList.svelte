<script lang="ts">
  // Every pair at once, each editable in place: one row per pair, the first
  // item on the left and the second on the right, with the same 9…1…9
  // control as the balance. The side a judgment favours is set in bold. Rows
  // are flagged only when the consistency check finds contradictions.
  import { describe, type CompareKind } from '../lib/scale';
  import type { MatrixReport, PairSuggestion, Scale } from '../lib/types';
  import StrengthScale from './StrengthScale.svelte';
  import StrengthWords from './StrengthWords.svelte';
  import WhyDialog from './WhyDialog.svelte';

  interface Item {
    id: string;
    name: string;
    color?: string;
  }
  let {
    items,
    get,
    set,
    scale,
    kind,
    on = '',
    report,
    onfix,
  }: {
    items: Item[];
    get: (i: number, j: number) => number | undefined;
    set: (i: number, j: number, v: number | undefined) => void;
    scale: Scale;
    kind: CompareKind;
    on?: string;
    report: MatrixReport | undefined;
    onfix: (a: string, b: string, v: number) => void;
  } = $props();

  const pairs = $derived(items.flatMap((_, i) => items.slice(i + 1).map((__, k) => [i, i + 1 + k] as const)));
  let hovered = $state<{ key: string; v: number } | null>(null);
  let why = $state<PairSuggestion | null>(null);
  const raw = (i: number, j: number) =>
    report?.suggestions.find((s) => (s.a === items[i].id && s.b === items[j].id) || (s.a === items[j].id && s.b === items[i].id)) ?? null;

  // The suggestion for a pair, oriented first-over-second, when the answers contradict each other.
  function suggestion(i: number, j: number) {
    if (!report || report.consistent) return null;
    for (const s of report.suggestions) {
      if (s.a === items[i].id && s.b === items[j].id) return s.suggested;
      if (s.a === items[j].id && s.b === items[i].id) return 1 / s.suggested;
    }
    return null;
  }
  const say = (v: number, i: number, j: number, preview = false) => describe(v, scale, kind, items[i].name, items[j].name, on, preview);
</script>

<div class="list">
  <div class="head" aria-hidden="true">
    <span></span>
    <StrengthWords {scale} sparse />
    <span></span>
  </div>
  <ol>
    {#each pairs as [i, j] (`${items[i].id}|${items[j].id}`)}
      {@const v = get(i, j)}
      {@const key = `${i}|${j}`}
      {@const fix = suggestion(i, j)}
      {@const preview = hovered?.key === key && hovered.v !== v ? say(hovered.v, i, j, true) : ''}
      <li class="row" class:flag={fix !== null} class:open={v === undefined}>
        <span class="name left" class:wins={v !== undefined && v > 1}>
          {#if items[i].color}<span class="dot" style:background={items[i].color}></span>{/if}{items[i].name}
        </span>
        <div class="control">
          <StrengthScale
            compact
            value={v}
            {scale}
            label="Compare {items[i].name} with {items[j].name}"
            valuetext={v === undefined ? 'Not compared yet' : say(v, i, j)}
            describeAt={(x) => say(x, i, j)}
            onchange={(x) => set(i, j, x)}
            onhover={(x) => (hovered = x === null ? null : { key, v: x })}
          />
        </div>
        <span class="name right" class:wins={v !== undefined && v < 1}>
          {items[j].name}{#if items[j].color}<span class="dot" style:background={items[j].color}></span>{/if}
        </span>
        <p class="sentence" class:preview={!!preview} class:unset={!preview && v === undefined}>
          {preview || (v === undefined ? 'Not compared yet' : say(v, i, j))}
        </p>
        {#if fix !== null}
          <p class="fix">
            <span><strong>Suggested by your other answers:</strong> {say(fix, i, j)}</span>
            <button class="btn small" onclick={() => onfix(items[i].id, items[j].id, fix)}>Use that</button>
            <button class="btn quiet small why" onclick={() => (why = raw(i, j))}>Why?</button>
          </p>
        {/if}
      </li>
    {/each}
  </ol>
</div>

{#if why && report}
  {@const s = why}
  <WhyDialog open {items} {report} suggestion={s} {scale} {kind} {on} onuse={() => onfix(s.a, s.b, s.suggested)} onclose={() => (why = null)} />
{/if}

<style>
  .list {
    --cols: minmax(6rem, 1fr) minmax(17rem, 2.6fr) minmax(6rem, 1fr);
    display: grid;
  }
  .head {
    display: grid;
    grid-template-columns: var(--cols);
    column-gap: 1rem;
    padding: 0 0.75rem 0.45rem;
    position: sticky;
    top: 3.3rem; /* below the app bar */
    background: var(--fog);
    z-index: 1;
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  .row {
    display: grid;
    grid-template-columns: var(--cols);
    column-gap: 1rem;
    row-gap: 0.3rem;
    align-items: center;
    padding: 0.75rem;
    border-bottom: 1px solid var(--line);
    transition: background 150ms ease;
  }
  .row:hover,
  .row:focus-within {
    background: color-mix(in oklab, var(--paper) 55%, var(--fog));
  }
  .row.flag {
    background: color-mix(in oklab, var(--rust-wash) 70%, var(--fog));
    box-shadow: inset 3px 0 0 var(--rust);
  }
  .name {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    min-width: 0;
    color: var(--ink-2);
    font-weight: 500;
    overflow-wrap: anywhere;
  }
  .name.left {
    justify-content: flex-end;
    text-align: right;
  }
  .name.wins {
    color: var(--ink);
    font-weight: 700;
  }
  .dot {
    flex: none;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
  }
  .sentence,
  .fix {
    grid-column: 2;
    margin: 0;
    font-size: 0.84rem;
    color: var(--ink-2);
    text-align: center;
    max-width: none;
  }
  .sentence.unset {
    color: var(--ink-3);
  }
  .sentence.preview {
    color: var(--ink-3);
    font-style: italic;
  }
  .fix {
    grid-column: 1 / -1;
    display: flex;
    justify-content: center;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem 0.75rem;
    color: var(--ink);
  }
  .fix .btn {
    padding: 0.25rem 0.6rem;
  }
  .fix .why {
    text-decoration: underline;
    text-underline-offset: 3px;
  }
  @media (max-width: 640px) {
    .list {
      --cols: 1fr 1fr;
    }
    .head {
      position: static; /* the app bar wraps taller on phones; don't slide under it */
    }
    .head span {
      display: none;
    }
    .head :global(.fit),
    .control,
    .sentence,
    .fix {
      grid-column: 1 / -1;
    }
    .name.left {
      justify-content: flex-start;
      text-align: left;
    }
    .name.right {
      justify-content: flex-end;
      text-align: right;
      grid-row: 1;
      grid-column: 2;
    }
    .control {
      grid-row: 2;
    }
  }
</style>
