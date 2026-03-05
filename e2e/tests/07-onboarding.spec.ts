import { test, expect } from '@playwright/test';

test.describe('Onboarding Tour', () => {
  test.beforeEach(async ({ page }) => {
    // Reset onboarding state before each test
    await page.request.delete('/api/me/onboarding');
  });

  test('tour auto-triggers on first Dashboard visit', async ({ page }) => {
    await page.goto('/');
    // Joyride renders a tooltip with role="alertdialog"
    const tooltip = page.locator('[role="alertdialog"], .__floater');
    await expect(tooltip.first()).toBeVisible({ timeout: 10000 });
    // Should see welcome text
    await expect(page.getByText(/welcome to/i)).toBeVisible();
  });

  test('tour can be dismissed via Skip', async ({ page }) => {
    await page.goto('/');
    const tooltip = page.locator('[role="alertdialog"], .__floater');
    await expect(tooltip.first()).toBeVisible({ timeout: 10000 });
    // Click skip button
    await page.getByRole('button', { name: /skip/i }).click();
    // Tooltip should disappear
    await expect(tooltip).not.toBeVisible({ timeout: 5000 });
  });

  test('tour does NOT re-trigger on second visit', async ({ page }) => {
    // First visit — complete or skip the tour
    await page.goto('/');
    const tooltip = page.locator('[role="alertdialog"], .__floater');
    await expect(tooltip.first()).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /skip/i }).click();
    await expect(tooltip).not.toBeVisible({ timeout: 5000 });

    // Navigate away and come back
    await page.goto('/catalog');
    // Skip catalog tour too
    const catTooltip = page.locator('[role="alertdialog"], .__floater');
    await expect(catTooltip.first()).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /skip/i }).click();

    // Go back to dashboard
    await page.goto('/');
    // Tour should NOT appear
    await page.waitForTimeout(2000);
    await expect(page.locator('[role="alertdialog"], .__floater')).not.toBeVisible();
  });

  test('"?" button replays the tour', async ({ page }) => {
    // First visit — skip tour
    await page.goto('/');
    const tooltip = page.locator('[role="alertdialog"], .__floater');
    await expect(tooltip.first()).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: /skip/i }).click();
    await expect(tooltip).not.toBeVisible({ timeout: 5000 });

    // Click the "?" button
    await page.locator('[data-tour="replay-tour"]').click();
    // Tour should appear again
    await expect(tooltip.first()).toBeVisible({ timeout: 10000 });
  });

  test('Catalog tour triggers on first visit', async ({ page }) => {
    // Mark dashboard as seen so it doesn't interfere
    await page.request.patch('/api/me/onboarding', { data: { tour: 'dashboard' } });

    await page.goto('/catalog');
    const tooltip = page.locator('[role="alertdialog"], .__floater');
    await expect(tooltip.first()).toBeVisible({ timeout: 10000 });
    // Should see catalog-specific text
    await expect(page.getByText(/catalog lists/i)).toBeVisible();
  });
});
