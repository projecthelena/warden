import { test } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';
import { SetupPage } from '../pages/SetupPage';

test.describe('Authentication Flow', () => {

    test('Login and Logout', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);
        const setup = new SetupPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        // Handle setup if app hasn't been set up yet
        if (page.url().includes('/setup')) {
            await setup.completeSetup();
        }

        // Handle login if needed
        await page.waitForLoadState('networkidle');
        if (page.url().includes('/login')) {
            await login.login();
        }

        // Wait for dashboard to fully render (sidebar included)
        await dashboard.waitForLoad();

        // Now logged in. Perform Logout.
        console.log('Performing Logout...');
        await login.logout();

        // Verify we are back at login
        if (await login.isVisible()) {
            console.log('Logout Successful.');
        } else {
            throw new Error('Logout failed, Login page not visible');
        }
    });

});
