import { render } from 'svelte/server';
import { expect, test } from 'vitest';
import Balance from './Balance.svelte';
import ConsistencyMeter from './ConsistencyMeter.svelte';
import MatrixGrid from './MatrixGrid.svelte';

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

test('the grid shows reciprocals and flags suggested pairs', () => {
  const items = [{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }, { id: 'c', name: 'C' }];
  const values: Record<string, number> = { '0,1': 3, '1,0': 1 / 3 };
  const { body } = render(MatrixGrid, {
    props: { items, get: (i: number, j: number) => values[`${i},${j}`], suggestions: [{ a: 'b', b: 'a', current: 1 / 3, suggested: 1 }], onpick: () => {} },
  });
  expect(text(body)).toContain('1/3');
  expect(body.match(/class="cell[^"]*\bflag\b/g)).toHaveLength(2); // both orientations of the pair
  expect(body.match(/class="cell[^"]*\bempty\b/g)).toHaveLength(4);
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
