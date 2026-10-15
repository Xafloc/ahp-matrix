<script lang="ts">
  import { scoreProgress, weighProgress } from '../lib/progress';
  import { href, type Step } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import Mark from './Mark.svelte';
  import Results from './Results.svelte';
  import Score from './Score.svelte';
  import Sensitivity from './Sensitivity.svelte';
  import Setup from './Setup.svelte';
  import ThemeToggle from './ThemeToggle.svelte';
  import Weigh from './Weigh.svelte';

  let { session, step, sub }: { session: ProjectSession; step: Step; sub: string } = $props();
  const p = $derived(session.project);

  const weigh = $derived(weighProgress(p, session.scoringAs));
  const score = $derived(scoreProgress(p, session.scoringAs));
  const ready = $derived(p.alternatives.length >= 2 && p.criteria.length >= 1);

  const steps = $derived([
    { id: 'setup' as Step, label: 'Set up', detail: `${p.alternatives.length} options, ${p.criteria.length} criteria`, done: ready },
    { id: 'weigh' as Step, label: 'Weigh criteria', detail: weigh.total ? `${weigh.done} of ${weigh.total} compared` : 'Nothing to compare yet', done: weigh.total > 0 && weigh.done === weigh.total },
    { id: 'score' as Step, label: 'Score options', detail: score.total ? `${score.done} of ${score.total} judged` : 'Nothing to score yet', done: score.total > 0 && score.done === score.total },
    { id: 'results' as Step, label: 'Results', detail: '', done: false },
    { id: 'sensitivity' as Step, label: 'What if', detail: 'How robust is the winner?', done: false },
  ]);

  const saveLabel = $derived(
    {
      saved: 'All changes saved',
      pending: 'Saving…',
      saving: 'Saving…',
      conflict: 'Not saved',
      error: 'Not saved',
    }[session.saveState],
  );
</script>

<div class="shell">
  <header class="bar no-print">
    <a class="home" href="#/" title="All decisions" aria-label="All decisions"><Mark size={26} /></a>
    <input
      class="field inline title"
      aria-label="Decision name"
      bind:value={session.project.name}
      oninput={() => session.changed()}
    />
    <span class="save small" class:warn={session.saveState === 'error' || session.saveState === 'conflict'} aria-live="polite">{saveLabel}</span>
    <span class="spacer"></span>
    {#if p.evaluators.length > 1}
      <label class="small who">
        Judging as
        <select class="field" bind:value={session.scoringAs}>
          {#each p.evaluators as e (e.id)}<option value={e.id}>{e.name}</option>{/each}
        </select>
      </label>
    {/if}
    <ThemeToggle />
  </header>

  {#if session.saveState === 'conflict'}
    <div class="banner no-print" role="alert">
      <span>{session.error} Keep your version, or load the other one and lose your recent edits?</span>
      <button class="btn" onclick={() => session.overwrite()}>Keep mine</button>
      <button class="btn quiet" onclick={() => session.reload()}>Load the other version</button>
    </div>
  {:else if session.saveState === 'error'}
    <div class="banner no-print" role="alert">
      <span><strong>Not saved.</strong> {session.error}</span>
      <button class="btn" onclick={() => session.retry()}>Try again</button>
    </div>
  {/if}

  <nav class="rail no-print" aria-label="Steps">
    <ol>
      {#each steps as s, i (s.id)}
        <li>
          <a href={href(p.id, s.id)} class:current={s.id === step} aria-current={s.id === step ? 'page' : undefined}>
            <span class="num" class:done={s.done}>{#if s.done}✓{:else}{i + 1}{/if}</span>
            <span class="label">
              <span>{s.label}</span>
              {#if s.detail}<span class="small muted">{s.detail}</span>{/if}
            </span>
          </a>
        </li>
      {/each}
    </ol>
    <a class="btn quiet small report" href={href(p.id, 'report')}>Printable report</a>
  </nav>

  <main class="content">
    {#if step === 'setup'}
      <Setup {session} />
    {:else if step === 'weigh'}
      <Weigh {session} {sub} />
    {:else if step === 'score'}
      <Score {session} {sub} />
    {:else if step === 'results'}
      <Results {session} />
    {:else}
      <Sensitivity {session} {sub} />
    {/if}
  </main>
</div>

<style>
  .shell {
    display: grid;
    grid-template-columns: 15.5rem 1fr;
    grid-template-rows: auto auto 1fr;
    grid-template-areas: 'bar bar' 'banner banner' 'rail content';
    min-height: 100vh;
  }
  .bar {
    grid-area: bar;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem clamp(0.75rem, 2vw, 1.5rem);
    border-bottom: 1px solid var(--line);
    background: var(--paper);
    position: sticky;
    top: 0;
    z-index: 5;
  }
  .home {
    display: inline-flex;
  }
  .title {
    font-family: var(--font-display);
    font-weight: 650;
    font-size: 1.15rem;
    max-width: 28rem;
  }
  .save {
    color: var(--ink-3);
    white-space: nowrap;
  }
  .save.warn {
    color: var(--rust);
  }
  .spacer {
    flex: 1;
  }
  .who {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--ink-2);
  }
  .who select {
    width: auto;
  }
  .banner {
    grid-area: banner;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 1.5rem;
    background: var(--rust-wash);
    border-bottom: 1px solid var(--line);
    flex-wrap: wrap;
  }
  .rail {
    grid-area: rail;
    position: sticky;
    top: 3.3rem;
    align-self: start;
    max-height: calc(100vh - 3.3rem);
    overflow-y: auto;
    padding: 1.5rem 0.75rem;
    border-right: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .rail ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.15rem;
  }
  .rail a {
    display: flex;
    gap: 0.7rem;
    align-items: flex-start;
    padding: 0.55rem 0.6rem;
    border-radius: var(--radius-s);
    text-decoration: none;
  }
  .rail a:hover {
    background: color-mix(in oklab, var(--paper) 60%, var(--fog));
  }
  .rail a.current {
    background: var(--paper);
    box-shadow: inset 3px 0 0 var(--brass);
  }
  .label {
    display: grid;
    line-height: 1.3;
    font-weight: 550;
  }
  .num {
    flex: none;
    width: 1.5rem;
    height: 1.5rem;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-size: 0.78rem;
    font-weight: 700;
    border: 1.5px solid var(--line-strong);
    color: var(--ink-2);
  }
  .num.done {
    background: var(--brass-ink);
    border-color: var(--brass-ink);
    color: var(--paper);
  }
  .report {
    align-self: flex-start;
  }
  .content {
    grid-area: content;
    padding: 2rem clamp(1rem, 3vw, 3rem) 4rem;
    min-width: 0;
  }
  @media (max-width: 820px) {
    .shell {
      grid-template-columns: 1fr;
      grid-template-areas: 'bar' 'banner' 'rail' 'content';
    }
    .rail {
      position: static;
      max-height: none;
      border-right: 0;
      border-bottom: 1px solid var(--line);
      padding: 0.5rem;
      overflow-x: auto;
    }
    .rail ol {
      grid-auto-flow: column;
    }
    .label .small,
    .report {
      display: none;
    }
    .who {
      display: none;
    }
  }
  @media print {
    .shell {
      display: block;
    }
  }
</style>
