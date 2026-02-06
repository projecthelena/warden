import { test, expect } from '@playwright/test';
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

test.describe('Monitor Pause/Resume', () => {

    test('Pause and Resume monitor via settings', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup - Login
        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        // 2. Create Group & Monitor
        const groupName = `Pause Test Group ${Date.now()}`;
        const monitorName = `Pause Test Monitor ${Date.now()}`;
        console.log(`Creating Group: ${groupName}`);
        await dashboard.createGroup(groupName);

        console.log(`Creating Monitor: ${monitorName}`);
        await dashboard.createMonitor(monitorName, 'https://httpbin.org/get');

        // Wait for monitor to show as operational
        await dashboard.verifyMonitorStatus('Operational');

        // 3. Pause the monitor via settings
        console.log(`Pausing Monitor: ${monitorName}`);
        await dashboard.pauseMonitorViaSettings(monitorName);

        // 4. Verify monitor is paused
        await dashboard.verifyMonitorPaused(monitorName);
        console.log('Monitor paused successfully');

        // 5. Resume the monitor via settings
        console.log(`Resuming Monitor: ${monitorName}`);
        await dashboard.resumeMonitorViaSettings(monitorName);

        // 6. Verify monitor is operational again
        await dashboard.verifyMonitorOperational(monitorName);
        console.log('Monitor resumed successfully');

        // 7. Cleanup
        await dashboard.deleteMonitor(monitorName);
        await dashboard.deleteGroup(groupName);

        console.log('Pause/Resume via settings test passed.');
    });

    test('Pause and Resume monitor via settings sheet', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup - Login
        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        // 2. Create Group & Monitor
        const groupName = `Settings Pause Group ${Date.now()}`;
        const monitorName = `Settings Pause Monitor ${Date.now()}`;
        console.log(`Creating Group: ${groupName}`);
        await dashboard.createGroup(groupName);

        console.log(`Creating Monitor: ${monitorName}`);
        await dashboard.createMonitor(monitorName, 'https://httpbin.org/get');

        // Wait for monitor to show as operational
        await dashboard.verifyMonitorStatus('Operational');

        // 3. Pause via settings
        console.log(`Pausing Monitor via settings: ${monitorName}`);
        await dashboard.pauseMonitorViaSettings(monitorName);

        // Wait for UI to update
        await page.waitForTimeout(1000);

        // 4. Verify monitor is paused
        await dashboard.verifyMonitorPaused(monitorName);
        console.log('Monitor paused via settings successfully');

        // 5. Resume via settings
        console.log(`Resuming Monitor via settings: ${monitorName}`);
        await dashboard.resumeMonitorViaSettings(monitorName);

        // Wait for UI to update
        await page.waitForTimeout(1000);

        // 6. Verify monitor is operational again
        await dashboard.verifyMonitorOperational(monitorName);
        console.log('Monitor resumed via settings successfully');

        // 7. Cleanup
        await dashboard.deleteMonitor(monitorName);
        await dashboard.deleteGroup(groupName);

        console.log('Pause/Resume via settings sheet test passed.');
    });

    test('Paused monitor persists after page refresh', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup - Login
        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        // 2. Create Group & Monitor
        const groupName = `Persist Pause Group ${Date.now()}`;
        const monitorName = `Persist Pause Monitor ${Date.now()}`;
        console.log(`Creating Group: ${groupName}`);
        await dashboard.createGroup(groupName);

        console.log(`Creating Monitor: ${monitorName}`);
        await dashboard.createMonitor(monitorName, 'https://httpbin.org/get');

        // Wait for monitor to show as operational
        await dashboard.verifyMonitorStatus('Operational');

        // 3. Pause the monitor via settings
        console.log(`Pausing Monitor: ${monitorName}`);
        await dashboard.pauseMonitorViaSettings(monitorName);
        await dashboard.verifyMonitorPaused(monitorName);

        // 4. Refresh the page
        console.log('Refreshing page...');
        await page.reload();
        await dashboard.waitForLoad();

        // 5. Navigate back to the group if needed
        await page.goto(`/groups/${groupName.toLowerCase().replace(/ /g, '-')}`);
        await page.waitForTimeout(2000);

        // 6. Verify monitor is still paused after refresh
        await expect(page.getByText('Paused')).toBeVisible({ timeout: 10000 });
        console.log('Monitor paused state persisted after refresh');

        // 7. Resume and cleanup
        await dashboard.resumeMonitorViaSettings(monitorName);
        await dashboard.deleteMonitor(monitorName);
        await dashboard.deleteGroup(groupName);

        console.log('Paused state persistence test passed.');
    });

});
