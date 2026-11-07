import { test, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';

test('browser → API → database → moderation → public text', async ({ page, request, baseURL }) => {
  test.skip(process.env.RUN_DATABASE_E2E !== '1', 'Set RUN_DATABASE_E2E=1 with the isolated test database and API binary.');
  const db = new URL(process.env.TEST_DATABASE_URL);
  if (!db.pathname.endsWith('_test') || process.env.DATABASE_URL !== process.env.TEST_DATABASE_URL || new URL(baseURL).hostname !== '127.0.0.1') {
    throw new Error('Real flow requires an isolated _test database and loopback API.');
  }
  const moderate = (action, id) => execFileSync(process.env.API_BINARY, ['moderate', action, String(id)], {
    env: { ...process.env, MODERATOR_ID: 'synthetic-e2e' }, stdio: 'pipe',
  });
  const text = `synthetic-${test.info().project.name}: <script>window.injected=true</script>`;
  let id;
  try {
    await page.goto('/');
    await page.getByLabel('Nama', { exact: true }).fill('synthetic browser');
    await page.getByLabel('Pesan', { exact: true }).fill(text);
    const response = page.waitForResponse((res) => res.url().endsWith('/api/guestbook') && res.request().method() === 'POST');
    await page.getByRole('button', { name: 'Kirim Pesan' }).click();
    const posted = await response;
    expect(posted.status()).toBe(201);
    const entry = await posted.json();
    id = entry.id;
    expect(entry.status).toBe('pending');
    await expect(page.getByText('Pesan diterima dan menunggu moderasi sebelum ditampilkan.')).toBeVisible();
    const pendingPage = await (await request.get('/api/guestbook')).json();
    expect(pendingPage.items.some((item) => item.id === id)).toBe(false);
    moderate('approve', id);
    await page.reload();
    await expect(page.getByText(text, { exact: true })).toBeVisible();
    expect(await page.evaluate(() => window.injected)).toBeUndefined();
    moderate('reject', id);
    await page.reload();
    await expect(page.getByText(text, { exact: true })).toHaveCount(0);
  } finally {
    if (id) moderate('delete', id);
  }
});
