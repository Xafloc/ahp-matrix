import { expect, test } from 'vitest';
import { agreement, rankers } from './agreement';

const names = new Map([
  ['a1', 'MacBook Pro'],
  ['a2', 'ThinkPad X1'],
  ['a3', 'Framework'],
]);
const person = (name: string, scores: number[] | null) => ({ name, scores: scores && scores.map((s, i) => ({ id: `a${i + 1}`, score: s })) });

test('everyone agrees', () => {
  expect(agreement([person('Darren', [0.5, 0.3, 0.2]), person('Priya', [0.6, 0.2, 0.2]), person('Sam', [0.4, 0.35, 0.25])], names)).toBe(
    'All 3 people rank MacBook Pro first.',
  );
});

test('two agree', () => {
  expect(agreement([person('Darren', [0.5, 0.3, 0.2]), person('Priya', [0.6, 0.2, 0.2])], names)).toBe('Both people rank MacBook Pro first.');
});

test('people disagree on the winner, grouped by pick', () => {
  expect(agreement([person('Darren', [0.5, 0.3, 0.2]), person('Priya', [0.2, 0.6, 0.2]), person('Sam', [0.5, 0.2, 0.3])], names)).toBe(
    'People disagree on the winner: Darren and Sam pick MacBook Pro; Priya picks ThinkPad X1.',
  );
});

test("a tie isn't counted as agreement", () => {
  expect(agreement([person('Darren', [0.4, 0.4, 0.2]), person('Priya', [0.6, 0.2, 0.2])], names)).toBe(
    'People disagree on the winner: Darren puts MacBook Pro and ThinkPad X1 level; Priya picks MacBook Pro.',
  );
});

test("people who haven't answered are left out, and said so", () => {
  expect(agreement([person('Darren', [0.5, 0.3, 0.2]), person('Priya', [0.6, 0.2, 0.2]), person('Sam', null)], names)).toBe(
    "Both people who've answered rank MacBook Pro first. Sam hasn't answered yet.",
  );
  expect(agreement([person('Darren', [0.5, 0.3, 0.2]), person('Sam', null)], names)).toBe('');
});

test('a tie everyone shares is agreement, not disagreement', () => {
  expect(agreement([person('Darren', [0.4, 0.4, 0.2]), person('Priya', [0.4, 0.4, 0.2])], names)).toBe(
    'Both people put MacBook Pro and ThinkPad X1 level at the top.',
  );
});

test('rankers leaves out people with nothing to judge, and keeps people yet to answer', () => {
  const evaluators = [
    { id: 'e1', name: 'Darren' },
    { id: 'e2', name: 'Priya' },
    { id: 'e3', name: 'Sam' },
  ];
  const scores = [{ id: 'a1', score: 1, rank: 1 }];
  const people = [
    { id: 'e1', done: 3, total: 3, answered: true, scores },
    { id: 'e2', done: 0, total: 0, answered: true }, // nothing to judge, though they answered once
    { id: 'e3', done: 0, total: 3, answered: false },
  ];
  expect(rankers(evaluators, people)).toEqual([
    { id: 'e1', name: 'Darren', scores },
    { id: 'e3', name: 'Sam', scores: null },
  ]);
});
