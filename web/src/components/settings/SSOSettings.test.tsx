import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SSOSettings } from "./SSOSettings";

const toast = vi.fn();
vi.mock("@/components/ui/use-toast", () => ({ useToast: () => ({ toast }) }));

const provider = {
    id: "idp-company",
    template: "oidc",
    name: "Company SSO",
    issuerUrl: "https://id.example.com",
    clientId: "warden",
    secretConfigured: true,
    allowedDomains: "example.com",
    autoProvision: true,
    enabled: true,
    sortOrder: 0,
};

describe("SSOSettings", () => {
    beforeEach(() => {
        toast.mockClear();
        HTMLElement.prototype.hasPointerCapture = vi.fn(() => false);
        HTMLElement.prototype.setPointerCapture = vi.fn();
        HTMLElement.prototype.releasePointerCapture = vi.fn();
        HTMLElement.prototype.scrollIntoView = vi.fn();
        vi.stubGlobal(
            "fetch",
            vi.fn().mockResolvedValue({
                ok: true,
                status: 200,
                json: async () => ({ providers: [provider] }),
            }),
        );
    });

    it("loads the ordered provider list", async () => {
        render(<SSOSettings />);
        expect(await screen.findByText("Company SSO")).toBeInTheDocument();
        expect(screen.getByText("Enabled")).toBeInTheDocument();
        expect(fetch).toHaveBeenCalledWith(
            "/api/sso/providers",
            expect.objectContaining({ credentials: "include" }),
        );
    });

    it("preserves the stored secret when editing", async () => {
        const user = userEvent.setup();
        render(<SSOSettings />);
        await user.click(await screen.findByText("Company SSO"));
        await user.clear(screen.getByTestId("sso-name"));
        await user.type(screen.getByTestId("sso-name"), "Internal SSO");
        await user.click(screen.getByTestId("sso-save"));

        await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
        const [, request] = vi.mocked(fetch).mock.calls[1];
        expect(request?.method).toBe("PUT");
        expect(JSON.parse(String(request?.body))).toMatchObject({
            name: "Internal SSO",
            clientSecret: "",
        });
    });

    it("creates a generic OIDC provider", async () => {
        vi.mocked(fetch).mockResolvedValueOnce({
            ok: true,
            status: 200,
            json: async () => ({ providers: [] }),
        } as Response);
        vi.mocked(fetch).mockResolvedValueOnce({
            ok: true,
            status: 201,
            json: async () => provider,
        } as Response);
        const user = userEvent.setup();
        render(<SSOSettings />);
        await user.click(
            await screen.findByRole("button", { name: "Add provider" }),
        );
        await user.click(screen.getByTestId("sso-template"));
        await user.click(
            screen.getByRole("option", { name: "OpenID Connect" }),
        );
        await user.type(
            screen.getByTestId("sso-issuer"),
            "https://id.example.com",
        );
        await user.type(screen.getByTestId("sso-client-id"), "warden");
        await user.type(screen.getByTestId("sso-client-secret"), "secret");
        await user.click(screen.getByTestId("sso-save"));

        await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
        expect(vi.mocked(fetch).mock.calls[1][0]).toBe("/api/sso/providers");
    });
});
