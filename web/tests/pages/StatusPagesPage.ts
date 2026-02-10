import { Page, Locator, expect } from '@playwright/test';

export class StatusPagesPage {
    readonly page: Page;

    constructor(page: Page) {
        this.page = page;
    }

    async goto() {
        await this.page.goto('/status-pages');
        await this.page.waitForLoadState('networkidle');
    }

    /** Navigate to Status Pages via sidebar */
    async navigateViaSidebar() {
        await this.page.getByRole('link', { name: 'Status Pages' }).click();
        await expect(this.page).toHaveURL(/.*status-pages/);
    }

    /** Get the row locator for a specific status page by slug */
    getRow(slug: string): Locator {
        return this.page.getByTestId(`status-page-row-${slug}`);
    }

    /** Get the badge locator for a specific status page */
    getBadge(slug: string): Locator {
        return this.page.getByTestId(`status-page-badge-${slug}`);
    }

    /** Get the enabled toggle for a specific status page */
    getEnabledToggle(slug: string): Locator {
        return this.page.getByTestId(`status-page-enabled-toggle-${slug}`);
    }

    /** Get the public toggle for a specific status page */
    getPublicToggle(slug: string): Locator {
        return this.page.getByTestId(`status-page-public-toggle-${slug}`);
    }

    /** Get the "Visit Page" link for a specific status page */
    getVisitLink(slug: string): Locator {
        return this.page.getByTestId(`status-page-visit-${slug}`);
    }

    /** Toggle the enabled state of a status page and wait for the toast */
    async toggleEnabled(slug: string) {
        await this.getEnabledToggle(slug).click();
        await this.waitForToast();
    }

    /** Toggle the public state of a status page and wait for the toast */
    async togglePublic(slug: string) {
        await this.getPublicToggle(slug).click();
        await this.waitForToast();
    }

    /** Wait for the "Status Page Updated" toast to appear and then dismiss */
    async waitForToast() {
        const toast = this.page.getByTestId('toast-title').first();
        await expect(toast).toBeVisible({ timeout: 5000 });
        // Wait for toast to disappear so subsequent toasts can be detected
        await expect(toast).toHaveCount(0, { timeout: 10000 });
    }

    /** Assert the badge text for a status page */
    async expectBadge(slug: string, text: 'Disabled' | 'Public' | 'Private') {
        await expect(this.getBadge(slug)).toHaveText(text, { timeout: 5000 });
    }

    /** Assert the "Visit Page" link is visible */
    async expectVisitLinkVisible(slug: string) {
        await expect(this.getVisitLink(slug)).toBeVisible({ timeout: 5000 });
    }

    /** Assert the "Visit Page" link is NOT visible */
    async expectVisitLinkHidden(slug: string) {
        await expect(this.getVisitLink(slug)).toHaveCount(0, { timeout: 5000 });
    }

    /** Assert the public toggle is disabled (not clickable) */
    async expectPublicToggleDisabled(slug: string) {
        await expect(this.getPublicToggle(slug)).toBeDisabled({ timeout: 5000 });
    }

    /** Assert the public toggle is enabled (clickable) */
    async expectPublicToggleEnabled(slug: string) {
        await expect(this.getPublicToggle(slug)).toBeEnabled({ timeout: 5000 });
    }

    /** Wait for the status pages to load */
    async waitForLoad() {
        await expect(this.page.getByRole('heading', { name: 'Status Pages' })).toBeVisible({ timeout: 10000 });
        // Wait for at least one row to appear (Global Status is always present)
        await expect(this.getRow('all')).toBeVisible({ timeout: 10000 });
    }

    /** Reset the "all" status page to disabled+private via API */
    async resetToDefaults() {
        await this.page.request.patch('/api/status-pages/all', {
            data: { enabled: false, public: false, title: 'Global Status' }
        });
    }
}
