import { expect, test } from '@playwright/test';
import { addAll, createDecision, openPeople, personRow, saved } from './helpers';

test('a person asked only to weigh the criteria has just those comparisons to make', async ({ page }) => {
  await createDecision(page, 'Roles');
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Speed', 'Cost', 'Comfort']);
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  await personRow(page, 'Priya').getByLabel('What Priya judges').selectOption({ label: 'Weigh criteria' });
  await saved(page);
  await expect(personRow(page, 'Priya')).toContainText('0 of 3 done'); // 3 criteria pairs, no option scoring
  await expect(personRow(page, 'Priya')).toContainText('Not invited');
});

test('inviting someone records it, and changing the decision afterwards marks their copy out of date', async ({ page }) => {
  await createDecision(page, 'Invites');
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Speed', 'Cost']);
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  await saved(page);

  const download = page.waitForEvent('download');
  await personRow(page, 'Priya').getByRole('button', { name: 'Invite' }).click();
  expect((await download).suggestedFilename()).toBe('invites-for-priya.ahp.json');
  await expect(personRow(page, 'Priya')).toContainText('Invited');
  await saved(page);

  await page.getByRole('link', { name: /Set up/ }).click();
  await addAll(page, 'New criterion', ['Comfort']);
  await saved(page);
  await openPeople(page);
  await expect(personRow(page, 'Priya')).toContainText('Out of date');
  await expect(personRow(page, 'Priya').getByRole('button', { name: 'Send updated copy' })).toBeVisible();
});

test('custom tasks pick which criteria a person scores the options on', async ({ page }) => {
  await createDecision(page, 'Custom');
  await addAll(page, 'New option', ['X', 'Y', 'Z']);
  await addAll(page, 'New criterion', ['Speed', 'Cost']);
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  await personRow(page, 'Priya').getByLabel('What Priya judges').selectOption({ label: 'Custom' });
  await personRow(page, 'Priya').getByLabel('Weigh the criteria').uncheck();
  await personRow(page, 'Priya').getByLabel('Score on Cost').uncheck();
  await saved(page);
  await expect(personRow(page, 'Priya')).toContainText('0 of 3 done'); // 3 option pairs on Speed
});

test('how people are combined can be switched to combining rankings', async ({ page }) => {
  await createDecision(page, 'Methods');
  await openPeople(page);
  await page.getByRole('radio', { name: /Combine rankings/ }).click();
  await saved(page);
  await page.reload();
  await expect(page.getByRole('radio', { name: /Combine rankings/ })).toBeChecked();
});

test('the People step fits a phone', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createDecision(page, 'Phone people');
  await openPeople(page);
  await addAll(page, 'New person', ['Priya Raghunathan-Okafor']);
  await saved(page);
  const width = await page.evaluate(() => document.documentElement.scrollWidth);
  expect(width).toBeLessThanOrEqual(390);
  await expect(personRow(page, 'Priya Raghunathan-Okafor').getByRole('button', { name: 'Invite' })).toBeInViewport({ ratio: 1 });
});
