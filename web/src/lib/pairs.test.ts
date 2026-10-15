import { expect, test } from 'vitest';
import { focusOrder, pairCount } from './pairs';

test('focus order links every item first, then covers every pair once', () => {
  const order = focusOrder(5);
  expect(order.slice(0, 4)).toEqual([[0, 1], [1, 2], [2, 3], [3, 4]]);
  expect(order).toHaveLength(pairCount(5));
  expect(new Set(order.map((p) => p.join())).size).toBe(10);
});

test('after the chain, pairs closest in weight come first', () => {
  // gaps: (0,2) ln(.4/.29) ≈ 0.32, (1,3) ln 30 ≈ 3.4, (0,3) ln 40 ≈ 3.7
  expect(focusOrder(4, [0.4, 0.3, 0.29, 0.01]).slice(3)).toEqual([[0, 2], [1, 3], [0, 3]]);
});
