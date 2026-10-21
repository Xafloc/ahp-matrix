<script lang="ts">
  // The 9…1…9 control for one pairwise judgment, used by the balance and by
  // every row of the pair list. Positions run left (the first item strongly
  // preferred) to right (the second strongly preferred). The stops between
  // equal and the choice are filled, so how far a judgment leans shows at a
  // glance. Arrow keys move along it; Backspace clears it.
  import { fraction, stops } from '../lib/scale';
  import type { Scale } from '../lib/types';

  let {
    value,
    scale,
    label,
    valuetext,
    describeAt,
    onchange,
    onhover = () => {},
    compact = false,
  }: {
    value: number | undefined;
    scale: Scale;
    label: string; // accessible name, e.g. "Compare Alpha with Beta"
    valuetext: string; // the current judgment in words
    describeAt: (v: number) => string; // a stop's judgment in words
    onchange: (v: number | undefined) => void;
    onhover?: (v: number | null) => void; // the stop under the pointer or focus, for previews
    compact?: boolean;
  } = $props();

  const positions = $derived([...stops(scale)].reverse());
  const middle = $derived((positions.length - 1) / 2);
  const index = $derived(value === undefined ? -1 : positions.findIndex((v) => Math.abs(Math.log(v / value)) < 1e-6));
  const filled = (i: number) => index >= 0 && index !== middle && i !== index && i >= Math.min(middle, index) && i <= Math.max(middle, index);

  function set(i: number) {
    onchange(positions[Math.max(0, Math.min(positions.length - 1, i))]);
  }

  function onKey(e: KeyboardEvent) {
    const from = index < 0 ? middle : index;
    const moves: Record<string, () => void> = {
      ArrowLeft: () => set(index < 0 ? middle - 1 : from - 1),
      ArrowRight: () => set(index < 0 ? middle + 1 : from + 1),
      // A slider's ↑/↓ raise and lower its value, here toward the right-hand side.
      ArrowUp: () => set(index < 0 ? middle + 1 : from + 1),
      ArrowDown: () => set(index < 0 ? middle - 1 : from - 1),
      Home: () => set(0),
      End: () => set(positions.length - 1),
      '=': () => set(middle),
      Backspace: () => onchange(undefined),
      Delete: () => onchange(undefined),
    };
    if (moves[e.key]) {
      e.preventDefault();
      moves[e.key]();
    }
  }

  // Each stop shows its strength toward the nearer side: 9 … 1 … 9.
  const shown = (v: number) => fraction(v >= 1 ? v : 1 / v);
</script>

<div
  class="stops"
  class:compact
  style:--n={positions.length}
  role="slider"
  tabindex="0"
  aria-label={label}
  aria-valuemin={0}
  aria-valuemax={positions.length - 1}
  aria-valuenow={index < 0 ? undefined : index}
  aria-valuetext={valuetext}
  onkeydown={onKey}
>
  {#each positions as v, i}
    <button
      type="button"
      tabindex="-1"
      class="stop"
      class:mid={i === middle}
      class:on={i === index}
      class:fill={filled(i)}
      aria-label={describeAt(v)}
      onclick={() => set(i)}
      onpointerenter={() => onhover(v)}
      onpointerleave={() => onhover(null)}
      onfocus={() => onhover(v)}
      onblur={() => onhover(null)}
    >
      {shown(v)}
    </button>
  {/each}
</div>

<style>
  .stops {
    display: grid;
    grid-template-columns: repeat(var(--n, 17), minmax(0, 1fr));
    width: 100%;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-s);
    background: var(--paper);
    overflow: hidden;
  }
  .stops:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 3px;
  }
  .stop {
    border: 0;
    border-right: 1px solid var(--line);
    background: transparent;
    padding: 0.55rem 0;
    cursor: pointer;
    font-weight: 600;
    font-size: 0.85rem;
    color: var(--ink-2);
    transition: background 120ms ease;
  }
  .compact .stop {
    padding: 0.38rem 0;
    font-size: 0.78rem;
  }
  .stop:last-child {
    border-right: 0;
  }
  .stop:hover {
    background: var(--brass-wash);
  }
  .stop.mid {
    color: var(--ink);
  }
  .stop.fill {
    background: color-mix(in oklab, var(--brass) 22%, var(--paper));
    color: var(--ink);
  }
  .stop.on {
    background: var(--ink);
    color: var(--paper);
  }
</style>
