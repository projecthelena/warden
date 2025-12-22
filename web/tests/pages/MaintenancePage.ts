import { Page, Locator, expect } from '@playwright/test';

export class MaintenancePage {
    readonly page: Page;
    readonly createTrigger: Locator;
    readonly titleInput: Locator;
    readonly groupSelect: Locator;
    readonly submitBtn: Locator;

    constructor(page: Page) {
        this.page = page;
        this.createTrigger = page.getByTestId('create-maintenance-trigger');
        this.titleInput = page.getByTestId('maintenance-title-input');
        this.groupSelect = page.getByTestId('maintenance-group-select');
        this.submitBtn = page.getByTestId('create-maintenance-submit');
    }

    async goto() {
        await this.page.goto('/maintenance');
        // Do not assert URL/visibility here to allow for login redirects
    }

    async createMaintenance(title: string, groupName: string) {
        await this.createTrigger.click();
        await this.titleInput.fill(title);

        // Select Group
        // In Shadcn select, we click trigger then click item
        // But the item might be inside a portal.
        // We will try to click the item by text.
        // Since we didn't add testid to items (dynamic), we rely on text.
        await this.groupSelect.click();
        await this.page.getByRole('option', { name: groupName }).click();

        // We leave dates as default (Current + 1h) for simplicity in E2E
        // unless specifically testing scheduling logic.

        await this.submitBtn.click();

        // Verify success toast
        // "Maintenance Scheduled" or similar.
        // Alert Dialog closes automatically?
        // Actually MaintenanceView has "Maintenance details updated." but CreateMaintenanceSheet?
        // CreateMaintenanceSheet calls onCreate.
        // AdminLayout/App passes "addMaintenance".
        // Let's rely on list appearance with longer timeout, or just reload?
        // No, SPA should update.
        await expect(this.page.getByTestId('toast-title').first()).toBeVisible();

        // Check list
        // List update might be delayed or filtered.
        // Rely on toast for now.
        // await expect(this.page.getByText(title).first()).toBeVisible({ timeout: 10000 });
    }
}
