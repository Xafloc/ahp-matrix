<script lang="ts">
  import { count } from '../lib/format';
  import { scoreProgress, weighProgress } from '../lib/progress';
  import { href, type Step } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { ALL_EVALUATORS, type MergeReport, type Project } from '../lib/types';
  import { api, exportUrl } from '../lib/api';
  import { slug } from '../lib/format';
  import { sender, weighs } from '../lib/people';
  import { mergedText, undoMerge } from '../lib/returned';
  import Mark from './Mark.svelte';
  import MergeDialog from './MergeDialog.svelte';
  import Toast from './Toast.svelte';
  import PeopleStep from './PeopleStep.svelte';
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
  const judge = $derived(p.evaluators.find((e) => e.id === session.scoringAs));
  const ready = $derived(p.alternatives.length >= 2 && p.criteria.length >= 1);
  const people = $derived(session.results[ALL_EVALUATORS]?.people ?? []);
  const finished = $derived(people.filter((x) => x.finished).length);

  const steps = $derived([
    { id: 'setup' as Step, label: 'Set up', detail: `${count(p.alternatives.length, 'option')}, ${count(p.criteria.length, 'criterion', 'criteria')}`, done: ready },
    {
      id: 'people' as Step,
      label: 'People',
      detail: p.evaluators.length === 1 ? 'Just you' : `${count(p.evaluators.length, 'person', 'people')}, ${finished} finished`,
      done: p.evaluators.length > 1 && finished === p.evaluators.length,
    },
    { id: 'weigh' as Step, label: 'Weigh criteria', detail: weigh.total ? `${weigh.done} of ${weigh.total} compared` : weighs(judge?.tasks) ? 'Nothing to compare yet' : 'Not assigned', done: weigh.total > 0 && weigh.done === weigh.total },
    { id: 'score' as Step, label: 'Score options', detail: score.total ? `${score.done} of ${score.total} judged` : judge?.tasks && !judge.tasks.scoreAll && !judge.tasks.score?.length ? 'Not assigned' : 'Nothing to score yet', done: score.total > 0 && score.done === score.total },
    { id: 'results' as Step, label: 'Results', detail: '', done: false },
    { id: 'sensitivity' as Step, label: 'What if', detail: 'How robust is the winner?', done: false },
  ]);

  // Merging returned answers: from the People step's button, or a file dropped anywhere.
  let merging = $state<File | null>(null);
  let dragging = $state(false);
  let toast = $state<{ text: string; undo?: { before: Project; version: number } } | null>(null);
  const hasFiles = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files');

  // A copy someone sent to be judged.
  const received = $derived(p.packet ? { from: sender(p.packet), me: p.evaluators[0]?.name ?? '' } : null);
  let sentBack = $state('');

  async function sendBack() {
    await session.flush();
    if (session.unsaved) {
      toast = { text: 'Fix the unsaved changes first; then send your answers back.' };
      return;
    }
    const a = document.createElement('a');
    a.href = exportUrl(p.id, 'ahp.json');
    a.download = '';
    a.click();
    sentBack = `${slug(p.name)}-answers-from-${slug(received?.me ?? '')}.ahp.json`;
  }

  async function startMerge(file: File) {
    if (received) {
      toast = { text: "This is a copy you were sent, so there's nothing to merge into it." };
      return;
    }
    await session.flush();
    if (session.unsaved) {
      toast = { text: 'Fix the unsaved changes first; then merge.' };
      return;
    }
    merging = file;
  }

  async function merged(r: { project: Project; report: MergeReport; before: Project }) {
    merging = null;
    await session.reload();
    toast = { text: mergedText(r.report), undo: r.report.judgments ? { before: r.before, version: r.project.version } : undefined };
  }

  // Undo restores the decision as it was before the merge, so it's only
  // offered until the decision changes again.
  $effect(() => {
    if (toast?.undo && session.saveState !== 'saved') toast = { text: toast.text };
  });

  async function undo() {
    const u = toast?.undo;
    if (!u) return;
    await session.flush();
    if (session.unsaved || session.project.version !== u.version) {
      toast = { text: "You've changed the decision since merging, so the merge can't be undone." };
      return;
    }
    try {
      await undoMerge(api, u.before, u.version);
      await session.reload();
      toast = { text: 'Merge undone.' };
    } catch (e) {
      toast = { text: `Couldn't undo: ${e instanceof Error ? e.message : e}` };
    }
  }

  $effect(() => {
    if (!toast) return;
    const t = setTimeout(() => (toast = null), toast.undo ? 20000 : 8000);
    return () => clearTimeout(t);
  });

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

