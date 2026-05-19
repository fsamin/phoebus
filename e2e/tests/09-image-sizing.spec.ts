import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

test.describe('Image Sizing in Markdown', () => {
  const contentSynced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));

  test.beforeEach(async ({ page }) => {
    // Ensure onboarding tours don't block interactions
    await page.request.patch('/api/me/onboarding', { data: { tour: 'dashboard' } });
    await page.request.patch('/api/me/onboarding', { data: { tour: 'catalog' } });
  });

  test('images with |WIDTH syntax are rendered with correct dimensions', async ({ page, request }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');

    // Use the API to find the containerization path and its first step (like 06-assets does)
    const pathsRes = await request.get('/api/learning-paths');
    expect(pathsRes.ok()).toBeTruthy();
    const paths = await pathsRes.json();

    const containerPath = paths.find((lp: { title: string }) =>
      lp.title?.toLowerCase().includes('container')
    );
    test.skip(!containerPath, 'Containerization learning path not found');

    const pathRes = await request.get(`/api/learning-paths/${containerPath.id}`);
    expect(pathRes.ok()).toBeTruthy();
    const pathData = await pathRes.json();

    // Find the first step with image sizing syntax in its content
    const modules = pathData.modules || [];
    let targetStepSlug: string | null = null;
    for (const mod of modules) {
      for (const step of mod.steps || []) {
        if (step.id) {
          const stepRes = await request.get(
            `/api/learning-paths/${containerPath.id}/steps/${step.id}`
          );
          if (stepRes.ok()) {
            const stepData = await stepRes.json();
            if (stepData.content_md && stepData.content_md.includes('|')) {
              // Check for image sizing pattern: ![...|SIZE](...)
              if (/!\[[^\]]*\|[^\]]*\]\([^)]+\)/.test(stepData.content_md)) {
                targetStepSlug = step.slug || step.id;
                break;
              }
            }
          }
        }
      }
      if (targetStepSlug) break;
    }
    test.skip(!targetStepSlug, 'No step with image sizing syntax found');

    // Navigate directly to the step page
    await page.goto(`/paths/${containerPath.slug}/steps/${targetStepSlug}`);
    await page.waitForLoadState('networkidle');

    const markdownBody = page.locator('.markdown-body');
    await expect(markdownBody).toBeVisible({ timeout: 15000 });

    // Check that images are rendered with sizing styles
    const images = markdownBody.locator('img');
    const count = await images.count();
    expect(count).toBeGreaterThanOrEqual(2);

    // First image: containers-vs-vms.svg with |80% → width: 80%
    const svgImg = images.nth(0);
    const svgStyle = await svgImg.getAttribute('style');
    expect(svgStyle).toContain('80%');
    const svgAlt = await svgImg.getAttribute('alt');
    expect(svgAlt).toBe('Containers vs VMs Architecture');

    // Second image: docker-logo.png with |200 → width: 200px
    const logoImg = images.nth(1);
    const logoStyle = await logoImg.getAttribute('style');
    expect(logoStyle).toContain('200px');
    const logoAlt = await logoImg.getAttribute('alt');
    expect(logoAlt).toBe('Docker Logo');
  });

  test('images without size syntax render normally', async ({ page, request }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');

    // Find any learning path with at least one step
    const pathsRes = await request.get('/api/learning-paths');
    expect(pathsRes.ok()).toBeTruthy();
    const paths = await pathsRes.json();
    const anyPath = paths[0];
    test.skip(!anyPath, 'No learning paths found');

    const pathRes = await request.get(`/api/learning-paths/${anyPath.id}`);
    expect(pathRes.ok()).toBeTruthy();
    const pathData = await pathRes.json();

    const modules = pathData.modules || [];
    let stepSlug: string | null = null;
    for (const mod of modules) {
      const steps = mod.steps || [];
      if (steps.length > 0) {
        stepSlug = steps[0].slug || steps[0].id;
        break;
      }
    }
    test.skip(!stepSlug, 'No step found');

    // Intercept step API to inject content without sizing
    await page.route('**/api/learning-paths/*/steps/*', async (route) => {
      const response = await route.fetch();
      const body = await response.json();
      body.content_md = '# Test\n\n![Normal image](https://via.placeholder.com/100x100.png)\n';
      await route.fulfill({ response, json: body });
    });

    await page.goto(`/paths/${anyPath.slug}/steps/${stepSlug}`);
    await page.waitForLoadState('networkidle');

    const markdownBody = page.locator('.markdown-body');
    await expect(markdownBody).toBeVisible({ timeout: 15000 });

    const img = markdownBody.locator('img').first();
    await expect(img).toBeVisible({ timeout: 5000 });
    const style = await img.getAttribute('style');
    expect(style).toContain('max-width');
    const alt = await img.getAttribute('alt');
    expect(alt).toBe('Normal image');
  });
});
