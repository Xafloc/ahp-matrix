import { expect, test } from 'vitest';
import { colorOf, count, percent, relativeTime } from './format';

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

test('count pluralizes', () => {
  expect(count(1, 'criterion', 'criteria')).toBe('1 criterion');
  expect(count(6, 'criterion', 'criteria')).toBe('6 criteria');
  expect(count(0, 'option')).toBe('0 options');
});

test('slug matches the server’s file names', async () => {
  const { slug } = await import('./format');
  expect(slug('Hiring a senior engineer')).toBe('hiring-a-senior-engineer');
  expect(slug("  Priya's  answers!! ")).toBe('priya-s-answers');
  expect(slug('Ünïcode only ☃')).toBe('n-code-only');
  expect(slug('!!!')).toBe('project');
  expect(slug('a'.repeat(50))).toHaveLength(40);
});
