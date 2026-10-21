import { expect, test, type Page } from '@playwright/test';
import { addAll, createDecision, dropFile, openPeople, personRow, saved } from './helpers';

/** Sets up a decision with Priya, invites her with the web page, and returns where it was saved. */
async function invitePage(page: Page, name: string, criteria: string[], path: string) {
  await createDecision(page, name);
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', criteria);
  await openPeople(page);
  await page.getByLabel("Person's name").first().fill('Darren');
  await addAll(page, 'New person', ['Priya']);
  await saved(page);
  await personRow(page, 'Priya').getByRole('button', { name: 'Invite' }).click();
  const download = page.waitForEvent('download');
  await page.getByRole('dialog').getByRole('button', { name: 'Download web page' }).click();
  await (await download).saveAs(path);
  await page.getByRole('dialog').getByRole('button', { name: 'Done' }).click();
  await saved(page);
}

test('someone without the app answers in the web page, and their answers merge back', async ({ page, context }, info) => {
  const html = info.outputPath('invite.html');
  await invitePage(page, 'Round trip', ['Speed', 'Cost'], html);

  const priya = await context.newPage();
  await priya.goto(`file://${html}`);
  await expect(priya.getByRole('heading', { name: 'Darren asked for your view on Round trip' })).toBeVisible();
  await priya.getByRole('button', { name: 'Start' }).click();
  for (const heading of ['Which matters more, and by how much?', 'Which is better on Speed, and by how much?', 'Which is better on Cost, and by how much?']) {
    await expect(priya.getByRole('heading', { name: heading })).toBeVisible();
    await priya.getByRole('slider').press('ArrowLeft');
    if (heading.includes('Cost')) break;
    await priya.getByRole('button', { name: 'Next step' }).click();
  }
  const back = priya.waitForEvent('download');
  await priya.getByRole('button', { name: 'Finish and send back' }).click();
  const answers = await back;
  expect(answers.suggestedFilename()).toBe('round-trip-answers-from-priya.ahp.json');
  await expect(priya.getByText('Email it to Darren')).toBeVisible();
  const returned = info.outputPath(answers.suggestedFilename());
  await answers.saveAs(returned);

  await dropFile(page, page.locator('main'), returned);
  const dialog = page.getByRole('dialog', { name: "Merge Priya's answers into Round trip?" });
  await expect(dialog).toContainText('3 answers');
  await dialog.getByRole('button', { name: 'Merge' }).click();
  await expect(personRow(page, 'Priya')).toContainText('Returned');
  await expect(personRow(page, 'Priya')).toContainText('3 of 3 done');
});

test('the web page keeps progress, flags contradictions with Why?, and fits a phone', async ({ page, context }, info) => {
  const html = info.outputPath('invite.html');
  await invitePage(page, 'Contradictions', ['Speed', 'Cost', 'Comfort'], html);

  const priya = await context.newPage();
  await priya.setViewportSize({ width: 390, height: 844 });
  await priya.goto(`file://${html}`);
  await priya.getByRole('button', { name: 'Start' }).click();
  await priya.getByRole('tab', { name: 'All pairs' }).click();
  const press = async (label: string, key: string) => {
    for (let i = 0; i < 8; i++) await priya.getByRole('slider', { name: label }).press(key);
  };
  await press('Compare Speed with Cost', 'ArrowLeft');
  await press('Compare Cost with Comfort', 'ArrowLeft');
  await press('Compare Speed with Comfort', 'ArrowRight');
  await expect(priya.getByRole('button', { name: 'Why?' }).first()).toBeVisible();
  expect(await priya.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);

  await priya.reload();
  await expect(priya.getByRole('button', { name: 'Continue where you left off' })).toBeVisible();
});
