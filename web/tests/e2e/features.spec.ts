import { test } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { MaintenancePage } from '../pages/MaintenancePage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe('System Features', () => {

    test('Maintenance Windows', async ({ page }) => {
        const maintenance = new MaintenancePage(page);
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup - Create a group first
        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        const groupName = `Maint Group ${Date.now()}`;
        console.log(`Creating Group: ${groupName}`);
        await dashboard.createGroup(groupName);

        // 2. Navigate to Maintenance (SPA Nav)
        // Expand Events menu if needed and click Maintenance link
        await page.getByRole('button', { name: 'Events' }).click();
        await page.getByRole('link', { name: 'Maintenance' }).click();

        // 3. Schedule Maintenance
        const title = `Upgrade ${Date.now()}`;
        console.log(`Scheduling Maintenance: ${title}`);
        await maintenance.createMaintenance(title, groupName);

        // 4. Cleanup (optional, or rely on system reset)
    });

});
