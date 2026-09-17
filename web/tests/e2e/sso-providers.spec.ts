import { expect, test } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

test.describe("identity providers", () => {
    test("creates, orders, publishes, edits, and removes providers", async ({
        page,
    }) => {
        const login = new LoginPage(page);
        await page.goto("/dashboard");
        if (await login.isVisible()) await login.login();

        await page.goto("/settings?tab=security");
        await expect(page.getByText("Identity providers", { exact: true })).toBeVisible();

        await addProvider(
            page,
            "Google",
            "Google Test",
            "google-client",
            "google-secret",
        );
        await addProvider(
            page,
            "OpenID Connect",
            "Example Identity",
            "oidc-client",
            "oidc-secret",
            "https://id.example.com",
        );

        const publicList = await page.request.get("/api/auth/sso/providers");
        expect(publicList.ok()).toBeTruthy();
        const publicProviders = (await publicList.json()).providers;
        expect(
            publicProviders.map((provider: { name: string }) => provider.name),
        ).toEqual(["Google Test", "Example Identity"]);
        expect(JSON.stringify(publicProviders)).not.toContain("secret");

        await page.getByLabel("Move Example Identity up").click();
        await expect
            .poll(async () => {
                const response = await page.request.get(
                    "/api/auth/sso/providers",
                );
                return (await response.json()).providers.map(
                    (provider: { name: string }) => provider.name,
                );
            })
            .toEqual(["Example Identity", "Google Test"]);

        await login.logout();
        const buttons = page.locator('[data-testid^="sso-provider-"]');
        await expect(buttons).toHaveCount(2);
        await expect(buttons.nth(0)).toHaveText(
            "Sign in with Example Identity",
        );
        await expect(buttons.nth(1)).toHaveText("Sign in with Google Test");

        await login.login();
        await page.goto("/settings?tab=security");
        await page.getByText("Example Identity", { exact: true }).click();
        await page.getByTestId("sso-enabled").click();
        await page.getByTestId("sso-save").click();
        await expect(
            page
                .getByTestId("toast-title")
                .filter({ hasText: "Identity provider updated" })
                .last(),
        ).toBeVisible();

        await login.logout();
        await expect(
            page.getByText("Sign in with Example Identity"),
        ).toHaveCount(0);
        await expect(page.getByText("Sign in with Google Test")).toBeVisible();

        await login.login();
        await page.goto("/settings?tab=security");
        await page.getByLabel("Actions for Example Identity").click();
        await page.getByRole("menuitem", { name: "Remove" }).click();
        await page.getByRole("button", { name: "Remove provider" }).click();
        await expect(
            page.getByText("Example Identity", { exact: true }),
        ).toHaveCount(0);
    });
});

async function addProvider(
    page: import("@playwright/test").Page,
    template: "Google" | "OpenID Connect",
    name: string,
    clientId: string,
    secret: string,
    issuer?: string,
) {
    await page.getByTestId("sso-add-provider").click();
    await page.getByTestId("sso-template").click();
    await page.getByRole("option", { name: template }).click();
    await page.getByTestId("sso-name").fill(name);
    if (issuer) await page.getByTestId("sso-issuer").fill(issuer);
    await page.getByTestId("sso-client-id").fill(clientId);
    await page.getByTestId("sso-client-secret").fill(secret);
    await page.getByTestId("sso-enabled").click();
    await page.getByTestId("sso-save").click();
    await expect(
        page
            .getByTestId("toast-title")
            .filter({ hasText: "Identity provider added" })
            .last(),
    ).toBeVisible();
}
