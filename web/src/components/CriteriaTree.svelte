<script lang="ts">
  import { addCriterion, indent, moveCriterion, outdent, removeCriterion, setDirection, setMethod, shift, type Placement } from '../lib/edits';
  import { pairCount } from '../lib/pairs';
  import type { ProjectSession } from '../lib/session.svelte';
  import { childrenOf, descendants, flatten, isLeaf } from '../lib/tree';
  import type { Direction, Method } from '../lib/types';

  let { session }: { session: ProjectSession } = $props();
  const p = $derived(session.project);
  const rows = $derived(flatten(p));

  let newName = $state('');
  let dragging = $state('');
  let target: { id: string; where: Placement } | null = $state(null);
  let confirmRemove = $state('');

  function edit(fn: () => unknown) {
    fn();
    session.changed();
  }

  function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    edit(() => addCriterion(session.project, newName));
    newName = '';
  }

  function onDragOver(e: DragEvent, id: string) {
    if (!dragging || descendants(p, dragging).includes(id)) return;
    e.preventDefault();
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const y = (e.clientY - r.top) / r.height;
    target = { id, where: y < 0.28 ? 'before' : y > 0.72 ? 'after' : 'inside' };
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
    if (dragging && target) edit(() => moveCriterion(session.project, dragging, target!.id, target!.where));
    dragging = '';
    target = null;
  }

  // Keyboard: Alt+↑/↓ reorder, Tab/Shift+Tab-like via Alt+→/← nest and un-nest.
  function onKey(e: KeyboardEvent, id: string) {
    if (!e.altKey) return;
    const moves: Record<string, () => boolean> = {
      ArrowUp: () => shift(session.project, id, -1),
      ArrowDown: () => shift(session.project, id, 1),
      ArrowRight: () => indent(session.project, id),
      ArrowLeft: () => outdent(session.project, id),
    };
    const move = moves[e.key];
    if (move) {
      e.preventDefault();
      edit(move);
    }
  }

  const methods: { id: Method; label: string }[] = [
    { id: 'pairwise', label: 'Compare in pairs' },
    { id: 'rating', label: 'Rate 1–10' },
    { id: 'value', label: 'Measured value' },
  ];

  const bigGroups = $derived(
    ['', ...p.criteria.map((c) => c.id)]
      .map((parent) => ({ parent, n: childrenOf(p, parent).length }))
      .filter((g) => g.n > 9),
  );
</script>

