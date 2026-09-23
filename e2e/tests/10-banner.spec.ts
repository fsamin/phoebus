import { test, expect, APIRequestContext } from '@playwright/test';

// The banner is instance-wide state and Playwright runs with workers: 1,
// fullyParallel: false — a banner left enabled would still be there when
// 07-onboarding drives the dashboard.
async function disableBanner(request: APIRequestContext) {
  await request.put('/api/admin/banner', {
    data: { enabled: false, level: 'info', message_md: '' },
  });
}

test.describe('Dashboard banner', () => {
  // Start from a known state: the first test toggles the switch, which would
  // turn the banner *off* if a previous run left it on.
  test.beforeEach(async ({ request }) => {
    await disableBanner(request);
  });

  test.afterEach(async ({ request }) => {
    await disableBanner(request);
  });

  // Covers what the Go tests cannot: that the three front-end wiring points
  // (route, menu entry, dashboard render) actually line up.
  test('admin publishes a banner and it shows on the dashboard', async ({ page }) => {
    await page.goto('/admin/banner');

    const textarea = page.locator('textarea');
    await expect(textarea).toBeVisible({ timeout: 10000 });
    await textarea.fill('**Scheduled maintenance** on Sunday');
    await page.locator('.ant-switch').first().click();

    // Wait for the save to land before navigating, otherwise the dashboard may
    // be fetched before the banner is stored.
    const saved = page.waitForResponse(
      (r) => r.url().includes('/api/admin/banner') && r.request().method() === 'PUT',
    );
    await page.locator('button[type="submit"]').click();
    expect((await saved).status()).toBe(200);

    await page.goto('/');
    const alert = page.locator('.ant-alert').first();
    await expect(alert).toBeVisible({ timeout: 10000 });
    await expect(alert).toContainText('Scheduled maintenance');
    // Markdown must be rendered, not printed as source
    await expect(alert.locator('strong')).toHaveText('Scheduled maintenance');
  });

  test('danger level renders as an error alert', async ({ page, request }) => {
    await request.put('/api/admin/banner', {
      data: { enabled: true, level: 'danger', message_md: 'Service degraded' },
    });

    await page.goto('/');
    const alert = page.locator('.ant-alert-error').first();
    await expect(alert).toBeVisible({ timeout: 10000 });
    await expect(alert).toContainText('Service degraded');
  });

  test('a disabled banner is not displayed', async ({ page, request }) => {
    await request.put('/api/admin/banner', {
      data: { enabled: true, level: 'info', message_md: 'Temporary notice' },
    });
    await disableBanner(request);

    await page.goto('/');
    await expect(page.locator('h2')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Temporary notice')).toHaveCount(0);
  });
});
