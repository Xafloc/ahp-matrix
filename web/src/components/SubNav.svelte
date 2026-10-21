<script lang="ts">
  export interface Entry {
    key: string;
    label: string;
    href: string;
    done: number;
    total: number;
    flag?: boolean; // contradictions to review
    detail?: string;
    off?: string; // greyed out, with the reason: "Not assigned to Priya"
  }
  let { entries, current, label }: { entries: Entry[]; current: string; label: string } = $props();
</script>

<nav aria-label={label}>
  <ul>
    {#each entries as e (e.key)}
      <li>
        <a href={e.href} class:current={e.key === current} class:off={!!e.off} aria-current={e.key === current ? 'true' : undefined}>
          <span class="text">
            <span class="name">{e.label}</span>
            {#if e.off}<span class="small muted">{e.off}</span>{:else if e.detail}<span class="small muted">{e.detail}</span>{/if}
          </span>
          {#if !e.off}
            <span class="small count" class:complete={e.total > 0 && e.done === e.total} class:flag={e.flag}>
              {#if e.flag}Review{:else if e.total > 0 && e.done === e.total}Done{:else}{e.done}/{e.total}{/if}
            </span>
          {/if}
        </a>
      </li>
    {/each}
  </ul>
</nav>

<style>
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 2px;
  }
  a {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 0.65rem;
    border-radius: var(--radius-s);
    text-decoration: none;
  }
  a:hover {
    background: color-mix(in oklab, var(--paper) 60%, var(--fog));
  }
  a.current {
    background: var(--paper);
    box-shadow: inset 3px 0 0 var(--brass);
  }
  .text {
    display: grid;
    min-width: 0;
  }
  .name {
    font-weight: 550;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  a.off .name {
    color: var(--ink-3);
    font-weight: 450;
  }
  .count {
    flex: none;
    color: var(--ink-3);
  }
  .count.complete {
    color: var(--good);
    font-weight: 600;
  }
  .count.flag {
    color: var(--rust);
    font-weight: 600;
  }
</style>
