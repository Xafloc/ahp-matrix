import { expect, test } from 'vitest';
import { strongestChain } from './why';

// Technical depth, System design, Communication, Team fit: the hiring screenshot.
const m = [
  [1, 1, 5, 1],
  [1, 1, 3, 1],
  [1 / 5, 1 / 3, 1, 1],
  [1, 1, 1, 1],
];

test('the chain that contradicts an answer most goes through the item it disagrees with most', () => {
  // Technical depth vs Communication: via Team fit (1 × 1 = 1, said 5) beats via System design (1 × 3 = 3).
  expect(strongestChain(m, 0, 2)).toEqual({ via: 3, implied: 1 });
});

test('there is no chain when nothing links the pair through a third item', () => {
  const sparse = [
    [1, 2, 0],
    [0.5, 1, 0],
    [0, 0, 1],
  ];
  expect(strongestChain(sparse, 0, 1)).toBeNull();
});
