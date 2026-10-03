import { expect, test } from "@playwright/test";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";
import { SetupPage } from "../pages/SetupPage";
import { StatusPagesPage } from "../pages/StatusPagesPage";

test.describe("Public status operations lifecycle", () => {
  test("publishes maintenance and outage/latency incidents through resolution without UI errors", async ({ page }) => {
    test.setTimeout(60_000);
    const pageErrors: string[] = [];
    page.on("pageerror", (error) => pageErrors.push(error.message));

    const dashboard = new DashboardPage(page);
    const login = new LoginPage(page);
    const setup = new SetupPage(page);
    const statusPages = new StatusPagesPage(page);
    const stamp = Date.now();
    const groupName = `Status lifecycle ${stamp}`;
    const maintenanceTitle = `Database maintenance ${stamp}`;
    const outageTitle = `API outage ${stamp}`;
    const latencyTitle = `High latency ${stamp}`;
    const incidentIds: string[] = [];
    let maintenanceId = "";
    let groupId = "";

    await dashboard.goto();
    if (page.url().includes("/setup")) await setup.completeSetup();
    if (!page.url().includes("/dashboard")) await login.login();
    await dashboard.waitForLoad();

    const groupResponse = await page.request.post("/api/groups", { data: { name: groupName } });
    expect(groupResponse.status()).toBe(201);
    groupId = ((await groupResponse.json()) as { id: string }).id;

    try {
      const monitorResponse = await page.request.post("/api/monitors", {
        data: {
          name: `Lifecycle API ${stamp}`,
          type: "http",
          url: `${process.env.WARDEN_API_BASE ?? "http://localhost:9096"}/healthz`,
          groupId,
          interval: 60,
        },
      });
      expect(monitorResponse.status()).toBe(201);
      await statusPages.enablePublicViaAPI("all");

      const startTime = new Date(Date.now() - 60_000).toISOString();
      const endTime = new Date(Date.now() + 30 * 60_000).toISOString();
      const maintenanceResponse = await page.request.post("/api/maintenance", {
        data: {
          title: maintenanceTitle,
          description: "Routine database work",
          startTime,
          endTime,
          affectedGroups: [groupId],
        },
      });
      expect(maintenanceResponse.status()).toBe(201);
      maintenanceId = ((await maintenanceResponse.json()) as { id: string }).id;

      // A maintenance window is published as maintenance only. It must not create an alert incident.
      const incidentsAfterMaintenance = await page.request.get("/api/incidents");
      expect(incidentsAfterMaintenance.ok()).toBeTruthy();
      const maintenanceAlerts = (await incidentsAfterMaintenance.json()) as Array<{ title: string }> | null;
      expect(maintenanceAlerts ?? []).not.toContainEqual(
        expect.objectContaining({ title: maintenanceTitle }),
      );

      await page.goto("/status/all");
      await expect(page.getByText("System Under Maintenance", { exact: true })).toBeVisible();
      await expect(page.getByText(maintenanceTitle, { exact: true })).toBeVisible();

      const completedMaintenance = await page.request.put(`/api/maintenance/${maintenanceId}`, {
        data: {
          title: maintenanceTitle,
          description: "Routine database work",
          status: "completed",
          startTime,
          endTime,
          affectedGroups: [groupId],
        },
      });
      expect(completedMaintenance.ok()).toBeTruthy();

      await page.reload();
      await expect(page.getByText(maintenanceTitle, { exact: true })).toHaveCount(0);

      for (const incident of [
        { title: outageTitle, description: "The API is returning errors", severity: "critical" as const },
        { title: latencyTitle, description: "Response time exceeded the latency threshold", severity: "major" as const },
      ]) {
        const response = await page.request.post("/api/incidents", {
          data: {
            ...incident,
            status: "investigating",
            public: true,
            affectedGroups: [],
            startTime: new Date(Date.now() - 60_000).toISOString(),
          },
        });
        expect(response.status()).toBe(201);
        const created = (await response.json()) as { id: string; public: boolean; status: string };
        expect(created.public).toBeTruthy();
        expect(created.status).toBe("investigating");
        incidentIds.push(created.id);
      }

      const publicStatusResponse = await page.request.get("/api/s/all");
      expect(publicStatusResponse.ok()).toBeTruthy();
      const publicStatus = (await publicStatusResponse.json()) as { incidents: Array<{ title: string }> };
      expect(publicStatus.incidents.map((incident) => incident.title)).toEqual(
        expect.arrayContaining([outageTitle, latencyTitle]),
      );

      await page.reload();
      await expect(page.getByRole("heading", { name: /Active Incidents/ })).toBeVisible();
      await expect(page.getByText(outageTitle, { exact: true })).toBeVisible();
      await expect(page.getByText(latencyTitle, { exact: true })).toBeVisible();
      await expect(page.getByText("System Outage", { exact: true })).toBeVisible();

      for (const [index, incident] of [
        { title: outageTitle, description: "The API is returning errors", severity: "critical" },
        { title: latencyTitle, description: "Response time exceeded the latency threshold", severity: "major" },
      ].entries()) {
        const response = await page.request.put(`/api/incidents/${incidentIds[index]}`, {
          data: {
            ...incident,
            status: "resolved",
            public: true,
            affectedGroups: [],
            startTime: new Date(Date.now() - 5 * 60_000).toISOString(),
            endTime: new Date().toISOString(),
          },
        });
        expect(response.ok()).toBeTruthy();
      }

      await page.reload();
      await expect(page.getByText("Active Incidents", { exact: true })).toHaveCount(0);
      await expect(page.getByText("Scheduled Maintenance", { exact: true })).toHaveCount(0);
      await expect(page.getByText("No active incidents or maintenance.", { exact: true })).toBeVisible();

      await page.getByRole("tab", { name: "History" }).click();
      await expect(page.getByText(outageTitle, { exact: true })).toBeVisible();
      await expect(page.getByText(latencyTitle, { exact: true })).toBeVisible();
      expect(pageErrors).toEqual([]);
    } finally {
      for (const id of incidentIds) await page.request.delete(`/api/incidents/${id}`);
      if (maintenanceId) await page.request.delete(`/api/maintenance/${maintenanceId}`);
      await page.request.patch("/api/status-pages/all", {
        data: { enabled: false, public: false, title: "Global Status" },
      });
      if (groupId) await page.request.delete(`/api/groups/${groupId}`);
    }
  });
});
