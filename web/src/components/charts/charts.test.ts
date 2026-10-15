import { render } from 'svelte/server';
import { expect, test } from 'vitest';
import { fixture } from '../../lib/fixture.test-helper';
import { colorOf } from '../../lib/format';
import type { Results } from '../../lib/types';
import Contributions from './Contributions.svelte';
import LineChart from './LineChart.svelte';
import PriorityBars from './PriorityBars.svelte';
import WeightTree from './WeightTree.svelte';

const text = (html: string) => html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');

const results: Results = {
  evaluator: 'all',
  scores: [
    { id: 'a1', score: 0.5, rank: 1 },
    { id: 'a2', score: 0.2, rank: 3 },
    { id: 'a3', score: 0.3, rank: 2 },
  ],
  criteria: [
    { id: 'c1', local: 0.75, global: 0.75, subtree: [0.5, 0.2, 0.3], stability: null },
    { id: 'c2', parentId: 'c1', local: 0.5, global: 0.375, subtree: [0.6, 0.2, 0.2], stability: null },
    { id: 'c3', parentId: 'c1', local: 0.5, global: 0.375, subtree: [0.4, 0.2, 0.4], stability: null },
    { id: 'c4', local: 0.25, global: 0.25, subtree: [0.5, 0.2, 0.3], stability: null },
  ],
  matrices: [],
  issues: [],
  disagreements: [],
  completeness: 1,
  consistent: true,
};

test('priority bars label every bar and can rank them', () => {
  const { body } = render(PriorityBars, { props: { names: ['A', 'B', 'C'], colors: ['red', 'green', 'blue'], values: [0.2, 0.5, 0.3], sorted: true } });
  expect(text(body)).toMatch(/1 B .*50\.0%.*2 C .*30\.0%.*3 A .*20\.0%/);
});

test('the weight tree shows local and global shares, indented', () => {
  const { body } = render(WeightTree, { props: { project: fixture(), results } });
  expect(text(body)).toMatch(/Performance 50% .*37\.5%/);
  expect(body).toMatch(/padding-left: 1\.7rem/); // depth 1
});

test('contributions are weight × priority, rows best first, with totals', () => {
  const { body } = render(Contributions, { props: { project: fixture(), results } });
  const t = text(body);
  expect(t.indexOf('Alpha')).toBeLessThan(t.indexOf('Gamma'));
  expect(t.indexOf('Gamma')).toBeLessThan(t.indexOf('Beta'));
  expect(t).toContain('22.5%'); // Alpha on Performance: 0.375 × 0.6
  expect(t).toContain('50.0%'); // Alpha's total
});

test('the line chart marks crossovers and labels lines directly', () => {
  const { body } = render(LineChart, {
    props: { line: { intercept: [0.2, 0.8], slope: [0.7, -0.7] }, names: ['Up', 'Down'], colors: ['red', 'blue'], current: 0.5, x: 0.3 },
  });
  expect(body.match(/class="cross/g)).toHaveLength(1);
  expect(body.match(/class="label/g)).toHaveLength(2);
  expect(text(body)).toContain('Ranking changes');
});

test('past eight options the colors run out, but every bar keeps its name', () => {
  const names = Array.from({ length: 9 }, (_, i) => `Option ${i + 1}`);
  const colors = names.map((_, i) => colorOf(i < 8 ? `series-${i + 1}` : ''));
  const { body } = render(PriorityBars, { props: { names, colors, values: names.map(() => 1 / 9) } });
  for (const n of names) expect(text(body)).toContain(n);
  expect(body).toContain('var(--series-none)');
});
