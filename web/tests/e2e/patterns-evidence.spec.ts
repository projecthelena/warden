import { createServer } from "node:http";
import { expect, test } from "@playwright/test";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";

// Historical pattern fixtures; check collection, persistence and exact-check navigation are real.
for (const traced of [true, false]) {
  test(`compact patterns and exact evidence ${traced ? "with" : "without"} traces`, async ({
    page,
  }) => {
    test.setTimeout(60_000);
    const target = createServer((_request, response) => {
      response.writeHead(503);
      response.end();
    });
    await new Promise<void>((resolve) =>
      target.listen(0, "127.0.0.1", resolve),
    );
    const address = target.address();
    if (!address || typeof address === "string")
      throw new Error("Missing target address");
    let groupId = "";
    try {
      const setupStatus = await page.request.get("/api/setup/status");
      if (!(await setupStatus.json()).isSetup) {
        const setup = await page.request.post("/api/setup", {
          headers: {
            "X-Admin-Secret":
              process.env.ADMIN_SECRET || "warden-e2e-magic-key",
          },
          data: {
            username: "admin",
            password: "password123!",
            timezone: "UTC",
          },
        });
        expect(setup.ok()).toBeTruthy();
      }
      const dashboard = new DashboardPage(page);
      await dashboard.goto();
      const login = new LoginPage(page);
      if (await login.isVisible()) await login.login();
      await dashboard.waitForLoad();
      const group = await page.request.post("/api/groups", {
        data: { name: `Patterns evidence ${Date.now()}` },
      });
      expect(group.ok()).toBeTruthy();
      groupId = (await group.json()).id;
      const ids: string[] = [];
      for (const name of ["Main", "Related 01"]) {
        const created = await page.request.post("/api/monitors", {
          data: {
            name,
            groupId,
            url: `http://127.0.0.1:${address.port}/check`,
            interval: 3600,
            confirmationThreshold: 1,
            requestConfig: { method: "GET", autoRetry: traced },
          },
        });
        expect(created.ok()).toBeTruthy();
        ids.push((await created.json()).id);
      }
      const checks = [];
      for (const id of ids) {
        await expect
          .poll(
            async () =>
              (
                await (
                  await page.request.get(`/api/monitors/${id}/checks`)
                ).json()
              ).length,
            { timeout: 20_000 },
          )
          .toBe(1);
        const check = (
          await (await page.request.get(`/api/monitors/${id}/checks`)).json()
        )[0];
        expect(Boolean(check.diagnostics)).toBe(traced);
        checks.push(check);
      }
      let coverage = traced
        ? { total: 10, traced: 10 }
        : { total: 10, traced: 0 };
      await page.route(`**/api/monitors/${ids[0]}/insights`, (route) =>
        route.fulfill({
          json: Array.from({ length: 40 }, (_, index) => ({
            id: index + 1,
            monitorId: ids[0],
            monitorName: "Main",
            kind: "co_failure",
            confidence: "medium",
            summary: "Repeated simultaneous failures.",
            detectedAt: new Date().toISOString(),
            traceCapture: traced ? "enabled" : "disabled",
            detail: {
              withMonitorId: index === 0 ? ids[1] : `fixture-${index}`,
              withMonitorName: `Related ${String(index + 1).padStart(2, "0")}`,
              matchedOutages: 3,
              overlap: 100,
              reverseOverlap: 100,
              checkEvidence: coverage,
              evidenceCheck: { id: checks[0].id },
              withCheck: index === 0 ? { id: checks[1].id } : undefined,
            },
          })),
        }),
      );
      await page.goto(`/monitors/${ids[0]}?tab=patterns`);
      await expect(
        page.getByText("Overlapping outages with 40 monitors"),
      ).toBeVisible();
      await expect(
        page.getByText(
          traced
            ? "All 10 checks in this window have traces."
            : "No traces were recorded in this window",
          { exact: false },
        ),
      ).toBeVisible();
      await page.getByText("View related monitors", { exact: true }).click();
      await expect(page.getByTestId("related-monitor-row")).toHaveCount(5);
      await page.getByRole("button", { name: "Next", exact: true }).click();
      await expect(page.getByText("6–10 of 40", { exact: true })).toBeVisible();
      await page
        .getByRole("searchbox", { name: "Find a related monitor" })
        .fill("Related 40");
      await expect(page.getByTestId("related-monitor-row")).toHaveCount(1);
      await page.getByRole("searchbox").fill("missing");
      await expect(page.getByText("No matching monitors.")).toBeVisible();
      await page.getByRole("searchbox").fill("");
      await page.setViewportSize({ width: 320, height: 800 });
      await expect(page.getByTestId("related-monitor-row")).toHaveCount(5);
      expect(
        await page.evaluate(
          () =>
            document.documentElement.scrollWidth <=
            document.documentElement.clientWidth,
        ),
      ).toBe(true);
      const response = page.waitForResponse((res) =>
        res
          .url()
          .includes(`/monitors/${ids[1]}/checks?checkId=${checks[1].id}`),
      );
      await page
        .getByRole("link", { name: "View check for Related 01", exact: true })
        .click();
      const selected = await (await response).json();
      expect(selected.map((check: { id: number }) => check.id)).toEqual([
        checks[1].id,
      ]);
      await expect(page.getByTestId("selected-check")).toHaveCount(1);
      await expect(page.getByTestId("selected-check")).toHaveAttribute(
        "open",
        "",
      );
      await expect(page.getByText("Last 24 hours")).toHaveCount(0);
      if (!traced)
        await expect(
          page.getByText("No HTTP trace was recorded for this check."),
        ).toBeVisible();
      await page.reload();
      await expect(page.getByTestId("selected-check")).toHaveCount(1);
      await page.goto(`/monitors/${ids[0]}?tab=patterns`);
      await page
        .getByRole("link", { name: "View example check", exact: true })
        .click();
      await expect(page).toHaveURL(new RegExp(`checkId=${checks[0].id}$`));
      await expect(page.getByTestId("selected-check")).toHaveCount(1);
      coverage = { total: 10, traced: 3 };
      await page.goto(`/monitors/${ids[0]}?tab=patterns`);
      await expect(
        page.getByText("Partial history: 3 of 10 checks have traces.", {
          exact: false,
        }),
      ).toBeVisible();
      await page.goto(`/monitors/${ids[0]}?tab=checks&checkId=999999999`);
      await expect(
        page.getByText("This check is no longer available.", { exact: false }),
      ).toBeVisible();
      await expect(page.getByTestId("selected-check")).toHaveCount(0);
      await page
        .getByRole("link", { name: "All check history", exact: true })
        .click();
      await expect(page.getByText("Last 24 hours")).toBeVisible();
      // Go serializes nil trace collections as null, including early request failures.
      const errors: string[] = [];
      page.on("pageerror", error => errors.push(error.message));
      await page.route(`**/api/monitors/${ids[0]}/checks?checkId=${checks[0].id}`, route => route.fulfill({ json: [{
        ...checks[0], diagnostics: { attempts: [{ status: "down", failurePhase: "request", totalMs: 0, hops: null }] },
      }] }));
      await page.goto(`/monitors/${ids[0]}?tab=checks&checkId=${checks[0].id}`);
      await expect(page.getByText("Check configuration needs attention", { exact: true })).toBeVisible();
      await expect(page.getByRole("region", { name: "HTTP diagnostics" })).toBeVisible();
      expect(errors).toEqual([]);

    } finally {
      if (groupId)
        expect(
          (await page.request.delete(`/api/groups/${groupId}`)).ok(),
        ).toBeTruthy();
      await new Promise<void>((resolve, reject) =>
        target.close((error) => (error ? reject(error) : resolve())),
      );
    }
  });
}
