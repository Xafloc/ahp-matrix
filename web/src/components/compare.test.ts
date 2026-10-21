import { render } from 'svelte/server';
import { expect, test } from 'vitest';
import Balance from './Balance.svelte';
import ConsistencyMeter from './ConsistencyMeter.svelte';
import PairList from './PairList.svelte';

const text = (html: string) => html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');

test('the balance states the judgment in words and marks the chosen stop', () => {
  const { body } = render(Balance, {
    props: { a: { name: 'Alpha' }, b: { name: 'Beta' }, value: 1 / 5, scale: 'saaty9', kind: 'alternatives', on: 'Price', onchange: () => {} },
  });
  expect(text(body)).toContain('Beta is strongly better than Alpha on Price');
  expect(body).toMatch(/aria-valuenow="12"/); // 1/5 sits four stops right of equal (8)
  expect(body.match(/class="stop[^"]*\bon\b/g)).toHaveLength(1);
});

test('an unanswered balance sits level and says so', () => {
  const { body } = render(Balance, {
    props: { a: { name: 'A' }, b: { name: 'B' }, value: undefined, scale: 'classic5', kind: 'criteria', onchange: () => {} },
  });
  expect(text(body)).toContain('Not compared yet');
  expect(body).toContain('rotate(0deg)');
  expect(body.match(/class="stop\b/g)).toHaveLength(5);
});

const report = (consistent: boolean) => ({
  context: 'criteria:root', items: ['a', 'b', 'c'], weights: [0.5, 0.3, 0.2], cr: consistent ? 0.01 : 0.3, threshold: 0.05, consistent,
  complete: true, connected: true, missing: 0, pairs: 3, suggestions: [{ a: 'a', b: 'c', current: 0.25, suggested: 3 }], matrix: [],
});
const listProps = (consistent: boolean) => ({
  items: [{ id: 'a', name: 'Alpha' }, { id: 'b', name: 'Beta' }, { id: 'c', name: 'Gamma' }],
  get: (i: number, j: number) => (i === 0 && j === 1 ? 3 : i === 0 && j === 2 ? 0.25 : undefined),
  set: () => {},
  scale: 'saaty9' as const,
  kind: 'criteria' as const,
  report: report(consistent),
  onfix: () => {},
});

test('the pair list has one editable row per pair, each saying where it stands', () => {
  const { body } = render(PairList, { props: listProps(true) });
  expect(body.match(/role="slider"/g)).toHaveLength(3);
  const t = text(body);
  expect(t).toContain('Alpha matters moderately more than Beta');
  expect(t).toContain('Gamma matters moderately to strongly more than Alpha');
  expect(t).toContain('Not compared yet');
  expect(t.match(/Extreme/g)).toHaveLength(2); // the strength words, once, above the list
  // The stops between equal and the choice are filled, showing how far it leans: 3 is two stops from 1.
  expect(body.match(/class="stop[^"]*\bfill\b/g)?.length).toBeGreaterThanOrEqual(2);
});

test('rows are flagged only when the answers actually contradict each other', () => {
  expect(render(PairList, { props: listProps(true) }).body).not.toMatch(/class="row[^"]*\bflag\b/);
  const { body } = render(PairList, { props: listProps(false) });
  expect(body.match(/class="row[^"]*\bflag\b/g)).toHaveLength(1);
  expect(text(body)).toContain('Use that');
});

test('the consistency meter explains contradictions and offers fixes', () => {
  const report = {
    context: 'alts:x', items: ['a', 'b', 'c'], weights: [0.5, 0.3, 0.2], cr: 0.3, threshold: 0.05, consistent: false,
    complete: true, connected: true, missing: 0, pairs: 3, suggestions: [{ a: 'a', b: 'c', current: 0.25, suggested: 3 }], matrix: [],
  };
  const { body } = render(ConsistencyMeter, {
    props: { report, items: [{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }, { id: 'c', name: 'C' }], kind: 'alternatives', scale: 'saaty9', onfix: () => {} },
  });
  expect(text(body)).toContain('Some answers contradict the others');
  expect(text(body)).toContain('You said: C is moderately to strongly better than A.');
  expect(text(body)).toContain('Your other answers suggest: A is moderately better than C.');
  expect(text(body)).toContain('Use that');
});

test('the balance names the strengths before anything is chosen', () => {
  const { body } = render(Balance, {
    props: { a: { name: 'A' }, b: { name: 'B' }, value: undefined, scale: 'saaty9', kind: 'criteria', onchange: () => {} },
  });
  const words = text(body);
  expect(words.match(/Extreme/g)).toHaveLength(2);
  expect(words.match(/Very strong/g)).toHaveLength(2);
  expect(words).toContain('Equal');
  const classic = text(render(Balance, { props: { a: { name: 'A' }, b: { name: 'B' }, value: undefined, scale: 'classic5', kind: 'criteria', onchange: () => {} } }).body);
  expect(classic.match(/Much/g)).toHaveLength(2);
});

test('the pair list header names only the main strengths, so the words never collide', () => {
  const t = text(render(PairList, { props: listProps(true) }).body);
  expect(t).not.toContain('Very strong');
  expect(t.match(/Strong/g)).toHaveLength(2);
});
