<script lang="ts">
  import { api, exportUrl } from '../lib/api';
  import { colorOf, percent, relativeTime } from '../lib/format';
  import { href, router } from '../lib/router.svelte';
  import { count } from '../lib/format';
  import type { ImportReport, ProjectSummary, StarterEntry } from '../lib/types';
  import Mark from './Mark.svelte';
  import ThemeToggle from './ThemeToggle.svelte';

  let projects = $state<ProjectSummary[] | null>(null);
  let error = $state('');
  let creating = $state(false);
  let newName = $state('');
  let confirmDelete = $state('');
  let query = $state('');
  let fileInput = $state<HTMLInputElement | undefined>();
  let startFrom = $state('');
  let templates = $state<StarterEntry[]>([]);
  let examples = $state<StarterEntry[]>([]);
  let imported = $state<{ id: string; name: string; report: ImportReport } | null>(null);

  api.templates().then((t) => (templates = t), () => {});
  api.samples().then((s) => (examples = s), () => {});

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
      const p = await api.createProject(newName.trim(), startFrom ? { template: startFrom } : {});
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

  async function openExample(id: string) {
    try {
      const p = await api.createProject('', { sample: id });
      router.go(href(p.id, 'results'));
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function importChosen() {
    const file = fileInput?.files?.[0];
    if (!file) return;
    try {
      const { project, report } = await api.importFile(file);
      if (report) {
        // A spreadsheet: say what came across before opening it.
        imported = { id: project.id, name: project.name, report };
        await load();
      } else {
        router.go(href(project.id, 'results'));
      }
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
      <input
        bind:this={fileInput}
        type="file"
        accept=".json,application/json,.xlsx"
        class="sr-only"
        onchange={importChosen}
        tabindex="-1"
        aria-label="Import a decision file or Priority Matrix spreadsheet"
      />
      <button class="btn primary" onclick={() => (creating = true)}>New decision</button>
    </div>
  </div>

  {#if creating}
    <form class="new panel" onsubmit={create}>
      <label for="new-name">What are you deciding?</label>
      <!-- svelte-ignore a11y_autofocus -->
      <input id="new-name" class="field" bind:value={newName} placeholder="Choose a CRM vendor" autofocus />
      <select class="field start" bind:value={startFrom} aria-label="Start from">
        <option value="">Start blank</option>
        {#each templates as t (t.id)}<option value={t.id}>Template: {t.name}</option>{/each}
      </select>
      <button class="btn primary" type="submit">Create</button>
      <button class="btn quiet" type="button" onclick={() => (creating = false)}>Cancel</button>
      {#if startFrom}
        <p class="small muted hint">{templates.find((t) => t.id === startFrom)?.description}</p>
      {/if}
    </form>
  {/if}

  {#if imported}
    <section class="notice panel" role="status">
      <p>
        Imported <strong>{imported.name}</strong>: {count(imported.report.alternatives, 'option')},
        {count(imported.report.criteria, 'criterion', 'criteria')} and {count(imported.report.judgments, 'comparison')}.
      </p>
      {#if imported.report.skipped.length}
        <p class="small">Some cells couldn't be used and were left out:</p>
        <ul class="small">{#each imported.report.skipped as s}<li>{s}</li>{/each}</ul>
      {/if}
      <div class="actions">
        <a class="btn primary" href={href(imported.id, 'results')}>Open it</a>
        <button class="btn quiet" onclick={() => (imported = null)}>Dismiss</button>
      </div>
    </section>
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
      <div class="actions">
        <button class="btn primary" onclick={() => (creating = true)}>New decision</button>
        <button class="btn" onclick={() => fileInput?.click()}>Import a Priority Matrix spreadsheet</button>
      </div>
      {#if examples.length}
        <h3>Or explore an example</h3>
        <ul class="examples">
          {#each examples as ex (ex.id)}
            <li>
              <button class="example" onclick={() => openExample(ex.id)}>
                <strong>{ex.name}</strong>
                <span class="small muted">{ex.description}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
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
                  {count(p.alternatives, 'option')}, {count(p.criteria, 'criterion', 'criteria')}
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
    {#if examples.length}
      <p class="small muted more-examples">
        Examples:
        {#each examples as ex, i (ex.id)}{#if i > 0}, {/if}<button class="link" onclick={() => openExample(ex.id)}>{ex.name}</button>{/each}
      </p>
    {/if}
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
    grid-template-columns: auto 1fr auto auto auto;
    align-items: center;
    gap: 0.6rem;
    padding: 0.9rem 1rem;
  }
  .new label {
    font-weight: 600;
  }
  .start {
    width: auto;
  }
  .hint {
    grid-column: 1 / -1;
  }
  .notice {
    display: grid;
    gap: 0.5rem;
    padding: 1rem 1.2rem;
  }
  .notice ul {
    margin: 0;
    padding-left: 1.2rem;
  }
  .examples {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.5rem;
    width: 100%;
  }
  .example {
    display: grid;
    gap: 0.15rem;
    width: 100%;
    text-align: left;
    padding: 0.75rem 1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    background: var(--paper);
    cursor: pointer;
  }
  .example:hover {
    border-color: var(--brass);
  }
  .link {
    border: 0;
    background: none;
    padding: 0;
    color: var(--ink-2);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
    font: inherit;
  }
  .more-examples {
    margin-top: 0.5rem;
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
