<script lang="ts">
  type Theme = 'system' | 'light' | 'dark';
  let theme = $state<Theme>((typeof document === 'undefined' ? undefined : (document.documentElement.dataset.theme as Theme)) ?? 'system');
  const next: Record<Theme, Theme> = { system: 'light', light: 'dark', dark: 'system' };
  const label: Record<Theme, string> = { system: 'Theme: match system', light: 'Theme: light', dark: 'Theme: dark' };

  function cycle() {
    theme = next[theme];
    if (theme === 'system') delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = theme;
    try {
      if (theme === 'system') localStorage.removeItem('theme');
      else localStorage.setItem('theme', theme);
    } catch {
      // not remembered, but still applied
    }
  }
</script>

<button class="btn quiet" onclick={cycle} title={label[theme]} aria-label={label[theme]}>
  <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8">
    {#if theme === 'dark'}
      <path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" />
    {:else if theme === 'light'}
      <circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    {:else}
      <circle cx="12" cy="12" r="8" /><path d="M12 4a8 8 0 0 1 0 16Z" fill="currentColor" />
    {/if}
  </svg>
</button>
