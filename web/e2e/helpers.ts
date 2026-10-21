import { readFileSync, writeFileSync } from 'node:fs';
import { basename } from 'node:path';
import { expect, type Locator, type Page } from '@playwright/test';

/** Creates a decision from the library and returns once its setup screen is open. */
export async function createDecision(page: Page, name: string) {
  await page.goto('/');
  await page.getByRole('button', { name: 'New decision' }).first().click();
  await page.getByLabel('What are you deciding?').fill(name);
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page.getByLabel('Decision name')).toHaveValue(name);
}

export async function addAll(page: Page, field: string, names: string[]) {
  for (const n of names) {
    await page.getByLabel(field).fill(n);
    await page.getByLabel(field).press('Enter');
  }
}

export const saved = (page: Page) => expect(page.getByText('All changes saved')).toBeVisible();

/** Opens the People step of the open decision. */
export async function openPeople(page: Page) {
  await page.getByRole('navigation', { name: 'Steps' }).getByRole('link', { name: /People/ }).click();
  await expect(page.getByRole('heading', { name: 'Who is judging' })).toBeVisible();
}

/** The People step's row for one person. */
export const personRow = (page: Page, name: string) =>
  page.getByRole('listitem').filter({ has: page.getByRole('button', { name: `Remove ${name}` }) });

/** Drops a file from disk onto an element, as if dragged in from the desktop. */
export async function dropFile(page: Page, target: Locator, path: string) {
  const b64 = readFileSync(path).toString('base64');
  const dt = await page.evaluateHandle(
    ({ b64, name }) => {
      const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
      const dt = new DataTransfer();
      dt.items.add(new File([bytes], name, { type: 'application/json' }));
      return dt;
    },
    { b64, name: basename(path) },
  );
  await target.dispatchEvent('dragover', { dataTransfer: dt });
  await target.dispatchEvent('drop', { dataTransfer: dt });
}

/**
 * Invites name from the People step and saves their copy to path, with one
 * answer filled in on each named criterion (the first two options, 3 to 1).
 */
export async function answeredCopy(page: Page, name: string, path: string, on: string[]) {
  await openPeople(page);
  const download = page.waitForEvent('download');
  await personRow(page, name).getByRole('button', { name: 'Invite' }).click();
  await (await download).saveAs(path);
  await saved(page);
  const copy = JSON.parse(readFileSync(path, 'utf8'));
  const who = copy.evaluators[0].id;
  for (const c of copy.criteria.filter((c: { name: string }) => on.includes(c.name)))
    copy.pairwise.push({ evaluatorId: who, context: `alts:${c.id}`, a: copy.alternatives[0].id, b: copy.alternatives[1].id, value: 3 });
  writeFileSync(path, JSON.stringify(copy));
}
