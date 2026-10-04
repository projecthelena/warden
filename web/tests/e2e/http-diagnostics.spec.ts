import { createServer, type Server } from "node:http";
import { expect, test } from "@playwright/test";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";

// Real target, scheduler, database and browser: no API response interception.
for (const scenario of [
  {
    name: "recovers",
    code: 503,
    recover: true,
    attempts: 2,
    title: "Recovered after 1 retry",
  },
  {
    name: "permanent",
    code: 401,
    recover: false,
    attempts: 1,
    title: "Received HTTP 401",
  },
  {
    name: "retry-after",
    code: 503,
    recover: false,
    attempts: 1,
    title: "Received HTTP 503",
    retryAfter: "30",
  },
  {
    name: "unsafe-post",
    code: 503,
    recover: false,
    attempts: 1,
    title: "Received HTTP 503",
    method: "POST",
  },
  {
    name: "persistent",
    code: 503,
    recover: false,
    attempts: 2,
    title: "Received HTTP 503",
  },
]) {
  test(`HTTP evidence: ${scenario.name}`, async ({ page }) => {
    test.setTimeout(60_000);
    let calls = 0;
    const target: Server = createServer((_req, res) => {
      calls++;
      if ("retryAfter" in scenario)
        res.setHeader("Retry-After", scenario.retryAfter!);
      res.writeHead(scenario.recover && calls > 1 ? 204 : scenario.code);
      res.end();
    });
    await new Promise<void>((resolve) =>
      target.listen(0, "127.0.0.1", resolve),
    );
    const address = target.address();
    if (!address || typeof address === "string")
      throw new Error("Missing target address");
    const dashboard = new DashboardPage(page);
    let monitorId = "";
    let groupId = "";
    try {
      await dashboard.goto();
      const login = new LoginPage(page);
      if (await login.isVisible()) await login.login();
      await dashboard.waitForLoad();
      const suffix = `${scenario.name}-${Date.now()}`;
      const group = await page.request.post("/api/groups", {
        data: { name: `Evidence ${suffix}` },
      });
      expect(group.ok()).toBeTruthy();
      groupId = (await group.json()).id;
      const created = await page.request.post("/api/monitors", {
        data: {
          name: `Evidence ${suffix}`,
          groupId,
          url: `http://127.0.0.1:${address.port}/check`,
          interval: 3600,
          confirmationThreshold: 1,
          ...("method" in scenario
            ? { requestConfig: { autoRetry: true, method: scenario.method } }
            : {}),
        },
      });
      expect(created.ok()).toBeTruthy();
      const monitor = await created.json();
      monitorId = monitor.id;
      expect(monitor.requestConfig.autoRetry).toBe(true);
      await expect
        .poll(
          async () => {
            const response = await page.request.get(
              `/api/monitors/${monitorId}/checks`,
            );
            expect(response.ok()).toBeTruthy();
            return (await response.json()).length;
          },
          { timeout: 20_000 },
        )
        .toBe(1);
      const checks = await (
        await page.request.get(`/api/monitors/${monitorId}/checks`)
      ).json();
      expect(checks[0].status).toBe(scenario.recover ? "up" : "down");
      expect(checks[0].diagnostics.attempts).toHaveLength(scenario.attempts);
      expect(calls).toBe(scenario.attempts);
      await page.goto(`/monitors/${monitorId}`);
      await page.getByRole("tab", { name: "Checks", exact: true }).click();
      const history = page.getByRole("region", { name: "Recent checks" });
      await expect(history.getByLabel("24-hour totals")).toHaveText(`Checks1Failed${scenario.recover ? 0 : 1}Recovered${scenario.recover ? 1 : 0}`);
      await history
        .locator("summary")
        .filter({ has: page.locator(".check-result") }).first()
        .click();
      if (scenario.name === "recovers") {
        for (const width of [320, 375, 414, 768, 1440]) {
          await page.setViewportSize({ width, height: 1000 });
          const tabs = page.getByRole("tablist");
          await expect(tabs.getByRole("tab")).toHaveCount(5);
          const geometry = await tabs.evaluate(element => {
            const box = element.getBoundingClientRect();
            const children = [...element.querySelectorAll('[role="tab"]')].map(tab => tab.getBoundingClientRect());
            return { aligned: children.every(child => Math.abs(child.top - children[0].top) < 1), contained: children.every(child => child.left >= box.left && child.right <= box.right + 1 && child.bottom <= box.bottom + 1), touch: children.every(child => child.height >= 44) };
          });
          expect(geometry, `Tabs at ${width}px`).toEqual({ aligned: true, contained: true, touch: true });
          if (width >= 768) {
            const tabBox = await tabs.boundingBox();
            const workspace = await page.getByTestId("monitor-page").boundingBox();
            expect(Math.abs(tabBox!.x - workspace!.x)).toBeLessThan(1);
            expect(Math.abs(tabBox!.width - workspace!.width)).toBeLessThan(1);
          }
          expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
          const breadcrumb = page.getByRole("navigation", { name: "breadcrumb" });
          expect(await breadcrumb.evaluate(element => {
            const box = element.getBoundingClientRect();
            const header = element.closest("header")!.getBoundingClientRect();
            return box.top >= header.top && box.bottom <= header.bottom;
          }), `Breadcrumb at ${width}px`).toBe(true);
          await expect(history.getByText(/^Between attempts \d/)).toBeVisible();
          await expect(history.getByText("Retry mode: automatic")).not.toBeVisible();
        }
        await page.setViewportSize({ width: 1280, height: 900 });
      }
      await history.getByText("Technical details", { exact: true }).click();
      const details = page.getByRole("region", { name: "HTTP diagnostics" });
      await expect(details).toContainText("Attempt 1");
      await expect(details).toContainText(`HTTP ${scenario.code}`);
      if (scenario.attempts === 2)
        await expect(details).toContainText("Attempt 2");
      else await expect(details).not.toContainText("Attempt 2");
      if (scenario.recover) await expect(details).toContainText("HTTP 204");
      await expect(details).toContainText("127.0.0.1");
      await expect(details).toContainText("not backend execution time");
      if (scenario.name === "recovers") {
        // Exercise the viewer tab layout independently of backend authorization tests.
        await page.route("**/api/auth/me", async route => {
          const response = await route.fetch();
          const body = await response.json();
          await route.fulfill({ response, json: { ...body, user: { ...body.user, role: "viewer" } } });
        });
        await page.reload();
        for (const width of [320, 375, 414, 768, 1440]) {
          await page.setViewportSize({ width, height: 1000 });
          const tabs = page.getByRole("tablist");
          await expect(tabs.getByRole("tab")).toHaveCount(4);
          await expect(page.getByRole("tab", { name: "Settings", exact: true })).toHaveCount(0);
          expect(await tabs.evaluate(element => {
            const box = element.getBoundingClientRect();
            const children = [...element.querySelectorAll('[role="tab"]')].map(tab => tab.getBoundingClientRect());
            return children.every(child => Math.abs(child.top - children[0].top) < 1 && child.left >= box.left && child.right <= box.right + 1 && child.bottom <= box.bottom + 1);
          }), `Viewer tabs at ${width}px`).toBe(true);
        }
      }
    } finally {
      try {
        if (monitorId)
          expect(
            (await page.request.delete(`/api/monitors/${monitorId}`)).ok(),
          ).toBeTruthy();
        if (groupId)
          expect(
            (await page.request.delete(`/api/groups/${groupId}`)).ok(),
          ).toBeTruthy();
      } finally {
        target.closeAllConnections();
        await new Promise<void>((resolve, reject) =>
          target.close((err) => (err ? reject(err) : resolve())),
        );
      }
    }
  });
}
