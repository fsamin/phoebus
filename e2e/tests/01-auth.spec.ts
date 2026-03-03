import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
  test.use({ storageState: { cookies: [], origins: [] } }); // Clear auth for this suite

  test('login with valid admin credentials', async ({ page }) => {
    await page.goto('/login');
    await page.locator('#username, input[id*="username"]').first().fill('admin');
    await page.locator('#password, input[id*="password"]').first().fill('admin');
    await page.getByRole('button', { name: /login|sign in/i }).click();
    await expect(page).toHaveURL(/\/(dashboard)?$/, { timeout: 10000 });
  });

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto('/login');
    await page.locator('#username, input[id*="username"]').first().fill('admin');
    await page.locator('#password, input[id*="password"]').first().fill('wrongpassword');
    await page.getByRole('button', { name: /login|sign in/i }).click();
    await expect(page.locator('.ant-alert, [class*="error"], [class*="Error"]').first()).toBeVisible({ timeout: 10000 });
  });

  test('register a new learner', async ({ page }) => {
    const username = `learner${Date.now()}`;
    await page.goto('/login');
    // Click "Create one" link to show register form
    await page.getByText(/create one/i).click();
    await page.waitForTimeout(500);
    // Now fill the register form (new fields appear)
    await page.getByLabel(/username/i).fill(username);
    await page.getByLabel('Display Name').fill('Test Learner');
    await page.getByLabel('Password', { exact: true }).fill('Test1234!');
    await page.getByLabel('Confirm Password').fill('Test1234!');
    await page.getByRole('button', { name: /register|sign up|create/i }).click();
    await expect(page).toHaveURL(/\/(dashboard)?$/, { timeout: 10000 });
  });

  test('logout redirects to login', async ({ page }) => {
    // First login
    await page.goto('/login');
    await page.locator('#username, input[id*="username"]').first().fill('admin');
    await page.locator('#password, input[id*="password"]').first().fill('admin');
    await page.getByRole('button', { name: /login|sign in/i }).click();
    await expect(page).toHaveURL(/\/(dashboard)?$/, { timeout: 10000 });

    // Then logout — look for logout button or user dropdown
    const logoutBtn = page.getByText(/logout|sign out/i);
    if (await logoutBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await logoutBtn.click();
    } else {
      // Try clicking user avatar/dropdown first
      await page.locator('.ant-dropdown-trigger, [class*="avatar"], [class*="Avatar"]').first().click();
      await page.getByText(/logout|sign out/i).click();
    }
    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });

  test('protected route redirects to login when not authenticated', async ({ page }) => {
    await page.goto('/admin/repositories');
    await expect(page).toHaveURL(/\/login/);
  });
});
