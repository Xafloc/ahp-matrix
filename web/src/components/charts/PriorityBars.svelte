<script lang="ts">
  // Horizontal bars, one per alternative, each labeled with its name and share
  // (labels carry identity; color only repeats it).
  import { percent } from '../../lib/format';

  let {
    names,
    colors,
    values,
    sorted = false,
    big = false,
  }: { names: string[]; colors: string[]; values: number[]; sorted?: boolean; big?: boolean } = $props();

  const rows = $derived.by(() => {
    const r = names.map((name, i) => ({ name, color: colors[i], value: values[i] ?? 0, i }));
    return sorted ? r.sort((a, b) => b.value - a.value) : r;
  });
  const max = $derived(Math.max(...rows.map((r) => r.value), 1e-9));
</script>

<ol class="bars" class:big>
  {#each rows as r, k (r.i)}
    <li title="{r.name}: {percent(r.value)}">
      <span class="name">{#if sorted}<span class="rank">{k + 1}</span>{/if}{r.name}</span>
      <span class="track"><span class="bar" style:width="{(r.value / max) * 100}%" style:background={r.color}></span></span>
      <span class="value">{percent(r.value)}</span>
    </li>
  {/each}
</ol>

<style>
  .bars {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.45rem;
  }
  li {
    display: grid;
    grid-template-columns: minmax(6rem, 12rem) 1fr 3.8rem;
    align-items: center;
    gap: 0.75rem;
  }
  .name {
    font-weight: 550;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rank {
    display: inline-block;
    width: 1.4rem;
    color: var(--ink-3);
    font-weight: 600;
  }
  .track {
    height: 12px;
  }
  .bar {
    display: block;
    height: 100%;
    min-width: 2px;
    border-radius: 0 4px 4px 0;
    transition: width 300ms ease;
  }
  .value {
    text-align: right;
    color: var(--ink-2);
  }
  .big li {
    grid-template-columns: minmax(7rem, 14rem) 1fr 4.5rem;
  }
  .big .track {
    height: 22px;
  }
  .big .name {
    font-size: 1.05rem;
  }
  .big .value {
    font-size: 1.05rem;
    font-weight: 600;
    color: var(--ink);
  }
</style>
