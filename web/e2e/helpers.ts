import { expect, type Page } from '@playwright/test';

/** Creates a decision from the library and returns once its setup screen is open. */
export async function createDecision(page: Page, name: string) {
  await page.goto('/');
  await page.getByRole('button', { name: 'New decision' }).first().click();
  await page.getByLabel('What are you deciding?').fill(name);
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page.getByLabel('Decision name')).toHaveValue(name);
}

export async function addAll(page: Page, field: string, names: string[]) {
  for (const n of names) {
    await page.getByLabel(field).fill(n);
    await page.getByLabel(field).press('Enter');
  }
}

export const saved = (page: Page) => expect(page.getByText('All changes saved')).toBeVisible();
