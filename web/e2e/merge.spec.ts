import { expect, test } from '@playwright/test';
import { addAll, answeredCopy, createDecision, dropFile, openPeople, personRow, saved } from './helpers';

async function decisionWithCarol(page: import('@playwright/test').Page, name: string) {
  await createDecision(page, name);
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Speed', 'Cost']);
  await openPeople(page);
  await addAll(page, 'New person', ['Carol']);
  await saved(page);
}

test('a returned file dropped on the decision shows what it holds, then merges', async ({ page }, info) => {
  await decisionWithCarol(page, 'Dropping');
  const path = info.outputPath('carol.ahp.json');
  await answeredCopy(page, 'Carol', path, ['Speed', 'Cost']);

  await dropFile(page, page.locator('main'), path);
  const dialog = page.getByRole('dialog', { name: "Merge Carol's answers into Dropping?" });
  await expect(dialog).toContainText('2 answers');
  await expect(dialog).toContainText('New here');
  await dialog.getByRole('button', { name: 'Merge' }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Merged' })).toContainText('Merged 2 judgments from Carol.');
  await expect(personRow(page, 'Carol')).toContainText('Returned');
  await expect(personRow(page, 'Carol')).toContainText('2 of 3 done');
});

test('Undo puts the decision back as it was before the merge', async ({ page }, info) => {
  await decisionWithCarol(page, 'Undoing');
  const path = info.outputPath('carol.ahp.json');
  await answeredCopy(page, 'Carol', path, ['Speed']);
  await page.getByLabel('Returned decision file to merge').setInputFiles(path);
  await page.getByRole('dialog').getByRole('button', { name: 'Merge' }).click();
  await expect(personRow(page, 'Carol')).toContainText('1 of 3 done');

  await page.getByRole('button', { name: 'Undo' }).click();
  await expect(page.getByText('Merge undone.')).toBeVisible();
  await expect(personRow(page, 'Carol')).toContainText('0 of 3 done');
  await expect(personRow(page, 'Carol')).toContainText('Invited');
});

test('answers about a criterion removed since the copy went out are listed, not lost silently', async ({ page }, info) => {
  await decisionWithCarol(page, 'Dropped lines');
  const path = info.outputPath('carol.ahp.json');
  await answeredCopy(page, 'Carol', path, ['Speed', 'Cost']);
  await page.getByRole('link', { name: /Set up/ }).click();
  await page.getByRole('button', { name: 'Remove Cost' }).click();
  await page.getByRole('button', { name: 'Remove', exact: true }).click();
  await saved(page);

  await dropFile(page, page.locator('main'), path);
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('1 answer');
  await expect(dialog).toContainText("1 answer can't come across");
  await expect(dialog).toContainText('Comparison of X with Y on Cost: Cost is no longer in this decision');
  await expect(dialog).toContainText('This copy was made before the decision last changed');
});

test('Import file recognises answers sent back and offers to merge them', async ({ page }, info) => {
  await decisionWithCarol(page, 'Recognised');
  const path = info.outputPath('carol.ahp.json');
  await answeredCopy(page, 'Carol', path, ['Speed']);
  await page.getByRole('link', { name: 'All decisions' }).click();

  await page.getByLabel('Import a decision file or Priority Matrix spreadsheet').setInputFiles(path);
  await expect(page.getByText('carol.ahp.json looks like answers sent back for Recognised.')).toBeVisible();
  await page.getByRole('button', { name: 'Merge into Recognised' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Merge' }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Merged' })).toContainText('Merged 1 judgment from Carol.');
  await expect(page.getByRole('link', { name: /Recognised/ })).toHaveCount(1); // no duplicate made
});

test('a returned file can be dropped on its row in the library', async ({ page }, info) => {
  await decisionWithCarol(page, 'Row drop');
  const path = info.outputPath('carol.ahp.json');
  await answeredCopy(page, 'Carol', path, ['Speed']);
  await page.getByRole('link', { name: 'All decisions' }).click();
  await dropFile(page, page.getByRole('listitem').filter({ hasText: 'Row drop' }), path);
  await expect(page.getByRole('dialog', { name: "Merge Carol's answers into Row drop?" })).toBeVisible();
});
