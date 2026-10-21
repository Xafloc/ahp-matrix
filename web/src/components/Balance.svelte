<script lang="ts">
  // A balance beam for one pairwise judgment: the preferred side sinks. The
  // row of stops underneath is the control; arrow keys move along it.
  import { describe, fraction, stopLabel, stops, type CompareKind } from '../lib/scale';
  import type { Scale } from '../lib/types';

  interface Side {
    name: string;
    color?: string;
  }
  let {
    a,
    b,
    value,
    scale,
    kind,
    on = '',
    onchange,
  }: {
    a: Side;
    b: Side;
    value: number | undefined;
    scale: Scale;
    kind: CompareKind;
    on?: string;
    onchange: (v: number | undefined) => void;
  } = $props();

  // Positions run left (a strongly preferred) to right (b strongly preferred).
  const positions = $derived([...stops(scale)].reverse());
  const max = $derived(positions[0]);
  const middle = $derived((positions.length - 1) / 2);
  const index = $derived(value === undefined ? -1 : positions.findIndex((v) => Math.abs(Math.log(v / value)) < 1e-6));
  const tilt = $derived(value === undefined ? 0 : (-14 * Math.log(value)) / Math.log(max));
  const sentence = $derived(value === undefined ? 'Not compared yet' : describe(value, scale, kind, a.name, b.name, on));
  // The stop under the pointer (or keyboard focus), previewed before it's chosen.
  let hovered = $state<number | null>(null);
  const preview = $derived(hovered !== null && hovered !== index ? describe(positions[hovered], scale, kind, a.name, b.name, on, true) : '');

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

  // Labels under the stops: strength toward the nearer side.
  const label = (v: number) => fraction(v >= 1 ? v : 1 / v);
</script>

<div class="balance">
  <svg viewBox="0 0 640 230" class="scene" aria-hidden="true">
    <line x1="320" y1="70" x2="320" y2="206" class="post" />
    <path d="M276 208h88" class="foot" />
    <g class="beam" class:unset={value === undefined} style:transform="rotate({tilt}deg)">
      <line x1="70" y1="70" x2="570" y2="70" class="bar" />
      {#each [{ x: 70, side: a }, { x: 570, side: b }] as end}
        <g class="pan" style:transform="rotate({-tilt}deg)" style:transform-origin="{end.x}px 70px">
          <line x1={end.x} y1="70" x2={end.x - 46} y2="132" />
          <line x1={end.x} y1="70" x2={end.x + 46} y2="132" />
          <path d="M{end.x - 62} 132h124a62 16 0 0 1-124 0Z" class="dish" style:--c={end.side.color ?? 'var(--ink-3)'} />
        </g>
      {/each}
    </g>
    <circle cx="320" cy="70" r="7" class="pivot" />
  </svg>

  <div class="names">
    <span class="side left"><span class="dot" style:background={a.color ?? 'var(--ink-3)'}></span>{a.name}</span>
    <span class="side right">{b.name}<span class="dot" style:background={b.color ?? 'var(--ink-3)'}></span></span>
  </div>

  <div
    class="stops"
    style:--n={positions.length}
    role="slider"
    tabindex="0"
    aria-label="Compare {a.name} with {b.name}"
    aria-valuemin={0}
    aria-valuemax={positions.length - 1}
    aria-valuenow={index < 0 ? undefined : index}
    aria-valuetext={sentence}
    onkeydown={onKey}
  >
    {#each positions as v, i}
      <button
        type="button"
        tabindex="-1"
        class="stop"
        class:mid={i === middle}
        class:on={i === index}
        class:toward-a={i < middle}
        class:toward-b={i > middle}
        aria-label={describe(v, scale, kind, a.name, b.name, on)}
        onclick={() => set(i)}
        onpointerenter={() => (hovered = i)}
        onpointerleave={() => (hovered = null)}
        onfocus={() => (hovered = i)}
        onblur={() => (hovered = null)}
      >
        <span>{label(v)}</span>
      </button>
    {/each}
  </div>

  <div class="words" style:--n={positions.length} aria-hidden="true">
    {#each positions as v, i}<span class:mid={i === middle}>{stopLabel(v, scale)}</span>{/each}
  </div>

  {#if preview}
    <p class="sentence preview">{preview}</p>
  {:else}
    <p class="sentence" class:unset={value === undefined} aria-live="polite">{sentence}</p>
  {/if}
</div>

<style>
  .balance {
    display: grid;
    gap: 0.6rem;
    justify-items: center;
  }
  .scene {
    width: min(100%, 40rem);
    height: auto;
    overflow: visible;
  }
  .post,
  .foot {
    stroke: var(--ink);
    stroke-width: 6;
    stroke-linecap: round;
    fill: none;
  }
  .beam {
    transform-origin: 320px 70px;
    transition: transform 420ms cubic-bezier(0.34, 1.4, 0.5, 1);
  }
  .bar {
    stroke: var(--brass);
    stroke-width: 7;
    stroke-linecap: round;
  }
  .beam.unset .bar {
    stroke-dasharray: 2 12;
    stroke: var(--line-strong);
  }
  .pan {
    transition: transform 420ms cubic-bezier(0.34, 1.4, 0.5, 1);
  }
  .pan line {
    stroke: var(--ink-3);
    stroke-width: 1.5;
  }
  .dish {
    fill: color-mix(in oklab, var(--c) 22%, var(--paper));
    stroke: var(--c);
    stroke-width: 2.5;
  }
  .pivot {
    fill: var(--paper);
    stroke: var(--ink);
    stroke-width: 4;
  }
  .names {
    width: min(100%, 40rem);
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    font-family: var(--font-display);
    font-size: 1.25rem;
    font-weight: 650;
    margin-top: -0.5rem;
  }
  .side {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    max-width: 48%;
  }
  .right {
    text-align: right;
  }
  .dot {
    flex: none;
    width: 0.7rem;
    height: 0.7rem;
    border-radius: 50%;
  }
  .stops {
    display: grid;
    grid-template-columns: repeat(var(--n, 17), minmax(0, 1fr));
    grid-auto-flow: column;
    width: min(100%, 40rem);
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
  .stop.on {
    background: var(--ink);
    color: var(--paper);
  }
  .words {
    display: grid;
    grid-template-columns: repeat(var(--n, 17), minmax(0, 1fr));
    width: min(100%, 40rem);
    margin-top: -0.35rem;
    font-size: 0.72rem;
    line-height: 1.15;
    color: var(--ink-3);
    text-align: center;
  }
  /* A word sits on one line, centred on its stop and spilling into the
     unlabelled stops beside it; the outermost align inwards so nothing
     pokes out past the row. */
  .words span {
    display: flex;
    justify-content: center;
    min-width: 0;
    white-space: nowrap;
  }
  .words span:first-child {
    justify-content: flex-start;
  }
  .words span:last-child {
    justify-content: flex-end;
  }
  .words .mid {
    color: var(--ink-2);
    font-weight: 600;
  }
  .sentence.preview {
    color: var(--ink-3);
    font-style: italic;
  }
  .sentence {
    font-size: 1.05rem;
    font-weight: 550;
    min-height: 1.6em;
    text-align: center;
  }
  .sentence.unset {
    color: var(--ink-3);
    font-weight: 400;
  }
</style>
