import { Page, Locator, expect } from '@playwright/test';

export class LoginPage {
    readonly page: Page;
    readonly usernameInput: Locator;
    readonly passwordInput: Locator;
    readonly submitBtn: Locator;
    readonly header: Locator;

    constructor(page: Page) {
        this.page = page;
        this.usernameInput = page.getByLabel('Username');
        this.passwordInput = page.getByLabel('Password');
        this.submitBtn = page.getByRole('button', { name: 'Sign in' });
        this.header = page.getByRole('heading', { name: 'Welcome back' });
    }

    async isVisible() {
        try {
            await expect(this.header).toBeVisible({ timeout: 2000 });
            return true;
        } catch {
            return false;
        }
    }

    async login(username = 'admin', password = 'password123!') {
        // Wait for login page to be ready (longer timeout for CI where SPA redirect is slow)
        await expect(this.header).toBeVisible({ timeout: 15000 });

        // Fill credentials
        await this.usernameInput.fill(username);
        await this.passwordInput.fill(password);

        // Check if form auto-submitted during fill (race condition in CI)
        // Give a brief moment for any auto-navigation to start
        await this.page.waitForTimeout(100);

        // If already navigating away from login, just wait for dashboard
        if (!this.page.url().includes('/login')) {
            await expect(this.page).toHaveURL(/\/dashboard/, { timeout: 15000 });
            return;
        }

        // Still on login page - submit form explicitly
        await this.submitBtn.click();

        // Wait for navigation to dashboard
        await expect(this.page).toHaveURL(/\/dashboard/, { timeout: 15000 });
    }

    async logout() {
        // Open user menu
        await this.page.getByTestId('user-menu-trigger').click();

        await this.page.getByTestId('logout-btn').click();

        // specific check for login page return
        await expect(this.header).toBeVisible();
    }
}
