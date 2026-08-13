import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe.configure({ mode: 'serial' });

// A TCP monitor needs a port that is definitely accepting connections. Whatever is
// serving the app under test is exactly that, so derive the target from the base URL
// instead of hardcoding a port that only holds in one setup.
function targetFromBaseURL(baseURL: string | undefined): string {
    const url = new URL(baseURL ?? 'http://localhost:5173');
    return `${url.hostname}:${url.port || (url.protocol === 'https:' ? '443' : '80')}`;
}

test.describe('Monitor Types', () => {

    test('Create a TCP monitor and see it come up', async ({ page, baseURL }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        const groupName = `TCP Group ${Date.now()}`;
        await dashboard.createGroup(groupName);

        await dashboard.createMonitorTrigger.click();

        const monitorName = `TCP Monitor ${Date.now()}`;
        await dashboard.createMonitorName.fill(monitorName);

        // Switching the type relabels the target field and changes what it accepts.
        await page.getByTestId('create-monitor-type-select').click();
        await page.getByRole('option', { name: 'TCP Port' }).click();
        await expect(page.getByText('Target Host and Port')).toBeVisible();

        // Padded on purpose: targets get pasted, and only the URL parser forgives that.
        await dashboard.createMonitorUrl.fill(`  ${targetFromBaseURL(baseURL)}  `);

        const groupSelect = page.getByTestId('create-monitor-group-select');
        await groupSelect.click();
        await page.getByRole('option', { name: groupName }).click();

        // HTTP-only settings are hidden for a TCP check.
        await page.getByText('+ Advanced Settings').click();
        await expect(page.getByText('Check Configuration')).toBeVisible({ timeout: 5000 });
        await expect(page.getByText('Request Configuration')).toHaveCount(0);

        await dashboard.createMonitorSubmit.click();

        await expect(page.getByText(`Monitor "${monitorName}" active and checking.`).first())
            .toBeVisible({ timeout: 15000 });
        await expect(page.getByText(monitorName).first()).toBeVisible();
        await dashboard.verifyMonitorStatus('Operational');
    });

    test('Selecting DNS reveals the DNS options and hides the HTTP ones', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }
        await dashboard.waitForLoad();

        await dashboard.createMonitorTrigger.click();
        await page.getByTestId('create-monitor-type-select').click();
        await page.getByRole('option', { name: 'DNS' }).click();

        await expect(page.getByText('Name to Resolve')).toBeVisible();

        await page.getByText('+ Advanced Settings').click();
        await expect(page.getByTestId('dns-record-type-select')).toBeVisible({ timeout: 5000 });
        await expect(page.getByTestId('dns-resolver-input')).toBeVisible();
        await expect(page.getByTestId('request-method-select')).toHaveCount(0);
    });

    test('Rejects a target that does not match the selected type', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }
        await dashboard.waitForLoad();

        await dashboard.createMonitorTrigger.click();
        await dashboard.createMonitorName.fill(`Bad Ping ${Date.now()}`);

        await page.getByTestId('create-monitor-type-select').click();
        await page.getByRole('option', { name: 'Ping (ICMP)' }).click();

        // An URL is a valid HTTP target but never a valid ping target.
        await dashboard.createMonitorUrl.fill('https://example.com');
        await dashboard.createMonitorSubmit.click();

        await expect(page.getByText('Invalid Target').first()).toBeVisible({ timeout: 5000 });
    });
});

test.describe('Monitor Type Editing', () => {

    test('Changing type with a stale target is caught before the sheet closes', async ({ page, baseURL }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        const groupName = `Edit Type Group ${Date.now()}`;
        await dashboard.createGroup(groupName);

        const monitorName = `Edit Type Monitor ${Date.now()}`;
        await dashboard.createMonitorTrigger.click();
        await dashboard.createMonitorName.fill(monitorName);
        await dashboard.createMonitorUrl.fill(`${baseURL}/healthz`);
        await page.getByTestId('create-monitor-group-select').click();
        await page.getByRole('option', { name: groupName }).click();
        await dashboard.createMonitorSubmit.click();
        await expect(page.getByText(`Monitor "${monitorName}" active and checking.`).first())
            .toBeVisible({ timeout: 15000 });

        // Open the card itself, not the toast that still carries the same name.
        await dashboard.verifyMonitorStatus('Operational');
        await page.locator('div.rounded-lg.bg-card').filter({ hasText: monitorName }).first().click();
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });
        await page.getByTestId('monitor-settings-tab').click();

        // Switch to TCP but leave the HTTP URL in place, which is what a hurried edit
        // looks like. The sheet has to say so instead of closing and dropping the edits.
        await page.getByTestId('monitor-edit-type-select').click();
        await page.getByRole('option', { name: 'TCP Port' }).click();
        await page.getByTestId('monitor-edit-save-btn').click();

        await expect(page.getByTestId('toast-title').filter({ hasText: 'Invalid Target' }))
            .toBeVisible({ timeout: 5000 });
        await expect(page.getByTestId('monitor-edit-url-input')).toBeVisible();

        // Fixing the target lets the same save through.
        await page.getByTestId('monitor-edit-url-input').fill(targetFromBaseURL(baseURL));
        await page.getByTestId('monitor-edit-save-btn').click();
        await expect(page.getByTestId('toast-title').filter({ hasText: 'Monitor Updated' }))
            .toBeVisible({ timeout: 10000 });
    });
});
