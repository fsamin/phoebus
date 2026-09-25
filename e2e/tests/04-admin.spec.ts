import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

test.describe('Admin', () => {
  test('repositories page lists repos', async ({ page }) => {
    await page.goto('/admin/repositories');
    await expect(page.locator('.ant-table, table').first()).toBeVisible({ timeout: 10000 });
  });

  test('sync logs page shows sync history', async ({ page }) => {
    const contentSynced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));
    test.skip(!contentSynced, 'Content not synced — skipping');

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

  // Search runs server-side: a user created before 25 others sits beyond the
  // first page, and must still be found, with a total describing the matches.
  test('users search finds users beyond the current page', async ({ page, request }) => {
    const tag = Date.now().toString(36);
    const create = (username: string) =>
      request.post('/api/admin/users', {
        data: { username, display_name: `Display ${username}`, role: 'learner', password: 'Test1234!' },
      });
    expect((await create(`needle-${tag}`)).status()).toBe(201);
    for (let i = 0; i < 25; i++) {
      expect((await create(`filler-${tag}-${i}`)).status()).toBe(201);
    }

    await page.goto('/admin/users');
    await expect(page.locator('.ant-table-row').first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('cell', { name: `needle-${tag}`, exact: true })).toHaveCount(0);
    // Searching from page 2 must bring the results back to page 1.
    await page.locator('.ant-pagination-item-2').click();
    await expect(page.locator('.ant-pagination-item-active')).toHaveText('2');

    await page.getByPlaceholder('Search users...').fill(`needle-${tag}`);
    await expect(page.getByRole('cell', { name: `needle-${tag}`, exact: true })).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.ant-table-row')).toHaveCount(1);
    await expect(page.getByText('1 users')).toBeVisible();
    await expect(page.locator('.ant-pagination-item-active')).toHaveText('1');
  });

  // Sorting by completed paths must be asked to the server, so that it ranks
  // every user rather than the 20 displayed, and restart from the first page.
  test('users sort by completed paths is server-side', async ({ page }) => {
    await page.goto('/admin/users');
    await expect(page.locator('.ant-table-row').first()).toBeVisible({ timeout: 10000 });
    await page.locator('.ant-pagination-item-2').click();
    await expect(page.locator('.ant-pagination-item-active')).toHaveText('2');

    const sorted = page.waitForRequest((r) => /\/api\/admin\/users\?.*sort=completed_paths&order=asc/.test(r.url()));
    await page.getByRole('columnheader', { name: /completed paths/i }).click();
    expect(new URL((await sorted).url()).searchParams.get('page')).toBe('1');
    await expect(page.locator('.ant-pagination-item-active')).toHaveText('1');

    const desc = page.waitForRequest((r) => r.url().includes('sort=completed_paths&order=desc'));
    await page.getByRole('columnheader', { name: /completed paths/i }).click();
    await desc;
  });

  test('health page shows application status', async ({ page }) => {
    await page.goto('/admin/health');
    await expect(page.getByText(/health|status|ok/i).first()).toBeVisible({ timeout: 10000 });
  });
});
