<script lang="ts">
  // Where each option's score comes from: a row per option, a column per leaf
  // criterion, and in each cell that criterion's weight × the option's
  // priority on it. Bars within a column share a scale, so columns compare
  // options; the total column is the overall score. This is the original
  // spreadsheet's summary grid, drawn.
  import { colorOf, percent } from '../../lib/format';
  import { leaves } from '../../lib/tree';
  import type { Project, Results } from '../../lib/types';

  let { project, results }: { project: Project; results: Results } = $props();
  const cols = $derived(
    leaves(project).map((c) => {
      const r = results.criteria.find((x) => x.id === c.id);
      const cells = project.alternatives.map((_, a) => (r ? r.global * r.subtree[a] : 0));
      return { c, cells, max: Math.max(...cells, 1e-9) };
    }),
  );
  const order = $derived([...results.scores.keys()].sort((x, y) => results.scores[y].score - results.scores[x].score));
</script>

<div class="wrap">
  <table>
    <thead>
      <tr>
        <th scope="col">Option</th>
        {#each cols as col (col.c.id)}<th scope="col" title={col.c.name}><span>{col.c.name}</span></th>{/each}
        <th scope="col" class="total">Total</th>
      </tr>
    </thead>
    <tbody>
      {#each order as a (a)}
        {@const alt = project.alternatives[a]}
        <tr>
          <th scope="row"><span class="dot" style:background={colorOf(alt.color)}></span>{alt.name}</th>
          {#each cols as col (col.c.id)}
            <td title="{alt.name} on {col.c.name}: {percent(col.cells[a], 2)} of the decision">
              <span class="cell">
                <span class="bar" style:width="{(col.cells[a] / col.max) * 100}%" style:background={colorOf(alt.color)}></span>
              </span>
              <span class="small muted">{percent(col.cells[a])}</span>
            </td>
          {/each}
          <td class="total">{percent(results.scores[a].score)}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .wrap {
    overflow-x: auto;
  }
  table {
    border-collapse: collapse;
    min-width: 100%;
  }
  th,
  td {
    padding: 0.45rem 0.6rem;
    border-bottom: 1px solid var(--line);
    text-align: left;
    vertical-align: middle;
  }
  thead th {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--ink-3);
    max-width: 8rem;
  }
  thead th span {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  tbody th {
    font-weight: 600;
    white-space: nowrap;
  }
  .dot {
    display: inline-block;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    margin-right: 0.45rem;
  }
  td {
    min-width: 6rem;
  }
  .cell {
    display: block;
    height: 8px;
    margin-bottom: 0.2rem;
  }
  .bar {
    display: block;
    height: 100%;
    min-width: 2px;
    border-radius: 0 4px 4px 0;
  }
  .total {
    text-align: right;
    font-weight: 650;
    white-space: nowrap;
  }
</style>
