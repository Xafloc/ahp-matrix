import { render } from 'svelte/server';
import { expect, test } from 'vitest';
import { fixture } from '../lib/fixture.test-helper';
import InviteDialog from './InviteDialog.svelte';

const text = (html: string) => html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');
const props = { busy: false, onsend: () => {}, onclose: () => {} };

test('a problem with the invitation shows in the dialog, where you are looking', () => {
  const p = fixture();
  const body = text(render(InviteDialog, { props: { ...props, project: p, person: p.evaluators[1], error: "This copy of AHP Matrix can't make web pages." } }).body);
  expect(body).toContain("This copy of AHP Matrix can't make web pages.");
});

test("someone with nothing to answer can't be invited, and the dialog says why", () => {
  const p = fixture();
  p.criteria = [{ id: 'c4', name: 'Cost', description: '', notes: '', method: 'value' }];
  const { body } = render(InviteDialog, { props: { ...props, project: p, person: p.evaluators[1], error: '' } });
  expect(text(body)).toContain('Bob has nothing to answer yet');
  expect(body).toMatch(/<button[^>]*disabled[^>]*>Download web page/);
});
