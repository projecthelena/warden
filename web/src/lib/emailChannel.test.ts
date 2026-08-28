import { describe, it, expect } from "vitest";
import { emptyEmailConfig, isEmailConfigured, emailConfigFromChannel } from "./emailChannel";

describe("isEmailConfigured", () => {
    it("needs a server, a sender and a recipient", () => {
        expect(isEmailConfigured(emptyEmailConfig)).toBe(false);
        expect(isEmailConfigured({ ...emptyEmailConfig, host: "smtp.example.com" })).toBe(false);
        expect(
            isEmailConfigured({
                ...emptyEmailConfig,
                host: "smtp.example.com",
                from: "alerts@example.com",
                to: "ops@example.com",
            })
        ).toBe(true);
    });

    it("does not count whitespace as a value", () => {
        expect(
            isEmailConfigured({
                ...emptyEmailConfig,
                host: "   ",
                from: "alerts@example.com",
                to: "ops@example.com",
            })
        ).toBe(false);
    });

    it("does not require credentials, so a local relay stays testable", () => {
        expect(
            isEmailConfigured({
                host: "localhost",
                port: "25",
                username: "",
                password: "",
                from: "alerts@example.com",
                to: "ops@example.com",
                allowInsecure: true,
            })
        ).toBe(true);
    });
});

describe("emailConfigFromChannel", () => {
    it("preloads the password so an edit does not blank it", () => {
        const config = emailConfigFromChannel({
            host: "smtp.example.com",
            port: "465",
            username: "alerts@example.com",
            password: "secret",
            from: "Warden <alerts@example.com>",
            to: "ops@example.com",
        });

        expect(config.password).toBe("secret");
        expect(config.username).toBe("alerts@example.com");
        expect(config.port).toBe("465");
        expect(config.allowInsecure).toBe(false);
    });

    it("falls back to the submission port rather than leaving it blank", () => {
        // A blank port would post an empty string and be rejected, on a field the operator
        // never touched.
        const config = emailConfigFromChannel({ host: "smtp.example.com" });
        expect(config.port).toBe("587");
    });

    it("reads a channel of another type as an empty form", () => {
        const config = emailConfigFromChannel({ webhookUrl: "https://hooks.slack.com/x" });

        expect(config.host).toBe("");
        expect(config.from).toBe("");
        expect(config.to).toBe("");
        expect(config.password).toBe("");
        expect(config.allowInsecure).toBe(false);
    });

    it("restores an explicit insecure-relay opt-in", () => {
        const config = emailConfigFromChannel({
            host: "localhost",
            port: "25",
            allowInsecure: true,
        });

        expect(config.allowInsecure).toBe(true);
    });
});
