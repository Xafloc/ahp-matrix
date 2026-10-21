import { expect, test } from '@playwright/test';
import { addAll, createDecision, openPeople, saved } from './helpers';

test('leaving with changes that could not be saved asks first, and can go back to them', async ({ page }) => {
  await createDecision(page, 'Risky');
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  await saved(page);
  await page.getByLabel('Weight').nth(1).fill('-1'); // the server rejects this
  await page.getByLabel('Weight').nth(1).blur(); // weights apply when you finish editing them
  await expect(page.getByText("Priya's weight must be more than 0.")).toBeVisible();
  await page.getByRole('link', { name: /Set up/ }).click();
  await page.getByLabel('What is this decision about?').fill('Notes I care about');
  await page.getByRole('link', { name: 'All decisions' }).click();

  await expect(page.getByRole('heading', { name: 'Your changes to Risky aren’t saved' })).toBeVisible();
  await page.getByRole('button', { name: 'Go back to Risky' }).click();
  await expect(page.getByLabel('What is this decision about?')).toHaveValue('Notes I care about');
  await openPeople(page);

  await page.getByLabel('Weight').nth(1).fill('1');
  await page.getByLabel('Weight').nth(1).blur();
  await saved(page);
  await page.getByRole('link', { name: 'All decisions' }).click();
  await expect(page.getByRole('heading', { name: 'Decisions' })).toBeVisible();
});

test('closing the tab with unsaved changes asks for confirmation', async ({ page }) => {
  await createDecision(page, 'Closing');
  await openPeople(page);
  await page.getByLabel('Weight').first().fill('-1');
  await page.getByLabel('Weight').first().blur();
  await expect(page.getByText('Not saved.')).toBeVisible();
  const prompted = page.waitForEvent('dialog').then((d) => (d.dismiss(), d.type()));
  await page.close({ runBeforeUnload: true });
  expect(await prompted).toBe('beforeunload');
});

test('Enter on a focused link or button does what it says, not "next pair"', async ({ page }) => {
  await createDecision(page, 'Keys');
  await addAll(page, 'New option', ['A', 'B']);
  await addAll(page, 'New criterion', ['One', 'Two']);
  await saved(page);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('tab', { name: 'All pairs' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('slider', { name: 'Compare One with Two' })).toBeVisible();
  await page.getByRole('link', { name: /Results/ }).focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/results$/);
});

test('nesting criteria with Alt+arrow keys keeps the focus on the criterion', async ({ page }) => {
  await createDecision(page, 'Tree keys');
  await addAll(page, 'New criterion', ['Cost', 'Licence', 'Support']);
  await saved(page);
  const licence = page.getByLabel('Criterion name').nth(1);
  await licence.focus();
  await page.keyboard.press('Alt+ArrowRight'); // into Cost
  await expect(page.getByLabel('Criterion name').nth(1)).toBeFocused();
  await page.keyboard.press('Alt+ArrowLeft'); // back out
  await page.keyboard.press('Alt+ArrowDown'); // below Support
  await expect(page.getByLabel('Criterion name').nth(2)).toBeFocused();
  await expect(page.getByLabel('Criterion name').nth(2)).toHaveValue('Licence');
});

test('the printed report is dark ink on white, with its bars, even in dark mode', async ({ page }) => {
  await createDecision(page, 'Printed');
  await addAll(page, 'New option', ['A', 'B']);
  await addAll(page, 'New criterion', ['Only']);
  await saved(page);
  await page.goto(page.url().replace(/\/setup$/, '/report'));
  await page.emulateMedia({ media: 'print', colorScheme: 'dark' });
  const heading = await page.getByRole('heading', { name: 'Printed' }).evaluate((el) => getComputedStyle(el).color);
  const [r, g, b] = heading.match(/\d+/g)!.map(Number);
  expect(r + g + b, `heading color ${heading}`).toBeLessThan(200);
  const adjust = await page.locator('.bar').first().evaluate((el) => getComputedStyle(el).printColorAdjust);
  expect(adjust).toBe('exact');
});

test('people can be switched on a phone', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createDecision(page, 'Phone judges');
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  await saved(page);
  await expect(page.getByLabel('Judging as')).toBeVisible();
});

test('pages load without security-policy errors', async ({ page }) => {
  const violations: string[] = [];
  page.on('console', (m) => m.type() === 'error' && /Content Security Policy/i.test(m.text()) && violations.push(m.text()));
  await createDecision(page, 'Clean console');
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  expect(violations).toEqual([]);
});
