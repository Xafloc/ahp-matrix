<script lang="ts">
  // Everyone's answer to one pair on the same 9…1…9 scale as the balance: a
  // named marker per person, and the group's combined answer in brass.
  // Answers that would overlap stack upwards. Markers sit over the strength
  // words below them, and can be focused to read the answer.
  import { onStops, placeMarkers, scaleX } from '../lib/peoplescale';
  import { describe, stops, type CompareKind } from '../lib/scale';
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
  const n = $derived(stops(scale).length);
  // The stops span all but half a column at each end, so overlaps are measured on that.
  const span = $derived(width ? (width * (n - 1)) / n : undefined);
  const placed = $derived(placeMarkers(answers.map((x) => ({ ...x, label: x.name })), scale, { width: span }));
  const rows = $derived(Math.max(1, ...placed.map((m) => m.row + 1)));
  const say = (v: number) => describe(v, scale, kind, a, b, on);
  const at = (x: number) => `${onStops(x, n) * 100}%`;

  // The answer being pointed at or focused, read out under the scale.
  let showing = $state('');
</script>

<div class="scale">
  <div class="ends small">
    <span>{a}</span>
    <span>{b}</span>
  </div>
  <div class="plot" style:--rows={rows} style:--inset="{50 / n}%" bind:clientWidth={width}>
    <span class="track" aria-hidden="true"></span>
    <span class="tick mid" aria-hidden="true"></span>
    {#if combined !== undefined}
      {@const line = `Everyone combined: ${say(combined)}`}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <span
        class="marker combined"
        style:left={at(scaleX(combined, scale))}
        role="img"
        tabindex="0"
        aria-label={line}
        onfocus={() => (showing = line)}
        onblur={() => (showing = '')}
        onpointerenter={() => (showing = line)}
        onpointerleave={() => (showing = '')}
      ></span>
    {/if}
    {#each placed as m (m.id)}
      {@const line = `${m.name}: ${say(m.v)}`}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <span
        class="marker person {m.align}"
        style:left={at(m.x)}
        style:--row={m.row}
        role="img"
        tabindex="0"
        aria-label={line}
        onfocus={() => (showing = line)}
        onblur={() => (showing = '')}
        onpointerenter={() => (showing = line)}
        onpointerleave={() => (showing = '')}
      >
        <span class="who small">{m.name}</span>
        <span class="dot" aria-hidden="true"></span>
      </span>
    {/each}
  </div>
  <StrengthWords {scale} sparse />
  <p class="readout small" aria-hidden="true">{showing}</p>
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
    overflow-x: clip;
    height: calc(1.2rem + var(--rows) * 1.55rem);
  }
  .track {
    position: absolute;
    left: var(--inset);
    right: var(--inset);
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
    outline: none;
  }
  .marker:focus-visible {
    border-radius: 3px;
    box-shadow: 0 0 0 2px var(--paper), 0 0 0 4px var(--ink);
  }
  .readout {
    margin: 0;
    min-height: 1.2em;
    color: var(--ink-2);
  }
  .combined {
    bottom: 0.15rem;
    width: 0.85rem;
    height: 0.85rem;
    background: var(--brass);
    border: 2px solid var(--paper);
    transform: translateX(-50%) rotate(45deg);
  }
  /* A person's marker is a point on the scale; the dot sits on it and the
     name hangs above, inwards near the ends so it stays on the scale. */
  .person {
    bottom: calc(1.2rem + var(--row) * 1.55rem);
    width: 0.6rem;
    height: 1.35rem;
    margin-left: -0.3rem;
    transform: none;
  }
  .person .dot {
    position: absolute;
    bottom: 0;
    left: 0;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    background: var(--ink);
    border: 2px solid var(--paper);
  }
  .person .who {
    position: absolute;
    top: 0;
    white-space: nowrap;
    color: var(--ink);
    line-height: 1.1;
  }
  .center .who {
    left: 50%;
    transform: translateX(-50%);
  }
  .start .who {
    left: 0;
  }
  .end .who {
    right: 0;
  }
</style>
