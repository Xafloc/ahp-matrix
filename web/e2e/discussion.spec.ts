import { expect, test } from '@playwright/test';
import { addAll, createDecision, openPeople, saved } from './helpers';

test('with several people, the bar says who you are answering as, and the compare screens repeat it', async ({ page }) => {
  await createDecision(page, 'Who answers');
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Speed', 'Cost']);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await expect(page.getByText(/Answering as/)).toHaveCount(0); // just one person
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  await saved(page);
  await page.getByLabel('Now judging as').selectOption({ label: 'Priya' });
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await expect(page.getByText('Answering as Priya')).toBeVisible();
});

test("results stay hidden from someone who hasn't finished, until they choose to look", async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  await openPeople(page);
  await addAll(page, 'New person', ['Sam']);
  await saved(page);
  await page.getByLabel('Now judging as').selectOption({ label: 'Sam' });

  await page.getByRole('link', { name: /What if/ }).click();
  await expect(page.getByText("Results are hidden until Sam finishes, so others' answers don't sway theirs.")).toBeVisible();
  await page.getByRole('navigation', { name: 'Steps' }).getByRole('link', { name: /Results/ }).click();
  await expect(page.getByText("Results are hidden until Sam finishes")).toBeVisible();
  await expect(page.getByRole('link', { name: 'Go to the next question' })).toHaveAttribute('href', /\/weigh\//);
  await page.getByRole('button', { name: 'Show anyway' }).click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();

  await page.getByLabel('Now judging as').selectOption({ label: 'Engineering lead' });
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
});

test("disagreements show everyone's answer on one scale, and Revise my answer opens that pair", async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  const section = page.getByRole('region', { name: 'Where people disagree most' });
  const first = section.getByRole('listitem').first();
  await expect(first.getByRole('img', { name: /^Engineering lead: / })).toBeVisible();
  await expect(first.getByRole('img', { name: /^Product manager: / })).toBeVisible();
  await expect(first.getByRole('img', { name: /^Everyone combined: / })).toBeVisible();
  const [a, b] = await first.locator('.ends span').allTextContents();
  await first.getByRole('link', { name: 'Revise my answer' }).click();
  await expect(page.getByRole('slider', { name: new RegExp(`Compare (${a} with ${b}|${b} with ${a})`) })).toBeVisible();
  await expect(page.getByText('Answering as Engineering lead')).toBeVisible();
});

test("everyone's own ranking sits beside the combined one, under a headline on whether they agree", async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  const section = page.getByRole('region', { name: "Each person's ranking" });
  await expect(section.getByText(/^(Both people rank .+ first\.|People disagree on the winner: .+\.)$/)).toBeVisible();
  const table = section.getByRole('table');
  await expect(table.getByRole('columnheader')).toHaveText(['Option', 'Everyone combined', 'Engineering lead', 'Product manager']);
  await expect(table.getByRole('row')).toHaveCount(4); // header + 3 candidates
});

test('the discussion views fit a phone without scrolling sideways', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  await page.goto('/');
  await page.getByRole('button', { name: /Hiring a senior engineer/ }).first().click();
  await expect(page.getByRole('region', { name: 'Where people disagree most' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(375);
});
