<script lang="ts">
  import { api, exportUrl } from '../lib/api';
  import { colorOf, percent, relativeTime } from '../lib/format';
  import { href, router } from '../lib/router.svelte';
  import type { ProjectSummary } from '../lib/types';
  import Mark from './Mark.svelte';
  import ThemeToggle from './ThemeToggle.svelte';

  let projects = $state<ProjectSummary[] | null>(null);
  let error = $state('');
  let creating = $state(false);
  let newName = $state('');
  let confirmDelete = $state('');
  let query = $state('');
  let fileInput = $state<HTMLInputElement | undefined>();

  const shown = $derived(
    (projects ?? []).filter((p) => !query.trim() || p.name.toLowerCase().includes(query.trim().toLowerCase())),
  );

  async function load() {
    try {
      projects = await api.listProjects();
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
  load();

  async function create(e: SubmitEvent) {
    e.preventDefault();
    try {
      const p = await api.createProject(newName.trim());
      router.go(href(p.id, 'setup'));
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    }
  }

  async function act(fn: () => Promise<unknown>) {
    try {
      await fn();
      await load();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function importChosen() {
    const file = fileInput?.files?.[0];
    if (!file) return;
    try {
      const p = await api.importFile(file);
      router.go(href(p.id, 'results'));
    } catch (e) {
      error = `${file.name} couldn't be imported: ${e instanceof Error ? e.message : e}`;
    }
    if (fileInput) fileInput.value = '';
  }

  // Ring geometry for the completeness indicator.
  const R = 9;
  const C = 2 * Math.PI * R;
</script>

<header class="top">
  <a class="brand" href="#/"><Mark /> <span>AHP Matrix</span></a>
  <ThemeToggle />
</header>

<main class="library">
  <div class="intro">
    <h1>Decisions</h1>
    <div class="actions">
      <button class="btn" onclick={() => fileInput?.click()}>Import file</button>
      <input bind:this={fileInput} type="file" accept=".json,application/json" class="sr-only" onchange={importChosen} tabindex="-1" />
      <button class="btn primary" onclick={() => (creating = true)}>New decision</button>
    </div>
  </div>

  {#if creating}
    <form class="new panel" onsubmit={create}>
      <label for="new-name">What are you deciding?</label>
      <!-- svelte-ignore a11y_autofocus -->
      <input id="new-name" class="field" bind:value={newName} placeholder="Choose a CRM vendor" autofocus />
      <button class="btn primary" type="submit">Create</button>
      <button class="btn quiet" type="button" onclick={() => (creating = false)}>Cancel</button>
    </form>
  {/if}

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if projects === null}
    {#if !error}<p class="muted">Loading…</p>{/if}
  {:else if projects.length === 0}
    <section class="empty">
      <Mark size={88} tilt={0} />
      <h2>Weigh your first decision</h2>
      <p class="muted">
        List the options, decide what matters, and compare them two at a time. AHP Matrix turns those comparisons into a
        ranking, and tells you when your answers contradict each other.
      </p>
      <button class="btn primary" onclick={() => (creating = true)}>New decision</button>
    </section>
  {:else}
    {#if projects.length > 6}
      <input class="field search" type="search" placeholder="Find a decision" bind:value={query} aria-label="Find a decision" />
    {/if}
    <ul class="rows">
      {#each shown as p (p.file)}
        <li class="row" class:broken={!!p.error}>
          {#if p.error}
            <div class="name">
              <span>{p.file}</span>
              <span class="small rust">{p.error}</span>
            </div>
          {:else}
            <a class="name" href={href(p.id!, p.alternatives && p.criteria ? 'results' : 'setup')}>
              <span class="title">{p.name}</span>
              <span class="small muted">
                {#if p.ranking.length && p.alternatives >= 2 && p.criteria > 0}
                  {p.ranking[0].name} leads with {percent(p.ranking[0].score, 0)}
                {:else}
                  {p.alternatives} options, {p.criteria} criteria
                {/if}
              </span>
            </a>
            <div class="strip" aria-hidden="true">
              {#each p.ranking as r}
                <span style:flex-grow={Math.max(r.score, 0.0001)} style:background={colorOf(r.color)} title="{r.name} {percent(r.score)}"></span>
              {/each}
            </div>
            <div class="status small">
              {#if p.alternatives < 2 || p.criteria === 0}
                <span class="muted">Not set up yet</span>
              {:else}
              <svg width="24" height="24" viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="12" r={R} fill="none" stroke="var(--line)" stroke-width="3" />
                <circle
                  cx="12"
                  cy="12"
                  r={R}
                  fill="none"
                  stroke="var(--brass)"
                  stroke-width="3"
                  stroke-dasharray="{C * p.completeness} {C}"
                  transform="rotate(-90 12 12)"
                  stroke-linecap="round"
                />
              </svg>
              <span>{percent(p.completeness, 0)} judged</span>
              {#if !p.consistent}<span class="rust">Contradictions to review</span>{/if}
              {/if}
            </div>
            <span class="small muted when">{relativeTime(p.updatedAt)}</span>
            <div class="more">
              {#if confirmDelete === p.id}
                <span class="small">Move to trash?</span>
                <button class="btn quiet danger" onclick={() => act(() => api.deleteProject(p.id!))}>Delete</button>
                <button class="btn quiet" onclick={() => (confirmDelete = '')}>Keep</button>
              {:else}
                <button class="btn quiet" onclick={() => act(() => api.duplicateProject(p.id!))}>Duplicate</button>
                <a class="btn quiet" href={exportUrl(p.id!)} download>Export</a>
                <button class="btn quiet" onclick={() => (confirmDelete = p.id!)}>Delete</button>
              {/if}
            </div>
          {/if}
        </li>
      {/each}
    </ul>
    {#if shown.length === 0}<p class="muted">No decision matches “{query}”.</p>{/if}
  {/if}
</main>

<style>
  .top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.9rem clamp(1rem, 4vw, 2.5rem);
  }
  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    text-decoration: none;
    font-family: var(--font-display);
    font-weight: 700;
    font-size: 1.1rem;
    letter-spacing: -0.01em;
  }
  .library {
    max-width: 72rem;
    margin: 0 auto;
    padding: 1.5rem clamp(1rem, 4vw, 2.5rem) 4rem;
    display: grid;
    gap: 1.25rem;
  }
  .intro {
    display: flex;
    justify-content: space-between;
    align-items: end;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .intro h1 {
    font-size: clamp(2.2rem, 5vw, 3.4rem);
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .new {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    align-items: center;
    gap: 0.6rem;
    padding: 0.9rem 1rem;
  }
  .new label {
    font-weight: 600;
  }
  .error {
    color: var(--rust);
  }
  .empty {
    display: grid;
    justify-items: start;
    gap: 0.9rem;
    padding: 3rem 0;
    max-width: 36rem;
  }
  .search {
    max-width: 22rem;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  .row {
    display: grid;
    grid-template-columns: minmax(12rem, 2fr) minmax(8rem, 1.4fr) minmax(9rem, 1fr) 7rem auto;
    align-items: center;
    gap: 1.25rem;
    padding: 0.9rem 0.25rem;
    border-bottom: 1px solid var(--line);
  }
  .row:hover {
    background: color-mix(in oklab, var(--paper) 55%, var(--fog));
  }
  .name {
    display: grid;
    text-decoration: none;
    min-width: 0;
  }
  .title {
    font-weight: 650;
    font-size: 1.05rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  a.name:hover .title {
    text-decoration: underline;
    text-decoration-color: var(--brass);
    text-underline-offset: 3px;
  }
  .strip {
    display: flex;
    gap: 2px;
    height: 10px;
    border-radius: 3px;
    overflow: hidden;
    background: var(--line);
  }
  .strip span {
    flex-basis: 0;
    min-width: 2px;
  }
  .status {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
  }
  .rust {
    color: var(--rust);
  }
  .more {
    display: flex;
    gap: 0.15rem;
    align-items: center;
    justify-content: end;
    opacity: 0.55;
  }
  .row:hover .more,
  .more:focus-within {
    opacity: 1;
  }
  .broken {
    grid-template-columns: 1fr;
  }
  @media (max-width: 860px) {
    .row {
      grid-template-columns: 1fr auto;
    }
    .strip {
      grid-column: 1 / -1;
      order: 3;
    }
    .status,
    .when {
      display: none;
    }
    .new {
      grid-template-columns: 1fr;
    }
  }
</style>
