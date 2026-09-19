import { expect, test } from "@playwright/test";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";

test.describe("Maintenance lifecycle", () => {
    test("uses the configured timezone and clears maintenance everywhere after it expires", async ({ page }) => {
        test.setTimeout(60_000);
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);
        const suffix = Date.now();
        const groupName = `Maintenance lifecycle ${suffix}`;
        const monitorName = `Router ${suffix}`;

        await dashboard.goto();
        if (await login.isVisible()) await login.login();
        await dashboard.waitForLoad();

        await page.goto("/settings");
        await page.getByTestId("timezone-select").click();
        await page.getByPlaceholder("Search timezone...").fill("America/Bogota");
        await page.getByRole("option", { name: "America/Bogota" }).click();
        const timezoneResponse = page.waitForResponse(
            response => response.url().includes("/api/auth/me") && response.request().method() === "PATCH",
        );
        await page.getByRole("button", { name: "Save Changes" }).click();
        expect((await timezoneResponse).ok()).toBeTruthy();
        await page.reload();
        await expect(page.getByTestId("timezone-select")).toHaveText(/America\/Bogota/);

        await dashboard.goto();
        await dashboard.waitForLoad();
        const groupPath = await dashboard.createGroup(groupName);
        const groupId = groupPath.split("/").pop()!;
        await dashboard.createMonitor(monitorName, "https://example.com");

        const activeResponse = await page.request.post("/api/maintenance", {
            data: {
                title: `Active maintenance ${suffix}`,
                description: "E2E active lifecycle",
                startTime: new Date(Date.now() - 60_000).toISOString(),
                endTime: new Date(Date.now() + 10 * 60_000).toISOString(),
                affectedGroups: [groupId],
            },
        });
        expect(activeResponse.status()).toBe(201);
        const active = await activeResponse.json();

        const expiredResponse = await page.request.post("/api/maintenance", {
            data: {
                title: `Expired maintenance ${suffix}`,
                description: "E2E timezone and history",
                startTime: "2026-09-15T22:13:00Z",
                endTime: "2026-09-15T22:18:00Z",
                affectedGroups: [groupId],
            },
        });
        expect(expiredResponse.status()).toBe(201);
        const expired = await expiredResponse.json();

        await page.goto(groupPath);
        const monitorCard = page.locator('[data-testid^="monitor-card-"]').filter({ hasText: monitorName });
        await expect(monitorCard).toBeVisible();
        await expect(monitorCard.getByText("Maintenance", { exact: true })).toBeVisible();

        await page.request.patch("/api/status-pages/all", {
            data: { enabled: true, public: true, title: "Global Status" },
        });
        await page.goto("/status/all");
        await expect(page.getByText(active.title)).toBeVisible();
        await expect(page.getByText("System Under Maintenance")).toBeVisible();
        await expect(page.getByText(expired.title)).toHaveCount(0);

        await page.goto("/maintenance");
        await page.getByRole("tab", { name: "History" }).click();
        const expiredCard = page.getByTestId(`maintenance-card-${expired.id}`);
        await expect(expiredCard).toContainText("Sep 15, 2026, 05:13:00 PM GMT-05:00");
        await page.getByTestId(`maintenance-actions-${expired.id}`).click();
        await page.getByRole("menuitem", { name: "Delete" }).click();
        await page.getByRole("button", { name: "Delete" }).click();
        await expect(expiredCard).toHaveCount(0);

        await page.request.delete(`/api/maintenance/${active.id}`);
        await page.goto(groupPath);
        await expect(monitorCard.getByText("Maintenance", { exact: true })).toHaveCount(0);

        await page.goto("/status/all");
        await expect(page.getByText(active.title)).toHaveCount(0);
        await expect(page.getByText("System Under Maintenance")).toHaveCount(0);

        await page.request.patch("/api/status-pages/all", {
            data: { enabled: false, public: false, title: "Global Status" },
        });
        await page.request.delete(`/api/groups/${groupId}`);
    });
});
