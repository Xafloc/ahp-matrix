import { describe, expect, it } from 'vitest';
import { reminder, roleOf, statusOf, tasksFor } from './people';
import type { Evaluator, PersonProgress } from './types';

const leaves = ['c2', 'c3'];

describe('roles and tasks', () => {
  it('reads a role back from the tasks it makes', () => {
    for (const role of ['all', 'weigh', 'score'] as const) expect(roleOf(tasksFor(role, leaves))).toBe(role);
  });
  it('treats no tasks, or weighing and scoring everything, as everything', () => {
    expect(roleOf(undefined)).toBe('all');
    expect(roleOf({ weigh: true, scoreAll: true })).toBe('all');
    expect(tasksFor('all', leaves)).toBeUndefined();
  });
  it('treats anything else as custom', () => {
    expect(roleOf({ weigh: true, scoreAll: false, score: ['c2'] })).toBe('custom');
    expect(roleOf({ weigh: false, scoreAll: false, score: ['c3'] })).toBe('custom');
  });
  it('starts custom from what the person already had, spelled out', () => {
    expect(tasksFor('custom', leaves, undefined)).toEqual({ weigh: true, scoreAll: false, score: ['c2', 'c3'] });
    expect(tasksFor('custom', leaves, { weigh: true, scoreAll: false })).toEqual({ weigh: true, scoreAll: false, score: [] });
    expect(tasksFor('custom', leaves, { weigh: false, scoreAll: true })).toEqual({ weigh: false, scoreAll: false, score: ['c2', 'c3'] });
  });
});

describe('status', () => {
  const progress = (outOfDate = false): PersonProgress => ({ id: 'e2', done: 0, total: 3, finished: false, consistent: true, structure: 'abc', outOfDate, answered: false });
  const priya = (invite?: Evaluator['invite']): Evaluator => ({ id: 'e2', name: 'Priya', weight: 1, invite });

  it('calls the first person you', () => {
    expect(statusOf(priya(), progress(), true).kind).toBe('you');
  });
  it('follows an invitation from sent to returned', () => {
    expect(statusOf(priya(), progress(), false).kind).toBe('not-invited');
    expect(statusOf(priya({ sentAt: '2026-10-20T10:00:00Z', structure: 'abc' }), progress(), false)).toEqual({ kind: 'invited', at: '2026-10-20T10:00:00Z' });
    expect(statusOf(priya({ sentAt: '2026-10-20T10:00:00Z', structure: 'abc', returnedAt: '2026-10-21T09:00:00Z' }), progress(), false)).toEqual({
      kind: 'returned',
      at: '2026-10-21T09:00:00Z',
    });
  });
  it('says out of date when the decision changed after the copy went out', () => {
    expect(statusOf(priya({ sentAt: '2026-10-20T10:00:00Z', structure: 'old' }), progress(true), false).kind).toBe('out-of-date');
  });
});

describe('reminder', () => {
  it('says who, what and how much is left', () => {
    const text = reminder('Laptops', 'Priya', { done: 2, total: 6 }, 'Darren');
    expect(text).toContain('Priya');
    expect(text).toContain('Laptops');
    expect(text).toContain('4 of 6');
    expect(text).toContain('Darren');
  });
});

describe('assigned work', () => {
  it('opens a person on the first thing they were asked to do', async () => {
    const { firstTask } = await import('./people');
    const { fixture } = await import('./fixture.test-helper');
    const p = fixture();
    const bob = p.evaluators[1];
    expect(firstTask(p, bob)).toEqual({ step: 'weigh', sub: 'root' });
    bob.tasks = { weigh: false, scoreAll: true };
    expect(firstTask(p, bob)).toEqual({ step: 'score', sub: 'c2' });
    bob.tasks = { weigh: false, scoreAll: false, score: ['c3'] };
    expect(firstTask(p, bob)).toEqual({ step: 'score', sub: 'c3' });
    bob.tasks = { weigh: false, scoreAll: false, score: [] };
    expect(firstTask(p, bob)).toEqual({ step: 'results', sub: '' });
  });

  it('says whether a leaf is theirs to score', async () => {
    const { assigns } = await import('./people');
    const { fixture } = await import('./fixture.test-helper');
    const p = fixture();
    const [perf, sec, cost] = [p.criteria[1], p.criteria[2], p.criteria[3]];
    expect([perf, sec, cost].map((c) => assigns(undefined, c))).toEqual([true, true, true]);
    expect([perf, sec, cost].map((c) => assigns({ weigh: true, scoreAll: false, score: ['c3'] }, c))).toEqual([false, true, false]);
    expect([perf, sec, cost].map((c) => assigns({ weigh: true, scoreAll: false }, c))).toEqual([false, false, false]);
  });
});

