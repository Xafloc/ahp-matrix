import { expect, it } from 'vitest';
import { combineNote } from './copy';

it('explains the combining method in use, and where to change it', () => {
  expect(combineNote('judgments')).toContain('geometric mean');
  expect(combineNote('judgments')).toContain('People step');
  expect(combineNote('priorities')).toContain("Each person's ranking");
  expect(combineNote('priorities')).toContain('criteria weights');
});
