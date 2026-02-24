import { request, FullConfig } from '@playwright/test';
import fs from 'fs';
import path from 'path';

const BASE_URL = process.env.BASE_URL || 'http://localhost:9080';
const GITHUB_TOKEN = process.env.GITHUB_TOKEN || '';
const CONTENT_REPO = 'https://github.com/fsamin/phoebus-content-samples.git';

async function waitForHealth(timeoutMs = 60_000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(`${BASE_URL}/api/health`);
      if (res.ok) return;
    } catch {}
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error(`Phœbus not ready after ${timeoutMs}ms`);
}

async function loginAdmin(ctx: ReturnType<typeof request.newContext> extends Promise<infer T> ? T : never) {
  const res = await ctx.post(`${BASE_URL}/api/auth/login`, {
    data: { username: 'admin', password: 'admin' },
  });
  if (res.status() !== 200) throw new Error(`Admin login failed: ${res.status()}`);
  return res;
}

async function addContentRepo(ctx: ReturnType<typeof request.newContext> extends Promise<infer T> ? T : never) {
  const body: Record<string, string> = {
    clone_url: CONTENT_REPO,
  };
  if (GITHUB_TOKEN) {
    body.auth_type = 'token';
    body.credentials = GITHUB_TOKEN;
  }

  const res = await ctx.post(`${BASE_URL}/api/admin/repos`, { data: body });
  if (res.status() !== 201 && res.status() !== 409) {
    throw new Error(`Failed to add repo: ${res.status()} ${await res.text()}`);
  }
  return res;
}

async function waitForSync(ctx: ReturnType<typeof request.newContext> extends Promise<infer T> ? T : never, timeoutMs = 120_000) {
  const reposRes = await ctx.get(`${BASE_URL}/api/admin/repos`);
  const repos = await reposRes.json();
  const repo = repos.find((r: { clone_url: string }) => r.clone_url.includes('phoebus-content-samples'));
  if (!repo) {
    console.log('⚠️  Repo content-samples not found after creation — skipping sync wait');
    return false;
  }

  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const logsRes = await ctx.get(`${BASE_URL}/api/admin/repos/${repo.id}/sync-logs`);
    const logs = await logsRes.json();
    if (logs.length > 0 && logs[0].status === 'completed') return true;
    if (logs.length > 0 && logs[0].status === 'failed') {
      console.log(`⚠️  Sync failed: ${logs[0].error}`);
      return false;
    }
    await new Promise((r) => setTimeout(r, 2000));
  }
  console.log('⚠️  Sync not completed within timeout');
  return false;
}

export default async function globalSetup(config: FullConfig) {
  console.log('⏳ Waiting for Phœbus to be ready…');
  await waitForHealth();
  console.log('✅ Phœbus is ready');

  const ctx = await request.newContext({ ignoreHTTPSErrors: true });

  // Login as admin and save storage state
  await loginAdmin(ctx);
  const storageDir = path.join(__dirname, 'storage-state');
  fs.mkdirSync(storageDir, { recursive: true });
  // Clean up stale content-synced flag from previous runs
  const syncFlagPath = path.join(storageDir, 'content-synced');
  if (fs.existsSync(syncFlagPath)) fs.unlinkSync(syncFlagPath);
  await ctx.storageState({ path: path.join(storageDir, 'admin.json') });
  console.log('✅ Admin logged in, storage state saved');

  // Add content repo if token is available
  if (GITHUB_TOKEN) {
    console.log('⏳ Adding content-samples repo…');
    await addContentRepo(ctx);
    console.log('⏳ Waiting for content sync…');
    const synced = await waitForSync(ctx);
    if (synced) {
      console.log('✅ Content synced');
      fs.writeFileSync(path.join(__dirname, 'storage-state', 'content-synced'), 'true');
    } else {
      console.log('⚠️  Content not available — content-dependent tests will be skipped');
    }
  } else {
    console.log('⚠️  GITHUB_TOKEN not set — skipping content repo setup');
  }

  await ctx.dispose();
}
