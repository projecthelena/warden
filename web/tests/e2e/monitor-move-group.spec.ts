import { API_BASE } from '../apiBase';
import { expect, test } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe.configure({ mode: 'serial' });

// Reads what the server holds for a monitor, and which group it holds it under. The UI
// redrawing the card in another column proves nothing on its own — a move that does not
// persist would look identical until the next reload.
async function monitorFromApi(page: import('@playwright/test').Page, name: string) {
    const resp = await page.request.get(`${API_BASE}/api/uptime`);
    expect(resp.ok(), 'GET /api/uptime should be authorised').toBeTruthy();
    const body = await resp.json();
    const seen: string[] = [];
    for (const group of body.groups ?? []) {
        for (const monitor of group.monitors ?? []) {
            seen.push(monitor.name);
            if (monitor.name === name) return { ...monitor, groupId: group.id, groupName: group.name };
        }
    }
    throw new Error(`monitor ${name} not found; the board has: ${seen.join(', ') || '(none)'}`);
}

test.describe('Move monitor between groups', () => {
    let dashboard: DashboardPage;
    let sourcePath: string | undefined;
    let targetPath: string | undefined;
    let source: string;
    let target: string;
    let monitorName: string;

    test.beforeEach(async ({ page }) => {
        dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }
        await dashboard.waitForLoad();

        const stamp = Date.now();
        source = `Move From ${stamp}`;
        target = `Move To ${stamp}`;
        monitorName = `Movable ${stamp}`;

        sourcePath = undefined;
        targetPath = undefined;
        sourcePath = await dashboard.createGroup(source);
        await dashboard.createMonitor(monitorName, `${API_BASE}/healthz`);

        // "New Group" only exists on the dashboard root, not inside a group.
        await dashboard.goto();
        await dashboard.waitForLoad();
        targetPath = await dashboard.createGroup(target);
    });

    // Guarded on the paths: if beforeEach fell over before creating a group, cleaning up
    // would throw over the top of the real failure and hide it.
    test.afterEach(async () => {
        if (targetPath) {
            await dashboard.openGroup(targetPath);
            await dashboard.deleteGroup(target);
        }
        if (sourcePath) {
            await dashboard.openGroup(sourcePath);
            await dashboard.deleteGroup(source);
        }
    });

    test('the monitor leaves one group and lands in the other', async ({ page }) => {
        expect((await monitorFromApi(page, monitorName)).groupName).toBe(source);

        await dashboard.openGroup(sourcePath!);
        expect(await dashboard.moveMonitorToGroup(monitorName, target)).toBe(200);

        // Gone from the group it left...
        await expect(page.getByText(monitorName)).toHaveCount(0, { timeout: 10000 });

        // ...present in the one it landed in...
        await dashboard.openGroup(targetPath!);
        await expect(page.getByText(monitorName).first()).toBeVisible({ timeout: 10000 });

        // ...and the server agrees, which is the part a reload would expose.
        expect((await monitorFromApi(page, monitorName)).groupName).toBe(target);
    });

    // A move is not a delete plus a create. If the uptime history did not follow the
    // monitor across, every SLA number on the board would reset on a reorganisation.
    test('the monitor keeps its history and its id', async ({ page }) => {
        await expect.poll(async () => (await monitorFromApi(page, monitorName)).history?.length ?? 0,
            { timeout: 30000 }).toBeGreaterThan(0);
        const before = await monitorFromApi(page, monitorName);

        await dashboard.openGroup(sourcePath!);
        await dashboard.moveMonitorToGroup(monitorName, target);

        const after = await monitorFromApi(page, monitorName);
        expect(after.id).toBe(before.id);
        expect(after.history.length).toBeGreaterThanOrEqual(before.history.length);
        expect(after.interval).toBe(before.interval);
        expect(after.status).toBe(before.status);
    });

    // Reopening the sheet has to show where the monitor actually is, otherwise the next
    // person to touch it moves it somewhere by accident.
    test('the group select shows the group the monitor is in', async ({ page }) => {
        await dashboard.openGroup(sourcePath!);
        await dashboard.openMonitorSettings(monitorName);
        await expect(page.getByTestId('monitor-edit-group-select')).toContainText(source);

        await page.keyboard.press('Escape');
        await dashboard.moveMonitorToGroup(monitorName, target);

        await dashboard.openGroup(targetPath!);
        await dashboard.openMonitorSettings(monitorName);
        await expect(page.getByTestId('monitor-edit-group-select')).toContainText(target);
        await page.keyboard.press('Escape');
    });

    // A regroup is visible outside the dashboard, so it is confirmed like a delete. The
    // Select fires on a stray keystroke while its trigger has focus; without the dialog
    // that keystroke would silently regroup the monitor and close the panel.
    test('cancelling the confirmation leaves the monitor where it was', async ({ page }) => {
        await dashboard.openGroup(sourcePath!);
        await dashboard.pickGroup(monitorName, target);

        await page.getByTestId('monitor-move-cancel').click();

        await expect(page.getByTestId('monitor-edit-group-select')).toContainText(source);
        expect((await monitorFromApi(page, monitorName)).groupName).toBe(source);
        await page.keyboard.press('Escape');
    });

    // The API is the surface the MCP server and any script go through, so it gets its own
    // check rather than being taken on trust from the UI.
    test('the API refuses a move to a group that does not exist', async ({ page }) => {
        const before = await monitorFromApi(page, monitorName);

        const resp = await page.request.post(`${API_BASE}/api/monitors/${before.id}/group`, {
            data: { groupId: 'g-does-not-exist' },
        });
        expect(resp.status()).toBe(404);

        expect((await monitorFromApi(page, monitorName)).groupName).toBe(source);
    });
});
