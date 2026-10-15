<script lang="ts">
  // The wordmark glyph: a beam balanced on a post. `tilt` is in degrees.
  let { size = 28, tilt = -8 }: { size?: number; tilt?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 32 32" aria-hidden="true">
  <path d="M16 9v17M8.5 26h15" stroke="var(--ink)" stroke-width="2.4" stroke-linecap="round" fill="none" />
  <g transform="rotate({tilt} 16 9)">
    <path d="M3.5 9h25" stroke="var(--brass)" stroke-width="2.4" stroke-linecap="round" />
    <circle cx="4.5" cy="9" r="2.6" fill="var(--brass)" />
    <circle cx="27.5" cy="9" r="2.6" fill="none" stroke="var(--brass)" stroke-width="1.8" />
  </g>
  <circle cx="16" cy="9" r="2" fill="var(--ink)" />
</svg>
