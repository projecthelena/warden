import { test } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe('Authentication Flow', () => {

    test('Login and Logout', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();

        // If not logged in, login first
        if (await login.isVisible()) {
            await login.login();
        }

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
