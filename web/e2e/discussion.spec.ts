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
