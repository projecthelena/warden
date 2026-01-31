import { test, expect } from '@playwright/test';
import { LoginPage } from './pages/LoginPage';
import { SetupPage } from './pages/SetupPage';
import { DashboardPage } from './pages/DashboardPage';

test.describe('ClusterUptime Smoke Tests', () => {

    test('Full System Flow & Edge Cases', async ({ page }) => {
        test.setTimeout(60000); // Increase timeout for full flow
        // Force clear any storage/cookies to ensure fresh start
        await page.context().clearCookies();
        // await page.evaluate(() => localStorage.clear()); // Causes SecurityError sometimes on initial blank page

        const loginPage = new LoginPage(page);
        const setupPage = new SetupPage(page);
        const dashboardPage = new DashboardPage(page);

        // 1. Initial Load - wait for app to stabilize
        await dashboardPage.goto();
        await page.waitForLoadState('networkidle');

        // 2. Handle Setup or Login based on URL (more reliable than element visibility)
        const currentUrl = page.url();
        if (currentUrl.includes('/setup')) {
            console.log('>> Setup Required.');
            await setupPage.completeSetup();
        }

        // 3. Handle Login (Check URL after setup might have redirected)
        await page.waitForLoadState('networkidle');
        if (page.url().includes('/login')) {
            console.log('>> Login Required.');
            await loginPage.login();
        } else {
            console.log('>> Already authenticated (or skipped login).');
        }

        // 3. Verify Dashboard Access
        await expect(page).toHaveURL(/.*dashboard|.*\/$/);

        // Wait for full load to avoid CI timeouts on "New Monitor" click
        console.log('>> Waiting for Dashboard Load...');
        await dashboardPage.waitForLoad();

        // 4. Edge Case: Invalid URL
        console.log('>> Testing Edge Case: Invalid URL...');
        await dashboardPage.createInvalidMonitor(`Invalid Mon ${Date.now()}`, 'not-a-url');

        // 5. Create Group
        console.log('>> Creating Group...');
        const groupName = `E2E Group ${Date.now()}`;
        await dashboardPage.createGroup(groupName);

        // 6. UX Verify: Pre-selection
        console.log('>> Verifying Pre-selection...');
        await dashboardPage.openNewMonitorSheet();
        await dashboardPage.verifyGroupSelected(groupName);

        // 7. Create Valid Monitor
        console.log('>> Creating Valid Monitor...');
        const monitorName = `Google Test ${Date.now()}`;
        // Note: Sheet is already open from step 6, but our createMonitor function clicks the trigger again.
        // We might need to close it or adjust logic. The `createMonitor` helper currently assumes starting from closed.
        // Let's close it first to be safe and use component's clean state.
        await page.getByRole('button', { name: 'Cancel' }).click();

        await dashboardPage.createMonitor(monitorName, 'https://google.com');

        // 8. Verify Status
        console.log('>> Verifying Status...');
        await dashboardPage.verifyMonitorStatus('Operational');

        console.log('>> Smoke Test Passed.');
    });

});
