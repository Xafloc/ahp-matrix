<script lang="ts">
  // A short note at the bottom of the screen, with at most one action (Undo).
  let {
    text,
    action = '',
    onaction,
    ondismiss,
  }: { text: string; action?: string; onaction?: () => void; ondismiss: () => void } = $props();
</script>

<div class="toast no-print" role="status">
  <span>{text}</span>
  {#if action && onaction}<button class="btn small" onclick={onaction}>{action}</button>{/if}
  <button class="btn quiet small" aria-label="Dismiss" onclick={ondismiss}>✕</button>
</div>

<style>
  .toast {
    position: fixed;
    left: 50%;
    bottom: 1.25rem;
    transform: translateX(-50%);
    z-index: 20;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: max-content;
    max-width: calc(100vw - 2rem);
    padding: 0.6rem 0.6rem 0.6rem 1rem;
    border-radius: var(--radius-m);
    background: var(--ink);
    color: var(--paper);
    box-shadow: var(--shadow-lift);
  }
  .toast .btn {
    background: transparent;
    color: var(--paper);
    border-color: color-mix(in oklab, var(--paper) 45%, transparent);
  }
  .toast .btn.quiet {
    border-color: transparent;
  }
</style>
