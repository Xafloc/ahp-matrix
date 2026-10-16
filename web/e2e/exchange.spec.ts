import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';
import { addAll, createDecision, saved } from './helpers';

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
  await page.getByRole('link', { name: /Set up/ }).click();

  const download = page.waitForEvent('download');
  await page.getByRole('listitem').filter({ has: page.getByRole('button', { name: 'Remove Product manager' }) }).getByRole('link', { name: 'Send their copy' }).click();
  const copy = await download;
  expect(copy.suggestedFilename()).toBe('hiring-a-senior-engineer-for-product-manager.ahp.json');
  const path = info.outputPath(copy.suggestedFilename());
  await copy.saveAs(path);

  await page.getByLabel('Returned decision file to merge').setInputFiles(path);
  await expect(page.getByRole('alert')).toContainText('Product manager already has judgments here.');
  await page.getByRole('button', { name: 'Replace' }).click();
  // 6 criteria pairs + 4 × 3 option pairs
  await expect(page.getByRole('status')).toContainText('Merged 18 judgments from Product manager.');
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
