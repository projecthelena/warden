import { expect, test } from "@playwright/test";
import { API_BASE } from "../apiBase";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";

test("patterns tab supports navigation, reload, mobile, empty and error states", async ({ page }) => {
    const dashboard = new DashboardPage(page);
    await dashboard.goto();
    const login = new LoginPage(page);
    if (await login.isVisible()) await login.login();
    await dashboard.waitForLoad();
    const groupName = `Patterns Group ${Date.now()}`;
    const monitorName = `Patterns Monitor ${Date.now()}`;
    await dashboard.createGroup(groupName);
    await dashboard.createMonitor(monitorName, `${API_BASE}/healthz`);
    try {
        const response = await page.request.get("/api/uptime");
        expect(response.ok()).toBeTruthy();
        const payload = await response.json();
        const monitor = payload.groups.flatMap((group: { monitors?: { id: string; name: string }[] }) => group.monitors ?? [])
            .find((item: { name: string }) => item.name === monitorName);
        expect(monitor).toBeTruthy();
        let requests = 0;
        let mode = "findings";
        await page.route(`**/api/monitors/${monitor.id}/insights`, route => {
            requests++;
            return route.fulfill({
                status: mode === "error" ? 500 : 200,
                contentType: "application/json",
                body: JSON.stringify(mode === "findings" ? [{
                    id: 1, monitorId: monitor.id, monitorName, kind: "time_of_day",
                    summary: "Failures concentrate in the evening.", confidence: "medium",
                    detectedAt: "2026-09-01T00:00:00Z",
                }] : []),
            });
        });
        await page.goto(`/monitors/${monitor.id}?date=2026-09-01`);
        await expect(page.getByTestId("response-time-chart")).toBeVisible();
        expect(requests).toBe(0);
        await expect(page.getByTestId("monitor-insights")).toHaveCount(0);
        await page.getByRole("tab", { name: "Patterns", exact: true }).click();
        await expect(page).toHaveURL(/date=2026-09-01&tab=patterns/);
        await expect(page.getByText("Failures concentrate in the evening.")).toBeVisible();
        await expect(page.getByText("worth a look")).toBeVisible();
        await expect(page.getByTestId("response-time-chart")).toHaveCount(0);
        await page.reload();
        await expect(page.getByRole("tab", { name: "Patterns", exact: true })).toHaveAttribute("aria-selected", "true");
        await expect(page.getByText("Failures concentrate in the evening.")).toBeVisible();
        await page.setViewportSize({ width: 320, height: 800 });
        for (const name of ["Overview", "Incidents", "Patterns", "Checks", "Settings"]) {
            await expect(page.getByRole("tab", { name, exact: true })).toBeInViewport();
        }
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
        await page.getByRole("tab", { name: "Overview", exact: true }).click();
        await expect(page).not.toHaveURL(/tab=patterns/);
        await expect(page.getByTestId("monitor-insights")).toHaveCount(0);
        await page.getByRole("tab", { name: "Incidents", exact: true }).click();
        await expect(page.getByRole("button", { name: "All monitor incidents" })).toBeVisible();
        mode = "empty";
        await page.goto(`/monitors/${monitor.id}?tab=patterns`);
        await expect(page.getByText("No patterns detected yet.")).toBeVisible();
        mode = "error";
        await page.reload();
        await expect(page.getByRole("alert").filter({ hasText: "Could not load patterns" })).toBeVisible({ timeout: 15000 });
        await expect(page.getByText("No patterns detected yet.")).toHaveCount(0);
    } finally {
        await page.setViewportSize({ width: 1280, height: 800 });
        await dashboard.deleteMonitor(monitorName);
        await dashboard.deleteGroup(groupName);
    }
});
