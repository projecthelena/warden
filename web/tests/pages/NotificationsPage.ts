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

        // Wait for the sheet to open
        await expect(this.page.getByTestId('delete-channel-btn')).toBeVisible({ timeout: 5000 });

        // Click Delete and wait for both the DELETE call and the subsequent GET refetch
        const deletePromise = this.page.waitForResponse(
            resp => resp.url().includes('/api/notifications/channels') && resp.request().method() === 'DELETE',
            { timeout: 10000 }
        );
        const refetchPromise = this.page.waitForResponse(
            resp => resp.url().includes('/api/notifications/channels') && resp.request().method() === 'GET' && resp.status() === 200,
            { timeout: 10000 }
        );
        await this.page.getByTestId('delete-channel-btn').click();
        await deletePromise;
        await refetchPromise;

        // Verify removal
        await expect(this.page.getByText(name)).toHaveCount(0, { timeout: 10000 });
    }
}
