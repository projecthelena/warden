import { expect, test } from "@playwright/test";
import { API_BASE } from "../apiBase";
import { LoginPage } from "../pages/LoginPage";
import { StatusPagesPage } from "../pages/StatusPagesPage";

const config = {
  description: "Public service health",
  logoUrl: "",
  faviconUrl: "",
  accentColor: "",
  theme: "dark",
  showUptimeBars: true,
  showUptimePercentage: true,
  showIncidentHistory: true,
  uptimeDaysRange: 90,
  headerContent: "title-only",
  headerAlignment: "center",
  headerArrangement: "inline",
  timezone: "UTC",
};

test("loads compact status first and service details on demand without mobile overflow", async ({ page }) => {
  let detailRequests = 0;
  await page.route("**/api/setup/status", (route) => route.fulfill({ json: { isSetup: true } }));
  await page.route("**/api/auth/me", (route) => route.fulfill({ status: 401, json: { error: "unauthenticated" } }));
  await page.route("**/api/s/all*", async (route) => {
    const url = new URL(route.request().url());
    if (url.searchParams.get("group") === "core") {
      detailRequests++;
      const pageNumber = Number(url.searchParams.get("page") || 1);
      const pageSize = Number(url.searchParams.get("page_size") || 25);
      const offset = (pageNumber - 1) * pageSize;
      await route.fulfill({
        json: {
          title: "Global Status",
          groups: [{
            id: "core",
            name: "Core services",
            status: "up",
            monitorCount: 1000,
            counts: { up: 1000, degraded: 0, down: 0, paused: 0 },
            monitors: Array.from({ length: pageSize }, (_, index) => ({
              id: `monitor-${offset + index}`,
              name: `Monitor ${offset + index}`,
              status: "up",
              uptimeDays: [],
              uptime: { percent: 100, totalChecks: 0, downChecks: 0, downtimeSeconds: 0 },
              checkIntervalSeconds: 10,
            })),
          }],
          incidents: [],
          pastIncidents: [],
          config,
          counts: { all: 1000, operational: 998, issues: 1, paused: 1 },
          pagination: { page: pageNumber, pageSize, total: 1000, totalPages: Math.ceil(1000 / pageSize) },
        },
      });
      return;
    }

    await route.fulfill({
      json: {
        title: "Global Status",
        groups: [{ id: "core", name: "Core services", status: "up", monitorCount: 1000, counts: { up: 1000, degraded: 0, down: 0, paused: 0 }, monitors: [] }],
        incidents: [],
        pastIncidents: [],
        config,
      },
    });
  });

  await page.goto("/status/all");
  await expect(page.getByText("1000", { exact: true }).first()).toBeVisible();
  expect(detailRequests).toBe(0);

  await page.getByRole("tab", { name: "Services" }).click();
  await page.getByRole("button", { name: /Core services/ }).click();
  await expect(page.getByText("Monitor 0", { exact: true })).toBeVisible();
  expect(detailRequests).toBe(1);
  await expect(page.getByText("Monitor 24", { exact: true })).toBeVisible();
  await expect(page.getByText("Monitor 25", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByText("Monitor 25", { exact: true })).toBeVisible();
  expect(detailRequests).toBe(2);

  for (const width of [320, 375, 414, 768]) {
    await page.setViewportSize({ width, height: 900 });
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
    expect(overflow, `horizontal overflow at ${width}px`).toBe(false);
  }
});

test("paginates a real public status group with more than 100 monitors", async ({ page }) => {
  const login = new LoginPage(page);
  await page.goto("/");
  if (!page.url().includes("/dashboard")) await login.login();

  const stamp = Date.now();
  const groupName = `Status Scale ${stamp}`;
  const groupResponse = await page.request.post("/api/groups", {
    data: { name: groupName },
  });
  expect(groupResponse.status()).toBe(201);
  const group = await groupResponse.json() as { id: string };
  const statusPages = new StatusPagesPage(page);

  try {
    const responses = await Promise.all(Array.from({ length: 105 }, (_, index) =>
      page.request.post("/api/monitors", {
        data: {
          name: `Status Monitor ${String(index + 1).padStart(3, "0")} ${stamp}`,
          type: "http",
          url: `${API_BASE}/healthz?status-scale=${index + 1}`,
          groupId: group.id,
          interval: 60,
        },
      }),
    ));
    expect(responses.every((response) => response.status() === 201)).toBeTruthy();
    const pausedResponse = await page.request.post("/api/monitors", {
      data: {
        name: `Paused Status Monitor ${stamp}`,
        type: "http",
        url: `${API_BASE}/healthz?status-scale=paused`,
        groupId: group.id,
        interval: 60,
      },
    });
    expect(pausedResponse.status()).toBe(201);
    const paused = await pausedResponse.json() as { id: string };
    expect((await page.request.post(`/api/monitors/${paused.id}/pause`)).ok()).toBeTruthy();

    await statusPages.enablePublicViaAPI("all");
    await page.goto("/status/all");
    await page.getByRole("tab", { name: "Services" }).click();
    await page.getByRole("button", { name: new RegExp(groupName) }).click();

    const monitorRows = page.getByText(new RegExp(`^Status Monitor \\d{3} ${stamp}$`));
    const allRows = page.getByText(new RegExp(`^(Status Monitor \\d{3}|Paused Status Monitor) ${stamp}$`));
    await expect(allRows).toHaveCount(25);
    await page.getByRole("combobox", { name: "Services per page" }).click();
    await page.getByRole("option", { name: "100 / page" }).click();
    await expect(allRows).toHaveCount(100);

    await page.getByRole("button", { name: "Next" }).click();
    await expect(allRows).toHaveCount(6);
    await expect(page.getByText("Page 2 of 2", { exact: true })).toBeVisible();

    const search = page.getByRole("textbox", { name: `Search services in ${groupName}` });
    await search.fill(`Status Monitor 105 ${stamp}`);
    await expect(page.getByText(`Status Monitor 105 ${stamp}`, { exact: true })).toBeVisible();
    await expect(monitorRows).toHaveCount(1);

    await search.clear();
    await expect(page.getByRole("button", { name: "All 106", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Paused 1", exact: true }).click();
    await expect(page.getByText(`Paused Status Monitor ${stamp}`, { exact: true })).toBeVisible();
  } finally {
    await page.request.patch("/api/status-pages/all", {
      data: { enabled: false, public: false, title: "Global Status" },
    });
    const cleanup = await page.request.delete(`/api/groups/${group.id}`);
    expect(cleanup.ok(), "cleanup status scale group").toBeTruthy();
  }
});
