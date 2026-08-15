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
        this.header = page.getByTestId('login-header');
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
        // The app may still be deciding where to send us: to the login form, or straight
        // to the dashboard when a session already exists. Reading the URL at one instant
        // gets it wrong in both directions, so wait for whichever of the two settles.
        await expect(async () => {
            if (this.page.url().includes('/dashboard')) return;
            await expect(this.header).toBeVisible({ timeout: 1000 });
        }).toPass({ timeout: 20000 });

        // Already signed in; there is no form to fill.
        if (this.page.url().includes('/dashboard')) {
            return;
        }

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
