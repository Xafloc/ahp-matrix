// The consistency math of internal/ahp (analyze.go, suggest.go, matrix.go),
// ported line for line so a page with no server gives the same answers.
// ahp.test.ts checks it against Go's golden files. Change both together.
import type { MatrixReport, Project, Scale } from './types';

/** m[i][j] is how strongly item i is preferred over j; 0 off the diagonal means unjudged. */
export type Matrix = number[][];

const bisectTolerance = 1e-14;
const bisectMaxIter = 200;
const inverseTolerance = 1e-15;
const inverseMaxIter = 20;

const saatyRI = [0, 0, 0, 0.58, 0.9, 1.12, 1.24, 1.32, 1.41, 1.45, 1.49, 1.51, 1.48, 1.56, 1.57, 1.59];

export function newMatrix(n: number): Matrix {
  return Array.from({ length: n }, (_, i) => Array.from({ length: n }, (__, j) => (i === j ? 1 : 0)));
}

/** Records that i is preferred over j by v, and the reciprocal. */
export function setPair(m: Matrix, i: number, j: number, v: number): void {
  m[i][j] = v;
  m[j][i] = 1 / v;
}

const judged = (m: Matrix, i: number, j: number) => m[i][j] !== 0;

function missing(m: Matrix): number {
  let n = 0;
  for (let i = 0; i < m.length; i++) for (let j = i + 1; j < m.length; j++) if (!judged(m, i, j)) n++;
  return n;
}

/** Saaty's random index for n ≤ 15, Alonso & Lamata's regression above. */
export function randomIndex(n: number): number {
  if (n < saatyRI.length) return saatyRI[n];
  return (1.7699 * n - 4.3513) / (n - 1);
}

export function crThreshold(n: number): number {
  return n === 3 ? 0.05 : n === 4 ? 0.08 : 0.1;
}

/** The values a judgment can take on a scale (model.Scale.Allowed). */
export function allowed(scale: Scale): number[] {
  if (scale === 'classic5') return [0.1, 0.2, 1, 5, 10];
  const out: number[] = [];
  for (let k = 9; k >= 2; k--) out.push(1 / k);
  for (let k = 1; k <= 9; k++) out.push(k);
  return out;
}

/** The allowed value closest to v on a log scale. */
export function snap(v: number, values: number[]): number {
  let best = values[0];
  let bestDist = Infinity;
  const lv = Math.log(v);
  for (const a of values) {
    const d = Math.abs(Math.log(a) - lv);
    if (d < bestDist) [best, bestDist] = [a, d];
  }
  return best;
}

export interface Analysis {
  weights: number[] | null; // null when the judgments don't link every item
  lambdaMax: number;
  ci: number;
  cr: number;
  threshold: number;
  consistent: boolean;
  missing: number;
  complete: boolean;
  connected: boolean;
  components: number[][];
}

/** Priorities and consistency for m, with Harker's method for missing pairs. */
export function analyze(m: Matrix): Analysis {
  const n = m.length;
  const a: Analysis = { weights: null, lambdaMax: 0, ci: 0, cr: 0, threshold: crThreshold(n), consistent: false, missing: missing(m), complete: false, connected: false, components: [] };
  a.complete = a.missing === 0;
  a.components = components(m);
  a.connected = a.components.length <= 1;
  if (n === 0) {
    a.consistent = true;
    return a;
  }
  if (!a.connected) return a;

  const h = newMatrix(n);
  for (let i = 0; i < n; i++)
    for (let j = 0; j < n; j++) {
      if (i === j) continue;
      h[i][j] = m[i][j];
      if (!judged(m, i, j)) h[i][i]++;
    }
  [a.weights, a.lambdaMax] = principal(h);
  if (n > 2) {
    a.ci = Math.max(0, (a.lambdaMax - n) / (n - 1));
    a.cr = a.ci / randomIndex(n);
  }
  a.consistent = a.cr <= a.threshold;
  return a;
}

// The principal eigenvector and λmax: λmax bracketed by bisection (μI−M is a
// nonsingular M-matrix exactly when μ > λmax), then inverse iteration.
function principal(m: Matrix): [number[], number] {
  const n = m.length;
  if (n === 1) return [[1], m[0][0]];
  let lo = Infinity;
  let hi = 0;
  for (const row of m) {
    const s = row.reduce((x, v) => x + v, 0);
    lo = Math.min(lo, s);
    hi = Math.max(hi, s);
  }
  hi++;
  for (let iter = 0; iter < bisectMaxIter && hi - lo > bisectTolerance * hi; iter++) {
    const mid = (lo + hi) / 2;
    if (shiftedSolve(m, mid, null).ok) hi = mid;
    else lo = mid;
  }
  let w = Array.from({ length: n }, () => 1 / n);
  for (let iter = 0; iter < inverseMaxIter; iter++) {
    const x = shiftedSolve(m, hi, w).x!;
    const sum = x.reduce((s, v) => s + v, 0);
    let delta = 0;
    for (let i = 0; i < n; i++) {
      x[i] /= sum;
      delta = Math.max(delta, Math.abs(x[i] - w[i]));
    }
    w = x;
    if (delta < inverseTolerance) break;
  }
  let lambda = 0;
  for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) lambda += m[i][j] * w[j];
  return [w, lambda];
}

