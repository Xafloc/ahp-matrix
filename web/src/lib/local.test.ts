import { expect, test } from 'vitest';
import { setJudgment } from './edits';
import { fixture } from './fixture.test-helper';
import { LocalSession } from './local.svelte';
import type { Project } from './types';

/** A copy sent to Bob, as model.Packet makes it. */
function packet(): Project {
  const p = fixture();
  p.evaluators = [p.evaluators[1]];
  p.pairwise = [];
  p.ratings = [];
  p.packet = { for: 'e2', from: 'Ann', sentAt: '2026-10-20T10:00:00Z', structure: 'abc' };
  return p;
}

function memory() {
  const data = new Map<string, string>();
  return { data, getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => void data.set(k, v) };
}

test('answers are checked for consistency as they are made, with no server', () => {
  const s = new LocalSession(packet(), memory());
  expect(s.scoringAs).toBe('e2');
  setJudgment(s.project, 'e2', 'alts:c2', 'a1', 'a2', 3);
  s.changed();
  const report = s.results.e2?.matrices.find((m) => m.context === 'alts:c2');
  expect(report?.pairs).toBe(3);
  expect(report?.missing).toBe(2);
});

test('progress is kept in the browser and picked up again', () => {
  const store = memory();
  const s = new LocalSession(packet(), store);
  setJudgment(s.project, 'e2', 'criteria:root', 'c1', 'c4', 5);
  s.changed();
  const again = LocalSession.restore(packet(), store);
  expect(again.project.pairwise).toHaveLength(1);
});

test('a saved copy of an older invitation is not picked up', () => {
  const store = memory();
  const s = new LocalSession(packet(), store);
  setJudgment(s.project, 'e2', 'criteria:root', 'c1', 'c4', 5);
  s.changed();
  const newer = packet();
  newer.packet!.structure = 'def';
  expect(LocalSession.restore(newer, store).project.pairwise).toHaveLength(0);
});

test('a browser that refuses storage still works, without resuming', () => {
  const broken = {
    getItem: () => {
      throw new Error('denied');
    },
    setItem: () => {
      throw new Error('denied');
    },
  };
  const s = LocalSession.restore(packet(), broken);
  setJudgment(s.project, 'e2', 'criteria:root', 'c1', 'c4', 5);
  expect(() => s.changed()).not.toThrow();
  expect(s.results.e2?.matrices.length).toBeGreaterThan(0);
});
