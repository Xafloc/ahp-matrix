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

test('all pairs can be weighed on one screen', async ({ page }) => {
  await createDecision(page, 'List');
  await addAll(page, 'New criterion', ['Cost', 'Speed', 'Support']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).click();
  await expect(page.getByRole('slider')).toHaveCount(3);

  // Cost 2× Speed, Speed 2× Support, Cost 4× Support: consistent.
  await page.getByRole('slider', { name: 'Compare Cost with Speed' }).press('ArrowLeft');
  await page.getByRole('slider', { name: 'Compare Cost with Support' }).getByRole('button', { name: 'Cost matters moderately to strongly more than Support' }).click();
  await page.getByRole('slider', { name: 'Compare Speed with Support' }).press('ArrowLeft');
  await saved(page);
  await expect(page.getByText('3 of 3 pairs compared')).toBeVisible();
  await expect(page.getByText('Cost matters slightly more than Speed', { exact: true })).toBeVisible();
  await expect(page.getByText('Speed matters slightly more than Support', { exact: true })).toBeVisible();
  await expect(page.getByText('Your answers agree with each other')).toBeVisible();
  await expect(page.getByText('Suggested by your other answers')).toHaveCount(0); // consistent: nothing flagged
});

test('a contradiction in the list is flagged on its row with a fix', async ({ page }) => {
  await createDecision(page, 'Loop');
  await addAll(page, 'New criterion', ['A', 'B', 'C']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).click();
  await page.getByRole('slider', { name: 'Compare A with B' }).press('Home');
  await page.getByRole('slider', { name: 'Compare B with C' }).press('Home');
  await page.getByRole('slider', { name: 'Compare A with C' }).press('End');
  await expect(page.getByText('Some answers contradict the others')).toBeVisible();
  await expect(page.locator('li.flag').first()).toBeVisible();
  await expect(page.getByText('You said:')).toHaveCount(0); // fixes live on the rows, not repeated below
  await page.locator('li.flag').first().getByRole('button', { name: 'Use that' }).click();
  await saved(page);
});

test('the pair list fits a phone', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createDecision(page, 'Phone list');
  await addAll(page, 'New criterion', ['Total cost of ownership', 'Speed', 'Support quality']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).click();
  await expect(page.getByRole('slider')).toHaveCount(3);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(1);
});
