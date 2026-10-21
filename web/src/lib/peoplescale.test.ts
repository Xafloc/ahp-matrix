import { expect, test } from 'vitest';
import { placeMarkers, scaleX } from './peoplescale';
import { parse } from './router.svelte';

test('answers sit where the balance would put them: the first item preferred is to the left', () => {
  expect(scaleX(9, 'saaty9')).toBe(0);
  expect(scaleX(1, 'saaty9')).toBe(0.5);
  expect(scaleX(1 / 9, 'saaty9')).toBe(1);
  expect(scaleX(4, "saaty9")).toBe(5 / 16);
  expect(scaleX(Math.sqrt(12), "saaty9")).toBeCloseTo(5.5 / 16); // halfway between 4 and 3, in ratio terms
  expect(scaleX(100, 'saaty9')).toBe(0); // clamped
});

test('answers that would overlap are stacked, in the order given', () => {
  const placed = placeMarkers(
    [
      { id: 'a', v: 3 },
      { id: 'b', v: 3 },
      { id: 'c', v: 1 / 7 },
      { id: 'd', v: 3 },
    ],
    'saaty9',
  );
  expect(placed.map((m) => m.row)).toEqual([0, 1, 0, 2]);
});

test('a link can open a particular pair', () => {
  expect(parse('#/p/P1/weigh/root/c1|c2')).toEqual({ name: 'project', id: 'P1', step: 'weigh', sub: 'root', pair: 'c1|c2' });
});

test('names that would overlap at the real width stack, even when the answers differ', () => {
  const answers = [
    { id: 'a', v: 2, label: 'Engineering lead' },
    { id: 'b', v: 1 / 2, label: 'Product manager' },
  ];
  expect(placeMarkers(answers, 'saaty9', { width: 1400 }).map((m) => m.row)).toEqual([0, 0]);
  expect(placeMarkers(answers, 'saaty9', { width: 360 }).map((m) => m.row)).toEqual([0, 1]);
});

test('names at the ends hang inwards, and stacking allows for it', () => {
  const placed = placeMarkers(
    [
      { id: 'a', v: 9, label: 'Alexandra Konstantinou' },
      { id: 'b', v: 6, label: 'Sam' },
      { id: 'c', v: 1 / 9, label: 'Priya' },
    ],
    'saaty9',
    { width: 700 },
  );
  expect(placed.map((m) => [m.align, m.row])).toEqual([
    ['start', 0],
    ['start', 1],
    ['end', 0],
  ]);
});
