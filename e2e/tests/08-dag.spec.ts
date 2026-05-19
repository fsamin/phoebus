import { test, expect } from '@playwright/test';

test.describe('Admin Dependencies', () => {
  test('admin can access dependencies page', async ({ page }) => {
    await page.goto('/admin/dependencies');
    await expect(page.getByText(/path dependencies/i)).toBeVisible({ timeout: 10000 });
    // Should show the create form
    await expect(page.getByText(/add dependency/i)).toBeVisible();
  });
});
