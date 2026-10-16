<script lang="ts">
  // Each option's overall score as one criterion's weight goes from 0 to 100%.
  // Lines are straight (the scores are linear in the weight). A dashed marker
  // shows the current weight, a solid one the what-if weight, and dots mark
  // where the ranking changes. Hover shows every score at that weight.
  import { percent } from '../../lib/format';
  import { at, crossovers, type Line } from '../../lib/sensitivity';

  let {
    line,
    names,
    colors,
    current,
    x,
  }: { line: Line; names: string[]; colors: string[]; current: number; x: number } = $props();

  const W = 640;
  const H = 300;
  const pad = { l: 48, r: 150, t: 16, b: 34 };
  const iw = W - pad.l - pad.r;
  const ih = H - pad.t - pad.b;

  const ends = $derived([at(line, 0), at(line, 1)]);
  const lo = $derived(Math.max(0, Math.min(...ends[0], ...ends[1]) - 0.02));
  const hi = $derived(Math.min(1, Math.max(...ends[0], ...ends[1]) + 0.02));
  const sx = (v: number) => pad.l + v * iw;
  const sy = $derived((v: number) => pad.t + (1 - (v - lo) / (hi - lo || 1)) * ih);
  const ticks = $derived(Array.from({ length: 5 }, (_, k) => lo + ((hi - lo) * k) / 4));
  const cross = $derived(crossovers(line));
  // Direct labels while they fit (up to eight lines); the legend is always there too.
  const direct = $derived(names.length <= 8);

  // Direct labels at the right end, nudged apart so they don't collide.
  const labels = $derived.by(() => {
    const ys = ends[1].map((v, a) => ({ a, y: sy(v) })).sort((p, q) => p.y - q.y);
    for (let k = 1; k < ys.length; k++) ys[k].y = Math.max(ys[k].y, ys[k - 1].y + 15);
    return ys;
  });

  let hover = $state<number | null>(null);
  let svg = $state<SVGSVGElement | undefined>();
  function move(e: PointerEvent) {
    if (!svg) return;
    const r = svg.getBoundingClientRect();
    const v = ((e.clientX - r.left) * (W / r.width) - pad.l) / iw;
    hover = v >= 0 && v <= 1 ? v : null;
  }
  const hoverRows = $derived(
    hover === null ? [] : at(line, hover).map((s, a) => ({ a, s })).sort((p, q) => q.s - p.s),
  );
</script>

<div class="chart">
  <svg bind:this={svg} viewBox="0 0 {W} {H}" role="img" aria-label="Scores as the weight changes" onpointermove={move} onpointerleave={() => (hover = null)}>
    {#each ticks as t}
      <line class="grid" x1={pad.l} x2={pad.l + iw} y1={sy(t)} y2={sy(t)} />
      <text class="axis" x={pad.l - 8} y={sy(t) + 4} text-anchor="end">{percent(t, 0)}</text>
    {/each}
    {#each [0, 0.25, 0.5, 0.75, 1] as t}
      <text class="axis" x={sx(t)} y={H - 10} text-anchor="middle">{percent(t, 0)}</text>
    {/each}
    <line class="now" x1={sx(current)} x2={sx(current)} y1={pad.t} y2={pad.t + ih} />
    <line class="whatif" x1={sx(x)} x2={sx(x)} y1={pad.t} y2={pad.t + ih} />
    {#each names as _, a}
      <line class="series" x1={sx(0)} y1={sy(ends[0][a])} x2={sx(1)} y2={sy(ends[1][a])} style:stroke={colors[a]} />
    {/each}
    {#each cross as c}
      <circle class="cross" cx={sx(c.x)} cy={sy(at(line, c.x)[c.a])} r="5" />
    {/each}
    {#each names as _, a}
      <circle class="dot" cx={sx(x)} cy={sy(at(line, x)[a])} r="4.5" style:fill={colors[a]} />
    {/each}
    {#if direct}
      {#each labels as l}
        <text class="label" x={sx(1) + 8} y={l.y + 4}>{names[l.a]}</text>
      {/each}
    {/if}
    {#if hover !== null}
      <line class="hair" x1={sx(hover)} x2={sx(hover)} y1={pad.t} y2={pad.t + ih} />
    {/if}
  </svg>
  {#if hover !== null}
    <div class="tip" style:left="{(sx(hover) / W) * 100}%">
      <strong>At {percent(hover, 0)}</strong>
      {#each hoverRows as r}
        <span><i style:background={colors[r.a]}></i>{names[r.a]} {percent(r.s)}</span>
      {/each}
    </div>
  {/if}
  <ul class="legend">
    {#each names as n, a}<li><i style:background={colors[a]}></i>{n}</li>{/each}
    <li><i class="key-now"></i>Current weight</li>
    <li><i class="key-cross"></i>Ranking changes</li>
  </ul>
</div>

<style>
  .chart {
    position: relative;
  }
  svg {
    width: 100%;
    height: auto;
    display: block;
    touch-action: none;
  }
  .grid {
    stroke: var(--line);
    stroke-width: 1;
  }
  .axis {
    fill: var(--ink-3);
    font-size: 11px;
  }
  .series {
    stroke-width: 2;
  }
  .now {
    stroke: var(--ink-3);
    stroke-dasharray: 4 4;
  }
  .whatif {
    stroke: var(--brass);
    stroke-width: 2;
  }
  .hair {
    stroke: var(--ink-3);
    stroke-width: 1;
  }
  .cross {
    fill: var(--paper);
    stroke: var(--ink);
    stroke-width: 2;
  }
  .dot {
    stroke: var(--paper);
    stroke-width: 2;
  }
  .label {
    fill: var(--ink);
    font-size: 12px;
    font-weight: 600;
  }
  .tip {
    position: absolute;
    top: 0.5rem;
    transform: translateX(-50%);
    display: grid;
    gap: 0.1rem;
    padding: 0.45rem 0.6rem;
    background: var(--paper);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-s);
    box-shadow: var(--shadow-lift);
    font-size: 0.82rem;
    pointer-events: none;
    white-space: nowrap;
  }
  i {
    display: inline-block;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    margin-right: 0.35rem;
  }
  .legend {
    list-style: none;
    margin: 0.4rem 0 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    font-size: 0.85rem;
    color: var(--ink-2);
  }
  .key-now {
    width: 1rem;
    height: 0;
    border-radius: 0;
    border-top: 1.5px dashed var(--ink-3);
    vertical-align: middle;
  }
  .key-cross {
    background: var(--paper);
    border: 2px solid var(--ink);
  }
</style>
