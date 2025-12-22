import { Page, Locator, expect } from '@playwright/test';

export class SettingsPage {
    readonly page: Page;
    readonly createKeyTrigger: Locator;
    readonly keyNameInput: Locator;
    readonly createKeySubmit: Locator;

    constructor(page: Page) {
        this.page = page;
        // The create trigger is in the API Keys sheet/page. 
        // Wait, user navigates to /api-keys to see the sheet trigger? 
        // App.tsx route /api-keys renders APIKeysPage.
        // APIKeysView renders the list and usage.
        // The header in App.tsx renders CreateAPIKeySheet if path is /api-keys.

        this.createKeyTrigger = page.getByTestId('create-apikey-trigger');
        this.keyNameInput = page.getByTestId('apikey-name-input');
        this.createKeySubmit = page.getByTestId('apikey-create-submit');
    }

    async gotoApiKeys() {
        await this.page.goto('/settings/api-keys');
    }

    async createApiKey(name: string) {
        // Wait for Auth Check to complete
        await expect(this.page.getByText('Wait ...')).toBeHidden();

        await expect(this.page).toHaveURL(/.*api-keys/);

        const path = await this.page.evaluate(() => window.location.pathname);
        console.log('Current Path:', path);

        // Robust wait for loading to finish
        // Check for App Level Loader first
        await expect(this.page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 10000 });

        // Check for View Level Loader
        await expect(this.page.locator('.animate-pulse')).toHaveCount(0, { timeout: 10000 });

        // Try semantic selector
        const createBtn = this.page.getByRole('button', { name: 'Create API Key' });
        await expect(createBtn).toBeVisible({ timeout: 5000 });
        await createBtn.click(); // Opens sheet

        await this.keyNameInput.fill(name);
        await this.createKeySubmit.click();

        // Verify Success Message or Key Display
        // The sheet shows "Success!" and the key code.
        await expect(this.page.getByText('Success!')).toBeVisible();

        // Close key sheet
        await this.page.getByRole('button', { name: 'Done' }).click();

        // Verify in list
        // APIKeysView probably lists keys. 
        // We didn't check APIKeysView source for testids, but text should work.
        await expect(this.page.getByText(name)).toBeVisible();
    }
}
