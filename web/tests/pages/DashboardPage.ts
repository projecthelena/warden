import { Page, Locator, expect } from '@playwright/test';

export class DashboardPage {
    readonly page: Page;
    readonly createGroupTrigger: Locator;
    readonly createGroupInput: Locator;
    readonly createGroupSubmit: Locator;

    readonly createMonitorTrigger: Locator;
    readonly createMonitorName: Locator;
    readonly createMonitorUrl: Locator;
    readonly createMonitorSubmit: Locator;

    constructor(page: Page) {
        this.page = page;

        // Group Creators
        this.createGroupTrigger = page.getByTestId('create-group-trigger');
        this.createGroupInput = page.getByTestId('create-group-name-input');
        this.createGroupSubmit = page.getByTestId('create-group-submit-btn');

        // Monitor Creators
        this.createMonitorTrigger = page.getByRole('button', { name: 'New Monitor' });
        this.createMonitorName = page.getByTestId('create-monitor-name-input');
        this.createMonitorUrl = page.getByTestId('create-monitor-url-input');
        this.createMonitorSubmit = page.getByTestId('create-monitor-submit-btn');
    }

    async goto() {
        await this.page.goto('/');
    }

    async waitForLoad() {
        // Wait for App Loader (Increase to 30s for CI)
        await expect(this.page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 30000 });
        // Wait for Auth Check
        await expect(this.page.getByText('Wait ...')).toHaveCount(0, { timeout: 30000 });
        // Wait for Trigger
        await expect(this.createMonitorTrigger).toBeVisible({ timeout: 30000 });
    }

    async createGroup(name: string) {
        await this.createGroupTrigger.click();
        await this.createGroupInput.fill(name);
        await this.createGroupSubmit.click();
        // Wait for redirect
        await expect(this.page).toHaveURL(/\/groups\//);
        // Robust check: first visible occurrence
        await expect(this.page.getByText(name).first()).toBeVisible();
    }

    async createMonitor(name: string, url: string) {
        await this.createMonitorTrigger.click();
        await this.createMonitorName.fill(name);
        await this.createMonitorUrl.fill(url);
        await this.createMonitorSubmit.click();

        // Verify toast and presence
        // Verify toast and presence
        // Actual text: Monitor "Name" active and checking.
        const toast = this.page.getByText(`Monitor "${name}" active and checking.`).first();
        await expect(toast).toBeVisible();
        await expect(this.page.getByText(name).first()).toBeVisible();
    }

    async verifyMonitorStatus(status: string = 'Operational') {
        const badge = this.page.getByText(status).first();
        await expect(badge).toBeVisible();
    }

    async deleteGroup(_groupName: string) {
        // Assume we are on a page where this group is visible in the sidebar or header
        // For deleting, we usually are on the group page, so we click the trash icon in header
        await expect(this.page.getByTestId('delete-group-trigger')).toBeVisible();
        await this.page.getByTestId('delete-group-trigger').click();
        await this.page.getByTestId('delete-group-confirm').click();

        // Use regex for flexible URL matching (dashboard or root)
        await expect(this.page).toHaveURL(/\/dashboard|^\/$/);
    }

    async deleteMonitor(monitorName: string) {
        // Open Monitor Details
        await this.page.getByText(monitorName).first().click();

        // Click Settings Tab
        await this.page.getByTestId('monitor-settings-tab').click();

        // Click Delete
        await this.page.getByTestId('delete-monitor-trigger').click();
        // Confirm
        await this.page.getByTestId('delete-monitor-confirm').click();

        // Verify it's gone
        await expect(this.page.getByText(monitorName)).toHaveCount(0);
    }

    async editMonitor(oldName: string, newName: string) {
        // Open Monitor Details
        await this.page.getByText(oldName).first().click();

        // Click Settings Tab
        await this.page.getByTestId('monitor-settings-tab').click();

        // Update Name using robust testid
        await this.page.getByTestId('monitor-edit-name-input').fill(newName);

        // Click Save
        await this.page.getByTestId('monitor-edit-save-btn').click();

        // Wait for sheet to close or verify change in list
        // Sheet might take a moment to close.
        await expect(this.page.getByText(newName).first()).toBeVisible();
    }

    async createInvalidMonitor(name: string, url: string) {
        await this.createMonitorTrigger.click();
        await this.createMonitorName.fill(name);
        await this.createMonitorUrl.fill(url);
        await this.createMonitorSubmit.click();

        // Expect Error Toast - targeted by testid
        const errorToast = this.page.getByTestId('toast-title').filter({ hasText: 'Invalid URL' });
        await expect(errorToast).toBeVisible();

        // Close sheet
        await this.page.getByRole('button', { name: 'Cancel' }).click();
    }

    async openNewMonitorSheet() {
        await this.createMonitorTrigger.click();
    }

    async verifyGroupSelected(groupName: string) {
        // Check ONLY the select trigger
        const trigger = this.page.getByTestId('create-monitor-group-select');
        await expect(trigger).toBeVisible();
        await expect(trigger).toContainText(groupName);
    }
}
