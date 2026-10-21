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
  await expect(page.getByText("Results are hidden until Sam finishes, so others' answers don't sway Sam's.")).toBeVisible();
  await page.getByRole('navigation', { name: 'Steps' }).getByRole('link', { name: /Results/ }).click();
  await expect(page.getByText("Results are hidden until Sam finishes")).toBeVisible();
  await expect(page.getByRole('link', { name: 'Go to the next comparison' })).toHaveAttribute('href', /\/weigh\//);
  await page.getByRole('button', { name: 'Show anyway' }).click();
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();

  await page.getByLabel('Now judging as').selectOption({ label: 'Engineering lead' });
  await expect(page.getByRole('heading', { name: /comes out ahead/ })).toBeVisible();
});
