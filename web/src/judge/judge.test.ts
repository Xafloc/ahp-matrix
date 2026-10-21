import { render } from 'svelte/server';
import { expect, test } from 'vitest';
import { fixture } from '../lib/fixture.test-helper';
import Judge from './Judge.svelte';

const text = (html: string) => html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');

test('the welcome says who asked, what for, and how long it takes', () => {
  const p = fixture();
  p.description = 'Pick the stack for the new service.';
  p.evaluators = [{ ...p.evaluators[1], tasks: { weigh: true, scoreAll: false } }];
  p.pairwise = [];
  p.packet = { for: 'e2', from: 'Ann', sentAt: '2026-10-20T10:00:00Z', structure: 'x' };
  const body = text(render(Judge, { props: { packet: JSON.stringify(p) } }).body);
  expect(body).toContain('Ann asked for your view on Fixture');
  expect(body).toContain('Pick the stack for the new service.');
  expect(body).toContain("You're answering as Bob: weigh the criteria. It takes about 1 minute.");
  expect(body).toContain('Start');
});

test('a page without its decision says what to do', () => {
  const body = text(render(Judge, { props: { packet: '' } }).body);
  expect(body).toContain('This page is missing its decision.');
});

test('someone with nothing to answer is told so, not thanked for a file', () => {
  const p = fixture();
  p.criteria = [{ id: 'c4', name: 'Cost', description: '', notes: '', method: 'value' }];
  p.evaluators = [p.evaluators[1]];
  p.packet = { for: 'e2', from: 'Ann', sentAt: '', structure: 'x' };
  const body = text(render(Judge, { props: { packet: JSON.stringify(p) } }).body);
  expect(body).toContain("There's nothing for you to answer in this decision");
  expect(body).not.toContain('Start');
});
