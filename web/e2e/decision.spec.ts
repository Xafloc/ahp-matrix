import { expect, test } from '@playwright/test';
import { addAll, createDecision, openPeople, saved } from './helpers';

test('a decision from an empty library to a robust answer', async ({ page }) => {
  await createDecision(page, 'Lunch spot');
  await addAll(page, 'New option', ['Noodle bar', 'Taco truck', 'Salad place']);
  await addAll(page, 'New criterion', ['Taste', 'Price']);
  await page.getByLabel('How Price is scored').selectOption('value');
  await page.getByLabel('Which values are better').selectOption('lower');
  await saved(page);

  // Weigh: Taste moderately more important than Price (two steps toward Taste).
  await page.getByRole('link', { name: 'Next: weigh the criteria' }).click();
  const slider = page.getByRole('slider');
  await slider.press('ArrowLeft');
  await slider.press('ArrowLeft');
  await expect(page.getByText('Taste matters moderately more than Price')).toBeVisible();

  // Score taste in pairs: each time, the left option slightly better.
  await page.getByRole('link', { name: /Score options/ }).click();
  await expect(page.getByRole('heading', { name: 'Which is better on Taste, and by how much?' })).toBeVisible();
  for (let k = 0; k < 3; k++) {
    await page.getByRole('slider').press('ArrowLeft');
    await page.getByRole('slider').press('Enter');
  }
  await expect(page.getByText('3 of 3 pairs compared')).toBeVisible();

  // Measured prices; lower is better.
  await page.getByRole('link', { name: /Price/ }).click();
  await expect(page.getByRole('heading', { name: "Enter each option's price" })).toBeVisible();
  for (const [name, price] of [['Noodle bar', '15'], ['Taco truck', '9'], ['Salad place', '12']]) {
    await page.getByLabel(name, { exact: true }).fill(price);
  }
  await saved(page);

  await page.getByRole('link', { name: /Results/ }).click();
  await expect(page.getByRole('heading', { name: /Noodle bar comes out ahead/ })).toBeVisible();
  await expect(page.getByRole('row', { name: /Options on Taste\s+Consistent/ })).toBeVisible();

  // If taste barely mattered, the cheapest option would win.
  await page.getByRole('link', { name: /What if/ }).click();
  await page.getByRole('slider').fill('0.1');
  await expect(page.getByText('Taco truck would lead instead of Noodle bar.')).toBeVisible();

  await page.getByRole('link', { name: 'Printable report' }).click();
  await expect(page.getByRole('heading', { name: 'Lunch spot' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Ranking' })).toBeVisible();

  // It was all saved: the library shows the leader after a reload.
  await page.goto('/');
  await page.reload();
  await expect(page.getByText('Noodle bar leads with')).toBeVisible();
});

test('contradictions are flagged with a fix that resolves them', async ({ page }) => {
  await createDecision(page, 'Cycle');
  await addAll(page, 'New option', ['A', 'B', 'C']);
  await addAll(page, 'New criterion', ['Only']);
  await saved(page);
  await page.getByRole('link', { name: /Score options/ }).click();
  // A > B, B > C, and then C > A strongly: a cycle.
  const slider = page.getByRole('slider');
  await slider.press('Home'); // A extremely better than B
  await slider.press('Enter');
  await slider.press('Home'); // B extremely better than C
  await slider.press('Enter');
  await slider.press('End'); // A vs C: C extremely better
  await expect(page.getByText('Some answers contradict the others')).toBeVisible();
  const ratio = async () => Number((await page.getByText(/Consistency ratio/).textContent())!.match(/ratio ([\d.]+)/)![1]);
  const before = await ratio();
  await page.getByRole('button', { name: 'Use that' }).first().click();
  await saved(page);
  await expect.poll(ratio).toBeLessThan(before);
});

test('an edit made in two windows asks which version to keep', async ({ page, context }) => {
  await createDecision(page, 'Shared');
  const url = page.url();
  const other = await context.newPage();
  await other.goto(url);
  await expect(other.getByLabel('Decision name')).toHaveValue('Shared');

  await page.getByLabel('Decision name').fill('Shared, edited here');
  await saved(page);
  await other.getByLabel('Decision name').fill('Shared, edited there');
  await expect(other.getByText('This project was changed in another window.')).toBeVisible();
  await other.getByRole('button', { name: 'Keep mine' }).click();
  await saved(other);

  await page.reload();
  await expect(page.getByLabel('Decision name')).toHaveValue('Shared, edited there');
});

test('leaving right after an edit still saves it', async ({ page }) => {
  await createDecision(page, 'Quick exit');
  await page.getByLabel('Decision name').fill('Quick exit, renamed');
  await page.getByRole('link', { name: 'All decisions' }).click(); // well inside the 600 ms autosave delay
  await expect(page.getByText('Quick exit, renamed')).toBeVisible();
  await page.reload();
  await expect(page.getByText('Quick exit, renamed')).toBeVisible();
});

test('removing the person you are judging as switches to someone else', async ({ page }) => {
  await createDecision(page, 'Two judges');
  await openPeople(page);
  await addAll(page, 'New person', ['Priya']);
  await page.getByLabel('Judging as').selectOption({ label: 'Priya' });
  await page.getByRole('button', { name: 'Remove Priya' }).click();
  await saved(page);
  await expect(page.getByLabel('Judging as')).toHaveCount(0); // only one person left
  await page.getByRole('link', { name: /Set up/ }).click();
  await addAll(page, 'New option', ['X', 'Y']);
  await addAll(page, 'New criterion', ['Speed', 'Cost']);
  await page.getByRole('link', { name: /Weigh criteria/ }).click();
  await page.getByRole('slider').press('ArrowLeft');
  await saved(page);
  await expect(page.getByText('1 of 1 pairs compared')).toBeVisible();
});
