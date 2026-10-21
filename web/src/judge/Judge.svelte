<script lang="ts">
  // The judging page: a copy of a decision sent to one person, answered in
  // the browser with no server. Consistency is checked in the page, progress
  // is kept in the browser, and Finish saves a file to send back.
  import { untrack } from 'svelte';
  import Mark from '../components/Mark.svelte';
  import PairCompare from '../components/PairCompare.svelte';
  import RatingRows from '../components/RatingRows.svelte';
  import ThemeToggle from '../components/ThemeToggle.svelte';
  import { getRating, setRating } from '../lib/edits';
  import { colorOf, count } from '../lib/format';
  import { answersFile, judgeSteps, minutes, readPacket, type JudgeStep } from '../lib/judge';
  import { LocalSession } from '../lib/local.svelte';
  import { describeTasks, sender } from '../lib/people';
  import { pairProgress } from '../lib/progress';

  let { packet }: { packet: string | null } = $props();

  // The page's decision is read once; the session then owns it.
  const read = readPacket(untrack(() => packet));
  const session = 'project' in read ? LocalSession.restore(read.project) : null;
  const p = $derived(session?.project);
  const me = $derived(p?.evaluators[0]);
  const from = $derived(sender(p?.packet));
  const steps = $derived(p && me ? judgeSteps(p, me) : []);
  // Only this person's own answers mean they've started (earlier rounds or saved progress).
  const started = $derived(!!p && !!me && [...p.pairwise, ...p.ratings].some((x) => x.evaluatorId === me.id));

  let phase = $state<'welcome' | 'steps' | 'check' | 'done'>('welcome');
  let at = $state(0);
  const step = $derived(steps[at]);

  function progressOf(s: JudgeStep) {
    if (!p || !me) return { done: 0, total: 0 };
    if (s.kind === 'rating') return { done: s.items.filter((a) => getRating(p, me.id, s.criterionId, a.id) !== undefined).length, total: s.items.length };
    return pairProgress(p, me.id, s.context, s.items.map((i) => i.id));
  }
  const unfinished = $derived(steps.filter((s) => {
    const x = progressOf(s);
    return x.done < x.total;
  }));
  const answered = $derived(steps.reduce((n, s) => n + progressOf(s).done, 0));
  const total = $derived(steps.reduce((n, s) => n + progressOf(s).total, 0));

  const alts = $derived((p?.alternatives ?? []).map((a) => ({ id: a.id, name: a.name, color: colorOf(a.color) })));

  function go(i: number) {
    at = Math.max(0, Math.min(i, steps.length - 1));
    phase = 'steps';
    window.scrollTo?.(0, 0);
  }

  function finish() {
    phase = unfinished.length ? 'check' : 'done';
    if (phase === 'done') save();
  }

  let file = $state('');
  function save() {
    if (!session) return;
    const blob = new Blob([JSON.stringify($state.snapshot(session.project), null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = file = answersFile(session.project);
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    phase = 'done';
  }
</script>

<header class="bar">
  <Mark size={26} />
  <span class="name">{p?.name ?? 'AHP Matrix'}</span>
  <span class="spacer"></span>
  {#if phase === 'steps' && total}<span class="small muted count">{answered} of {total} answered</span>{/if}
  <ThemeToggle />
</header>

{#if 'error' in read || !p || !me}
  <main class="welcome">
    <h1>Nothing to judge here</h1>
    <p>{'error' in read ? read.error : ''}</p>
  </main>
{:else if phase === 'welcome'}
  <main class="welcome">
    <h1>{from ? `${from} asked for your view on ${p.name}` : `Your view on ${p.name}`}</h1>
    {#if p.description}<p class="lede">{p.description}</p>{/if}
    <p>
      You're answering as {me.name}: {describeTasks(p, me.tasks)}. It takes about {count(minutes(p, me), 'minute')}.
    </p>
    <ol class="plan">
      {#each steps as s (s.key)}<li>{s.title}</li>{/each}
    </ol>
    <p class="small muted">
      Answer from your own view; you won't see anyone else's answers. When you finish, this page saves a file for you to send back{from ? ` to ${from}` : ''}.
      Nothing is sent anywhere by itself.
    </p>
    <div class="actions">
      <button class="btn primary" onclick={() => go(started ? steps.findIndex((s) => unfinished.includes(s)) : 0)}>
        {started ? 'Continue where you left off' : 'Start'}
      </button>
    </div>
  </main>
{:else if phase === 'steps' && step}
  <div class="judging">
    <nav class="steps" aria-label="Steps">
      <ol>
        {#each steps as s, i (s.key)}
          {@const x = progressOf(s)}
          <li>
            <button class:current={i === at} aria-current={i === at ? 'step' : undefined} onclick={() => go(i)}>
              <span>{s.title}</span>
              <span class="small" class:complete={x.done === x.total}>{x.done === x.total ? 'Done' : `${x.done}/${x.total}`}</span>
            </button>
          </li>
        {/each}
      </ol>
    </nav>
    <main class="main">
      <p class="small muted">Step {at + 1} of {steps.length}</p>
      {#key step.key}
        {#if step.kind === 'rating'}
          <h1>{step.title}</h1>
          <p class="muted">1 is poor, 10 is excellent.</p>
          <RatingRows
            items={alts}
            get={(id) => getRating(p, me.id, step.criterionId, id)}
            set={(id, v) => {
              setRating(session!.project, me.id, step.criterionId, id, v);
              session!.changed();
            }}
          />
        {:else}
          <h1>{step.title.replace('?', ', and by how much?')}</h1>
          <PairCompare
            session={session!}
            context={step.context}
            items={step.kind === 'weigh' ? step.items : alts}
            kind={step.kind === 'weigh' ? 'criteria' : 'alternatives'}
            on={step.kind === 'weigh' ? '' : p.criteria.find((c) => c.id === step.criterionId)?.name}
          />
        {/if}
      {/key}
      <div class="nav">
        <button class="btn quiet" disabled={at === 0} onclick={() => go(at - 1)}>Back</button>
        {#if at < steps.length - 1}
          <button class="btn primary" onclick={() => go(at + 1)}>Next step</button>
        {:else}
          <button class="btn primary" onclick={finish}>Finish and send back</button>
        {/if}
      </div>
    </main>
  </div>
{:else if phase === 'check'}
  <main class="welcome">
    <h1>A few answers are still open</h1>
    <p>These steps aren't finished. Unanswered pairs simply don't count, so you can send what you have.</p>
    <ul class="plan">
      {#each unfinished as s (s.key)}
        {@const x = progressOf(s)}
        <li><button class="link" onclick={() => go(steps.indexOf(s))}>{s.title}</button> <span class="muted small">{x.done} of {x.total}</span></li>
      {/each}
    </ul>
    <div class="actions">
      <button class="btn primary" onclick={save}>Finish anyway</button>
      <button class="btn quiet" onclick={() => go(steps.indexOf(unfinished[0]))}>Keep answering</button>
    </div>
  </main>
{:else}
  <main class="welcome" role="status">
    <h1>Thank you</h1>
    <p>
      Your answers are saved as <strong>{file}</strong>, in your downloads. Email it to {from || 'the person who sent you this page'}. That's all;
      you can close this page.
    </p>
    <div class="actions">
      <button class="btn" onclick={save}>Save the file again</button>
      <button class="btn quiet" onclick={() => go(0)}>Change my answers</button>
    </div>
  </main>
{/if}

<style>
  .bar {
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
  .name {
    font-family: var(--font-display);
    font-weight: 650;
    font-size: 1.1rem;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .spacer {
    flex: 1;
  }
  .count {
    white-space: nowrap;
  }
  .welcome {
    max-width: 40rem;
    margin: 0 auto;
    padding: clamp(1.5rem, 6vw, 4rem) 1rem 4rem;
    display: grid;
    gap: 1rem;
  }
  .welcome h1 {
    font-size: clamp(1.8rem, 5vw, 2.6rem);
    line-height: 1.1;
  }
  .lede {
    font-size: 1.1rem;
    color: var(--ink-2);
  }
  .plan {
    margin: 0;
    padding-left: 1.3rem;
    display: grid;
    gap: 0.3rem;
    color: var(--ink-2);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem;
  }
  .link {
    background: none;
    border: 0;
    padding: 0;
    font: inherit;
    color: var(--ink);
    text-decoration: underline;
    text-underline-offset: 3px;
    cursor: pointer;
  }
  .judging {
    display: grid;
    grid-template-columns: 16rem minmax(0, 1fr);
    gap: 2rem;
    padding: 1.5rem clamp(1rem, 3vw, 3rem) 4rem;
    max-width: 78rem;
    margin: 0 auto;
  }
  .steps ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 2px;
    position: sticky;
    top: 4.5rem;
  }
  .steps button {
    width: 100%;
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
    align-items: center;
    text-align: left;
    padding: 0.5rem 0.65rem;
    border: 0;
    border-radius: var(--radius-s);
    background: none;
    font: inherit;
    color: var(--ink);
    cursor: pointer;
  }
  .steps button:hover {
    background: color-mix(in oklab, var(--paper) 60%, var(--fog));
  }
  .steps button.current {
    background: var(--paper);
    box-shadow: inset 3px 0 0 var(--brass);
  }
  .steps .small {
    flex: none;
    color: var(--ink-3);
  }
  .steps .complete {
    color: var(--good);
    font-weight: 600;
  }
  .main {
    display: grid;
    gap: 1.1rem;
    max-width: 54rem;
    align-content: start;
  }
  .main > p {
    margin: 0;
  }
  .nav {
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
    padding-top: 1rem;
    border-top: 1px solid var(--line);
  }
  @media (max-width: 820px) {
    .judging {
      grid-template-columns: minmax(0, 1fr);
      padding-top: 1rem;
    }
    .steps {
      display: none;
    }
  }
</style>
