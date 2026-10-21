<script lang="ts">
  // Shows what merging a returned file would do (who, how many answers,
  // whose answers it replaces, what can't come across) before doing it.
  import { api } from '../lib/api';
  import { count, list, relativeTime } from '../lib/format';
  import type { MergePreview, MergeReport, Project } from '../lib/types';

  let {
    target,
    file,
    onclose,
    ondone,
  }: {
    target: { id: string; name: string };
    file: File;
    onclose: () => void;
    ondone: (r: { project: Project; report: MergeReport; before: Project }) => void;
  } = $props();

  let dialog = $state<HTMLDialogElement>();
  let preview = $state<MergePreview | null>(null);
  let error = $state('');
  let busy = $state(false);

  $effect(() => {
    if (dialog && !dialog.open) dialog.showModal();
  });

  $effect(() => {
    api.previewMerge(target.id, file).then(
      (p) => (preview = p),
      (e) => (error = `${file.name} can't be merged into ${target.name}: ${e instanceof Error ? e.message : e}`),
    );
  });

  const names = $derived(preview?.people.map((x) => x.name) ?? []);
  const replacing = $derived(preview?.people.filter((x) => x.replacesExisting) ?? []);
  const fresh = $derived((preview?.people.length ?? 0) - replacing.length);

  async function merge(policy?: 'replace' | 'skip') {
    busy = true;
    error = '';
    try {
      const before = await api.getProject(target.id);
      const { project, report } = await api.mergeFile(target.id, file, policy);
      ondone({ project, report, before });
    } catch (e) {
      error = `${file.name} couldn't be merged: ${e instanceof Error ? e.message : e}`;
    } finally {
      busy = false;
    }
  }
</script>

<!-- Opened with showModal(), never the open attribute, so it sits above everything with a backdrop. -->
<dialog bind:this={dialog} onclose={onclose} aria-labelledby="merge-title">
  <div class="body">
    <header>
      <h2 id="merge-title">
        {#if preview && names.length}
          Merge {list(names)}'s answers into <em>{target.name}</em>?
        {:else}
          Merge into <em>{target.name}</em>
        {/if}
      </h2>
      <button class="btn quiet close" aria-label="Close" onclick={onclose}>✕</button>
    </header>

    {#if error}
      <p class="problem" role="alert">{error}</p>
    {:else if !preview}
      <p class="muted">Reading {file.name}…</p>
    {:else if preview.people.length === 0}
      <p>{file.name} doesn't have anyone's answers in it.</p>
    {:else}
      <ul class="people">
        {#each preview.people as person (person.id)}
          <li>
            <strong>{person.name}</strong>
            <span>{count(person.judgments, 'answer')}</span>
            <span class="small" class:replace={person.replacesExisting}>
              {#if person.replacesExisting}
                Replaces the answers they already have here{#if person.previouslyReturnedAt}, returned {relativeTime(person.previouslyReturnedAt)}{/if}
              {:else}
                New here
              {/if}
            </span>
          </li>
        {/each}
      </ul>

      {#if preview.outOfDate}
        <p class="warn">
          This copy was made before the decision last changed. Answers about things that still exist will come across, but anything added since is
          still to judge: send an updated copy from the People step.
        </p>
      {/if}

      {#if preview.dropped.length}
        <details class="dropped" open={preview.dropped.length <= 4}>
          <summary>{count(preview.dropped.length, 'answer')} can't come across</summary>
          <ul class="small">
            {#each preview.dropped as line}<li>{line}</li>{/each}
          </ul>
        </details>
      {/if}
    {/if}
  </div>
  <footer>
    {#if preview && preview.people.length && !error}
      {#if replacing.length}
        <button class="btn primary" disabled={busy} onclick={() => merge('replace')}>Replace their answers</button>
        {#if fresh > 0}
          <button class="btn" disabled={busy} onclick={() => merge('skip')}>Keep current, add the new ones</button>
        {:else}
          <button class="btn" disabled={busy} onclick={onclose}>Keep current</button>
        {/if}
      {:else}
        <button class="btn primary" disabled={busy} onclick={() => merge()}>Merge</button>
      {/if}
    {/if}
    <button class="btn quiet" onclick={onclose}>Cancel</button>
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
  h2 em {
    font-style: normal;
    color: var(--brass-ink);
  }
  .close {
    padding: 0.2rem 0.5rem;
  }
  .people {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  .people li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.25rem 0.75rem;
    padding: 0.55rem 0;
    border-bottom: 1px solid var(--line);
  }
  .people .small {
    flex-basis: 100%;
    color: var(--ink-2);
  }
  .people .replace {
    color: var(--brass-ink);
  }
  .warn,
  .problem {
    margin: 0;
    padding: 0.6rem 0.8rem;
    border-radius: var(--radius-s);
    background: var(--rust-wash);
  }
  .dropped summary {
    cursor: pointer;
    font-weight: 600;
  }
  .dropped ul {
    margin: 0.4rem 0 0;
    padding-left: 1.2rem;
    display: grid;
    gap: 0.2rem;
    color: var(--ink-2);
  }
  footer {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding: 0.85rem 1.4rem 1.1rem;
    border-top: 1px solid var(--line);
  }
</style>
