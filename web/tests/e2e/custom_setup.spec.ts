import { test, expect } from '@playwright/test';
import { SetupPage } from '../pages/SetupPage';

// Standard password for the 'admin' user, used across all E2E tests.
// Requires: 8+ chars, number, special character
const STANDARD_ADMIN_PASSWORD = 'password123!';

test.describe('Custom Username Setup', () => {

    test.afterEach(async ({ request }) => {
        console.log(">> [TEARDOWN] Ensuring clean state (Admin Reset via Test Key)...");

        // 1. Reset DB via Admin Secret (Bypassing Auth)
        const resetRes = await request.post('http://localhost:9096/api/admin/reset', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' }
        });

        if (!resetRes.ok()) {
            console.error(">> [TEARDOWN] Failed to reset DB! Status:", resetRes.status());
        }

        // 2. Restore Admin user
        const setupRes = await request.post('http://localhost:9096/api/setup', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' },
            data: {
                username: 'admin',
                password: STANDARD_ADMIN_PASSWORD,
                timezone: 'UTC'
            }
        });

        if (setupRes.ok()) {
            console.log(">> [TEARDOWN] Default admin restored.");
        }
    });

    test('Should allow setting up with a non-admin username', async ({ page, request }) => {
        page.on('console', msg => console.log('BROWSER LOG:', msg.text()));

        // 1. Reset DB via Admin Secret (Bypassing Auth)
        const resetRes = await request.post('http://localhost:9096/api/admin/reset', {
            headers: { 'X-Admin-Secret': 'clusteruptime-e2e-magic-key' }
        });
        expect(resetRes.ok()).toBeTruthy();

        await page.goto('/');

        const setupPage = new SetupPage(page);
        await expect(setupPage.welcomeHeader).toBeVisible({ timeout: 10000 });

        const customUser = `customuser_${Date.now()}`;
        const customPass = 'MySecure1!'; // Strong password: 8+ chars, number, special char

        console.log(`>> Setting up with User: ${customUser}`);
        console.log(`>> Password: ${customPass}`);

        await setupPage.completeSetup(customUser, customPass);

        // 3. Verification
        console.log(">> Verifying redirect...");
        try {
            await expect(page).toHaveURL(/.*dashboard|.*\/$/, { timeout: 10000 });
            console.log(">> Custom setup verification passed (Auto-login worked).");
        } catch {
            console.log(">> Auto-login might have failed (landed on Login), attempting manual login verification...");

            // Check if we are indeed on Login page
            if (page.url().includes('login')) {
                await page.getByLabel('Username').fill(customUser);
                await page.getByLabel('Password').fill(customPass);
                await page.getByRole('button', { name: 'Sign in' }).click();

                await expect(page).toHaveURL(/.*dashboard|.*\/$/, { timeout: 10000 });
                console.log(">> Manual login successful. Custom user creation verified.");
            } else {
                console.log(`>> Failed on unexpected URL: ${page.url()}`);
                throw new Error("Setup failed to redirect to Dashboard or Login.");
            }
        }
    });
});
