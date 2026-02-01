import { Page, Locator, expect } from '@playwright/test';

export class SetupPage {
    readonly page: Page;
    readonly welcomeHeader: Locator;
    readonly startBtn: Locator;
    readonly usernameInput: Locator;
    readonly passwordInput: Locator;
    readonly launchBtn: Locator;

    constructor(page: Page) {
        this.page = page;
        this.welcomeHeader = page.getByTestId('setup-welcome');
        this.startBtn = page.getByTestId('setup-start-btn');
        this.usernameInput = page.getByTestId('setup-username-input');
        this.passwordInput = page.getByTestId('setup-password-input');
        this.launchBtn = page.getByTestId('setup-launch-btn');
    }

    async isVisible() {
        return await this.welcomeHeader.isVisible();
    }

    async completeSetup(username = 'admin', password = 'password123!') {
        // Step 0: Welcome - click Get Started
        await this.startBtn.click();

        // Step 1: Create Account
        await this.usernameInput.fill(username);
        await this.passwordInput.fill(password);
        await this.launchBtn.click();

        // Wait for successful redirection to Dashboard (auto-login enabled)
        await expect(this.page).toHaveURL(/.*(dashboard|login|\/$)/, { timeout: 30000 });
    }
}