{#if rows.length === 0}
  <p class="muted small">No criteria yet. Start with the few that matter most; you can group them later.</p>
{:else}
  <ul class="tree" aria-label="Criteria">
    {#each rows as { criterion: c, depth } (c.id)}
      {@const leaf = isLeaf(p, c.id)}
      <li
        class="node"
        class:nested={depth > 0}
        class:dragging={dragging === c.id}
        class:drop-before={target?.id === c.id && target.where === 'before'}
        class:drop-after={target?.id === c.id && target.where === 'after'}
        class:drop-inside={target?.id === c.id && target.where === 'inside'}
        style:--depth={depth}
        ondragover={(e) => onDragOver(e, c.id)}
        ondragleave={() => target?.id === c.id && (target = null)}
        ondrop={onDrop}
      >
        <span
          class="grip"
          draggable="true"
          role="button"
          tabindex="-1"
          aria-label="Drag {c.name}"
          ondragstart={(e) => {
            dragging = c.id;
            e.dataTransfer?.setData('text/plain', c.id);
          }}
          ondragend={() => {
            dragging = '';
            target = null;
          }}>⠿</span
        >
        <input
          class="field inline name"
          class:group={!leaf}
          aria-label="Criterion name"
          title="Alt+arrow keys move it"
          bind:value={c.name}
          oninput={() => session.changed()}
          onkeydown={(e) => onKey(e, c.id)}
        />
        {#if leaf}
          <select
            class="field method"
            aria-label="How {c.name} is scored"
            value={c.method ?? 'pairwise'}
            onchange={(e) => edit(() => setMethod(session.project, c.id, e.currentTarget.value as Method))}
          >
            {#each methods as m}<option value={m.id}>{m.label}</option>{/each}
          </select>
          {#if c.method === 'value'}
            <select
              class="field dir"
              aria-label="Which values are better"
              value={c.direction ?? 'higher'}
              onchange={(e) => edit(() => setDirection(session.project, c.id, e.currentTarget.value as Direction))}
            >
              <option value="higher">Higher is better</option>
              <option value="lower">Lower is better</option>
            </select>
            <input class="field unit" placeholder="Unit" aria-label="Unit" bind:value={c.unit} oninput={() => session.changed()} />
          {/if}
        {:else}
          <span class="small muted kids">{childrenOf(p, c.id).length} inside</span>
        {/if}
        <span class="tools">
          <button class="btn quiet" title="Add a criterion inside" aria-label="Add inside {c.name}" onclick={() => edit(() => addCriterion(session.project, 'New criterion', c.id))}>＋</button>
          <button class="btn quiet" title="Move into the one above (Alt+→)" aria-label="Nest {c.name}" onclick={() => edit(() => indent(session.project, c.id))}>→</button>
          <button class="btn quiet" title="Move out of its group (Alt+←)" aria-label="Un-nest {c.name}" onclick={() => edit(() => outdent(session.project, c.id))}>←</button>
          {#if confirmRemove === c.id}
            <button class="btn quiet danger" onclick={() => edit(() => removeCriterion(session.project, c.id))}>
              Remove{leaf ? '' : ' group'}
            </button>
          {:else}
            <button class="btn quiet" title="Remove" aria-label="Remove {c.name}" onclick={() => (confirmRemove = c.id)}>✕</button>
          {/if}
        </span>
      </li>
    {/each}
  </ul>
{/if}

{#each bigGroups as g}
  <p class="small warn">
    {g.parent ? `“${p.criteria.find((c) => c.id === g.parent)?.name}” holds` : 'The top level holds'}
    {g.n} criteria: comparing them in pairs takes {pairCount(g.n)} judgments. Grouping them makes that much quicker.
  </p>
{/each}

<form class="add" onsubmit={submit}>
  <input class="field" placeholder="Add a criterion, then press Enter" aria-label="New criterion" bind:value={newName} />
  <button class="btn" type="submit" disabled={!newName.trim()}>Add</button>
</form>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  .node {
    --indent: calc(var(--depth) * 1.6rem);
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.25rem 0 0.25rem var(--indent);
    border-bottom: 1px solid var(--line);
    position: relative;
  }
  .node::before {
    content: '';
    position: absolute;
    left: calc(var(--indent) - 0.85rem);
    top: 0;
    bottom: 50%;
    width: 0.6rem;
    border-left: 1px solid var(--line-strong);
    border-bottom: 1px solid var(--line-strong);
    display: none;
  }
  .node.nested::before {
    display: block;
  }
  .node.dragging {
    opacity: 0.4;
  }
  .node.drop-before {
    box-shadow: inset 0 2px 0 var(--brass);
  }
  .node.drop-after {
    box-shadow: inset 0 -2px 0 var(--brass);
  }
  .node.drop-inside {
    background: var(--brass-wash);
  }
  .grip {
    cursor: grab;
    color: var(--ink-3);
    padding: 0 0.15rem;
    user-select: none;
  }
  .name {
    flex: 1 1 12rem;
    font-weight: 550;
  }
  .name.group {
    font-weight: 700;
  }
  .method {
    width: auto;
    flex: none;
  }
  .dir {
    width: auto;
    flex: none;
  }
  .unit {
    width: 5.5rem;
    flex: none;
  }
  .kids {
    flex: none;
  }
  .tools {
    display: flex;
    flex: none;
    opacity: 0.5;
  }
  .node:hover .tools,
  .node:focus-within .tools {
    opacity: 1;
  }
  .tools .btn {
    padding: 0.3rem 0.45rem;
  }
  .warn {
    color: var(--rust);
  }
  .add {
    display: flex;
    gap: 0.5rem;
    max-width: 30rem;
  }
</style>
