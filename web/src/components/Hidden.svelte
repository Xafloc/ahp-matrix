<script lang="ts">
  // In place of results while the person judging still has answers to give.
  import { count } from '../lib/format';
  import { nextTask } from '../lib/independence';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { ALL_EVALUATORS } from '../lib/types';

  let { session, back = false }: { session: ProjectSession; back?: boolean } = $props();
  const p = $derived(session.project);
  const me = $derived(p.evaluators.find((e) => e.id === session.scoringAs));
  const prog = $derived(session.results[ALL_EVALUATORS]?.people.find((x) => x.id === session.scoringAs));
  const next = $derived(me ? nextTask(p, me) : null);
</script>

<section class="hidden">
  <h1>Results are hidden until {me?.name} finishes, so others' answers don't sway theirs.</h1>
  {#if prog}<p class="muted">{count(prog.total - prog.done, 'answer')} to go.</p>{/if}
  <div class="actions">
    {#if next}<a class="btn primary" href={href(p.id, next.step, next.sub)}>Go to the next question</a>{/if}
    {#if back}<a class="btn quiet" href={href(p.id, 'people')}>Back to the decision</a>{/if}
    <button class="btn quiet" onclick={() => session.revealed.push(session.scoringAs)}>Show anyway</button>
  </div>
</section>

<style>
  .hidden {
    display: grid;
    gap: 1rem;
    max-width: 38rem;
    padding-top: 1rem;
  }
  h1 {
    font-size: clamp(1.5rem, 3.5vw, 2rem);
    line-height: 1.2;
  }
  p {
    margin: 0;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem;
  }
</style>
