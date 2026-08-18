import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe.configure({ mode: 'serial' });

// Reads what the server holds for a monitor. The button is only meaningful if the flag
// actually persists — a toggle that flips in the UI and forgets on reload is worse than no
// toggle, because the operator believes a monitor is muted when it is not.
async function monitorFromApi(page: import('@playwright/test').Page, name: string) {
    const resp = await page.request.get(`${API_BASE}/api/uptime`);
    expect(resp.ok(), 'GET /api/uptime should be authorised').toBeTruthy();
    const body = await resp.json();
    const seen: string[] = [];
    for (const group of body.groups ?? []) {
        for (const monitor of group.monitors ?? []) {
            seen.push(monitor.name);
            if (monitor.name === name) return monitor;
        }
    }
    throw new Error(`monitor ${name} not found; the board has: ${seen.join(', ') || '(none)'}`);
}

// Opens a monitor's details sheet on its Settings tab, where pausing, muting and deleting
// live.
async function openMonitorSettings(page: import('@playwright/test').Page, name: string) {
    await page.getByText(name).first().click();
    await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 10000 });
    await page.getByRole('tab', { name: 'Settings' }).click();
}

test.describe('Per-monitor alert mute', () => {
    let monitorName: string;

    test.beforeEach(async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });
    });

    test('mutes and unmutes a monitor, and the flag survives a reload', async ({ page }) => {
        const dashboard = new DashboardPage(page);

        const groupName = `Mute Group ${Date.now()}`;
        await dashboard.createGroup(groupName);

        monitorName = `Mute Monitor ${Date.now()}`;
        await dashboard.createMonitorTrigger.click();
        await dashboard.createMonitorName.fill(monitorName);
        await dashboard.createMonitorUrl.fill(`${API_BASE}/healthz`);
        await page.getByTestId('create-monitor-group-select').click();
        await page.getByRole('option', { name: groupName }).click();

        // Wait on the POST rather than a toast: the creation is what the rest of the test
        // depends on, and a silent validation failure would otherwise show up much later
        // as a confusing "monitor not found".
        const created = page.waitForResponse(
            resp => resp.url().includes('/api/monitors') && resp.request().method() === 'POST',
        );
        await dashboard.createMonitorSubmit.click();
        const createResp = await created;
        expect(createResp.status(), await createResp.text()).toBe(201);

        // A new monitor starts audible.
        await expect.poll(async () => (await monitorFromApi(page, monitorName)).alertsMuted,
            { timeout: 10000 }).toBe(false);

        // Open the details sheet and mute it. The control lives on the Settings tab,
        // alongside pausing and deleting.
        await openMonitorSettings(page, monitorName);
        const muteBtn = page.getByTestId('monitor-mute-alerts-btn');
        await expect(muteBtn).toBeVisible({ timeout: 10000 });
        await expect(muteBtn).toContainText('Mute Alerts');

        const muted = page.waitForResponse(
            resp => resp.url().includes('/alerts') && resp.request().method() === 'POST',
        );
        await muteBtn.click();
        expect((await muted).status()).toBe(200);

        await expect.poll(async () => (await monitorFromApi(page, monitorName)).alertsMuted,
            { timeout: 10000 }).toBe(true);

        // The button now offers the opposite action, and says what muting actually means —
        // still checked, still recorded, just silent.
        await expect(muteBtn).toContainText('Unmute Alerts', { timeout: 10000 });
        await expect(page.getByText(/still appears in the daily digest/i)).toBeVisible();

        // It survives a reload rather than living only in the sheet's state.
        await page.reload();
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });
        await openMonitorSettings(page, monitorName);
        await expect(page.getByTestId('monitor-mute-alerts-btn')).toContainText('Unmute Alerts', {
            timeout: 10000,
        });

        // And it turns back off.
        const unmuted = page.waitForResponse(
            resp => resp.url().includes('/alerts') && resp.request().method() === 'POST',
        );
        await page.getByTestId('monitor-mute-alerts-btn').click();
        expect((await unmuted).status()).toBe(200);

        await expect.poll(async () => (await monitorFromApi(page, monitorName)).alertsMuted,
            { timeout: 10000 }).toBe(false);
    });

    // Muting is not pausing. A muted monitor keeps being checked, which is the whole point:
    // its history and the daily digest stay intact, only the interruptions stop.
    test('muting does not pause the monitor', async ({ page }) => {
        const monitor = await monitorFromApi(page, monitorName);
        expect(monitor.active, 'a muted monitor must still be checked').toBe(true);
    });

    test('the API rejects a mute for a monitor that does not exist', async ({ page }) => {
        const resp = await page.request.post(`${API_BASE}/api/monitors/does-not-exist/alerts`, {
            data: { muted: true },
        });
        expect(resp.status()).toBe(404);
    });

    test('the API rejects a malformed body', async ({ page }) => {
        const monitor = await monitorFromApi(page, monitorName);
        const resp = await page.request.post(`${API_BASE}/api/monitors/${monitor.id}/alerts`, {
            data: 'not json',
            headers: { 'Content-Type': 'application/json' },
        });
        expect(resp.status()).toBe(400);
    });
});
