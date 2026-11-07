import { test, expect } from '@playwright/test';

const approved = { id: 1, name: `<b>${'a'.repeat(70)}</b>`, message: '<img src=x onerror="window.injected=true">', status: 'approved' };
const empty = { items: [], next_cursor: '' };

test('approved HTML is text, pagination works, CSP allows hydration and layout fits', async ({ page }) => {
  const violations = [];
  await page.addInitScript(() => {
    window.cspViolations = [];
    document.addEventListener('securitypolicyviolation', (event) => window.cspViolations.push(event.violatedDirective));
  });
  page.on('pageerror', (error) => violations.push(error.message));
  await page.route('**/api/guestbook?**', async (route) => {
    const second = new URL(route.request().url()).searchParams.has('cursor');
    await route.fulfill({ json: second ? { items: [{ ...approved, id: 2, name: 'next synthetic' }], next_cursor: '' } : { items: [approved], next_cursor: 'synthetic-cursor' } });
  });
  await page.goto('/');
  await expect(page.getByText(approved.message, { exact: true })).toBeVisible();
  expect(await page.evaluate(() => window.injected)).toBeUndefined();
  await page.getByRole('button', { name: 'Muat pesan berikutnya' }).click();
  await expect(page.getByText('next synthetic', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
  expect(violations).toEqual([]);
});

for (const failure of ['http', 'network', 'json']) {
  test(`submission ${failure} failure retains input and clears loading`, async ({ page }) => {
    await page.route('**/api/guestbook**', async (route) => {
      if (route.request().method() === 'GET') return route.fulfill({ json: empty });
      if (failure === 'network') return route.abort();
      if (failure === 'json') return route.fulfill({ contentType: 'application/json', body: 'invalid' });
      return route.fulfill({ status: 503, json: { error: { code: 'unavailable' } } });
    });
    await page.goto('/');
    await page.getByLabel('Nama', { exact: true }).fill('synthetic visitor');
    await page.getByLabel('Pesan', { exact: true }).fill('synthetic message');
    await page.getByRole('button', { name: 'Kirim Pesan' }).click();
    await expect(page.getByRole('button', { name: 'Kirim Pesan' })).toBeEnabled();
    await expect(page.getByLabel('Nama', { exact: true })).toHaveValue('synthetic visitor');
    await expect(page.getByLabel('Pesan', { exact: true })).toHaveValue('synthetic message');
    await expect(page.locator('form [role="alert"]')).not.toBeEmpty();
  });
}

test('pending success is acknowledged, duplicate submission is blocked', async ({ page }) => {
  let posts = 0;
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  await page.route('**/api/guestbook**', async (route) => {
    if (route.request().method() === 'GET') return route.fulfill({ json: empty });
    posts++;
    await gate;
    await route.fulfill({ status: 201, json: { id: 3, status: 'pending' } });
  });
  await page.goto('/');
  await page.getByLabel('Nama', { exact: true }).fill('synthetic visitor');
  await page.getByLabel('Pesan', { exact: true }).fill('pending synthetic');
  await page.getByRole('button', { name: 'Kirim Pesan' }).click();
  await expect(page.getByRole('button', { name: 'Mengirim...' })).toBeDisabled();
  await page.locator('form').evaluate((form) => form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  release();
  await expect(page.getByText('Pesan diterima dan menunggu moderasi sebelum ditampilkan.')).toBeVisible();
  expect(posts).toBe(1);
  await expect(page.getByLabel('Nama', { exact: true })).toHaveValue('');
  await expect(page.getByText('pending synthetic', { exact: true })).toHaveCount(0);
});

for (const failure of ['http', 'network', 'json']) test(`list ${failure} failure can be retried`, async ({ page }) => {
  let calls = 0;
  await page.route('**/api/guestbook?**', (route) => {
    calls++;
    if (calls !== 1) return route.fulfill({ json: empty });
    if (failure === 'network') return route.abort();
    if (failure === 'json') return route.fulfill({ contentType: 'application/json', body: 'invalid' });
    return route.fulfill({ status: 503 });
  });
  await page.goto('/');
  await page.getByRole('button', { name: 'Coba muat lagi' }).click();
  await expect(page.getByText('Belum ada pesan yang disetujui.')).toBeVisible();
});

test('image error fallback runs under the production CSP', async ({ page }) => {
  await page.route('**/api/guestbook?**', (route) => route.fulfill({ json: empty }));
  await page.route('**/screenshots/jdih-talaud.png', (route) => route.fulfill({ status: 404 }));
  await page.route('https://placehold.co/**', (route) => route.fulfill({
    contentType: 'image/png',
    body: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jH9sAAAAASUVORK5CYII=', 'base64'),
  }));
  await page.goto('/');
  await expect(page.getByAltText('JDIH Talaud Application Interface')).toHaveAttribute('src', /^https:\/\/placehold\.co\//);
});

test('Unicode character limits match backend', async ({ page }) => {
  await page.route('**/api/guestbook?**', (route) => route.fulfill({ json: empty }));
  await page.goto('/');
  await page.getByLabel('Nama', { exact: true }).fill('😀'.repeat(80));
  await page.getByLabel('Pesan', { exact: true }).fill('界'.repeat(2000));
  await expect(page.getByRole('button', { name: 'Kirim Pesan' })).toBeEnabled();
  await page.getByLabel('Nama', { exact: true }).fill('😀'.repeat(81));
  await expect(page.getByRole('button', { name: 'Kirim Pesan' })).toBeDisabled();
});
