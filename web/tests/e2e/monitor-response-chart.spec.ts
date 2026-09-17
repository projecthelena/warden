import { expect, test } from "@playwright/test";
import { API_BASE } from "../apiBase";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";

test.describe.configure({ mode: "serial" });

test("response chart separates missing data and failed checks", async ({ page }) => {
    const dashboard = new DashboardPage(page);
    const login = new LoginPage(page);
    await dashboard.goto();
    if (await login.isVisible()) await login.login();
    await dashboard.waitForLoad();

    const groupName = `Chart Group ${Date.now()}`;
    const monitorName = `Chart Monitor ${Date.now()}`;
    await dashboard.createGroup(groupName);
    await dashboard.createMonitor(monitorName, `${API_BASE}/healthz`);

    const uptime = await page.request.get("/api/uptime");
    expect(uptime.ok()).toBeTruthy();
    const payload = await uptime.json();
    const monitor = payload.groups
        .flatMap((group: { monitors?: Array<{ id: string; name: string }> }) => group.monitors ?? [])
        .find((item: { name: string }) => item.name === monitorName);
    expect(monitor).toBeTruthy();

    const base = Date.UTC(2026, 8, 17, 6);
    await page.route(`**/api/monitors/${monitor.id}/latency?range=*`, route => route.fulfill({
        contentType: "application/json",
        body: JSON.stringify([
            { timestamp: new Date(base).toISOString(), latency: 120, totalChecks: 60, successfulChecks: 60, failedChecks: 0, state: "up" },
            { timestamp: new Date(base + 3_600_000).toISOString(), latency: 140, totalChecks: 60, successfulChecks: 60, failedChecks: 0, state: "up" },
            { timestamp: new Date(base + 7_200_000).toISOString(), latency: null, totalChecks: 0, successfulChecks: 0, failedChecks: 0, state: "no_data" },
            { timestamp: new Date(base + 10_800_000).toISOString(), latency: null, totalChecks: 4, successfulChecks: 0, failedChecks: 4, state: "down" },
            { timestamp: new Date(base + 14_400_000).toISOString(), latency: 180, totalChecks: 60, successfulChecks: 58, failedChecks: 2, state: "mixed" },
            { timestamp: new Date(base + 18_000_000).toISOString(), latency: 160, totalChecks: 60, successfulChecks: 60, failedChecks: 0, state: "up" },
        ]),
    }));

    await page.goto(`/monitors/${monitor.id}`);
    const chart = page.getByTestId("response-time-chart");
    await expect(chart).toBeVisible();
    await expect(chart).toHaveAttribute("data-motion", "animated");
    await expect(page.getByLabel("Chart legend")).toContainText("Failed checks");
    await expect(page.getByLabel("Chart legend")).toContainText("No data");

    await expect(chart.locator(".recharts-reference-area-rect")).toHaveCount(3);
    await page.waitForTimeout(600);
    const curve = chart.locator(".recharts-area-curve");
    const path = await curve.getAttribute("d");
    expect(path?.match(/M/g)?.length ?? 0).toBeGreaterThan(1);

    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width: 320, height: 800 });
    await page.reload();
    await expect(page.getByTestId("response-time-chart")).toHaveAttribute("data-motion", "reduced");
    const viewportFits = await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);
    expect(viewportFits).toBeTruthy();

    await dashboard.deleteMonitor(monitorName);
    await dashboard.deleteGroup(groupName);
});
