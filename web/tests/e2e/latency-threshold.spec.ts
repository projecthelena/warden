import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe.configure({ mode: 'serial' });

test.describe('Per-Monitor Latency Threshold', () => {

    test('Create monitor with custom latency threshold and verify in settings', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup - Login
        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        // 2. Create Group
        const groupName = `LT Group ${Date.now()}`;
        const groupPath = await dashboard.createGroup(groupName);

        // 3. Open New Monitor sheet
        await dashboard.createMonitorTrigger.click();

        // 4. Fill in basic fields
        const monitorName = `LT Monitor ${Date.now()}`;
        await dashboard.createMonitorName.fill(monitorName);
        await dashboard.createMonitorUrl.fill(`${API_BASE}/healthz`);

        // Select the group
        const groupSelect = page.getByTestId('create-monitor-group-select');
        await groupSelect.click();
        await page.getByRole('option', { name: groupName }).click();

        // 5. Expand Advanced Settings
        await page.getByText('+ Advanced Settings').click();
        await expect(page.getByLabel('Latency Threshold (ms)')).toBeVisible({ timeout: 5000 });

        // 6. Set Latency Threshold
        await page.getByLabel('Latency Threshold (ms)').fill('2000');

        // 7. Submit
        await dashboard.createMonitorSubmit.click();

        // Wait for creation toast
        const toast = page.getByText(`Monitor "${monitorName}" active and checking.`).first();
        await expect(toast).toBeVisible({ timeout: 15000 });

        // 8. Verify the value persisted in the monitor workspace
        await page.waitForTimeout(1000);
        await page.getByText(monitorName).first().click();
        await expect(page).toHaveURL(/\/monitors\//, { timeout: 10000 });
        await page.getByRole('tab', { name: 'Settings' }).click();
        await page.getByRole('button', { name: /Alerting/ }).click();
        await page.waitForTimeout(500);

        // Verify the latency threshold is persisted
        const latencyInput = page.getByLabel('Latency Threshold (ms)');
        await expect(latencyInput).toBeVisible({ timeout: 5000 });
        await expect(latencyInput).toHaveValue('2000');

        // 9. Update the latency threshold
        await latencyInput.clear();
        await latencyInput.fill('5000');

        // Save
        await page.getByRole('button', { name: 'Save changes' }).click();
        await page.waitForTimeout(1000);

        // 10. Verify the updated value remains in the canonical settings view
        await expect(page.getByLabel('Latency Threshold (ms)')).toHaveValue('5000');

        // 11. Clear the threshold (back to global default)
        await page.getByLabel('Latency Threshold (ms)').clear();
        await page.getByRole('button', { name: 'Save changes' }).click();
        await page.waitForTimeout(1000);

        // Should be empty (global default)
        await expect(page.getByLabel('Latency Threshold (ms)')).toHaveValue('');

        // 12. Cleanup
        await page.goto(groupPath);
        await dashboard.deleteMonitor(monitorName);
        await dashboard.deleteGroup(groupName);

        console.log('Per-Monitor Latency Threshold test passed.');
    });

});
