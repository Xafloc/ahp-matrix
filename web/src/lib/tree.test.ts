import { expect, test } from 'vitest';
import { fixture } from './fixture.test-helper';
import { childrenOf, descendants, flatten, groups, isLeaf, leaves, path } from './tree';

test('flatten walks depth first with depths', () => {
  expect(flatten(fixture()).map((f) => `${f.criterion.id}@${f.depth}`)).toEqual(['c1@0', 'c2@1', 'c3@1', 'c4@0']);
});

test('leaves, children and descendants', () => {
  const p = fixture();
  expect(leaves(p).map((c) => c.id)).toEqual(['c2', 'c3', 'c4']);
  expect(childrenOf(p).map((c) => c.id)).toEqual(['c1', 'c4']);
  expect(isLeaf(p, 'c1')).toBe(false);
  expect(descendants(p, 'c1')).toEqual(['c1', 'c2', 'c3']);
  expect(path(p, 'c3')).toEqual(['Technical', 'Security']);
});

test('groups lists sibling groups of two or more, top level first', () => {
  expect(groups(fixture()).map((g) => [g.parentId, g.items.map((c) => c.id)])).toEqual([
    ['', ['c1', 'c4']],
    ['c1', ['c2', 'c3']],
  ]);
});
