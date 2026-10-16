import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { ApiError, type Client } from './api';
import { fixture } from './fixture.test-helper';
import { ProjectSession } from './session.svelte';
import type { Project, Results } from './types';

function fakeClient() {
  let stored = fixture();
  const saves: Project[] = [];
  const client: Client = {
    listProjects: async () => [],
    createProject: async () => fixture(),
    getProject: async () => structuredClone(stored),
    saveProject: async (p) => {
      saves.push(structuredClone(p));
      if (p.version !== stored.version) throw new ApiError(409, 'conflict', 'changed', { currentVersion: stored.version });
      stored = { ...structuredClone(p), version: p.version + 1, updatedAt: '2026-10-15T12:01:00Z' };
      return structuredClone(stored);
    },
    deleteProject: async () => {},
    duplicateProject: async () => fixture(),
    results: async (_id, ev) => ({ evaluator: ev }) as Results,
    importFile: async () => ({ project: fixture() }),
    mergeFile: async () => ({ project: fixture(), report: { added: [], replaced: [], skipped: [], judgments: 0, dropped: 0 } }),
    samples: async () => [],
    templates: async () => [],
  };
  return { client, saves, setStored: (p: Project) => (stored = p), stored: () => stored };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

test('edits within the delay are saved once, then results refresh', async () => {
  const f = fakeClient();
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'One';
  s.changed();
  s.project.name = 'Two';
  s.changed();
  expect(s.saveState).toBe('pending');
  await vi.advanceTimersByTimeAsync(600);
  expect(f.saves.map((p) => p.name)).toEqual(['Two']);
  expect(s.saveState).toBe('saved');
  expect(s.project.version).toBe(2);
  expect(s.results.all?.evaluator).toBe('all');
});

test('an edit made while saving is saved afterwards', async () => {
  const f = fakeClient();
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'First';
  s.changed();
  const flushing = s.flush();
  s.project.name = 'Second'; // lands while the first save is in flight
  s.changed();
  await flushing;
  await vi.advanceTimersByTimeAsync(600);
  expect(f.saves.map((p) => p.name)).toEqual(['First', 'Second']);
  expect(s.project.version).toBe(3);
});

test('a conflict stops autosave until the person chooses', async () => {
  const f = fakeClient();
  f.setStored({ ...fixture(), version: 5, name: 'Elsewhere' });
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'Mine';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  expect(s.saveState).toBe('conflict');
  s.changed(); // further edits don't retry on their own
  await vi.advanceTimersByTimeAsync(600);
  expect(f.saves).toHaveLength(1);

  await s.overwrite();
  expect(s.saveState).toBe('saved');
  expect(f.stored().name).toBe('Mine');
});

test('reload discards local edits after a conflict', async () => {
  const f = fakeClient();
  f.setStored({ ...fixture(), version: 5, name: 'Elsewhere' });
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'Mine';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  await s.reload();
  expect(s.project.name).toBe('Elsewhere');
  expect(s.saveState).toBe('saved');
});

test('when the server is unreachable, edits wait for a retry', async () => {
  const f = fakeClient();
  const real = f.client.saveProject;
  let down = true;
  f.client.saveProject = async (p) => {
    if (down) throw new ApiError(0, 'offline', "Can't reach AHP Matrix. Is it still running?");
    return real(p);
  };
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.name = 'Unsaved';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  expect(s.saveState).toBe('error');
  expect(s.error).toContain("Can't reach AHP Matrix");

  down = false;
  await s.retry();
  expect(s.saveState).toBe('saved');
  expect(f.stored().name).toBe('Unsaved');
});

test('a rejected save explains what to fix in plain words', async () => {
  const f = fakeClient();
  f.client.saveProject = async () => {
    throw new ApiError(422, 'invalid', 'invalid project: alternatives[0].name: is required', [{ field: 'alternatives[0].name', message: 'is required' }]);
  };
  const s = new ProjectSession(fixture(), f.client, 600);
  s.project.alternatives[0].name = '';
  s.changed();
  await vi.advanceTimersByTimeAsync(600);
  expect(s.saveState).toBe('error');
  expect(s.error).toBe('Option 1 needs a name. Your changes will save as soon as that’s fixed.');
});

test('a slow, older results response never replaces a newer one', async () => {
  const f = fakeClient();
  let calls = 0;
  const pending: ((r: Results) => void)[] = [];
  f.client.results = (_id, ev) =>
    new Promise((resolve) => {
      const n = ++calls;
      pending.push(() => resolve({ evaluator: ev, completeness: n } as Results));
    });
  const s = new ProjectSession(fixture(), f.client, 600);
  const first = s.refreshResults(['all']);
  const second = s.refreshResults(['all']);
  pending[1]({} as Results); // the newer request answers first
  pending[0]({} as Results);
  await Promise.all([first, second]);
  expect(s.results.all.completeness).toBe(2);
});
