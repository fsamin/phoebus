import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

test.describe('Image Sizing in Markdown', () => {
  const contentSynced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));

  test('images with |WIDTH syntax are rendered with correct dimensions', async ({ page }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');

    // Navigate to the containerization learning path which has sized images
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    // Find the containerization learning path
    const containerCard = page.locator('[class*="ant-card"]').filter({ hasText: /container/i }).first();
    await expect(containerCard).toBeVisible({ timeout: 10000 });
    await containerCard.click();

    // Click on the first step (Containers & Images) which has sized images
    const stepItem = page.locator('.ant-list-item').first();
    await expect(stepItem).toBeVisible({ timeout: 10000 });
    await stepItem.click();

    await page.waitForTimeout(2000);

    const markdownBody = page.locator('.markdown-body');
    await expect(markdownBody).toBeVisible({ timeout: 10000 });

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

  test('images without size syntax render normally', async ({ page }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');

    // Use route interception to inject content without sizing
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');

    const firstCard = page.locator('[class*="ant-card"]').first();
    await expect(firstCard).toBeVisible({ timeout: 10000 });
    await firstCard.click();

    const stepItem = page.locator('.ant-list-item').first();
    await expect(stepItem).toBeVisible({ timeout: 10000 });

    // Intercept step API to inject content without sizing
    await page.route('**/api/learning-paths/*/steps/*', async (route) => {
      const response = await route.fetch();
      const body = await response.json();
      body.content_md = '# Test\n\n![Normal image](https://via.placeholder.com/100x100.png)\n';
      await route.fulfill({ response, json: body });
    });

    await stepItem.click();
    await page.waitForTimeout(2000);

    const markdownBody = page.locator('.markdown-body');
    await expect(markdownBody).toBeVisible({ timeout: 10000 });

    const img = markdownBody.locator('img').first();
    await expect(img).toBeVisible({ timeout: 5000 });
    const style = await img.getAttribute('style');
    expect(style).toContain('max-width');
    const alt = await img.getAttribute('alt');
    expect(alt).toBe('Normal image');
  });
});
