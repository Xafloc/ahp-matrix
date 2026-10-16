import { expect, test } from 'vitest';
import {
  addAlternative,
  addCriterion,
  changeScale,
  getJudgment,
  indent,
  moveCriterion,
  outdent,
  removeAlternative,
  removeCriterion,
  removeEvaluator,
  setEvaluatorWeight,
  setJudgment,
  setRating,
  setValue,
  shift,
} from './edits';
import { fixture } from './fixture.test-helper';
import { childrenOf } from './tree';

const ids = (cs: { id: string }[]) => cs.map((c) => c.id);

test('new alternatives take the first free series color, then none', () => {
  const p = fixture();
  removeAlternative(p, 'a2');
  expect(addAlternative(p, ' Delta ').color).toBe('series-2');
  for (let i = 0; i < 5; i++) addAlternative(p, `X${i}`);
  expect(addAlternative(p, 'Ninth').color).toBe('');
  expect(p.alternatives.find((a) => a.name === 'Delta')).toBeTruthy();
});

test('removing an alternative removes its judgments', () => {
  const p = fixture();
  removeAlternative(p, 'a1');
  expect(p.pairwise.some((j) => j.a === 'a1' || j.b === 'a1')).toBe(false);
  expect(p.ratings).toHaveLength(0);
  expect(p.values).toHaveLength(0);
});

test('removing a criterion removes its subtree and every judgment about it', () => {
  const p = fixture();
  removeCriterion(p, 'c1');
  expect(ids(p.criteria)).toEqual(['c4']);
  expect(p.pairwise).toHaveLength(0);
  expect(p.ratings).toHaveLength(0);
});

test('judgments read back in either orientation and replace each other', () => {
  const p = fixture();
  expect(getJudgment(p, 'e1', 'alts:c2', 'a2', 'a1')).toBe(0.5);
  setJudgment(p, 'e1', 'alts:c2', 'a2', 'a1', 3);
  expect(getJudgment(p, 'e1', 'alts:c2', 'a1', 'a2')).toBeCloseTo(1 / 3);
  expect(p.pairwise.filter((j) => j.context === 'alts:c2')).toHaveLength(1);
  setJudgment(p, 'e1', 'alts:c2', 'a1', 'a2', undefined);
  expect(getJudgment(p, 'e1', 'alts:c2', 'a1', 'a2')).toBeUndefined();
  expect(getJudgment(p, 'e2', 'criteria:root', 'c1', 'c4')).toBeUndefined();
});

test('ratings and values replace earlier entries; non-numbers clear values', () => {
  const p = fixture();
  setRating(p, 'e1', 'c3', 'a1', 9);
  expect(p.ratings).toEqual([{ evaluatorId: 'e1', criterionId: 'c3', alternativeId: 'a1', value: 9 }]);
  setValue(p, 'c4', 'a1', Number.NaN);
  expect(p.values).toHaveLength(0);
});

test('moving a criterion inside another drops judgments that no longer apply', () => {
  const p = fixture();
  expect(moveCriterion(p, 'c4', 'c1', 'inside')).toBe(true);
  expect(ids(childrenOf(p, 'c1'))).toEqual(['c2', 'c3', 'c4']);
  expect(getJudgment(p, 'e1', 'criteria:root', 'c1', 'c4')).toBeUndefined(); // no longer siblings
  expect(p.values).toHaveLength(1); // alternatives' data survives
  expect(moveCriterion(p, 'c1', 'c2', 'inside')).toBe(false); // into its own subtree
});

test('indent, outdent and shift', () => {
  const p = fixture();
  expect(indent(p, 'c4')).toBe(true);
  expect(p.criteria.find((c) => c.id === 'c4')?.parentId).toBe('c1');
  expect(outdent(p, 'c4')).toBe(true);
  expect(ids(childrenOf(p))).toEqual(['c1', 'c4']);
  expect(indent(p, 'c1')).toBe(false); // no previous sibling
  expect(shift(p, 'c4', -1)).toBe(true);
  expect(ids(childrenOf(p))).toEqual(['c4', 'c1']);
  expect(shift(p, 'c4', -1)).toBe(false);
});

test('adding a child keeps the parent\'s alternative judgments (it just stops using them)', () => {
  const p = fixture();
  addCriterion(p, 'Latency', 'c2');
  expect(getJudgment(p, 'e1', 'alts:c2', 'a1', 'a2')).toBe(2);
});

test('the last evaluator stays; others go with their judgments', () => {
  const p = fixture();
  expect(removeEvaluator(p, 'e1')).toBe(true);
  expect(p.pairwise).toHaveLength(0);
  expect(removeEvaluator(p, 'e2')).toBe(false);
});

test('changing the scale snaps judgments', () => {
  const p = fixture();
  changeScale(p, 'classic5');
  expect(p.pairwise.map((j) => j.value)).toEqual([5, 1, 1]);
});

test('re-judging a pair keeps its note', () => {
  const p = fixture();
  p.pairwise[2].note = 'Benchmarks from March';
  setJudgment(p, 'e1', 'alts:c2', 'a2', 'a1', 4);
  expect(p.pairwise.find((j) => j.context === 'alts:c2')?.note).toBe('Benchmarks from March');
});

test('a blank or zero weight counts as 1, as the server treats it', () => {
  const p = fixture();
  setEvaluatorWeight(p, 'e2', Number.NaN);
  expect(p.evaluators[1].weight).toBe(1);
  setEvaluatorWeight(p, 'e2', 0);
  expect(p.evaluators[1].weight).toBe(1);
  setEvaluatorWeight(p, 'e2', 2.5);
  expect(p.evaluators[1].weight).toBe(2.5);
  setEvaluatorWeight(p, 'e2', -1); // kept, so the save explains it
  expect(p.evaluators[1].weight).toBe(-1);
});
