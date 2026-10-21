import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, test } from 'vitest';
import { allowed, analyze, crWith, newMatrix, setPair, suggestions } from './ahp';
import type { Project, Results } from './types';

const dir = fileURLToPath(new URL('../../../internal/model/testdata/golden', import.meta.url));
const golden = readdirSync(dir)
  .filter((f) => f.endsWith('.json'))
  .map((f) => ({ name: f, ...(JSON.parse(readFileSync(join(dir, f), 'utf8')) as { project: Project; results: Results }) }));

const close = (got: number, want: number) => expect(Math.abs(got - want)).toBeLessThanOrEqual(1e-9 * Math.max(1, Math.abs(want)));

describe.each(golden)('$name: every matrix matches Go', ({ project, results }) => {
  test.each(results.matrices.map((m) => [m.context, m] as const))('%s', (_, want) => {
    const m = want.matrix.map((row) => [...row]);
    const a = analyze(m);
    expect(a.connected).toBe(want.connected);
    expect(a.missing).toBe(want.missing);
    expect(a.complete).toBe(want.complete);
    if (!want.connected) return;
    a.weights!.forEach((w, i) => close(w, want.weights![i]));
    close(a.cr, want.cr);
    expect(a.threshold).toBe(want.threshold);
    expect(a.consistent).toBe(want.consistent);
    const got = suggestions(m, a.weights!, allowed(project.scale), 3);
    expect(got.map((s) => [want.items[s.i], want.items[s.j], s.suggested])).toEqual(want.suggestions.map((s) => [s.a, s.b, s.suggested]));
    got.forEach((s, k) => {
      close(s.current, want.suggestions[k].current);
      close(crWith(m, s.i, s.j, s.suggested), want.suggestions[k].crAfter);
    });
  });
});

test('a consistent 3×3 has the obvious weights and no inconsistency', () => {
  const m = newMatrix(3);
  setPair(m, 0, 1, 2);
  setPair(m, 1, 2, 2);
  setPair(m, 0, 2, 4);
  const a = analyze(m);
  [4 / 7, 2 / 7, 1 / 7].forEach((w, i) => close(a.weights![i], w));
  close(a.cr, 0);
  expect(a.consistent).toBe(true);
});

test('an incomplete but linked matrix still has weights (Harker)', () => {
  const m = newMatrix(4);
  setPair(m, 0, 1, 2);
  setPair(m, 1, 2, 2);
  setPair(m, 2, 3, 2);
  const a = analyze(m);
  expect(a.connected).toBe(true);
  expect(a.missing).toBe(3);
  [8 / 15, 4 / 15, 2 / 15, 1 / 15].forEach((w, i) => close(a.weights![i], w));
});

test('unlinked items give groups and no weights', () => {
  const m = newMatrix(4);
  setPair(m, 0, 1, 3);
  setPair(m, 2, 3, 3);
  const a = analyze(m);
  expect(a.connected).toBe(false);
  expect(a.weights).toBeNull();
  expect(a.components).toEqual([
    [0, 1],
    [2, 3],
  ]);
});

describe.each(golden.filter((g) => g.results.evaluator !== 'all'))('$name: one person’s report matches Go', ({ project, results }) => {
  test.each(results.matrices.map((m) => [m.context, m] as const))('%s', async (_, want) => {
    const { matrixReport } = await import('./ahp');
    const got = matrixReport(project, results.evaluator, want.context, want.items);
    got.matrix.forEach((row, i) => row.forEach((v, j) => close(v, want.matrix[i][j])));
    expect(got.connected).toBe(want.connected);
    expect(got.missing).toBe(want.missing);
    expect(got.pairs).toBe(want.pairs);
    got.weights!.forEach((w, i) => close(w, want.weights![i]));
    close(got.cr, want.cr);
    expect(got.consistent).toBe(want.consistent);
    expect(got.components).toEqual(want.components);
    expect(got.suggestions.map((s) => [s.a, s.b, s.suggested])).toEqual(want.suggestions.map((s) => [s.a, s.b, s.suggested]));
    got.suggestions.forEach((s, k) => close(s.crAfter, want.suggestions[k].crAfter));
  });
});
