<script lang="ts">
  import { criteriaContext } from '../lib/edits';
  import { count } from '../lib/format';
  import { pairProgress } from '../lib/progress';
  import { href } from '../lib/router.svelte';
  import type { ProjectSession } from '../lib/session.svelte';
  import { weighs } from '../lib/people';
  import { groups, path } from '../lib/tree';
  import PairCompare from './PairCompare.svelte';
  import SubNav from './SubNav.svelte';

  let { session, sub }: { session: ProjectSession; sub: string } = $props();
  const p = $derived(session.project);
  const all = $derived(groups(p));
  const key = (parentId: string) => parentId || 'root';
  const group = $derived(all.find((g) => key(g.parentId) === sub) ?? all[0]);
  const results = $derived(session.results[session.scoringAs]);
  const me = $derived(p.evaluators.find((e) => e.id === session.scoringAs));
  const mine = $derived(weighs(me?.tasks));

  const entries = $derived(
    all.map((g) => {
      const ctx = criteriaContext(g.parentId);
      const prog = pairProgress(p, session.scoringAs, ctx, g.items.map((c) => c.id));
      const report = results?.matrices.find((m) => m.context === ctx);
      return {
        key: key(g.parentId),
        label: g.parentId ? path(p, g.parentId).join(' › ') : 'Top-level criteria',
        href: href(p.id, 'weigh', key(g.parentId)),
        done: prog.done,
        total: prog.total,
        flag: !!report && report.connected && !report.consistent,
        detail: count(g.items.length, 'criterion', 'criteria'),
        off: mine ? undefined : `Not assigned to ${me?.name}`,
      };
    }),
  );
</script>

{#if !group}
  <div class="empty">
    <h1>Nothing to weigh yet</h1>
    <p class="muted">Weighing compares criteria with each other, so you need at least two. Add them in setup.</p>
    <a class="btn primary" href={href(p.id, 'setup')}>Go to setup</a>
  </div>
{:else}
  <div class="layout" class:single={all.length === 1}>
    {#if all.length > 1}
      <SubNav label="Criteria groups" {entries} current={key(group.parentId)} />
    {/if}
    <section class="main">
      {#if p.evaluators.length > 1}<p class="as small">Answering as <strong>{me?.name}</strong></p>{/if}
      {#if !mine}
        <p class="unassigned">{me?.name} isn't asked to weigh the criteria. Anything entered here still counts.</p>
      {/if}
      <h1>
        {group.parentId
          ? `Within ${p.criteria.find((c) => c.id === group.parentId)?.name}, which matters more, and by how much?`
          : 'Which matters more, and by how much?'}
      </h1>
      {#key group.parentId}
        <PairCompare
          {session}
          context={criteriaContext(group.parentId)}
          items={group.items.map((c) => ({ id: c.id, name: c.name }))}
          kind="criteria"
        />
      {/key}
    </section>
  </div>
{/if}

<style>
  .as {
    margin: 0;
    color: var(--ink-2);
  }
  .unassigned {
    margin: 0;
    padding: 0.6rem 0.85rem;
    border-radius: var(--radius-s);
    background: var(--paper);
    border: 1px dashed var(--line-strong);
    color: var(--ink-2);
  }
  .layout {
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    gap: 2rem;
    align-items: start;
  }
  .layout.single {
    grid-template-columns: minmax(0, 1fr);
  }
  .main {
    display: grid;
    gap: 1.25rem;
    max-width: 54rem;
  }
  .empty {
    display: grid;
    gap: 0.9rem;
    justify-items: start;
    max-width: 34rem;
  }
  @media (max-width: 1000px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
</style>
