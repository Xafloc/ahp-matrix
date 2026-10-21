import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { expect, test } from 'vitest';

// Checks the built judging page (npm run build); skipped until there is one.
const path = fileURLToPath(new URL('../../dist/judge.html', import.meta.url));

test.skipIf(!existsSync(path))('the judging page is one self-contained file, small enough to email', () => {
  const html = readFileSync(path, 'utf8');
  expect(html.match(/\s(?:src|href)=["'](?!data:|#)[^"']+/g) ?? []).toEqual([]);
  expect(html.match(/<script type="application\/json" id="ahp-packet"><\/script>/g)).toHaveLength(1);
  expect(html).toContain('font/woff2');
  expect(html.length).toBeLessThan(600_000);
});
