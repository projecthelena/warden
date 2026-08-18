import { Page, Locator, expect } from '@playwright/test';

// AlertingSettingsPage drives the Notifications tab of Settings, where the sustained-alert
// ladder lives. The controls sit inside a collapsed accordion, so every helper opens its
// section first — a field that is merely present but collapsed is not something a user can
// set, and Playwright will happily fill one that nobody can see.
export class AlertingSettingsPage {
    readonly page: Page;
    readonly sustained: Locator;
    readonly reminder: Locator;
    readonly repeat: Locator;
    readonly save: Locator;

    constructor(page: Page) {
        this.page = page;
        this.sustained = page.getByTestId('alert-sustained');
        this.reminder = page.getByTestId('alert-reminder');
        this.repeat = page.getByTestId('alert-repeat');
        this.save = page.getByTestId('save-notification-settings');
    }

    async goto() {
        await this.page.goto('/settings?tab=notifications');
        await expect(this.page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });
        await expect(this.save).toBeVisible({ timeout: 10000 });
    }

    // The accordion is `type="multiple"`, so opening one section leaves the others alone.
    // Clicking an already-open section would close it, hence the visibility check.
    async openSection(title: string, marker: Locator) {
        if (await marker.isVisible()) return;
        await this.page.getByRole('button', { name: title }).click();
        await expect(marker).toBeVisible({ timeout: 5000 });
    }

    async openLadder() {
        await this.openSection('Alerting Thresholds', this.page.getByTestId('alert-ladder'));
    }

    async openLatency() {
        await this.openSection('High Latency', this.page.getByTestId('adaptive-latency-switch'));
    }

    async openDigest() {
        await this.openSection('Daily Digest', this.page.getByTestId('digest-enabled'));
    }

    async readLadder() {
        await this.openLadder();
        return {
            sustained: await this.sustained.inputValue(),
            reminder: await this.reminder.inputValue(),
            repeat: await this.repeat.inputValue(),
        };
    }

    async setLadder(sustained: string, reminder: string, repeat: string) {
        await this.openLadder();
        await this.sustained.fill(sustained);
        await this.reminder.fill(reminder);
        await this.repeat.fill(repeat);
    }

    // Saving is a PATCH; waiting on the response rather than a toast keeps the assertion
    // about the thing that actually persisted.
    async saveAndWait() {
        const patched = this.page.waitForResponse(
            resp => resp.url().includes('/api/settings') && resp.request().method() === 'PATCH',
        );
        await this.save.click();
        const resp = await patched;
        return resp.status();
    }
}
