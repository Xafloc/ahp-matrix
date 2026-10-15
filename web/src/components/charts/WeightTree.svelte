<script lang="ts">
  // The criteria hierarchy with each criterion's share of the whole decision
  // (bar) and its share within its group (number).
  import { percent } from '../../lib/format';
  import { flatten } from '../../lib/tree';
  import type { Project, Results } from '../../lib/types';

  let { project, results }: { project: Project; results: Results } = $props();
  const byId = $derived(new Map(results.criteria.map((c) => [c.id, c])));
  const rows = $derived(flatten(project).map((f) => ({ ...f, r: byId.get(f.criterion.id) })));
</script>

<table class="tree">
  <thead>
    <tr>
      <th scope="col">Criterion</th>
      <th scope="col" class="num">In its group</th>
      <th scope="col">Share of the decision</th>
    </tr>
  </thead>
  <tbody>
    {#each rows as { criterion: c, depth, r } (c.id)}
      <tr>
        <th scope="row" style:padding-left="{(0.4 + depth * 1.3).toFixed(1)}rem" class:group={depth === 0}>{c.name}</th>
        <td class="num muted">{r ? percent(r.local, 0) : ''}</td>
        <td class="share">
          <span class="bar" style:width="{(r?.global ?? 0) * 100}%" style:opacity={1 - depth * 0.22}></span>
          <span class="value">{r ? percent(r.global) : ''}</span>
        </td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  .tree {
    width: 100%;
    border-collapse: collapse;
  }
  th,
  td {
    padding: 0.4rem 0.4rem;
    border-bottom: 1px solid var(--line);
    text-align: left;
    font-weight: 400;
  }
  thead th {
    font-size: 0.85rem;
    color: var(--ink-3);
    font-weight: 600;
  }
  tbody th {
    font-weight: 550;
  }
  tbody th.group {
    font-weight: 700;
  }
  .num {
    text-align: right;
    width: 6rem;
  }
  .share {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .bar {
    display: block;
    height: 10px;
    min-width: 2px;
    max-width: calc(100% - 4rem);
    border-radius: 0 4px 4px 0;
    background: var(--ink-2);
  }
  .value {
    color: var(--ink-2);
    font-size: 0.9rem;
  }
</style>
