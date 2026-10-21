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
  await page.getByRole('link', { name: /People/ }).click();
  await expect(page.getByRole('radio', { name: /Combine answers/ })).toBeChecked();
  await expect(page.getByRole('radio', { name: /Combine answers/ })).toContainText('weighted geometric mean');
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

test('Why? explains a suggestion with the answers behind it, and can apply it', async ({ page }) => {
  await createDecision(page, 'Why');
  await addAll(page, 'New criterion', ['Technical depth', 'System design', 'Communication', 'Team fit']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).click();
  const pick = (a: string, b: string, label: string) =>
    page.getByRole('slider', { name: `Compare ${a} with ${b}` }).getByRole('button', { name: label, exact: true }).click();
  // The answers from the screenshot: everything equal except Technical depth 5× and System design 3× Communication.
  await pick('Technical depth', 'System design', 'Technical depth and System design matter equally');
  await pick('Technical depth', 'Communication', 'Technical depth matters strongly more than Communication');
  await pick('Technical depth', 'Team fit', 'Technical depth and Team fit matter equally');
  await pick('System design', 'Communication', 'System design matters moderately more than Communication');
  await pick('System design', 'Team fit', 'System design and Team fit matter equally');
  await pick('Communication', 'Team fit', 'Communication and Team fit matter equally');
  await saved(page);
  await expect(page.getByText('Some answers contradict the others')).toBeVisible();

  const row = page.locator('li.flag').filter({ hasText: 'Technical depth matters strongly more than Communication' });
  await row.getByRole('button', { name: 'Why?' }).click();
  const dialog = page.getByRole('dialog', { name: 'Why this suggestion?' });
  await expect(dialog).toContainText('You said: Technical depth matters strongly more than Communication (5).');
  await expect(dialog).toContainText('Together, these mean Technical depth and Communication should matter about equally.');
  await expect(dialog).toContainText('the consistency ratio goes from 0.10 to 0.06, within the 0.08 limit');

  await dialog.getByRole('button', { name: 'Use the suggestion' }).click();
  await expect(dialog).toBeHidden();
  await saved(page);
  await expect(page.getByText('Technical depth matters moderately more than Communication', { exact: true })).toBeVisible();
  await expect(page.getByText('Your answers agree with each other')).toBeVisible();
});

test('Why? is also there in the one-pair view, and Keep my answer changes nothing', async ({ page }) => {
  await createDecision(page, 'Why one');
  await addAll(page, 'New criterion', ['A', 'B', 'C']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  const slider = page.getByRole('slider');
  await slider.press('Home');
  await slider.press('Enter');
  await slider.press('Home');
  await slider.press('Enter');
  await slider.press('End');
  await expect(page.getByText('Some answers contradict the others')).toBeVisible();
  await page.getByRole('button', { name: 'Why?' }).first().click();
  const dialog = page.getByRole('dialog', { name: 'Why this suggestion?' });
  await expect(dialog).toContainText('You said:');
  await dialog.getByRole('button', { name: 'Keep my answer' }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText('Some answers contradict the others')).toBeVisible();
});

test('the Why? window sits above everything, with a backdrop', async ({ page }) => {
  await createDecision(page, 'Modal');
  await addAll(page, 'New criterion', ['A', 'B', 'C']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).click();
  await page.getByRole('slider', { name: 'Compare A with B' }).press('Home');
  await page.getByRole('slider', { name: 'Compare B with C' }).press('Home');
  await page.getByRole('slider', { name: 'Compare A with C' }).press('End');
  await page.locator('li.flag').first().getByRole('button', { name: 'Why?' }).click();
  const modal = await page.locator('dialog').evaluate((d) => d.matches(':modal'));
  expect(modal).toBe(true);
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toBeHidden();
});

test('on a phone, the Why? actions are visible without scrolling', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 700 });
  await createDecision(page, 'Phone why');
  await addAll(page, 'New criterion', ['Technical depth', 'System design', 'Communication']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).click();
  await page.getByRole('slider', { name: 'Compare Technical depth with System design' }).press('Home');
  await page.getByRole('slider', { name: 'Compare System design with Communication' }).press('Home');
  await page.getByRole('slider', { name: 'Compare Technical depth with Communication' }).press('End');
  await page.locator('li.flag').first().getByRole('button', { name: 'Why?' }).click();
  await expect(page.getByRole('button', { name: 'Use the suggestion' })).toBeInViewport({ ratio: 1 });
  await expect(page.getByRole('button', { name: 'Keep my answer' })).toBeInViewport({ ratio: 1 });
});

test('the balance names fewer strengths on a phone, so the words never overlap', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createDecision(page, 'Phone balance');
  await addAll(page, 'New criterion', ['Speed', 'Price']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  const words = page.locator('.words span').filter({ hasText: /\S/ });
  const boxes = await words.evaluateAll((els) =>
    els.filter((e) => getComputedStyle(e).visibility !== 'hidden').map((e) => {
      const r = document.createRange();
      r.selectNodeContents(e);
      const b = r.getBoundingClientRect();
      return [b.left, b.right];
    }),
  );
  for (let i = 1; i < boxes.length; i++) expect(boxes[i][0], `label ${i} overlaps the one before`).toBeGreaterThanOrEqual(boxes[i - 1][1]);
});
