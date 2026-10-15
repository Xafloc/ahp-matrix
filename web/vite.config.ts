import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The Go server embeds dist/ and serves the API on the same origin. In
// development, `npm run dev` proxies /api to a running `ahp-matrix --no-open`.
export default defineConfig({
  plugins: [svelte()],
  build: { outDir: 'dist', emptyOutDir: true, assetsDir: 'assets' },
  server: { proxy: { '/api': 'http://127.0.0.1:7777' } },
  test: { include: ['src/**/*.test.ts'] },
});
