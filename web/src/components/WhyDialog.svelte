<script lang="ts">
  // "Why this suggestion?": the answer as given, the two other answers that
  // contradict it most, what all the answers imply together, and what using
  // the suggestion would do to the consistency ratio.
  import { describe, fraction, type CompareKind } from '../lib/scale';
  import { should, strongestChain } from '../lib/why';
  import { percent } from '../lib/format';
  import type { MatrixReport, PairSuggestion, Scale } from '../lib/types';

  let {
    open,
    items,
    report,
    suggestion,
    scale,
    kind,
    on = '',
    onuse,
    onclose,
  }: {
    open: boolean;
    items: { id: string; name: string; color?: string }[];
    report: MatrixReport;
    suggestion: PairSuggestion;
    scale: Scale;
    kind: CompareKind;
    on?: string;
    onuse: () => void;
    onclose: () => void;
  } = $props();

  let dialog = $state<HTMLDialogElement>();
  $effect(() => {
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  });

  const idx = (id: string) => report.items.indexOf(id);
  const name = (i: number) => items.find((x) => x.id === report.items[i])?.name ?? report.items[i];
  const i = $derived(idx(suggestion.a));
  const j = $derived(idx(suggestion.b));
  const chain = $derived(i >= 0 && j >= 0 ? strongestChain(report.matrix, i, j) : null);
  const ratio = $derived(report.weights && i >= 0 && j >= 0 ? report.weights[i] / report.weights[j] : null);
  const ranked = $derived(
    (report.weights ?? []).map((w, k) => ({ name: name(k), w })).sort((x, y) => y.w - x.w),
  );
  const within = $derived(suggestion.crAfter <= report.threshold);
  const say = (v: number, a: number, b: number) => describe(v, scale, kind, name(a), name(b), on);
</script>

<!-- Opened with showModal() (see the effect above), never the open attribute:
     only a modal dialog sits in the top layer, above sticky headers, with a backdrop. -->
<dialog bind:this={dialog} onclose={onclose} aria-labelledby="why-title">
  <div class="body">
    <header>
      <h2 id="why-title">Why this suggestion?</h2>
      <button class="btn quiet close" aria-label="Close" onclick={onclose}>✕</button>
    </header>

    <p class="said"><strong>You said:</strong> {say(suggestion.current, i, j)} ({fraction(suggestion.current)}).</p>

    {#if chain}
      <section>
        <p>But two of your other answers point another way:</p>
        <ul>
          <li>{say(report.matrix[i][chain.via], i, chain.via)}.</li>
          <li>{say(report.matrix[chain.via][j], chain.via, j)}.</li>
        </ul>
        <p class="together">Together, these mean {should(chain.implied, scale, kind, name(i), name(j), on)}.</p>
      </section>
    {/if}

    {#if ratio !== null}
      <section>
        <p><strong>All your answers, weighed together:</strong></p>
        <ol class="weights">
          {#each ranked as r}
            <li>
              <span class="n">{r.name}</span>
              <span class="track"><span class="bar" style:width="{(r.w / ranked[0].w) * 100}%"></span></span>
              <span class="v">{percent(r.w, 0)}</span>
            </li>
          {/each}
        </ol>
        <p>
          That puts {ratio >= 1 ? `${name(i)} at about ${ratio.toFixed(1)}× ${name(j)}` : `${name(j)} at about ${(1 / ratio).toFixed(1)}× ${name(i)}`}.
          The closest point on the scale is the suggestion: {say(suggestion.suggested, i, j)} ({fraction(suggestion.suggested)}).
        </p>
      </section>
    {/if}

    <p class="effect" class:good={within}>
      If you use the suggestion, the consistency ratio goes from {report.cr.toFixed(2)} to {suggestion.crAfter.toFixed(2)},
      {within ? `within the ${report.threshold.toFixed(2)} limit` : `still above the ${report.threshold.toFixed(2)} limit`}.
    </p>

    <p class="small muted note">
      Suggestions point at the answers that fit least with the rest. If your answer is what you believe, keep it, and change one of
      the others instead.
    </p>

  </div>
  <footer>
    <button
      class="btn primary"
      onclick={() => {
        onuse();
        onclose();
      }}>Use the suggestion</button
    >
    <button class="btn" onclick={onclose}>Keep my answer</button>
  </footer>
</dialog>

<style>
  dialog {
    width: min(34rem, calc(100vw - 2rem));
    max-height: calc(100vh - 2rem);
    padding: 0;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-m);
    background: var(--paper);
    color: var(--ink);
    box-shadow: var(--shadow-lift);
  }
  /* Explanation scrolls; the two actions stay in view, even on a phone. */
  dialog[open] {
    display: flex;
    flex-direction: column;
  }
  dialog::backdrop {
    background: rgb(16 22 29 / 0.55);
  }
  .body {
    display: grid;
    gap: 1rem;
    padding: 1.25rem 1.4rem 1rem;
    overflow-y: auto;
    min-height: 0;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: start;
    gap: 1rem;
  }
  .close {
    padding: 0.2rem 0.5rem;
  }
  section {
    display: grid;
    gap: 0.45rem;
  }
  ul {
    margin: 0;
    padding-left: 1.2rem;
    display: grid;
    gap: 0.2rem;
  }
  .together {
    font-weight: 600;
    padding-left: 0.75rem;
    border-left: 3px solid var(--rust);
  }
  .weights {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.3rem;
  }
  .weights li {
    display: grid;
    grid-template-columns: minmax(6rem, 10rem) 1fr 3rem;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.9rem;
  }
  .track {
    height: 8px;
  }
  .bar {
    display: block;
    height: 100%;
    border-radius: 0 4px 4px 0;
    background: var(--brass);
  }
  .v {
    text-align: right;
    color: var(--ink-2);
  }
  .effect {
    padding: 0.6rem 0.8rem;
    border-radius: var(--radius-s);
    background: var(--rust-wash);
  }
  .effect.good {
    background: color-mix(in oklab, var(--good) 14%, var(--paper));
  }
  .note {
    max-width: none;
  }
  footer {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
    padding: 0.85rem 1.4rem;
    border-top: 1px solid var(--line);
    background: var(--paper);
  }
</style>
