/** An unordered pair of item indices, i < j. */
export type Pair = [number, number];

/**
 * The order in which the one-pair-at-a-time view asks about n items. First a
 * chain that links every item (so results exist after only n−1 answers), then
 * the remaining pairs, closest in current weight first: those are the ones
 * where another answer changes the ranking most.
 */
export function focusOrder(n: number, weights?: number[] | null): Pair[] {
  const chain: Pair[] = [];
  for (let i = 1; i < n; i++) chain.push([i - 1, i]);
  const rest: Pair[] = [];
  for (let i = 0; i < n; i++) for (let j = i + 2; j < n; j++) rest.push([i, j]);
  if (weights && weights.length === n) {
    const gap = ([i, j]: Pair) => Math.abs(Math.log(weights[i] / weights[j]));
    rest.sort((x, y) => gap(x) - gap(y) || x[0] - y[0] || x[1] - y[1]);
  }
  return [...chain, ...rest];
}

/** n·(n−1)/2: the number of pairs among n items. */
export const pairCount = (n: number) => (n * (n - 1)) / 2;
