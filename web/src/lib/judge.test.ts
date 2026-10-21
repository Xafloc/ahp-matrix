import { describe, expect, test } from 'vitest';
import { fixture } from './fixture.test-helper';
import { answersFile, judgeSteps, minutes, readPacket } from './judge';

describe('the steps on the judging page', () => {
  test('everything: each criteria group, then each leaf to score, skipping measured values', () => {
    const p = fixture();
    expect(judgeSteps(p, p.evaluators[1]).map((s) => [s.kind, s.title])).toEqual([
      ['weigh', 'Which matters more?'],
      ['weigh', 'Within Technical, which matters more?'],
      ['pairwise', 'Which is better on Performance?'],
      ['rating', 'Rate each option on Security'],
    ]);
  });
  test('weigh only', () => {
    const p = fixture();
    p.evaluators[1].tasks = { weigh: true, scoreAll: false };
    expect(judgeSteps(p, p.evaluators[1]).map((s) => s.kind)).toEqual(['weigh', 'weigh']);
  });
  test('rough time: about ten seconds an answer, at least a minute', () => {
    const p = fixture();
    expect(minutes(p, p.evaluators[1])).toBe(1); // 1 + 1 + 3 pairs + 3 ratings = 8 answers
    p.alternatives = Array.from({ length: 10 }, (_, i) => ({ id: `x${i}`, name: `X${i}`, description: '', color: '' }));
    expect(minutes(p, p.evaluators[1])).toBe(10); // 2 + 45 + 10 = 57 answers
  });
});

describe('reading the page’s decision', () => {
  test('reads the copy it was made with', () => {
    const p = fixture();
    p.schemaVersion = 2;
    p.packet = { for: 'e2', from: 'Ann', sentAt: '', structure: 'x' };
    const got = readPacket(JSON.stringify(p));
    expect('project' in got && got.project.name).toBe('Fixture');
  });
  test('says what to do when the decision is missing or too new', () => {
    for (const bad of [null, '', '{nope', JSON.stringify({ ...fixture(), schemaVersion: 3 })]) {
      const got = readPacket(bad);
      expect('error' in got && got.error).toMatch(/Ask .* for a new invitation/);
    }
  });
});

test('the answers file is named for the decision and the person', () => {
  const p = fixture();
  p.packet = { for: 'e2', from: 'Ann', sentAt: '', structure: 'x' };
  p.evaluators = [p.evaluators[1]];
  expect(answersFile(p)).toBe('fixture-answers-from-bob.ahp.json');
});

describe('edge cases', () => {
  test('nothing to do when every leaf is a measured value', () => {
    const p = fixture();
    p.criteria = [{ id: 'c4', name: 'Cost', description: '', notes: '', method: 'value' }];
    expect(judgeSteps(p, p.evaluators[1])).toEqual([]);
  });
  test('no scoring with fewer than two options', () => {
    const p = fixture();
    p.alternatives = p.alternatives.slice(0, 1);
    expect(judgeSteps(p, p.evaluators[1]).every((s) => s.kind === 'weigh')).toBe(true);
  });
  test('headings are built, not patched, so a "?" in a name survives', () => {
    const p = fixture();
    p.criteria[1].name = 'Remote?';
    const s = judgeSteps(p, p.evaluators[1]).find((x) => x.criterionId === 'c2')!;
    expect(s.heading).toBe('Which is better on Remote?, and by how much?');
  });
});
