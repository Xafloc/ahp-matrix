<script lang="ts">
  // The strength words under (or above) a row of stops: only the anchor
  // strengths are named, each centred on its stop and spilling into the
  // unlabelled stops beside it; the outermost align inwards.
  import { stopLabel, stops } from '../lib/scale';
  import type { Scale } from '../lib/types';

  // sparse names only Extreme, Strong and Equal, for narrow rows where every
  // anchor word wouldn't fit.
  let { scale, sparse = false }: { scale: Scale; sparse?: boolean } = $props();
  const positions = $derived([...stops(scale)].reverse());
  const middle = $derived((positions.length - 1) / 2);
  const word = (v: number) => {
    const w = stopLabel(v, scale);
    return sparse && (w === 'Very strong' || w === 'Moderate') ? '' : w;
  };
</script>

<div class="words" style:--n={positions.length} aria-hidden="true">
  {#each positions as v, i}<span class:mid={i === middle}>{word(v)}</span>{/each}
</div>

<style>
  .words {
    display: grid;
    grid-template-columns: repeat(var(--n, 17), minmax(0, 1fr));
    width: 100%;
    font-size: 0.72rem;
    line-height: 1.15;
    color: var(--ink-3);
  }
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
</style>
