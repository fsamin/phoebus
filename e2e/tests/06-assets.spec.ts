import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

const BASE_URL = process.env.BASE_URL || 'http://localhost:9080';

test.describe('Assets', () => {
  test.beforeEach(async () => {
    const synced = fs.existsSync(path.join(__dirname, '..', 'storage-state', 'content-synced'));
    test.skip(!synced, 'Content not synced — skipping asset tests');
  });

  test('asset endpoint returns image with correct headers', async ({ page, context }) => {
    // Navigate to trigger cookie loading from storageState
    await page.goto('/');

    // Use the context's request which carries the auth cookies
    const request = context.request;

    // Get learning paths
    const pathsRes = await request.get(`${BASE_URL}/api/learning-paths`);
    expect(pathsRes.ok()).toBeTruthy();
    const paths = await pathsRes.json();

    // Find the containerization path (has assets in docker-fundamentals)
    const containerPath = paths.find((lp: { slug: string }) =>
      lp.slug.includes('containerization')
    );
    test.skip(!containerPath, 'Containerization learning path not found');

    // Get learning path detail with modules and steps
    const pathRes = await request.get(`${BASE_URL}/api/learning-paths/${containerPath.id}`);
    expect(pathRes.ok()).toBeTruthy();
    const pathData = await pathRes.json();

    // Find steps across all modules
    const assetPattern = /\/api\/assets\/([a-f0-9]{64})/;
    let assetHash: string | null = null;

    const modules = pathData.modules || [];
    for (const mod of modules) {
      const steps = mod.steps || [];
      for (const step of steps) {
        // Try to get step content if not inline
        if (step.content_md) {
          const match = step.content_md.match(assetPattern);
          if (match) {
            assetHash = match[1];
            break;
          }
        }
        // Try fetching step detail
        if (!assetHash && step.id) {
          const stepRes = await request.get(
            `${BASE_URL}/api/learning-paths/${containerPath.id}/steps/${step.id}`
          );
          if (stepRes.ok()) {
            const stepData = await stepRes.json();
            if (stepData.content_md) {
              const match = stepData.content_md.match(assetPattern);
              if (match) {
                assetHash = match[1];
                break;
              }
            }
          }
        }
      }
      if (assetHash) break;
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
