import { test } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe('Monitor Management', () => {

    test('Create and Delete Group & Monitor', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup
        await dashboard.goto();
        // Assuming already logged in from previous runs or manual login
        // But for isolation, we should check.
        if (await login.isVisible()) {
            await login.login();
        }

        // 2. Create Group
        const groupName = `DeleteMe Group ${Date.now()}`;
        console.log(`Creating Group: ${groupName}`);
        await dashboard.createGroup(groupName);

        // 3. Create Monitor
        const monitorName = `DeleteMe Monitor ${Date.now()}`;
        console.log(`Creating Monitor: ${monitorName}`);
        await dashboard.createMonitor(monitorName, 'https://example.com');

        // 4. Verify Creation
        await dashboard.verifyMonitorStatus();

        // 4.5 Edit Monitor
        const updatedName = monitorName + " (Edited)";
        console.log(`Editing Monitor to: ${updatedName}`);
        await dashboard.editMonitor(monitorName, updatedName);

        // 5. Delete Monitor (using updated name)
        console.log(`Deleting Monitor: ${updatedName}`);
        await dashboard.deleteMonitor(updatedName);

        // 6. Delete Group
        console.log(`Deleting Group: ${groupName}`);
        await dashboard.deleteGroup(groupName);

        console.log('Monitor Management Test Passed.');
    });

});
