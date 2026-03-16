import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe.configure({ mode: 'serial' });

test.describe('Per-Monitor Latency Threshold', () => {

    test('Create monitor with custom latency threshold and verify in edit sheet', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup - Login
        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        // 2. Create Group
        const groupName = `LT Group ${Date.now()}`;
        await dashboard.createGroup(groupName);

        // 3. Open New Monitor sheet
        await dashboard.createMonitorTrigger.click();

        // 4. Fill in basic fields
        const monitorName = `LT Monitor ${Date.now()}`;
        await dashboard.createMonitorName.fill(monitorName);
        await dashboard.createMonitorUrl.fill('http://localhost:9096/healthz');

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

        // 8. Verify the value persisted by opening the edit sheet
        await page.waitForTimeout(1000);
        await page.getByText(monitorName).first().click();

        // Wait for sheet to open
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });

        // Switch to Settings tab
        await page.getByRole('tab', { name: 'Settings' }).click();
        await page.waitForTimeout(500);

        // Verify the latency threshold is persisted
        const latencyInput = page.getByLabel('Latency Threshold (ms)');
        await expect(latencyInput).toBeVisible({ timeout: 5000 });
        await expect(latencyInput).toHaveValue('2000');

        // 9. Update the latency threshold
        await latencyInput.clear();
        await latencyInput.fill('5000');

        // Save
        await page.getByRole('button', { name: 'Save' }).click();
        await page.waitForTimeout(1000);

        // 10. Re-open and verify updated value
        // Close the sheet first
        await page.keyboard.press('Escape');
        await page.waitForTimeout(500);

        await page.getByText(monitorName).first().click();
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });
        await page.getByRole('tab', { name: 'Settings' }).click();
        await page.waitForTimeout(500);

        await expect(page.getByLabel('Latency Threshold (ms)')).toHaveValue('5000');

        // 11. Clear the threshold (back to global default)
        await page.getByLabel('Latency Threshold (ms)').clear();
        await page.getByRole('button', { name: 'Save' }).click();
        await page.waitForTimeout(1000);

        // Close and reopen
        await page.keyboard.press('Escape');
        await page.waitForTimeout(500);

        await page.getByText(monitorName).first().click();
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });
        await page.getByRole('tab', { name: 'Settings' }).click();
        await page.waitForTimeout(500);

        // Should be empty (global default)
        await expect(page.getByLabel('Latency Threshold (ms)')).toHaveValue('');

        // 12. Cleanup
        await page.keyboard.press('Escape');
        await page.waitForTimeout(500);
        await dashboard.deleteMonitor(monitorName);
        await dashboard.deleteGroup(groupName);

        console.log('Per-Monitor Latency Threshold test passed.');
    });

});
