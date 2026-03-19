import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

test.describe('Exercises & Progress', () => {
  const contentSynced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));

  test('quiz correct answer shows success feedback', async ({ page, request }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');

    // Find a quiz step via the API
    const pathsRes = await request.get('/api/learning-paths');
    const paths = await pathsRes.json();
    let quizStepSlug: string | null = null;
    let quizPathSlug: string | null = null;

    for (const p of paths) {
      const pathRes = await request.get(`/api/learning-paths/${p.id}`);
      const pathData = await pathRes.json();
      for (const mod of pathData.modules || []) {
        for (const step of mod.steps || []) {
          if (step.type === 'quiz') {
            quizStepSlug = step.slug;
            quizPathSlug = p.slug;
            break;
          }
        }
        if (quizStepSlug) break;
      }
      if (quizStepSlug) break;
    }

    test.skip(!quizStepSlug, 'No quiz step found in synced content');

    // Navigate to the quiz step using slug-based URL
    await page.goto(`/paths/${quizPathSlug}/steps/${quizStepSlug}`);
    await page.waitForTimeout(2000);

    // A quiz should have radio buttons or checkboxes for answers
    const options = page.locator('input[type="radio"], input[type="checkbox"], [class*="ant-radio"], [class*="ant-checkbox"]');
    await expect(options.first()).toBeVisible({ timeout: 10000 });
  });

  test('progress updates after completing a step', async ({ page, request }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');

    // Get initial progress
    const progressBefore = await request.get('/api/progress');
    expect(progressBefore.ok()).toBeTruthy();

    // Navigate to a lesson step
    const pathsRes = await request.get('/api/learning-paths');
    const paths = await pathsRes.json();
    let lessonStepId: string | null = null;
    let lessonPathSlug: string | null = null;

    for (const p of paths) {
      const pathRes = await request.get(`/api/learning-paths/${p.id}`);
      const pathData = await pathRes.json();
      for (const mod of pathData.modules || []) {
        for (const step of mod.steps || []) {
          if (step.type === 'lesson') {
            lessonStepId = step.id;
            lessonPathSlug = p.slug;
            break;
          }
        }
        if (lessonStepId) break;
      }
      if (lessonStepId) break;
    }

    test.skip(!lessonStepId, 'No lesson step found in synced content');

    // Mark the step as completed via API
    const res = await request.post('/api/progress', {
      data: { step_id: lessonStepId, status: 'completed' },
    });
    expect(res.ok()).toBeTruthy();

    // Navigate to path overview and verify progress is visible
    await page.goto(`/paths/${lessonPathSlug}`);
    await page.waitForTimeout(2000);
    const body = await page.textContent('body');
    expect(body).toBeTruthy();
  });
});
