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
        await this.usernameInput.fill(username);
        await this.passwordInput.fill(password);
        await this.submitBtn.click();

        // Wait for redirection to Dashboard
        // App redirects to /dashboard or / (which redirects to /dashboard)
        await expect(this.page).toHaveURL(/\/dashboard/);
    }

    async logout() {
        // Open user menu
        await this.page.getByTestId('user-menu-trigger').click();

        await this.page.getByTestId('logout-btn').click();

        // specific check for login page return
        await expect(this.header).toBeVisible();
    }
}
