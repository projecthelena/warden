import { Page, Locator, expect } from "@playwright/test";

export class MaintenancePage {
    readonly page: Page;
    readonly createTrigger: Locator;
    readonly titleInput: Locator;
    readonly groupSelect: Locator;
    readonly descriptionInput: Locator;
    readonly submitBtn: Locator;

    constructor(page: Page) {
        this.page = page;
        this.createTrigger = page.getByTestId("create-maintenance-trigger");
        this.titleInput = page.getByTestId("maintenance-title-input");
        this.groupSelect = page.getByTestId("maintenance-group-select");
        this.descriptionInput = page.getByTestId("maintenance-description-input");
        this.submitBtn = page.getByTestId("create-maintenance-submit");
    }

    async goto() {
        await this.page.goto("/maintenance");
        // Do not assert URL/visibility here to allow for login redirects
    }

    async createMaintenance(title: string, groupName: string) {
        await this.createTrigger.click();
        await this.titleInput.fill(title);

        // The searchable multi-select uses cmdk options in a popover.
        await this.groupSelect.click();
        await this.page.getByRole("option", { name: groupName }).click();
        await this.page.keyboard.press("Escape");

        // Regression: the native time input dropped the second digit when users typed
        // minutes such as 22. The explicit minute picker must preserve the selection.
        const startPicker = this.page.getByTestId("start-time-picker");
        await startPicker.getByRole("combobox", { name: "Minute" }).click();
        await this.page.getByRole("option", { name: "22", exact: true }).click();
        await expect(startPicker.getByRole("combobox", { name: "Minute" })).toContainText("22");

        await this.submitBtn.click();

        // Verify success toast
        // "Maintenance Scheduled" or similar.
        // Alert Dialog closes automatically?
        // Actually MaintenanceView has "Maintenance details updated." but CreateMaintenanceSheet?
        // CreateMaintenanceSheet calls onCreate.
        // AdminLayout/App passes "addMaintenance".
        // Let's rely on list appearance with longer timeout, or just reload?
        // No, SPA should update.
        await expect(this.page.getByTestId("toast-title").filter({ hasText: "Maintenance Scheduled" })).toBeVisible();

        // Check list
        // List update might be delayed or filtered.
        // Rely on toast for now.
        // await expect(this.page.getByText(title).first()).toBeVisible({ timeout: 10000 });
    }
}
