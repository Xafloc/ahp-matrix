import { expect, test } from 'vitest';
import { fixture } from './fixture.test-helper';
import { hiddenFor, nextTask } from './independence';
import type { PersonProgress } from './types';

const prog = (id: string, done: number, total: number): PersonProgress => ({ id, done, total, finished: total > 0 && done === total, consistent: true, structure: '', outOfDate: false });

test('results stay hidden from someone who has judging left to do, when others are judging too', () => {
  const people = [prog('e1', 5, 5), prog('e2', 1, 5)];
  expect(hiddenFor(people, 'e2', 2)).toBe(true);
  expect(hiddenFor(people, 'e1', 2)).toBe(false);
  expect(hiddenFor([prog('e2', 1, 5)], 'e2', 1)).toBe(false); // alone: nobody to be swayed by
  expect(hiddenFor([prog('e1', 0, 0), prog('e2', 0, 0)], 'e1', 2)).toBe(false); // nothing to judge
  expect(hiddenFor([], 'e2', 2)).toBe(false); // results not in yet
});

test('the next thing to answer is the first unfinished step', () => {
  const p = fixture();
  expect(nextTask(p, p.evaluators[1])).toEqual({ step: 'weigh', sub: 'root' });
  expect(nextTask(p, p.evaluators[0])).toEqual({ step: 'score', sub: 'c2' });
  p.evaluators[1].tasks = { weigh: false, scoreAll: false, score: ['c3'] };
  expect(nextTask(p, p.evaluators[1])).toEqual({ step: 'score', sub: 'c3' });
});