function shiftedSolve(m: Matrix, mu: number, b: number[] | null): { x: number[] | null; ok: boolean } {
  const n = m.length;
  const a = m.map((row, i) => row.map((v, j) => -v + (i === j ? mu : 0)));
  const x = b ? [...b] : null;
  for (let k = 0; k < n; k++) {
    if (!(a[k][k] > 0)) return { x: null, ok: false };
    for (let i = k + 1; i < n; i++) {
      const f = a[i][k] / a[k][k];
      for (let j = k; j < n; j++) a[i][j] -= f * a[k][j];
      if (x) x[i] -= f * x[k];
    }
  }
  for (let i = n - 1; x && i >= 0; i--) {
    let s = x[i];
    for (let j = i + 1; j < n; j++) s -= a[i][j] * x[j];
    x[i] = s / a[i][i];
  }
  return { x, ok: true };
}

function components(m: Matrix): number[][] {
  const n = m.length;
  const seen = new Array<boolean>(n).fill(false);
  const groups: number[][] = [];
  for (let start = 0; start < n; start++) {
    if (seen[start]) continue;
    seen[start] = true;
    const group: number[] = [];
    const stack = [start];
    while (stack.length) {
      const i = stack.pop()!;
      group.push(i);
      for (let j = 0; j < n; j++)
        if (j !== i && !seen[j] && judged(m, i, j)) {
          seen[j] = true;
          stack.push(j);
        }
    }
    groups.push(group.sort((x, y) => x - y));
  }
  return groups;
}

export interface Suggestion {
  i: number;
  j: number;
  current: number;
  suggested: number;
  deviation: number;
}

/** Up to k judged pairs that disagree most with the weights w, most first. */
export function suggestions(m: Matrix, w: number[], values: number[], k: number): Suggestion[] {
  const out: Suggestion[] = [];
  for (let i = 0; i < m.length; i++)
    for (let j = i + 1; j < m.length; j++) {
      if (!judged(m, i, j)) continue;
      const deviation = Math.abs(Math.log((m[i][j] * w[j]) / w[i]));
      const suggested = snap(w[i] / w[j], values);
      if (deviation < 1e-9 || suggested === m[i][j]) continue;
      out.push({ i, j, current: m[i][j], suggested, deviation });
    }
  // A stable sort with near-ties kept in pair order, as Go does.
  const ranked = out.map((s, idx) => ({ s, idx })).sort((x, y) => (x.s.deviation > y.s.deviation + 1e-9 ? -1 : y.s.deviation > x.s.deviation + 1e-9 ? 1 : x.idx - y.idx));
  return ranked.slice(0, k).map((r) => r.s);
}

/** m's consistency ratio with the judgment for (i, j) set to v. */
export function crWith(m: Matrix, i: number, j: number, v: number): number {
  const trial = m.map((row) => [...row]);
  setPair(trial, i, j, v);
  return analyze(trial).cr;
}

const maxSuggestions = 3;

/**
 * One person's comparison report for a context, as model.Compute gives it
 * for that person alone. Unlinked items get equal weights.
 */
export function matrixReport(p: Project, evaluatorId: string, context: string, items: string[]): MatrixReport {
  const n = items.length;
  const idx = new Map(items.map((id, i) => [id, i]));
  const m = newMatrix(n);
  for (const j of p.pairwise) {
    if (j.evaluatorId !== evaluatorId || j.context !== context) continue;
    const a = idx.get(j.a);
    const b = idx.get(j.b);
    if (a === undefined || b === undefined || a === b) continue;
    setPair(m, a, b, j.value);
  }
  const a = analyze(m);
  const report: MatrixReport = {
    context,
    items,
    weights: a.weights,
    cr: a.cr,
    threshold: a.threshold,
    consistent: a.consistent,
    complete: a.complete,
    connected: a.connected,
    missing: a.missing,
    pairs: (n * (n - 1)) / 2,
    suggestions: [],
    matrix: m,
  };
  if (!a.connected) {
    report.components = a.components.map((g) => g.map((i) => items[i]));
    report.weights = Array.from({ length: n }, () => 1 / n);
    return report;
  }
  report.suggestions = suggestions(m, a.weights!, allowed(p.scale), maxSuggestions).map((s) => ({
    a: items[s.i],
    b: items[s.j],
    current: s.current,
    suggested: s.suggested,
    crAfter: crWith(m, s.i, s.j, s.suggested),
  }));
  return report;
}
