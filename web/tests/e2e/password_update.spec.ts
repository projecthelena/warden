
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';

test.describe('Password Update Flow', () => {
    // We need a fresh user for this test to avoid messing up the main admin
    const TEST_USER = `user_${Date.now()}`;
    const TEST_PASS = 'InitialPass1!'; // Strong password: 8+ chars, number, special char
    const NEW_PASS = 'NewSecure2@'; // Strong password: 8+ chars, number, special char

    test.beforeAll(async ({ request }) => {
        const resetRes = await request.post('http://localhost:9096/api/admin/reset', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' }
        });
        expect(resetRes.ok()).toBeTruthy();

        // 2. Perform Setup
        const setupRes = await request.post('http://localhost:9096/api/setup', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' },
            data: {
                username: TEST_USER,
                password: TEST_PASS,
                timezone: 'UTC'
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
        await page.fill('input[name="currentPassword"]', 'WrongPass1!');
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

    test.afterAll(async ({ request }) => {
        // Restore Admin user for subsequent tests
        const resetRes = await request.post('http://localhost:9096/api/admin/reset', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' }
        });
        expect(resetRes.ok()).toBeTruthy();

        const setupRes = await request.post('http://localhost:9096/api/setup', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' },
            data: {
                username: 'admin',
                password: 'password123!',
                timezone: 'UTC'
            }
        });
        expect(setupRes.ok()).toBeTruthy();
    });
});
