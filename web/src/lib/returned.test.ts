import { describe, expect, it } from 'vitest';
import { mergeTarget, peek } from './returned';
import type { ProjectSummary } from './types';

const summary = (id: string, received = false): ProjectSummary => ({
  id,
  file: `${id}.ahp.json`,
  name: `Decision ${id}`,
  description: '',
  updatedAt: '',
  alternatives: 2,
  criteria: 1,
  ranking: [],
  completeness: 0,
  consistent: true,
  received,
});
const file = (body: unknown, name = 'back.ahp.json') => new File([JSON.stringify(body)], name, { type: 'application/json' });
const packet = { for: 'e2', from: 'Darren', sentAt: '2026-10-20T10:00:00Z', structure: 'abc' };

describe('peek', () => {
  it('reads the id, origin and packet of a decision file', async () => {
    expect(await peek(file({ id: 'P1', origin: 'P0', name: 'Laptops', packet }))).toEqual({ id: 'P1', origin: 'P0', name: 'Laptops', packet });
  });
  it('gives up quietly on spreadsheets and broken files', async () => {
    expect(await peek(new File(['PK'], 'pm.xlsx'))).toBeNull();
    expect(await peek(file('not an object'))).toBeNull();
    expect(await peek(new File(['{oops'], 'x.ahp.json'))).toBeNull();
  });
});

describe('mergeTarget', () => {
  const library = [summary('P1'), summary('P2', true)];
  it('finds the decision a returned copy came from, by id or by origin', () => {
    expect(mergeTarget({ id: 'P1', name: '', packet }, library)?.id).toBe('P1');
    expect(mergeTarget({ id: 'NEW', origin: 'P1', name: '', packet }, library)?.id).toBe('P1');
  });
  it("doesn't offer to merge a file that isn't a sent copy", () => {
    expect(mergeTarget({ id: 'P1', name: '' }, library)).toBeNull();
  });
  it("doesn't offer to merge into a copy you were sent yourself", () => {
    expect(mergeTarget({ id: 'P2', name: '', packet }, library)).toBeNull();
  });
});

describe('after merging', () => {
  it('says what came across, and what was left out', async () => {
    const { mergedText } = await import('./returned');
    expect(mergedText({ added: ['Priya'], replaced: [], skipped: [], judgments: 18, dropped: [] })).toBe('Merged 18 judgments from Priya.');
    expect(mergedText({ added: [], replaced: ['Bob'], skipped: [], judgments: 1, dropped: ['x', 'y'] })).toBe(
      "Merged 1 judgment from Bob. 2 answers about things no longer in this decision were left out.",
    );
    expect(mergedText({ added: [], replaced: [], skipped: ['Bob'], judgments: 0, dropped: [] })).toBe('Nothing merged: the current answers were kept.');
  });

  it('undo saves the earlier project over the merged one', async () => {
    const { undoMerge } = await import('./returned');
    const { fixture } = await import('./fixture.test-helper');
    const before = fixture();
    const saved: unknown[] = [];
    const client = { saveProject: async (p: unknown) => (saved.push(p), p) } as never;
    await undoMerge(client, before, 7);
    expect(saved).toEqual([{ ...before, version: 7 }]);
  });
});
