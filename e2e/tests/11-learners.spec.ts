import { test, expect, request as apiRequest } from '@playwright/test';
import fs from 'fs';
import path from 'path';

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

  // "x/y steps" must count against the whole path, like the progress bar next
  // to it: a learner who finished one step out of five is not at 1/1.
  test('enrolled path shows completed steps out of the whole path', async ({ page, request, baseURL }) => {
    const contentSynced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));
    test.skip(!contentSynced, 'Content not synced — skipping');

    let target: { id: string; title: string; steps: number; lessonId: string } | null = null;
    for (const p of await (await request.get('/api/learning-paths')).json()) {
      const detail = await (await request.get(`/api/learning-paths/${p.id}`)).json();
      const steps = (detail.modules || []).flatMap((m: { steps?: Array<{ id: string; type: string }> }) => m.steps || []);
      const lesson = steps.find((st: { type: string }) => st.type === 'lesson');
      if (steps.length >= 2 && lesson) {
        target = { id: p.id, title: p.title, steps: steps.length, lessonId: lesson.id };
        break;
      }
    }
    test.skip(!target, 'No path with a lesson and at least 2 steps in synced content');

    const username = `kpi-${tag}-steps`;
    const created = await request.post('/api/admin/users', {
      data: { username, display_name: username, role: 'learner', password: 'Test1234!' },
    });
    expect(created.status()).toBe(201);
    const learnerId = (await created.json()).id;

    const learner = await apiRequest.newContext({ baseURL });
    expect((await learner.post('/api/auth/login', { data: { username, password: 'Test1234!' } })).ok()).toBeTruthy();
    expect((await learner.post('/api/progress', { data: { step_id: target!.lessonId, status: 'completed' } })).ok()).toBeTruthy();
    await learner.dispose();

    await page.goto(`/analytics/learners/${learnerId}`);
    const row = page.locator('div', { has: page.getByRole('link', { name: target!.title }) }).last();
    await expect(row).toContainText(`1/${target!.steps} steps`, { timeout: 10000 });
  });

  test('learner detail links back to the learners list', async ({ page }) => {
    await page.goto(`/analytics/learners?q=${tag}`);
    await page.getByText(learners[1].display_name).click();
    await page.locator('.ant-breadcrumb').getByRole('link', { name: 'Learners' }).click();
    await expect(page).toHaveURL(/\/analytics\/learners$/);
    await expect(page.locator('.ant-table-row').first()).toBeVisible({ timeout: 10000 });
  });
});
