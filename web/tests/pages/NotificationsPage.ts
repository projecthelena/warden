import { Page, Locator, expect } from '@playwright/test';

export class NotificationsPage {
    readonly page: Page;
    readonly addChannelTrigger: Locator;

    // Sheet Elements
    readonly typeSelect: Locator;
    readonly nameInput: Locator;
    readonly webhookInput: Locator;
    readonly submitBtn: Locator;

    constructor(page: Page) {
        this.page = page;
        this.addChannelTrigger = page.getByTestId('create-channel-trigger');
        this.typeSelect = page.getByTestId('channel-type-select');
        this.nameInput = page.getByTestId('channel-name-input');
        this.webhookInput = page.getByTestId('channel-webhook-input');
        this.submitBtn = page.getByTestId('create-channel-submit');
    }

    async goto() {
        await this.page.goto('/notifications');
        // Do not assert URL/visibility here to allow for login redirects
    }

    async createSlackChannel(name: string, webhook: string) {
        // Wait for Auth Check to complete
        await expect(this.page.getByText('Wait ...')).toBeHidden();

        await expect(this.page).toHaveURL(/.*notifications/);

        // Check if View content is visible
        await expect(this.page.getByRole('heading', { name: /Notification/i }).first()).toBeVisible({ timeout: 5000 });
        console.log('View Header Visible!');

        // Debugging visibility
        console.log('Waiting for Debug Button...');
        await expect(this.page.getByTestId('debug-button-always')).toBeVisible({ timeout: 5000 });
        console.log('Debug Button Visible!');

        await expect(this.addChannelTrigger).toBeVisible();
        await this.addChannelTrigger.click();

        await this.typeSelect.click();
        await this.page.getByTestId('channel-type-slack').click();

        await this.nameInput.fill(name);
        await this.webhookInput.fill(webhook);

        await this.submitBtn.click();

        // Verify creation
        await expect(this.page.getByText(name)).toBeVisible();
    }

    async deleteChannel(name: string) {
        // Open details
        await this.page.getByText(name).click();

        // Click Delete
        // The delete button in sheet is 'delete-channel-btn'
        await this.page.getByTestId('delete-channel-btn').click();

        // Verify removal
        await expect(this.page.getByText(name)).toHaveCount(0);
    }
}
