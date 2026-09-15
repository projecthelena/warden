import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { OIDCSettings } from "./OIDCSettings";

const { mockStore, toast } = vi.hoisted(() => ({ mockStore: vi.fn(), toast: vi.fn() }));

vi.mock("@/lib/store", () => ({ useMonitorStore: mockStore }));
vi.mock("@/components/ui/use-toast", () => ({ useToast: () => ({ toast }) }));

describe("OIDCSettings", () => {
    const fetchSettings = vi.fn().mockResolvedValue(undefined);
    const updateSettings = vi.fn().mockResolvedValue(undefined);

    beforeEach(() => {
        fetchSettings.mockClear();
        updateSettings.mockClear();
        toast.mockClear();
        mockStore.mockReturnValue({
            settings: {
                "sso.oidc.enabled": "true",
                "sso.oidc.issuer_url": "https://id.example.com",
                "sso.oidc.client_id": "warden",
                "sso.oidc.secret_configured": "true",
                "sso.oidc.provider_name": "Company SSO",
                "sso.oidc.auto_provision": "true",
            },
            fetchSettings,
            updateSettings,
        });
    });

    it("keeps the stored secret when saving another field", async () => {
        const user = userEvent.setup();
        render(<OIDCSettings />);

        const provider = screen.getByDisplayValue("Company SSO");
        await user.clear(provider);
        await user.type(provider, "Internal SSO");
        await user.click(screen.getByRole("button", { name: "Save OIDC Settings" }));

        await waitFor(() => expect(updateSettings).toHaveBeenCalledOnce());
        const values = updateSettings.mock.calls[0][0];
        expect(values["sso.oidc.provider_name"]).toBe("Internal SSO");
        expect(values).not.toHaveProperty("sso.oidc.client_secret");
    });

    it("does not enable OIDC until issuer, client ID, and secret are configured", () => {
        mockStore.mockReturnValue({ settings: {}, fetchSettings, updateSettings });
        render(<OIDCSettings />);
        expect(screen.getByRole("switch", { name: "Enable OIDC" })).toBeDisabled();
    });
});
