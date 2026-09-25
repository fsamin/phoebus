import { test, expect } from '@playwright/test';

// KPI values are covered by the Go tests; these tests cover the front-end
// wiring: route and entry point, server-side search / filter / sort driven by
// the URL, and the learner detail KPIs.
test.describe('Learners analytics', () => {
  const tag = Date.now().toString(36);
  const learners = ['a', 'b'].map((s) => ({ username: `kpi-${tag}-${s}`, display_name: `KPI Learner ${s.toUpperCase()} ${tag}` }));

  test.beforeAll(async ({ request }) => {
    for (const l of learners) {
      const res = await request.post('/api/admin/users', {
        data: { ...l, role: 'learner', password: 'Test1234!' },
      });
      expect(res.status()).toBe(201);
    }
  });

  test('instructor finds learners, filters them and gets back to the same list', async ({ page }) => {
    await page.goto('/analytics');
    await page.getByRole('button', { name: /learners/i }).click();
    await expect(page).toHaveURL(/\/analytics\/learners$/);

    await page.getByPlaceholder('Search learners...').fill(tag);
    await expect(page).toHaveURL(new RegExp(`q=${tag}`));
    await expect(page.locator('.ant-table-row')).toHaveCount(2, { timeout: 10000 });
    await expect(page.getByText('2 learners')).toBeVisible();

    // Freshly created learners have no progress: they are "not started", never "stuck".
    const filters = page.locator('.ant-segmented');
    await filters.getByText('Stuck').click();
    await expect(page).toHaveURL(/status=stuck/);
    await expect(page.locator('.ant-table-row')).toHaveCount(0);
    await filters.getByText('Not started').click();
    await expect(page).toHaveURL(/status=not_started/);
    await expect(page.locator('.ant-table-row')).toHaveCount(2);
    await expect(page.locator('.ant-table-row .ant-tag', { hasText: 'Not started' })).toHaveCount(2);

    // Sorting is server-side and reflected in the URL.
    await page.getByRole('columnheader', { name: /last activity/i }).click();
    await expect(page).toHaveURL(/sort=last_activity/);

    await page.getByText(learners[0].display_name).click();
    await expect(page).toHaveURL(/\/analytics\/learners\/[0-9a-f-]{36}$/);
    await expect(page.getByRole('heading', { level: 2 })).toContainText(learners[0].display_name);
    for (const kpi of ['Paths completed', 'Progress', 'First-try success', 'Avg attempts / exercise', 'Active days (30d)', 'Current streak']) {
      await expect(page.getByText(kpi, { exact: true })).toBeVisible();
    }

    // Coming back restores the list as it was left.
    await page.goBack();
    await expect(page).toHaveURL(/status=not_started/);
    await expect(page).toHaveURL(new RegExp(`q=${tag}`));
    await expect(page.getByPlaceholder('Search learners...')).toHaveValue(tag);
    await expect(page.locator('.ant-table-row')).toHaveCount(2);
  });

  test('learner detail links back to the learners list', async ({ page }) => {
    await page.goto(`/analytics/learners?q=${tag}`);
    await page.getByText(learners[1].display_name).click();
    await page.locator('.ant-breadcrumb').getByRole('link', { name: 'Learners' }).click();
    await expect(page).toHaveURL(/\/analytics\/learners$/);
    await expect(page.locator('.ant-table-row').first()).toBeVisible({ timeout: 10000 });
  });
});
