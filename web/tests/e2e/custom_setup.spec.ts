import { test, expect } from '@playwright/test';
import { SetupPage } from '../pages/SetupPage';

// Standard password for the 'admin' user, used across all E2E tests.
// DO NOT CHANGE THIS unless you update all other test files (auth.spec.ts, etc.)
const STANDARD_ADMIN_PASSWORD = 'password123!';

// Helper to generate a complex password meeting strict security requirements
function generateStrongPassword(): string {
    const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
    const special = "*!@#$%^&()_+-=[]{}|;:,.<>?~`";

    let pass = "";
    // Ensure complexity requirements
    pass += chars[Math.floor(Math.random() * 62)];
    pass += "0123456789"[Math.floor(Math.random() * 10)];
    pass += "*"; // Explicit asterisk
    pass += special[Math.floor(Math.random() * special.length)];

    const allChars = chars + special;
    for (let i = 0; i < 20; i++) {
        pass += allChars[Math.floor(Math.random() * allChars.length)];
    }
    return pass;
}

test.describe('Custom Username Setup', () => {

    test.afterEach(async ({ page, request }) => {
        console.log(">> [TEARDOWN] Ensuring clean state (Admin Reset via Test Key)...");

        // 1. Reset DB using MAGIC KEY (Bypassing Auth)
        const resetRes = await request.post('/api/admin/reset', {
            headers: { 'X-Cluster-Test-Key': 'clusteruptime-e2e-magic-key' }
        });

        if (!resetRes.ok()) {
            console.error(">> [TEARDOWN] Failed to reset DB! Status:", resetRes.status());
        }

        // 2. Restore Admin user
        // MUST use the standard password so other tests don't break.
        const setupRes = await request.post('/api/setup', {
            data: {
                username: 'admin',
                password: STANDARD_ADMIN_PASSWORD,
                timezone: 'UTC',
                createDefaults: true
            }
        });

        if (setupRes.ok()) {
            console.log(">> [TEARDOWN] Default admin restored.");
        }
    });

    test('Should allow setting up with a non-admin username and strong password', async ({ page, request }) => {
        page.on('console', msg => console.log('BROWSER LOG:', msg.text()));

        // 1. Prepare: Reset DB via Test Key
        const resetRes = await request.post('/api/admin/reset', {
            headers: { 'X-Cluster-Test-Key': 'clusteruptime-e2e-magic-key' }
        });
        expect(resetRes.ok()).toBeTruthy();

        await page.goto('/');

        const setupPage = new SetupPage(page);
        await expect(setupPage.welcomeHeader).toBeVisible({ timeout: 10000 });

        const customUser = `CustomUser_${Date.now()}`;
        const customPass = generateStrongPassword();

        console.log(`>> Setting up with User: ${customUser}`);
        console.log(`>> Strong Password Generated: ${customPass}`);

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
