import type { Scale } from './types';

/** The values a judgment may take on a scale, ascending (mirrors model.Scale.Allowed). */
export function stops(scale: Scale): number[] {
  if (scale === 'classic5') return [0.1, 0.2, 1, 5, 10];
  const out: number[] = [];
  for (let k = 9; k >= 2; k--) out.push(1 / k);
  for (let k = 1; k <= 9; k++) out.push(k);
  return out;
}

/** The allowed value closest to v on a log scale. */
export function snap(v: number, scale: Scale): number {
  let best = 1;
  let bestDist = Infinity;
  for (const s of stops(scale)) {
    const d = Math.abs(Math.log(s) - Math.log(v));
    if (d < bestDist) [best, bestDist] = [s, d];
  }
  return best;
}

/** Index of v in stops(scale), after snapping. */
export function stopIndex(v: number, scale: Scale): number {
  return stops(scale).indexOf(snap(v, scale));
}

const saatyWords: Record<number, string> = {
  2: 'slightly',
  3: 'moderately',
  4: 'moderately to strongly',
  5: 'strongly',
  6: 'strongly to very strongly',
  7: 'very strongly',
  8: 'very to extremely',
  9: 'extremely',
};

const classicWords: Record<number, string> = { 5: 'somewhat', 10: 'much' };

/** How strongly a value ≥ 1 prefers one side, as a word ("strongly"). */
export function strength(v: number, scale: Scale): string {
  const k = Math.round(v);
  return (scale === 'classic5' ? classicWords : saatyWords)[k] ?? '';
}

export type CompareKind = 'criteria' | 'alternatives';

/**
 * A sentence describing a judgment that a is preferred over b by v, such as
 * "Cost matters strongly more than Speed" or "Alpha is moderately better than
 * Beta on Price".
 */
export function describe(v: number, scale: Scale, kind: CompareKind, a: string, b: string, on?: string): string {
  const s = snap(v, scale);
  if (s === 1) {
    return kind === 'criteria' ? `${a} and ${b} matter equally` : `${a} and ${b} are equal${on ? ` on ${on}` : ''}`;
  }
  const [hi, lo, k] = s > 1 ? [a, b, s] : [b, a, 1 / s];
  const w = strength(k, scale);
  return kind === 'criteria'
    ? `${hi} matters ${w} more than ${lo}`
    : `${hi} is ${w} better than ${lo}${on ? ` on ${on}` : ''}`;
}

/** Formats a judgment value as people write it: 5, 1/5, 1. */
export function fraction(v: number): string {
  if (v >= 1) return String(Math.round(v * 100) / 100);
  const inv = Math.round(1 / v);
  return Math.abs(1 / inv - v) < 1e-9 ? `1/${inv}` : String(Math.round(v * 100) / 100);
}
