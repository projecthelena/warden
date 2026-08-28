import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { EmailChannelFields } from "./EmailChannelFields";
import { emptyEmailConfig } from "@/lib/emailChannel";

describe("EmailChannelFields", () => {
    it("reports one changed field without dropping the others", async () => {
        const onChange = vi.fn();
        const config = { ...emptyEmailConfig, host: "smtp.example.com", from: "alerts@example.com" };

        render(<EmailChannelFields config={config} onChange={onChange} />);
        await userEvent.type(screen.getByTestId("channel-smtp-to-input"), "o");

        // One keystroke, one call, carrying the whole config: a handler that sent only the
        // edited key would blank the server and the sender on every character typed.
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(onChange).toHaveBeenCalledWith({ ...config, to: "o" });
    });

    it("shows the stored values rather than an empty form", () => {
        render(
            <EmailChannelFields
                config={{
                    host: "smtp.example.com",
                    port: "465",
                    username: "alerts@example.com",
                    password: "secret",
                    from: "Warden <alerts@example.com>",
                    to: "ops@example.com",
                    allowInsecure: false,
                }}
                onChange={vi.fn()}
            />
        );

        expect(screen.getByTestId("channel-smtp-host-input")).toHaveValue("smtp.example.com");
        expect(screen.getByTestId("channel-smtp-port-input")).toHaveValue("465");
        expect(screen.getByTestId("channel-smtp-from-input")).toHaveValue("Warden <alerts@example.com>");
        expect(screen.getByTestId("channel-smtp-to-input")).toHaveValue("ops@example.com");
    });

    // The password is the one field a shoulder can read off the screen, and browsers offer
    // to fill saved credentials into anything that looks like a login.
    it("keeps the password masked and out of autofill", () => {
        render(<EmailChannelFields config={{ ...emptyEmailConfig, password: "secret" }} onChange={vi.fn()} />);

        const password = screen.getByTestId("channel-smtp-password-input");
        expect(password).toHaveAttribute("type", "password");
        expect(password).toHaveAttribute("autoComplete", "new-password");
    });

    // Credentials are optional: a relay on the same host is a normal self-hosted setup, and
    // marking them required would make that unconfigurable.
    it("requires a server, a sender and a recipient, but not credentials", () => {
        render(<EmailChannelFields config={emptyEmailConfig} onChange={vi.fn()} />);

        expect(screen.getByTestId("channel-smtp-host-input")).toBeRequired();
        expect(screen.getByTestId("channel-smtp-from-input")).toBeRequired();
        expect(screen.getByTestId("channel-smtp-to-input")).toBeRequired();
        expect(screen.getByTestId("channel-smtp-username-input")).not.toBeRequired();
        expect(screen.getByTestId("channel-smtp-password-input")).not.toBeRequired();
    });

    it("requires an explicit opt-in before showing the plaintext warning", async () => {
        const onChange = vi.fn();
        render(<EmailChannelFields config={emptyEmailConfig} onChange={onChange} />);

        expect(screen.queryByText("Alert contents may be readable on the network.")).not.toBeInTheDocument();
        await userEvent.click(screen.getByTestId("channel-smtp-allow-insecure-switch"));

        expect(onChange).toHaveBeenCalledWith({ ...emptyEmailConfig, allowInsecure: true });
    });
});
