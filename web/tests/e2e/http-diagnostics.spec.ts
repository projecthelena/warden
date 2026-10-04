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
      await expect(history).toContainText(
        `1 checks · ${scenario.recover ? 0 : 1} failed · ${scenario.recover ? 1 : 0} recovered after retry`,
      );
      await history
        .locator("summary")
        .filter({ hasText: scenario.title })
        .click();
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
