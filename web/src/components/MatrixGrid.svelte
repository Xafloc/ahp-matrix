<script lang="ts">
  // Every pair at once: row item compared with column item. Cells lean brass
  // when the row is preferred and slate when the column is; contradictions
  // flagged by the consistency check are outlined.
  import { fraction } from '../lib/scale';
  import type { PairSuggestion } from '../lib/types';

  let {
    items,
    get,
    suggestions,
    onpick,
  }: {
    items: { id: string; name: string; color?: string }[];
    get: (i: number, j: number) => number | undefined;
    suggestions: PairSuggestion[];
    onpick: (i: number, j: number) => void;
  } = $props();

  const flagged = (i: number, j: number) =>
    suggestions.some((s) => (s.a === items[i].id && s.b === items[j].id) || (s.a === items[j].id && s.b === items[i].id));

  // How far a judgment leans: 0 (equal) to 1 (extreme), toward the row (+) or column (−).
  const lean = (v: number) => Math.max(-1, Math.min(1, Math.log(v) / Math.log(9)));
  const shade = (v: number | undefined) => {
    if (v === undefined) return 'var(--paper)';
    const l = lean(v);
    return l >= 0
      ? `color-mix(in oklab, var(--paper), var(--brass) ${Math.round(l * 55)}%)`
      : `color-mix(in oklab, var(--paper), var(--ink) ${Math.round(-l * 35)}%)`;
  };
</script>

<div class="wrap">
  <table>
    <thead>
      <tr>
        <th></th>
        {#each items as c}<th scope="col"><span class="col">{c.name}</span></th>{/each}
      </tr>
    </thead>
    <tbody>
      {#each items as r, i}
        <tr>
          <th scope="row"><span class="dot" style:background={r.color ?? 'var(--ink-3)'}></span>{r.name}</th>
          {#each items as c, j}
            {#if i === j}
              <td class="self"></td>
            {:else}
              {@const v = get(i, j)}
              <td>
                <button
                  class="cell"
                  class:empty={v === undefined}
                  class:flag={flagged(i, j)}
                  style:background={shade(v)}
                  aria-label="{r.name} compared with {c.name}: {v === undefined ? 'not compared' : fraction(v)}"
                  onclick={() => onpick(i, j)}
                >
                  {v === undefined ? '·' : fraction(v)}
                </button>
              </td>
            {/if}
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>
<p class="small muted">Read across: how the row compares with the column. Click a cell to change it.</p>

<style>
  .wrap {
    overflow-x: auto;
  }
  table {
    border-collapse: separate;
    border-spacing: 3px;
  }
  th {
    font-weight: 600;
    font-size: 0.87rem;
    text-align: left;
    white-space: nowrap;
    padding: 0.2rem 0.5rem;
  }
  thead th {
    vertical-align: bottom;
  }
  .col {
    display: inline-block;
    max-width: 7rem;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .dot {
    display: inline-block;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    margin-right: 0.4rem;
  }
  td {
    padding: 0;
  }
  .self {
    background: repeating-linear-gradient(135deg, var(--line) 0 1px, transparent 1px 6px);
    border-radius: var(--radius-s);
  }
  .cell {
    width: 3.4rem;
    height: 2.4rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-s);
    cursor: pointer;
    font-weight: 600;
  }
  .cell.empty {
    color: var(--ink-3);
    border-style: dashed;
  }
  .cell.flag {
    outline: 2px solid var(--rust);
    outline-offset: -1px;
  }
  .cell:hover {
    border-color: var(--ink-3);
  }
</style>