describe('received copies', () => {
  it('names the sender, unless they never named themselves', async () => {
    const { sender } = await import('./people');
    expect(sender({ for: 'e2', from: 'Darren', sentAt: '', structure: '' })).toBe('Darren');
    expect(sender({ for: 'e2', from: 'Me', sentAt: '', structure: '' })).toBe('');
    expect(sender(undefined)).toBe('');
  });

  it('describes what someone was asked to do', async () => {
    const { describeTasks } = await import('./people');
    const { fixture } = await import('./fixture.test-helper');
    const p = fixture();
    expect(describeTasks(p, undefined)).toBe('weigh the criteria and score the options on every criterion');
    expect(describeTasks(p, { weigh: true, scoreAll: false })).toBe('weigh the criteria');
    expect(describeTasks(p, { weigh: false, scoreAll: true })).toBe('score the options on every criterion');
    expect(describeTasks(p, { weigh: false, scoreAll: false, score: ['c2', 'c3'] })).toBe('score the options on Performance and Security');
    expect(describeTasks(p, { weigh: false, scoreAll: false, score: [] })).toBe('nothing yet');
  });
});

describe('invitation message', () => {
  it('says what, how long, how to answer and by when', async () => {
    const { invitation } = await import('./people');
    const { fixture } = await import('./fixture.test-helper');
    const p = fixture();
    const bob = { ...p.evaluators[1], tasks: { weigh: true, scoreAll: false } };
    const page = invitation(p, bob, { from: 'Ann', kind: 'page', deadline: '2026-10-24' });
    expect(page).toBe(
      'Hi Bob, could you help me decide "Fixture"? I\'d like you to weigh the criteria. It takes about 1 minute. ' +
        'Open the attached page in any browser (there\'s nothing to install), answer, then press Finish and send back, then email me the file it saves. ' +
        'Please answer by Sat 24 Oct. Thanks, Ann',
    );
    const app = invitation(p, bob, { from: '', kind: 'app' });
    expect(app).toContain('Open the attached file with Import file in AHP Matrix, answer, then press Send your answers back');
    expect(app).not.toContain('Please answer by');
    expect(app.endsWith('Thanks!')).toBe(true);
  });

  it('reminds people how to answer the way they were invited', async () => {
    const { reminder } = await import('./people');
    expect(reminder('Laptops', 'Priya', { done: 0, total: 6 }, 'Darren', 'page')).toContain('Open the page I sent in any browser');
    expect(reminder('Laptops', 'Priya', { done: 0, total: 6 }, 'Darren', 'app')).toContain('Import file in AHP Matrix');
  });
});

describe('nothing to judge', () => {
  it("doesn't promise scoring when every leaf is measured", async () => {
    const { describeTasks } = await import('./people');
    const { fixture } = await import('./fixture.test-helper');
    const p = fixture();
    p.criteria = [{ id: 'c4', name: 'Cost', description: '', notes: '', method: 'value' }];
    expect(describeTasks(p, { weigh: false, scoreAll: true })).toBe('nothing yet');
  });
  it('says "then email", not "and email"', async () => {
    const { reminder } = await import('./people');
    expect(reminder('L', 'P', { done: 0, total: 1 }, 'D', 'page')).toContain('press Finish and send back, then email me the file it saves');
  });
});
