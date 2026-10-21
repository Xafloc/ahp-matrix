<script lang="ts">
  // Everyone's answer to one pair on the same 9…1…9 scale as the balance: a
  // named marker per person, and the group's combined answer in brass.
  // Answers that would overlap stack upwards.
  import { placeMarkers, scaleX } from '../lib/peoplescale';
  import { describe, type CompareKind } from '../lib/scale';
  import type { Scale } from '../lib/types';
  import StrengthWords from './StrengthWords.svelte';

  let {
    a,
    b,
    scale,
    kind,
    on = '',
    answers,
    combined,
  }: {
    a: string;
    b: string;
    scale: Scale;
    kind: CompareKind;
    on?: string;
    answers: { id: string; name: string; v: number }[];
    combined: number | undefined;
  } = $props();

  let width = $state(0);
  const placed = $derived(placeMarkers(answers.map((x) => ({ ...x, label: x.name })), scale, { width: width || undefined }));
  const rows = $derived(Math.max(1, ...placed.map((m) => m.row + 1)));
  const say = (v: number) => describe(v, scale, kind, a, b, on);
</script>

<div class="scale">
  <div class="ends small">
    <span>{a}</span>
    <span>{b}</span>
  </div>
  <div class="plot" style:--rows={rows} bind:clientWidth={width}>
    <span class="track" aria-hidden="true"></span>
    <span class="tick mid" aria-hidden="true"></span>
    {#if combined !== undefined}
      <span class="marker combined" style:left="{scaleX(combined, scale) * 100}%" role="img" aria-label="Everyone combined: {say(combined)}" title="Everyone combined: {say(combined)}"></span>
    {/if}
    {#each placed as m (m.id)}
      <span
        class="marker person"
        style:left="{m.x * 100}%"
        style:--row={m.row}
        role="img"
        aria-label="{m.name}: {say(m.v)}"
        title="{m.name}: {say(m.v)}"
      >
        <span class="dot" aria-hidden="true"></span>
        <span class="who small" class:right={m.x > 0.8} class:left={m.x < 0.2}>{m.name}</span>
      </span>
    {/each}
  </div>
  <StrengthWords {scale} sparse />
</div>

<style>
  .scale {
    display: grid;
    gap: 0.3rem;
  }
  .ends {
    display: flex;
    justify-content: space-between;
    font-weight: 600;
    color: var(--ink-2);
  }
  .plot {
    position: relative;
    height: calc(1.2rem + var(--rows) * 1.55rem);
  }
  .track {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0.55rem;
    height: 2px;
    background: var(--line-strong);
  }
  .tick.mid {
    position: absolute;
    left: 50%;
    bottom: 0.25rem;
    width: 2px;
    height: 0.65rem;
    margin-left: -1px;
    background: var(--ink-3);
  }
  .marker {
    position: absolute;
    transform: translateX(-50%);
  }
  .combined {
    bottom: 0.15rem;
    width: 0.85rem;
    height: 0.85rem;
    background: var(--brass);
    border: 2px solid var(--paper);
    transform: translateX(-50%) rotate(45deg);
  }
  .person {
    bottom: calc(1.2rem + var(--row) * 1.55rem);
    display: grid;
    justify-items: center;
  }
  .person .dot {
    order: 2;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    background: var(--ink);
    border: 2px solid var(--paper);
  }
  .person .who {
    order: 1;
    white-space: nowrap;
    color: var(--ink);
    line-height: 1.1;
  }
  .who.left {
    justify-self: start;
    transform: translateX(calc(50% - 0.4rem));
  }
  .who.right {
    justify-self: end;
    transform: translateX(calc(-50% + 0.4rem));
  }
</style>
