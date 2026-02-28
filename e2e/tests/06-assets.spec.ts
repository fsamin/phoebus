import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

const BASE_URL = process.env.BASE_URL || 'http://localhost:9080';

test.describe('Assets', () => {
  test.beforeEach(async () => {
    const synced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));
    test.skip(!synced, 'Content not synced — skipping asset tests');
  });

  test('asset endpoint returns image with correct headers', async ({ page }) => {
    // Use page.request which inherits the authenticated storageState
    const request = page.context().request;

    // Get catalog to find a learning path with assets
    const catalogRes = await request.get(`${BASE_URL}/api/catalog`);
    expect(catalogRes.ok()).toBeTruthy();
    const catalog = await catalogRes.json();

    // Find the containerization path (has assets in docker-fundamentals)
    const containerPath = catalog.find((lp: { slug: string }) =>
      lp.slug.includes('containerization')
    );
    test.skip(!containerPath, 'Containerization learning path not found');

    // Get modules for this learning path
    const pathRes = await request.get(`${BASE_URL}/api/catalog/${containerPath.id}`);
    expect(pathRes.ok()).toBeTruthy();
    const pathData = await pathRes.json();

    // Find the first module with steps
    const firstModule = pathData.modules?.[0];
    test.skip(!firstModule, 'No modules found');

    // Get steps for this module
    const moduleRes = await request.get(
      `${BASE_URL}/api/catalog/${containerPath.id}/modules/${firstModule.id}`
    );
    expect(moduleRes.ok()).toBeTruthy();
    const moduleData = await moduleRes.json();

    // Look for asset references in step content (pattern: /api/assets/{hash})
    const assetPattern = /\/api\/assets\/([a-f0-9]{64})/;
    let assetHash: string | null = null;

    for (const step of moduleData.steps || []) {
      if (step.content_md) {
        const match = step.content_md.match(assetPattern);
        if (match) {
          assetHash = match[1];
          break;
        }
      }
    }

    test.skip(!assetHash, 'No asset references found in step content');

    // Fetch the asset (public endpoint, no auth needed)
    const assetRes = await request.get(`${BASE_URL}/api/assets/${assetHash}`);
    expect(assetRes.status()).toBe(200);

    // Verify headers
    const headers = assetRes.headers();
    expect(headers['content-type']).toBeTruthy();
    expect(headers['cache-control']).toContain('immutable');
    expect(headers['x-content-type-options']).toBe('nosniff');

    // Verify body is not empty
    const body = await assetRes.body();
    expect(body.length).toBeGreaterThan(0);
  });

  test('invalid asset hash returns 400', async ({ request }) => {
    const res = await request.get(`${BASE_URL}/api/assets/invalid-hash`, {
      failOnStatusCode: false,
    });
    expect(res.status()).toBe(400);
  });

  test('non-existent asset returns 404', async ({ request }) => {
    const hash = '0000000000000000000000000000000000000000000000000000000000000000';
    const res = await request.get(`${BASE_URL}/api/assets/${hash}`, {
      failOnStatusCode: false,
    });
    expect(res.status()).toBe(404);
  });
});
