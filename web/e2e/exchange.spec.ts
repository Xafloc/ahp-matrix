import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';
import { addAll, createDecision, openPeople, personRow, saved } from './helpers';

const fixture = fileURLToPath(new URL('./fixtures/Priority Matrix - Example.xlsx', import.meta.url));

test('an example opens straight into its results', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Buying a family car/ }).first().click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
  await expect(page.getByLabel('Decision name')).toHaveValue('Buying a family car');
});

test('a template starts with its criteria and no options', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'New decision' }).first().click();
  await page.getByLabel('What are you deciding?').fill('CRM vendors');
  await page.getByLabel('Start from').selectOption({ label: 'Template: Vendor selection' });
  await expect(page.getByText('The 23 criteria of the original Priority Matrix spreadsheet')).toBeVisible();
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page.getByLabel('Decision name')).toHaveValue('CRM vendors');
  await expect(page.getByLabel('Criterion name')).toHaveCount(27);
  await expect(page.getByLabel('Option name')).toHaveCount(0);
});

test('a Priority Matrix spreadsheet imports with a summary of what came across', async ({ page }) => {
  await page.goto('/');
  await page.locator('input[type=file]').setInputFiles(fixture);
  await expect(page.getByRole('status')).toContainText('Imported Priority Matrix - Example: 3 options, 3 criteria and 7 comparisons.');
  await expect(page.getByRole('status')).toContainText('Criterion 3!E7: “n/a” is not a number');
  await page.getByRole('link', { name: 'Open it' }).click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
});

test('results download as Excel and CSV', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Choosing a cloud provider/ }).first().click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
  for (const [link, file] of [['Download Excel', 'choosing-a-cloud-provider.xlsx'], ['Download CSV', 'choosing-a-cloud-provider.csv']]) {
    const download = page.waitForEvent('download');
    await page.getByRole('link', { name: link }).click();
    expect((await download).suggestedFilename()).toBe(file);
  }
});

test("a colleague's returned copy merges back, after asking", async ({ page }, info) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  await openPeople(page);

  const download = page.waitForEvent('download');
  await personRow(page, 'Product manager').getByRole('button', { name: 'Invite' }).click();
  const copy = await download;
  expect(copy.suggestedFilename()).toBe('hiring-a-senior-engineer-for-product-manager.ahp.json');
  const path = info.outputPath(copy.suggestedFilename());
  await copy.saveAs(path);

  await page.getByLabel('Returned decision file to merge').setInputFiles(path);
  await expect(page.getByRole('alert')).toContainText('Product manager already has judgments here.');
  await page.getByRole('button', { name: 'Replace' }).click();
  // 6 criteria pairs + 4 × 3 option pairs
  await expect(page.getByRole('status').filter({ hasText: 'Merged' })).toContainText('Merged 18 judgments from Product manager.');
});

test('↑ and ↓ move the balance like → and ←', async ({ page }) => {
  await createDecision(page, 'Arrows');
  await addAll(page, 'New criterion', ['Left', 'Right']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  const slider = page.getByRole('slider');
  await slider.press('ArrowUp');
  await expect(page.getByText('Right matters slightly more than Left')).toBeVisible();
  await slider.press('ArrowDown');
  await slider.press('ArrowDown');
  await expect(page.getByText('Left matters slightly more than Right')).toBeVisible();
});

test("someone's first returned copy merges without asking", async ({ page }, info) => {
  await createDecision(page, 'First return');
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Only']);
  await openPeople(page);
  await addAll(page, 'New person', ['Carol']);
  await saved(page);
  const download = page.waitForEvent('download');
  await personRow(page, 'Carol').getByRole('button', { name: 'Invite' }).click();
  const path = info.outputPath('carol.ahp.json');
  await (await download).saveAs(path);
  // Carol fills in her one comparison.
  const copy = JSON.parse(readFileSync(path, 'utf8'));
  const carol = copy.evaluators[0].id;
  copy.pairwise.push({ evaluatorId: carol, context: `alts:${copy.criteria[0].id}`, a: copy.alternatives[0].id, b: copy.alternatives[1].id, value: 3 });
  writeFileSync(path, JSON.stringify(copy));

  await page.getByLabel('Returned decision file to merge').setInputFiles(path);
  await expect(page.getByRole('status').filter({ hasText: 'Merged' })).toContainText('Merged 1 judgment from Carol.');
  await expect(page.getByText('already has judgments here')).toHaveCount(0);
});

test('a returned file that fails to merge clears the question and says why', async ({ page }, info) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  await openPeople(page);
  const download = page.waitForEvent('download');
  await personRow(page, 'Product manager').getByRole('button', { name: 'Invite' }).click();
  const path = info.outputPath('pm.ahp.json');
  await (await download).saveAs(path);
  const copy = JSON.parse(readFileSync(path, 'utf8'));
  copy.pairwise[0].value = -3; // corrupted on the way back
  writeFileSync(path, JSON.stringify(copy));

  await page.getByLabel('Returned decision file to merge').setInputFiles(path);
  await page.getByRole('button', { name: 'Replace' }).click();
  await expect(page.getByRole('alert')).toContainText("pm.ahp.json couldn't be merged");
  await expect(page.getByRole('button', { name: 'Replace' })).toHaveCount(0);
});
