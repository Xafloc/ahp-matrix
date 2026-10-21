<script lang="ts">
  import Library from './components/Library.svelte';
  import Hidden from './components/Hidden.svelte';
  import Report from './components/Report.svelte';
  import Workspace from './components/Workspace.svelte';
  import { untrack } from 'svelte';
  import { ProjectSession } from './lib/session.svelte';
  import { router } from './lib/router.svelte';

  let session = $state<ProjectSession | null>(null);
  let loadError = $state('');
  // False while the project being left is still saving, so the next screen
  // (which reads from the server) never shows stale data.
  let settled = $state(true);
  let leaving: Promise<void> = Promise.resolve();
  // A project left with edits the server wouldn't take, kept so the person can go back to it.
  let stranded = $state<ProjectSession | null>(null);

  const projectId = $derived(router.route.name === 'library' ? '' : router.route.id);

  // Open the routed project; save the previous one before switching away.
  $effect(() => {
    const id = projectId;
    const previous = untrack(() => session);
    if (previous && previous.project.id !== id) {
      settled = false;
      leaving = previous.flush().finally(() => {
        if (previous.unsaved) stranded = previous;
        settled = true;
      });
    }
    if (!id) {
      session = null;
      return;
    }
    if (previous?.project.id === id) return;
    loadError = '';
    session = null;
    leaving.then(() => ProjectSession.open(id)).then(
      (s) => {
        if (projectId === id) session = s;
      },
      (e) => (loadError = e instanceof Error ? e.message : String(e)),
    );
  });

  // A new screen starts at the top, like a page load would.
  $effect(() => {
    void router.route;
    window.scrollTo(0, 0);
  });

  function goBack() {
    session = stranded;
    stranded = null;
    history.back();
  }

  function beforeUnload(e: BeforeUnloadEvent) {
    if (session?.unsaved) {
      void session.flush();
      e.preventDefault();
      e.returnValue = ''; // older browsers need this to show the prompt
    }
  }
</script>

<svelte:window onbeforeunload={beforeUnload} />

{#if !settled}
  <main class="message" aria-busy="true"><p class="muted">Saving…</p></main>
{:else if stranded}
  <main class="message">
    <h1>Your changes to {stranded.project.name} aren’t saved</h1>
    <p>{stranded.error}</p>
    <div class="choices">
      <button class="btn primary" onclick={goBack}>Go back to {stranded.project.name}</button>
      <button class="btn quiet" onclick={() => (stranded = null)}>Leave without saving</button>
    </div>
  </main>
{:else if router.route.name === 'library'}
  <Library />
{:else if loadError}
  <main class="message">
    <h1>This project couldn't be opened</h1>
    <p>{loadError}</p>
    <a class="btn" href="#/">Back to all decisions</a>
  </main>
{:else if !session}
  <main class="message" aria-busy="true"><p class="muted">Opening…</p></main>
{:else if router.route.name === 'report' && session.hidesResults}
  <main class="message"><Hidden {session} /></main>
{:else if router.route.name === 'report'}
  <Report {session} />
{:else}
  <Workspace {session} step={router.route.step} sub={router.route.sub} pair={router.route.pair ?? ''} />
{/if}

<style>
  .choices {
    display: flex;
    gap: 0.5rem;
  }
  .message {
    max-width: 40rem;
    margin: 6rem auto;
    padding: 0 1rem;
    display: grid;
    gap: 1rem;
    justify-items: start;
  }
</style>