<div
  class="shell"
  role="presentation"
  ondragover={(e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    dragging = true;
  }}
  ondragleave={(e) => {
    if (!e.relatedTarget) dragging = false;
  }}
  ondrop={(e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    dragging = false;
    const file = e.dataTransfer?.files[0];
    if (file) startMerge(file);
  }}
>
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
        <span class="initial" aria-hidden="true">{(judge?.name.trim()[0] ?? '?').toUpperCase()}</span>
        Now judging as
        <select class="field" bind:value={session.scoringAs}>
          {#each p.evaluators as e (e.id)}<option value={e.id}>{e.name}</option>{/each}
        </select>
      </label>
    {/if}
    <ThemeToggle />
  </header>

  <div class="banners">
  {#if received}
    <div class="received no-print">
      <p>
        {received.from ? `${received.from} asked you to judge this decision.` : 'You were asked to judge this decision.'}
        You're judging as {received.me}.
      </p>
      <button class="btn primary small" onclick={sendBack}>Send your answers back</button>
      {#if sentBack}
        <p class="small next" role="status">
          Downloaded. Email {sentBack} to {received.from || 'whoever sent you this decision'}. It holds only
          your answers; they'll merge it into theirs.
        </p>
      {/if}
    </div>
  {/if}

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
  </div>

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
    {:else if step === 'people'}
      <PeopleStep {session} onmerge={startMerge} />
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

{#if dragging}
  <div class="drop no-print" aria-hidden="true">
    <p>Drop to merge returned answers into <strong>{p.name}</strong></p>
  </div>
{/if}
{#if merging}
  <MergeDialog target={{ id: p.id, name: p.name }} file={merging} onclose={() => (merging = null)} ondone={merged} />
{/if}
{#if toast}
  <Toast text={toast.text} action={toast.undo ? 'Undo' : ''} onaction={undo} ondismiss={() => (toast = null)} />
{/if}

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
  .who {
    padding: 0.2rem 0.25rem 0.2rem 0.3rem;
    border: 1px solid var(--line);
    border-radius: 999px;
    background: var(--fog);
  }
  .who select {
    width: auto;
    font-weight: 600;
    border-radius: 999px;
  }
  .initial {
    display: grid;
    place-items: center;
    width: 1.6rem;
    height: 1.6rem;
    border-radius: 50%;
    background: var(--ink);
    color: var(--paper);
    font-weight: 700;
    font-size: 0.8rem;
  }
  .banner {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 1.5rem;
    background: var(--rust-wash);
    border-bottom: 1px solid var(--line);
    flex-wrap: wrap;
  }
  .banners {
    grid-area: banner;
  }
  .received {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 1rem;
    padding: 0.6rem clamp(0.75rem, 2vw, 1.5rem);
    background: var(--brass-wash);
    border-bottom: 1px solid var(--line);
  }
  .received p {
    margin: 0;
  }
  .received .next {
    flex-basis: 100%;
    color: var(--ink-2);
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
  .drop {
    position: fixed;
    inset: 0.75rem;
    z-index: 30;
    display: grid;
    place-items: center;
    border: 2px dashed var(--brass);
    border-radius: var(--radius-m);
    background: color-mix(in oklab, var(--paper) 88%, transparent);
    pointer-events: none;
  }
  .drop p {
    font-size: 1.15rem;
    max-width: 24rem;
    text-align: center;
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
    .bar {
      flex-wrap: wrap;
    }
    .title {
      flex: 1 1 10rem;
      min-width: 0;
    }
  }
  @media print {
    .shell {
      display: block;
    }
  }
</style>
