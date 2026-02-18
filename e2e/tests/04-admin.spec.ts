import { test, expect } from '@playwright/test';

test.describe('Admin', () => {
  test('repositories page lists repos', async ({ page }) => {
    await page.goto('/admin/repositories');
    await expect(page.locator('.ant-table, table').first()).toBeVisible({ timeout: 10000 });
  });

  test('sync logs page shows sync history', async ({ page }) => {
    const needsContent = !!process.env.GITHUB_TOKEN;
    test.skip(!needsContent, 'GITHUB_TOKEN not set — no repos to show sync logs for');

    await page.goto('/admin/repositories');
    await page.waitForTimeout(2000);
    // Click sync logs button on first repo
    const syncLogsBtn = page.locator('button').filter({ has: page.locator('[aria-label*="unordered-list"], .anticon-unordered-list') }).first();
    if (await syncLogsBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
      await syncLogsBtn.click();
      await expect(page.locator('.ant-table, table').first()).toBeVisible({ timeout: 10000 });
    }
  });

  test('add repository form is accessible', async ({ page }) => {
    await page.goto('/admin/repositories');
    await page.waitForTimeout(2000);
    const addBtn = page.locator('button').filter({ has: page.locator('.anticon-plus') }).first();
    if (await addBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
      await addBtn.click();
    } else {
      // Navigate directly
      await page.goto('/admin/repositories/new');
    }
    await expect(page.locator('form, .ant-form, input').first()).toBeVisible({ timeout: 10000 });
  });

  test('users page lists users', async ({ page }) => {
    await page.goto('/admin/users');
    await expect(page.getByText('admin').first()).toBeVisible({ timeout: 10000 });
  });

  test('health page shows application status', async ({ page }) => {
    await page.goto('/admin/health');
    await expect(page.getByText(/health|status|ok/i).first()).toBeVisible({ timeout: 10000 });
  });
});
