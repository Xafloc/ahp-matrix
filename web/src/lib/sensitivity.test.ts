import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, test } from 'vitest';
import { at, crossovers, leaderAt, lineFor, shiftLabel, stability } from './sensitivity';
import type { Project, Results } from './types';

// Golden files are written by `go test ./internal/model -run Golden -update`.
const dir = fileURLToPath(new URL('../../../internal/model/testdata/golden', import.meta.url));
const golden = readdirSync(dir)
  .filter((f) => f.endsWith('.json'))
  .map((f) => ({ name: f, ...(JSON.parse(readFileSync(join(dir, f), 'utf8')) as { project: Project; results: Results }) }));
type Golden = (typeof golden)[number];
type CriterionResult = Results['criteria'][number];

describe.each(golden)('$name matches Go', ({ results }: Golden) => {
  test.each(results.criteria.map((c): [string, CriterionResult] => [c.id, c]))('criterion %s', (_id: string, c: CriterionResult) => {
    const line = lineFor(results, c.id);
    const siblings = results.criteria.filter((x) => x.id !== c.id && (x.parentId ?? '') === (c.parentId ?? ''));
    if (siblings.length === 0) {
      expect(line).toBeNull();
      expect(c.stability).toBeNull();
      return;
    }
    expect(line).not.toBeNull();
    // At the current weight the line reproduces Go's scores...
    at(line!, c.local).forEach((s, a) => expect(s).toBeCloseTo(results.scores[a].score, 12));
    // ...and its stability matches Go's.
    const st = stability(line!, c.local);
    if (c.stability === null) expect(st).toBeNull();
    else expect(st).toBeCloseTo(c.stability, 9);
  });
});

test('crossovers are where the leader changes', () => {
  const line = { intercept: [0.2, 0.8], slope: [0.7, -0.7] }; // meet at 3/7
  expect(crossovers(line)).toEqual([{ x: expect.closeTo(3 / 7, 12), a: 0, b: 1 }]);
  expect(leaderAt(line, 0.4)).toBe(1);
  expect(leaderAt(line, 0.5)).toBe(0);
  expect(stability(line, 0.5)).toBeCloseTo(0.5 - 3 / 7, 12);
});

test('stability reads in whole points, never "0 points"', () => {
  expect(shiftLabel(null)).toBe('Holds at any weight');
  expect(shiftLabel(0.004)).toBe('Flips at less than 1 point');
  expect(shiftLabel(0.124)).toBe('Flips at ±12 points');
  expect(shiftLabel(0.01)).toBe('Flips at ±1 point');
});
