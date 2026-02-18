import { test, expect } from '@playwright/test';

test.describe('RBAC — Menu visibility', () => {
  test('admin sees Admin and Analytics menus', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(2000);
    // Admin should see Admin menu item
    await expect(page.getByRole('menuitem', { name: /admin/i })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: /analytics/i })).toBeVisible();
  });

  test('learner does not see Admin menu', async ({ page, request }) => {
    // Register a fresh learner
    const username = `rbacl${Date.now()}`;
    const regRes = await request.post('/api/auth/register', {
      data: { username, password: 'Test1234!', display_name: 'RBAC Learner' },
    });
    expect(regRes.status()).toBe(201);

    // Login as learner via UI
    await page.context().clearCookies();
    await page.goto('/login');
    await page.getByLabel(/username/i).fill(username);
    await page.getByLabel(/password/i).fill('Test1234!');
    await page.getByRole('button', { name: /login|sign in/i }).click();
    await expect(page).toHaveURL(/\/(dashboard)?$/, { timeout: 10000 });

    // Check that admin menu items are NOT visible
    await page.waitForTimeout(2000);
    const adminItem = page.getByRole('menuitem', { name: /admin/i });
    await expect(adminItem).not.toBeVisible();
  });

  test('learner cannot access admin API', async ({ page, request }) => {
    // Register a fresh learner
    const username = `rbacl2${Date.now()}`;
    await request.post('/api/auth/register', {
      data: { username, password: 'Test1234!', display_name: 'RBAC Learner 2' },
    });

    // Login as learner via UI to get cookie
    await page.context().clearCookies();
    await page.goto('/login');
    await page.getByLabel(/username/i).fill(username);
    await page.getByLabel(/password/i).fill('Test1234!');
    await page.getByRole('button', { name: /login|sign in/i }).click();
    await expect(page).toHaveURL(/\/(dashboard)?$/, { timeout: 10000 });

    // Try to access admin repos via API using page context (which has learner cookie)
    const response = await page.request.get('/api/admin/repos');
    expect(response.status()).toBe(403);
  });
});
