import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { SettingsPage } from '../pages/SettingsPage';

test.describe('Settings & API Keys', () => {

    test('Generate API Key', async ({ page }) => {
        const settings = new SettingsPage(page);
        const login = new LoginPage(page);

        // 1. Setup
        // Start at Dashboard (Common entry point to avoid reload on specific routes if possible)
        await page.goto('/dashboard');

        // Wait for Auth Check
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });

        // Handle Login if needed
        if (await login.isVisible()) {
            await login.login();
            await expect(page).toHaveURL(/.*dashboard/);
        }

        // 2. Navigate via SPA (Sidebar)
        // Expand Settings (Robust selector)
        await page.locator('button:has(span:text-is("Settings"))').click();
        await page.getByRole('link', { name: 'API Keys' }).click();

        // Verify URL
        await expect(page).toHaveURL(/.*settings\/api-keys/);

        // 2. Create Key
        const keyName = `CI Key ${Date.now()}`;
        console.log(`Generating API Key: ${keyName}`);
        await settings.createApiKey(keyName);

        // We don't delete it in this test (no delete btn in sheet, probably only in list).
        // For now, testing creation is sufficient for "Use Case Expansion".
    });

});
