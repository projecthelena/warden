
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';

test.describe('Password Update Flow', () => {
    // We need a fresh user for this test to avoid messing up the main admin
    const TEST_USER = `user_${Date.now()}`;
    const TEST_PASS = 'InitialPass123!';
    const NEW_PASS = 'NewPass456!';

    test.beforeAll(async ({ request }) => {
        // 1. Create a user via API (Reset DB first to ensure state? No, let's just use API to create user if possible?
        // Actually, our system is single-tenant. We should reset DB or use existing admin.
        // Let's rely on the standard "Reset DB" mechanism to have a clean slate.
        // But reset removes all users. So we must go through setup or use the backdoor if exists.
        // Our setup creates "admin" user usually.
        // Let's use the X-Cluster-Test-Key to reset DB and then Setup API to create a known user.

        const resetRes = await request.post('http://localhost:9096/api/admin/reset', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' }
        });
        expect(resetRes.ok()).toBeTruthy();

        // 2. Perform Setup
        const setupRes = await request.post('http://localhost:9096/api/setup', {
            data: {
                username: TEST_USER,
                password: TEST_PASS,
                timezone: 'UTC',
                createDefaults: true
            }
        });
        expect(setupRes.ok()).toBeTruthy();
    });

    test('should validate current password before allowing change', async ({ page }) => {
        const loginPage = new LoginPage(page);
        await page.goto('/'); // Ensure we are on the app
        await loginPage.login(TEST_USER, TEST_PASS);

        // Go to Settings
        await page.goto('/settings');
        await expect(page.getByText('Account Settings')).toBeVisible();

        // 1. Try to change password without current password
        await page.fill('input[name="password"]', NEW_PASS);
        await page.click('button[type="submit"]');

        const toastError = page.locator("ol li", { hasText: 'current password required' });
        await expect(toastError).toBeVisible();

        // 2. Try with WRONG current password
        await page.fill('input[name="currentPassword"]', 'WrongPass123!');
        await page.click('button[type="submit"]');

        const toastWrong = page.locator("ol li", { hasText: 'current password incorrect' });
        await expect(toastWrong).toBeVisible();
        // Wait for toast to disappear or close? Or just proceed.

        // 3. Success Case
        await page.fill('input[name="currentPassword"]', TEST_PASS);
        await page.click('button[type="submit"]');

        const toastSuccess = page.locator("ol li", { hasText: 'Settings updated' });
        await expect(toastSuccess).toBeVisible();

        // 4. Verify Login with New Password
        await loginPage.logout();
        await loginPage.login(TEST_USER, NEW_PASS);
        await expect(page).toHaveURL(/\/dashboard/);
    });
});
