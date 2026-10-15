import { expect, test } from '@playwright/test';
import { addAll, createDecision, saved } from './helpers';

test('duplicate, export and delete from the library', async ({ page }) => {
  await createDecision(page, 'Office chairs');
  await page.goto('/');
  const row = page.getByRole('listitem').filter({ has: page.getByText('Office chairs', { exact: true }) });
  await row.getByRole('button', { name: 'Duplicate' }).click();
  await expect(page.getByText('Office chairs (copy)')).toBeVisible();

  const download = page.waitForEvent('download');
  await row.getByRole('link', { name: 'Export' }).click();
  expect((await download).suggestedFilename()).toBe('office-chairs.ahp.json');

  const copy = page.getByRole('listitem').filter({ hasText: 'Office chairs (copy)' });
  await copy.getByRole('button', { name: 'Delete' }).click();
  await copy.getByRole('button', { name: 'Delete' }).click(); // confirm
  await expect(page.getByText('Office chairs (copy)')).toHaveCount(0);
});

test('phone-sized screens never scroll sideways', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createDecision(page, 'Phones with long option names to wrap');
  await addAll(page, 'New option', ['A rather long option name', 'Another quite long option', 'Third']);
  await addAll(page, 'New criterion', ['Battery endurance', 'Camera quality']);
  await saved(page);
  for (const step of ['setup', 'weigh', 'score', 'results', 'sensitivity']) {
    await page.goto(page.url().replace(/\/(setup|weigh|score|results|sensitivity)$/, `/${step}`));
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow, `${step} overflows by ${overflow}px`).toBeLessThanOrEqual(1);
  }
});

test('importing a file that is not a project explains what went wrong', async ({ page }) => {
  await page.goto('/');
  await page.locator('input[type=file]').setInputFiles({ name: 'notes.json', mimeType: 'application/json', buffer: Buffer.from('{"hello": "world"}') });
  await expect(page.getByRole('alert')).toContainText("notes.json couldn't be imported: not an AHP Matrix project file");
});
