import { expect, test, type Page } from '@playwright/test';
import { addAll, createDecision, openPeople, personRow, saved } from './helpers';

/** Sets up "name" with Priya asked only to score the options, invites her, and imports her copy as she would. */
async function openPriyasCopy(page: Page, name: string, path: string) {
  await createDecision(page, name);
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Speed', 'Cost']);
  await openPeople(page);
  await page.getByLabel("Person's name").first().fill('Darren');
  await addAll(page, 'New person', ['Priya']);
  await personRow(page, 'Priya').getByLabel('What Priya judges').selectOption({ label: 'Score options' });
  await saved(page);
  const download = page.waitForEvent('download');
  await personRow(page, 'Priya').getByRole('button', { name: 'Invite' }).click();
  await (await download).saveAs(path);
  await saved(page);

  await page.getByRole('link', { name: 'All decisions' }).click();
  await page.getByLabel('Import a decision file or Priority Matrix spreadsheet').setInputFiles(path);
}

test('a copy you were sent opens on your first task, says who asked, and keeps the structure fixed', async ({ page }, info) => {
  await openPriyasCopy(page, 'Sent to Priya', info.outputPath('priya.ahp.json'));
  await expect(page.getByText("Darren asked you to judge this decision. You're judging as Priya.")).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Which is better on Speed, and by how much?' })).toBeVisible();

  await page.getByRole('link', { name: /Set up/ }).click();
  await expect(page.getByText('The options and criteria are set by Darren')).toBeVisible();
  await expect(page.getByLabel('New option')).toBeDisabled();
  await expect(page.getByLabel('New criterion')).toBeDisabled();
  await expect(page.getByLabel('Option name').first()).toBeDisabled();

  await openPeople(page);
  await expect(page.getByRole('button', { name: 'Invite' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Merge returned answers' })).toHaveCount(0);
  await expect(page.getByLabel('New person')).toHaveCount(0);
});

test('Send your answers back downloads a file named for the person, and says what to do with it', async ({ page }, info) => {
  await openPriyasCopy(page, 'Answers back', info.outputPath('priya.ahp.json'));
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Send your answers back' }).click();
  expect((await download).suggestedFilename()).toBe('answers-back-answers-from-priya.ahp.json');
  await expect(page.getByText('Email answers-back-answers-from-priya.ahp.json to Darren')).toBeVisible();
});

test("the compare screens grey out what isn't yours to judge", async ({ page }) => {
  await createDecision(page, 'Filtered');
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Speed', 'Cost']);
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  const row = personRow(page, 'Priya');
  await row.getByLabel('What Priya judges').selectOption({ label: 'Custom' });
  await row.getByLabel('Weigh the criteria').uncheck();
  await row.getByLabel('Score on Speed').uncheck();
  await saved(page);
  await page.getByLabel('Judging as').selectOption({ label: 'Priya' });

  await page.getByRole('navigation', { name: 'Steps' }).getByRole('link', { name: /Score options/ }).click();
  await expect(page.getByRole('heading', { name: 'Which is better on Cost, and by how much?' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Criteria to score' }).getByRole('link', { name: /Speed/ })).toContainText('Not assigned to Priya');

  await page.getByRole('navigation', { name: 'Steps' }).getByRole('link', { name: /Weigh criteria/ }).click();
  await expect(page.getByText("Priya isn't asked to weigh the criteria.")).toBeVisible();
});
