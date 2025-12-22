import { Page, Locator, expect } from '@playwright/test';

export class SetupPage {
    readonly page: Page;
    readonly welcomeHeader: Locator;
    readonly startBtn: Locator;
    readonly usernameInput: Locator;
    readonly passwordInput: Locator;
    readonly continueBtn: Locator;
    readonly continueBtn2: Locator;
    readonly launchBtn: Locator;

    constructor(page: Page) {
        this.page = page;
        this.welcomeHeader = page.getByTestId('setup-welcome');
        this.startBtn = page.getByTestId('setup-start-btn');
        this.usernameInput = page.getByTestId('setup-username-input');
        this.passwordInput = page.getByTestId('setup-password-input');
        this.continueBtn = page.getByTestId('setup-continue-btn');
        this.continueBtn2 = page.getByTestId('setup-continue-btn-2');
        this.launchBtn = page.getByTestId('setup-launch-btn');
    }

    async isVisible() {
        return await this.welcomeHeader.isVisible();
    }

    async completeSetup(username = 'admin', password = 'password123!') {
        await this.startBtn.click();
        await this.usernameInput.fill(username);
        await this.passwordInput.fill(password);
        await this.continueBtn.click();
        await this.continueBtn2.click(); // Timezone
        await this.launchBtn.click();   // Launch

        // Wait for successful redirection to Dashboard
        // The setup page uses window.location.href = "/" which eventually hits /dashboard
        // Wait for successful redirection to Dashboard OR Login (if auth state didn't persist)
        await expect(this.page).toHaveURL(/.*(dashboard|login)/, { timeout: 30000 });
    }
}
