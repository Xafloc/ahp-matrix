import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The judging page: one bundle with everything inlined (fonts included), so
// scripts/inline-judge.mjs can fold it into a single file that works when
// opened from disk, with no server.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist-judge',
    emptyOutDir: true,
    assetsInlineLimit: () => true,
    cssCodeSplit: false,
    modulePreload: false,
    rollupOptions: { input: 'judge.html', output: { inlineDynamicImports: true } },
  },
});
