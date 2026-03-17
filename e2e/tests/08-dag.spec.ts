import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

test.describe('Catalog DAG View', () => {
  const contentSynced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));

  test('toggle between grid and DAG view', async ({ page }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');
    await page.goto('/catalog');
    await expect(page.getByText(/catalog/i).first()).toBeVisible();

    // Should start in grid view (cards visible)
    const cards = page.locator('[class*="ant-card"]');
    await expect(cards.first()).toBeVisible({ timeout: 10000 });

    // Click DAG toggle (ApartmentOutlined icon button)
    const dagToggle = page.locator('[class*="ant-segmented"] .anticon-apartment').first();
    await dagToggle.click();

    // DAG container should appear (reactflow wrapper)
    const dagContainer = page.locator('.react-flow');
    await expect(dagContainer).toBeVisible({ timeout: 10000 });

    // Cards should no longer be visible
    await expect(cards.first()).not.toBeVisible();

    // Switch back to grid view
    const gridToggle = page.locator('[class*="ant-segmented"] .anticon-appstore').first();
    await gridToggle.click();
    await expect(cards.first()).toBeVisible({ timeout: 10000 });
  });

  test('DAG shows learning path nodes', async ({ page }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');
    await page.goto('/catalog');
    await expect(page.getByText(/catalog/i).first()).toBeVisible();

    // Switch to DAG view
    const dagToggle = page.locator('[class*="ant-segmented"] .anticon-apartment').first();
    await dagToggle.click();

    // Should see reactflow nodes
    const nodes = page.locator('.react-flow__node');
    await expect(nodes.first()).toBeVisible({ timeout: 10000 });
    const count = await nodes.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test('clicking a DAG node shows popover with details', async ({ page }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');
    await page.goto('/catalog');

    // Switch to DAG view
    const dagToggle = page.locator('[class*="ant-segmented"] .anticon-apartment').first();
    await dagToggle.click();

    // Click the first node
    const firstNode = page.locator('.react-flow__node').first();
    await expect(firstNode).toBeVisible({ timeout: 10000 });
    await firstNode.click();

    // Popover should appear with "View Path" button
    const viewPathButton = page.getByRole('button', { name: /view path/i });
    await expect(viewPathButton).toBeVisible({ timeout: 5000 });
  });

  test('DAG view persists after page reload', async ({ page }) => {
    test.skip(!contentSynced, 'Content not synced — skipping');
    await page.goto('/catalog');

    // Switch to DAG view
    const dagToggle = page.locator('[class*="ant-segmented"] .anticon-apartment').first();
    await dagToggle.click();
    await expect(page.locator('.react-flow')).toBeVisible({ timeout: 10000 });

    // Reload page
    await page.reload();

    // DAG should still be shown (persisted via localStorage)
    await expect(page.locator('.react-flow')).toBeVisible({ timeout: 10000 });
  });
});

test.describe('Admin Dependencies', () => {
  test('admin can access dependencies page', async ({ page }) => {
    await page.goto('/admin/dependencies');
    await expect(page.getByText(/path dependencies/i)).toBeVisible({ timeout: 10000 });
    // Should show the create form
    await expect(page.getByText(/add dependency/i)).toBeVisible();
  });
});
