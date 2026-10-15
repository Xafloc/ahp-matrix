import { expect, test } from 'vitest';
import { colorOf, percent, relativeTime } from './format';

test('percent', () => {
  expect(percent(0.4567)).toBe('45.7%');
  expect(percent(0.05, 0)).toBe('5%');
});

test('colorOf maps series slots to tokens and passes hex through', () => {
  expect(colorOf('series-3')).toBe('var(--series-3)');
  expect(colorOf('#aabbcc')).toBe('#aabbcc');
  expect(colorOf('')).toBe('var(--series-none)');
  expect(colorOf('red; background:url(x)')).toBe('var(--series-none)');
});

test('relativeTime', () => {
  const now = Date.parse('2026-10-15T12:00:00Z');
  expect(relativeTime('2026-10-15T11:59:30Z', now)).toBe('just now');
  expect(relativeTime('2026-10-15T11:55:00Z', now)).toBe('5 minutes ago');
  expect(relativeTime('2026-10-14T11:00:00Z', now)).toBe('yesterday');
});
