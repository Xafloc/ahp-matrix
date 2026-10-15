// Mirrors internal/ahp/sensitivity.go so the sensitivity slider can redraw
// without a round trip. Parity with Go is checked against the golden files in
// internal/model/testdata/golden (see sensitivity.test.ts).
import type { Results } from './types';

/** Each alternative's overall score as intercept + slope·x, where x is one criterion's local weight. */
export interface Line {
  intercept: number[];
  slope: number[];
}

export const at = (l: Line, x: number) => l.intercept.map((c, a) => c + l.slope[a] * x);

/**
 * The scores as a function of criterion id's local weight, with its siblings
 * rescaled to share the rest (equally, if they currently have no weight).
 * Null when the criterion has no siblings.
 */
export function lineFor(r: Results, id: string): Line | null {
  const node = r.criteria.find((c) => c.id === id);
  if (!node) return null;
  const parentId = node.parentId ?? '';
  const sibs = r.criteria.filter((c) => c.id !== id && (c.parentId ?? '') === parentId);
  if (sibs.length === 0) return null;
  const gp = parentId ? (r.criteria.find((c) => c.id === parentId)?.global ?? 0) : 1;
  const w = node.local;
  const line: Line = { intercept: [], slope: [] };
  r.scores.forEach((s, a) => {
    let others = 0;
    for (const sb of sibs) others += sb.local * sb.subtree[a];
    const group = w * node.subtree[a] + others;
    let q: number;
    if (1 - w > 1e-12) q = others / (1 - w);
    else q = sibs.reduce((t, sb) => t + sb.subtree[a], 0) / sibs.length;
    const rest = s.score - gp * group;
    line.intercept.push(rest + gp * q);
    line.slope.push(gp * (node.subtree[a] - q));
  });
  return line;
}

/** The weight in [0, 1] where alternatives a and b score equally, if any. */
function meet(l: Line, a: number, b: number): number | null {
  const ds = l.slope[a] - l.slope[b];
  if (Math.abs(ds) < 1e-15) return null;
  const x = (l.intercept[b] - l.intercept[a]) / ds;
  return x >= 0 && x <= 1 ? x : null;
}

export interface Crossover {
  x: number;
  a: number;
  b: number;
}

/** Every weight in [0, 1] where two alternatives swap places, sorted by weight. */
export function crossovers(l: Line): Crossover[] {
  const out: Crossover[] = [];
  for (let a = 0; a < l.slope.length; a++)
    for (let b = a + 1; b < l.slope.length; b++) {
      const x = meet(l, a, b);
      if (x !== null) out.push({ x, a, b });
    }
  return out.sort((p, q) => p.x - q.x);
}

const argmax = (v: number[]) => v.reduce((best, x, i) => (x > v[best] ? i : best), 0);

/** The smallest change from weight w at which the leader changes, or null if it never does. */
export function stability(l: Line, w: number): number | null {
  const top = argmax(at(l, w));
  let best: number | null = null;
  for (let b = 0; b < l.slope.length; b++) {
    if (b === top) continue;
    const x = meet(l, top, b);
    if (x !== null && (best === null || Math.abs(x - w) < best)) best = Math.abs(x - w);
  }
  return best;
}

/** The index of the leading alternative at weight x. */
export const leaderAt = (l: Line, x: number) => argmax(at(l, x));
