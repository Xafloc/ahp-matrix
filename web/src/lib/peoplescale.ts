import { stops } from './scale';
import type { Scale } from './types';

/**
 * Where an answer sits on the 9…1…9 scale, from 0 (the first item extremely
 * preferred, on the left, as on the balance) to 1. Answers between stops,
 * like a group's combined answer, are placed proportionally in ratio terms.
 */
export function scaleX(v: number, scale: Scale): number {
  const s = [...stops(scale)].reverse(); // left to right: strongest for the first item first
  const lv = Math.log(v);
  if (lv >= Math.log(s[0])) return 0;
  if (lv <= Math.log(s[s.length - 1])) return 1;
  for (let i = 0; i < s.length - 1; i++) {
    const [hi, lo] = [Math.log(s[i]), Math.log(s[i + 1])];
    if (lv <= hi && lv >= lo) return (i + (hi - lv) / (hi - lo)) / (s.length - 1);
  }
  return 0.5;
}

const CHAR = 7.5; // rough width of a small-text character, in pixels
const PAD = 10;

export type Align = 'start' | 'center' | 'end';

/** How a name hangs off its marker: inwards near the ends, so it stays on the scale. */
export const alignAt = (x: number): Align => (x < 0.2 ? 'start' : x > 0.8 ? 'end' : 'center');

/**
 * Places people's answers on the scale, stacking ones whose markers, or
 * names (given the plot's width in pixels), would overlap.
 */
export function placeMarkers<T extends { v: number; label?: string }>(
  values: T[],
  scale: Scale,
  o: { gap?: number; width?: number } = {},
): (T & { x: number; row: number; align: Align })[] {
  const gap = o.gap ?? 0.05;
  // The stretch of the scale (0–1) a marker and its name cover.
  const span = (x: number, label: string | undefined, align: Align): [number, number] => {
    const w = o.width ? ((label?.length ?? 0) * CHAR + PAD) / o.width : 0;
    const [lo, hi] = align === 'start' ? [x, x + w] : align === 'end' ? [x - w, x] : [x - w / 2, x + w / 2];
    return [Math.min(lo, x - gap / 2), Math.max(hi, x + gap / 2)];
  };
  const out: (T & { x: number; row: number; align: Align; at: [number, number] })[] = [];
  for (const m of values) {
    const x = scaleX(m.v, scale);
    const align = alignAt(x);
    const at = span(x, m.label, align);
    let row = 0;
    while (out.some((p) => p.row === row && p.at[0] < at[1] && at[0] < p.at[1])) row++;
    out.push({ ...m, x, row, align, at });
  }
  return out.map(({ at: _, ...m }) => m as T & { x: number; row: number; align: Align });
}

/**
 * Where a place on the scale (0–1, from scaleX) falls across a row of n
 * stops laid out as equal columns, the way the strength words are: the first
 * stop is the middle of the first column, not its edge.
 */
export const onStops = (x: number, n: number): number => (x * (n - 1) + 0.5) / n;
