// Explaining a consistency suggestion in terms of the person's own answers.
import { snap, strength, type CompareKind } from './scale';
import type { Scale } from './types';

/**
 * The two-step chain i → via → j (both steps judged) whose implied answer
 * (m[i][via] × m[via][j]) disagrees most with the direct answer m[i][j]:
 * the clearest way to show why the direct answer doesn't fit. Null when no
 * third item links the pair.
 */
export function strongestChain(m: number[][], i: number, j: number): { via: number; implied: number } | null {
  let best: { via: number; implied: number } | null = null;
  let worst = -1;
  for (let k = 0; k < m.length; k++) {
    if (k === i || k === j || !m[i][k] || !m[k][j]) continue;
    const implied = m[i][k] * m[k][j];
    const gap = Math.abs(Math.log(implied / m[i][j]));
    if (gap > worst) [best, worst] = [{ via: k, implied }, gap];
  }
  return best;
}

/** "A and B should matter about equally", "A should be about strongly better than B on Price". */
export function should(v: number, scale: Scale, kind: CompareKind, a: string, b: string, on = ''): string {
  const s = snap(v, scale);
  const where = on ? ` on ${on}` : '';
  if (s === 1) return kind === 'criteria' ? `${a} and ${b} should matter about equally` : `${a} and ${b} should be about equal${where}`;
  const [hi, lo, k] = s > 1 ? [a, b, s] : [b, a, 1 / s];
  const w = strength(k, scale);
  return kind === 'criteria' ? `${hi} should matter about ${w} more than ${lo}` : `${hi} should be about ${w} better than ${lo}${where}`;
}
