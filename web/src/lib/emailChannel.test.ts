import { describe, it, expect } from "vitest";
import { emptyEmailConfig, isEmailConfigured } from "./emailChannel";

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
            })
        ).toBe(true);
    });
});
