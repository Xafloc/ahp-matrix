import { expect, test } from 'vitest';
import { fixture } from './fixture.test-helper';
import { leafProgress, scoreProgress, weighProgress } from './progress';

test('weigh progress counts every sibling group', () => {
  expect(weighProgress(fixture(), 'e1')).toEqual({ done: 2, total: 2 });
  expect(weighProgress(fixture(), 'e2')).toEqual({ done: 0, total: 2 });
});

test('score progress follows each leaf\'s method', () => {
  const p = fixture();
  expect(leafProgress(p, 'e1', p.criteria[1])).toEqual({ done: 1, total: 3 }); // pairwise
  expect(leafProgress(p, 'e1', p.criteria[2])).toEqual({ done: 1, total: 3 }); // rating
  expect(leafProgress(p, 'e2', p.criteria[3])).toEqual({ done: 1, total: 3 }); // values are shared
  expect(scoreProgress(p, 'e1')).toEqual({ done: 3, total: 9 });
});

test("progress counts only the work a person's role gives them", () => {
  const p = fixture();
  p.evaluators[1].tasks = { weigh: false, scoreAll: true };
  expect(weighProgress(p, 'e2')).toEqual({ done: 0, total: 0 });
  p.evaluators[1].tasks = { weigh: true, scoreAll: false, score: ['c2'] };
  expect(scoreProgress(p, 'e2')).toEqual({ done: 0, total: 3 }); // option pairs on Performance only
  expect(weighProgress(p, 'e2')).toEqual({ done: 0, total: 2 });
});
