import http from "node:http";
import { AddressInfo } from "node:net";
import { expect, test } from "@playwright/test";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";

test.describe.configure({ mode: "serial" });

test.describe("Docker container monitoring", () => {
    let engine: http.Server;
    let endpoint: string;

    test.beforeAll(async () => {
        engine = http.createServer((request, response) => {
            response.setHeader("Content-Type", "application/json");
            if (request.url === "/_ping") {
                response.setHeader("Content-Type", "text/plain");
                response.end("OK");
                return;
            }
            if (request.url === "/containers/json?all=1") {
                response.end(JSON.stringify([{ Id: "container-123", Names: ["/warden-api-1"], Image: "projecthelena/warden:test", State: "running", Status: "Up 2 minutes (healthy)", Labels: {} }]));
                return;
            }
            if (request.url === "/containers/container-123/json") {
                response.end(JSON.stringify({ RestartCount: 0, State: { Status: "running", Running: true, Paused: false, Restarting: false, OOMKilled: false, ExitCode: 0, Error: "", Health: { Status: "healthy", Log: [{ Output: "ready" }] } } }));
                return;
            }
            response.statusCode = 404;
            response.end(JSON.stringify({ message: "not found" }));
        });
        await new Promise<void>((resolve, reject) => {
            engine.once("error", reject);
            engine.listen(0, "127.0.0.1", resolve);
        });
        endpoint = `http://127.0.0.1:${(engine.address() as AddressInfo).port}`;
    });

    test.afterAll(async () => {
        await new Promise<void>((resolve, reject) => engine.close(error => error ? reject(error) : resolve()));
    });

    test("connects a host, discovers a container, and creates a healthy monitor", async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);
        const suffix = Date.now();
        const hostName = `Docker E2E ${suffix}`;
        const monitorName = `Docker API ${suffix}`;

        await dashboard.goto();
        if (await login.isVisible()) await login.login();
        await dashboard.waitForLoad();

        await dashboard.createMonitorTrigger.click();
        await dashboard.createMonitorName.fill(monitorName);
        await page.getByTestId("create-monitor-type-select").click();
        await page.getByRole("option", { name: "Docker Container" }).click();

        await page.getByRole("button", { name: "Manage hosts" }).click();
        await page.getByRole("button", { name: "Connect Docker host" }).click();
        const hostDialog = page.getByRole("dialog", { name: "Docker hosts" });
        await hostDialog.getByRole("textbox", { name: "Name" }).fill(hostName);
        await hostDialog.getByRole("textbox", { name: "Endpoint" }).fill(endpoint);
        await page.getByRole("button", { name: "Save and test" }).click();
        await expect(page.getByText("Connection verified. Now choose the container to monitor.", { exact: true })).toBeVisible();

        await page.getByTestId("docker-container-select").click();
        await page.getByRole("option", { name: /warden-api-1/ }).click();
        await page.getByTestId("create-monitor-group-select").click();
        await page.getByRole("option", { name: "Default", exact: true }).click();
        await dashboard.createMonitorSubmit.click();
        await expect(page.getByText(`Monitor "${monitorName}" active and checking.`).first()).toBeVisible({ timeout: 15_000 });

        await dashboard.openMonitorSettings(monitorName);
        await expect(page.getByRole("heading", { name: monitorName })).toBeVisible();
        await expect(page.getByText("DOCKER", { exact: true })).toBeVisible();
        await expect(page.getByTestId("docker-host-select")).toContainText(hostName);
        await expect(page.getByTestId("docker-container-select")).toContainText("warden-api-1");

        await dashboard.deleteMonitor(monitorName);
        await dashboard.createMonitorTrigger.click();
        await page.getByTestId("create-monitor-type-select").click();
        await page.getByRole("option", { name: "Docker Container" }).click();
        await page.getByRole("button", { name: "Manage hosts" }).click();
        await page.getByRole("button", { name: `Delete ${hostName}` }).click();
        await expect(page.getByText(hostName)).toHaveCount(0);
    });
});
