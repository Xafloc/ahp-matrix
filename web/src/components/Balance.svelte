<script lang="ts">
  // A balance beam for one pairwise judgment: the preferred side sinks. The
  // row of stops underneath (StrengthScale) is the control.
  import { describe, type CompareKind } from '../lib/scale';
  import StrengthScale from './StrengthScale.svelte';
  import StrengthWords from './StrengthWords.svelte';
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

  const max = $derived(scale === 'classic5' ? 10 : 9);
  const tilt = $derived(value === undefined ? 0 : (-14 * Math.log(value)) / Math.log(max));
  const sentence = $derived(value === undefined ? 'Not compared yet' : describe(value, scale, kind, a.name, b.name, on));
  // The stop under the pointer (or keyboard focus), previewed before it's chosen.
  let hovered = $state<number | null>(null);
  const preview = $derived(hovered !== null && hovered !== value ? describe(hovered, scale, kind, a.name, b.name, on, true) : '');
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

  <div class="control">
    <StrengthScale
      {value}
      {scale}
      label="Compare {a.name} with {b.name}"
      valuetext={sentence}
      describeAt={(v) => describe(v, scale, kind, a.name, b.name, on)}
      {onchange}
      onhover={(v) => (hovered = v)}
    />
    <StrengthWords {scale} />
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
  .control {
    display: grid;
    gap: 0.25rem;
    width: min(100%, 40rem);
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
