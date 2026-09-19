import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { MaintenancePage } from '../pages/MaintenancePage';
import { DashboardPage } from '../pages/DashboardPage';

// Run tests serially to avoid conflicts
test.describe.configure({ mode: 'serial' });

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
        // Maintenance is a top-level sidebar link since the nav was flattened
        await page.getByRole('link', { name: 'Maintenance' }).click();

        // 3. Schedule Maintenance
        const title = `Upgrade ${Date.now()}`;
        console.log(`Scheduling Maintenance: ${title}`);
        await maintenance.createMaintenance(title, groupName);

        // This suite shares one database. Remove the active window so later lifecycle
        // tests do not correctly interpret it as unrelated global maintenance.
        const maintenanceResponse = await page.request.get('/api/maintenance');
        expect(maintenanceResponse.ok()).toBeTruthy();
        const windows = await maintenanceResponse.json();
        const created = windows.find((window: { id: string; title: string }) => window.title === title);
        expect(created).toBeTruthy();
        const deleteResponse = await page.request.delete(`/api/maintenance/${created.id}`);
        expect(deleteResponse.ok()).toBeTruthy();
    });

});
