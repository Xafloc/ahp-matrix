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
  const progress = (outOfDate = false): PersonProgress => ({ id: 'e2', done: 0, total: 3, finished: false, consistent: true, structure: 'abc', outOfDate });
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
