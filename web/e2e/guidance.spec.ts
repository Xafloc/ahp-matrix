import { expect, test } from '@playwright/test';
import { addAll, createDecision, saved } from './helpers';

test('hovering a strength previews the sentence before choosing it', async ({ page }) => {
  await createDecision(page, 'Preview');
  await addAll(page, 'New criterion', ['Performance', 'Portability']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await expect(page.getByRole('heading', { name: 'Which matters more, and by how much?' })).toBeVisible();
  await expect(page.getByText('Very strong').first()).toBeVisible(); // words are there before any choice

  await page.getByRole('button', { name: 'Performance matters strongly more than Portability' }).hover();
  await expect(page.getByText('Performance would matter strongly more than Portability')).toBeVisible();
  await page.mouse.move(0, 0);
  await expect(page.getByText('Not compared yet')).toBeVisible();
});

test('the results say how several people are combined', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  await expect(page.getByText('Comparisons are combined with a weighted geometric mean')).toBeVisible();
  await page.getByRole('link', { name: /Set up/ }).click();
  await page.getByText('How are people combined?').click();
  await expect(page.getByText('Comparisons are combined with a weighted geometric mean')).toBeVisible();
});
