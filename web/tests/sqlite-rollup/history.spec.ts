import { expect, test } from '@playwright/test';

type Monitor = {
  id: string;
  uptime: { percent: number; totalChecks: number; downChecks: number };
  uptimeDays: { totalChecks: number; upChecks: number }[];
};

test('startup backfill preserves exact history across all SQLite batches', async ({ request }) => {
  // Wait for the real startup worker, without a test-only rollup endpoint.
  await expect.poll(async () => {
    const response = await request.get('/api/s/rollup?group=rollup&page_size=100');
    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    return body.groups[0].monitors[0]?.uptime.totalChecks;
  }).toBe(8);

  const monitors: Monitor[] = [];
  for (const page of [1, 2]) {
    const response = await request.get(`/api/s/rollup?group=rollup&page_size=100&page=${page}`);
    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    monitors.push(...body.groups[0].monitors);
  }
  expect(monitors).toHaveLength(123);
  expect(new Set(monitors.map((monitor) => monitor.id)).size).toBe(123);
  for (const monitor of monitors) {
    expect(monitor.uptime, monitor.id).toMatchObject({ percent: 75, totalChecks: 8, downChecks: 2 });
    const populated = monitor.uptimeDays.filter((day) => day.totalChecks > 0);
    expect(populated, monitor.id).toHaveLength(2);
    for (const day of populated) expect(day).toMatchObject({ totalChecks: 4, upChecks: 3 });
  }
  expect((await request.get('/readyz')).status()).toBe(200);
});

test('public page displays rolled-up uptime on both sides of batch boundaries', async ({ page }) => {
  await page.goto('/status/rollup');
  await page.getByRole('tab', { name: 'Services' }).click();
  await page.getByRole('button', { name: /Rollup services/ }).click();
  const search = page.getByRole('textbox', { name: 'Search services in Rollup services' });
  for (const index of [0, 49, 50, 99, 100, 122]) {
    const name = `Rollup service ${String(index).padStart(3, '0')}`;
    await search.fill(name);
    await expect(page.getByText(/^Rollup service \d{3}$/)).toHaveCount(1);
    await expect(page.getByText(name, { exact: true })).toBeVisible();
    await expect(page.getByText('75.00%', { exact: true })).toBeVisible();
  }
});
