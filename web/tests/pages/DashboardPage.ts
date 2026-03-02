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
        // Actual text: Monitor "Name" active and checking.
        const toast = this.page.getByText(`Monitor "${name}" active and checking.`).first();
        await expect(toast).toBeVisible({ timeout: 15000 });
        await expect(this.page.getByText(name).first()).toBeVisible();
    }

    async verifyMonitorStatus(status: string = 'Operational', timeout: number = 30000) {
        const badge = this.page.getByText(status).first();
        await expect(badge).toBeVisible({ timeout });
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

        // Wait for settings tab content to load
        await expect(this.page.getByTestId('delete-monitor-trigger')).toBeVisible({ timeout: 5000 });

        // Click Delete
        await this.page.getByTestId('delete-monitor-trigger').click();

        // Wait for confirmation dialog
        await expect(this.page.getByTestId('delete-monitor-confirm')).toBeVisible({ timeout: 5000 });

        // Confirm deletion
        await this.page.getByTestId('delete-monitor-confirm').click();

        // Wait for sheet/dialog to close (this indicates the action completed)
        await expect(this.page.locator('[data-state="open"].fixed.inset-0')).toHaveCount(0, { timeout: 15000 });

        // Wait for React Query to refetch and update UI
        await this.page.waitForTimeout(1500);

        // Verify the monitor card is gone by checking the monitor list area specifically
        // Use a more targeted approach - wait for the element to be detached
        const monitorCard = this.page.locator('div.rounded-lg.bg-card').filter({ hasText: monitorName });
        await expect(monitorCard).toHaveCount(0, { timeout: 15000 });
    }

    async editMonitor(oldName: string, newName: string) {
        // Open Monitor Details
        await this.page.getByText(oldName).first().click();

        // Wait for sheet to open
        await expect(this.page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });

        // Click Settings Tab
        await this.page.getByTestId('monitor-settings-tab').click();

        // Wait for settings content to load
        await expect(this.page.getByTestId('monitor-edit-name-input')).toBeVisible({ timeout: 5000 });

        // Update Name
        await this.page.getByTestId('monitor-edit-name-input').fill(newName);

        // Click Save and wait for the PUT call and the subsequent GET refetch
        const putPromise = this.page.waitForResponse(
            resp => resp.url().includes('/api/monitors/') && resp.request().method() === 'PUT',
            { timeout: 10000 }
        );
        const refetchPromise = this.page.waitForResponse(
            resp => resp.url().includes('/api/uptime') && resp.request().method() === 'GET' && resp.status() === 200,
            { timeout: 10000 }
        );
        await this.page.getByTestId('monitor-edit-save-btn').click();
        await putPromise;
        await refetchPromise;

        // Wait for sheet to close
        await expect(this.page.locator('[data-state="open"].fixed.inset-0')).toHaveCount(0, { timeout: 10000 });

        // Verify the new name appears in the monitor list
        await expect(this.page.getByText(newName).first()).toBeVisible({ timeout: 10000 });
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

    async verifyMonitorPaused(monitorName: string) {
        // Verify the monitor card shows "Paused" badge
        const monitorCard = this.page.locator('div.rounded-lg.bg-card').filter({ hasText: monitorName });
        await expect(monitorCard.getByText('Paused')).toBeVisible({ timeout: 15000 });
    }

    async verifyMonitorOperational(monitorName: string) {
        // Verify the monitor card shows "Operational" badge
        const monitorCard = this.page.locator('div.rounded-lg.bg-card').filter({ hasText: monitorName });
        await expect(monitorCard.getByText('Operational')).toBeVisible({ timeout: 15000 });
    }

    async pauseMonitorViaSettings(monitorName: string) {
        // Open monitor details by clicking on the monitor card
        const monitorCard = this.page.locator('div.rounded-lg.bg-card').filter({ hasText: monitorName }).first();
        await monitorCard.click();

        // Wait for sheet to open
        await expect(this.page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });

        // Click Settings Tab
        await this.page.getByTestId('monitor-settings-tab').click();

        // Wait for settings tab content to fully load
        await expect(this.page.getByText('Monitor Status')).toBeVisible({ timeout: 5000 });

        // Wait for Pause Monitor button to be visible and enabled
        const pauseBtn = this.page.getByRole('button', { name: 'Pause Monitor' });
        await expect(pauseBtn).toBeVisible({ timeout: 10000 });
        await expect(pauseBtn).toBeEnabled({ timeout: 5000 });

        // Click the button
        await pauseBtn.click();

        // Instead of waiting for toast, verify the action succeeded by checking:
        // The button changes to "Resume Monitor" (indicating state changed)
        const resumeBtn = this.page.getByRole('button', { name: 'Resume Monitor' });
        await expect(resumeBtn).toBeVisible({ timeout: 15000 });

        // Close sheet via X button and wait for it to fully close
        await this.page.getByRole('button', { name: 'Close' }).click();
        await expect(this.page.locator('[data-state="open"].fixed.inset-0')).toHaveCount(0, { timeout: 5000 });

        // Wait for any pending React updates after sheet closes
        await this.page.waitForTimeout(500);
    }

    async resumeMonitorViaSettings(monitorName: string) {
        // Open monitor details by clicking on the monitor card
        const monitorCard = this.page.locator('div.rounded-lg.bg-card').filter({ hasText: monitorName }).first();
        await monitorCard.click();

        // Wait for sheet to open
        await expect(this.page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });

        // Click Settings Tab
        await this.page.getByTestId('monitor-settings-tab').click();

        // Wait for settings tab content to fully load
        await expect(this.page.getByText('Monitor Status')).toBeVisible({ timeout: 5000 });

        // Wait for Resume Monitor button to be visible and enabled
        const resumeBtn = this.page.getByRole('button', { name: 'Resume Monitor' });
        await expect(resumeBtn).toBeVisible({ timeout: 10000 });
        await expect(resumeBtn).toBeEnabled({ timeout: 5000 });

        // Click the button
        await resumeBtn.click();

        // Instead of waiting for toast, verify the action succeeded by checking:
        // 1. The button changes to "Pause Monitor" (indicating state changed)
        const pauseBtn = this.page.getByRole('button', { name: 'Pause Monitor' });
        await expect(pauseBtn).toBeVisible({ timeout: 15000 });

        // Close sheet via X button and wait for it to fully close
        await this.page.getByRole('button', { name: 'Close' }).click();
        await expect(this.page.locator('[data-state="open"].fixed.inset-0')).toHaveCount(0, { timeout: 5000 });

        // Wait for any pending React updates after sheet closes
        await this.page.waitForTimeout(500);
    }
}
