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

/**
 * Places people's answers on the scale, stacking ones whose markers, or
 * names (given the plot's width in pixels), would overlap.
 */
export function placeMarkers<T extends { v: number; label?: string }>(
  values: T[],
  scale: Scale,
  o: { gap?: number; width?: number } = {},
): (T & { x: number; row: number })[] {
  const gap = o.gap ?? 0.05;
  const half = (m: T) => (o.width ? ((m.label?.length ?? 0) * CHAR + PAD) / 2 / o.width : 0);
  const out: (T & { x: number; row: number })[] = [];
  for (const m of values) {
    const x = scaleX(m.v, scale);
    let row = 0;
    while (out.some((p) => p.row === row && Math.abs(p.x - x) < Math.max(gap, half(p) + half(m)))) row++;
    out.push({ ...m, x, row });
  }
  return out;
}
