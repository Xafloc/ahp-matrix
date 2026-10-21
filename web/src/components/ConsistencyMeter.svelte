<script lang="ts">
  import { describe, type CompareKind } from '../lib/scale';
  import type { MatrixReport, Scale } from '../lib/types';

  let {
    report,
    items,
    kind,
    scale,
    onfix,
    fixes = true,
  }: {
    report: MatrixReport | undefined;
    items: { id: string; name: string }[];
    kind: CompareKind;
    scale: Scale;
    onfix: (a: string, b: string, v: number) => void;
    fixes?: boolean; // list the suggested fixes (the pair list shows them on its rows instead)
  } = $props();

  const name = (id: string) => items.find((i) => i.id === id)?.name ?? id;
  // The meter spans 0 to three times the threshold, so "fine" fills a third.
  const span = $derived(report ? Math.max(report.threshold * 3, report.cr) : 0.3);
</script>

{#if report && items.length >= 3}
  <section class="meter" aria-label="Consistency">
    {#if !report.connected}
      <p class="small muted">Compare a few more pairs so every one is linked to the others; then the consistency check can run.</p>
    {:else}
      <div class="line">
        <span class="verdict" class:bad={!report.consistent}>
          {report.consistent ? 'Your answers agree with each other' : 'Some answers contradict the others'}
        </span>
        <span class="small muted">
          Consistency ratio {report.cr.toFixed(2)}, acceptable up to {report.threshold.toFixed(2)}{report.complete ? '' : ' (provisional until every pair is compared)'}
        </span>
      </div>
      <div class="track" aria-hidden="true">
        <span class="fill" class:bad={!report.consistent} style:width="{Math.min(100, (report.cr / span) * 100)}%"></span>
        <span class="tick" style:left="{(report.threshold / span) * 100}%"></span>
      </div>
      {#if fixes && !report.consistent && report.suggestions.length}
        <ul class="fixes">
          {#each report.suggestions as s}
            <li>
              <span>
                <span class="said">You said: {describe(s.current, scale, kind, name(s.a), name(s.b))}.</span>
                <span class="small">Your other answers suggest: {describe(s.suggested, scale, kind, name(s.a), name(s.b))}.</span>
              </span>
              <button class="btn" onclick={() => onfix(s.a, s.b, s.suggested)}>Use that</button>
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </section>
{/if}

<style>
  .meter {
    display: grid;
    gap: 0.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--line);
  }
  .line {
    display: flex;
    gap: 0.75rem;
    align-items: baseline;
    flex-wrap: wrap;
  }
  .verdict {
    font-weight: 650;
    color: var(--good);
  }
  .verdict.bad {
    color: var(--rust);
  }
  .track {
    position: relative;
    height: 6px;
    border-radius: 3px;
    background: var(--line);
    max-width: 28rem;
  }
  .fill {
    position: absolute;
    inset: 0 auto 0 0;
    border-radius: 3px;
    background: var(--good);
    transition: width 300ms ease;
  }
  .fill.bad {
    background: var(--rust);
  }
  .tick {
    position: absolute;
    top: -4px;
    bottom: -4px;
    width: 2px;
    background: var(--ink);
  }
  .fixes {
    list-style: none;
    margin: 0.25rem 0 0;
    padding: 0;
    display: grid;
    gap: 0.5rem;
  }
  .fixes li > span {
    display: grid;
  }
  .said {
    font-weight: 600;
  }
  .fixes li {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    justify-content: space-between;
    padding: 0.55rem 0.75rem;
    background: var(--rust-wash);
    border-radius: var(--radius-s);
  }
</style>
