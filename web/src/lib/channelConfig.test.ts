import { describe, it, expect } from "vitest";
import { parseChannelConfig, channelDisplayValue } from "./channelConfig";

describe("parseChannelConfig", () => {
    it("turns the stored JSON string into the object the sheets read", () => {
        const channel = parseChannelConfig({
            id: "nc1",
            type: "email",
            name: "On-call",
            config: '{"host":"smtp.example.com","to":"ops@example.com"}',
            enabled: true,
        });

        expect(channel.config.host).toBe("smtp.example.com");
        expect(channel.config.to).toBe("ops@example.com");
    });

    // Without this the failure is silent rather than loud: config stays a string, every
    // config.<key> is undefined, and the edit sheet opens on a blank form that overwrites
    // a working channel the moment somebody saves it.
    it("leaves nothing readable as a bare string", () => {
        const channel = parseChannelConfig({
            id: "nc1",
            type: "slack",
            name: "Alerts",
            config: '{"webhookUrl":"https://hooks.slack.com/services/T/B/X"}',
            enabled: true,
        });

        expect(typeof channel.config).toBe("object");
        expect(channel.config.webhookUrl).toBe("https://hooks.slack.com/services/T/B/X");
    });

    it("passes through a config that is already an object", () => {
        const config = { host: "smtp.example.com" };
        const channel = parseChannelConfig({ id: "nc1", type: "email", name: "Mail", config, enabled: true });

        expect(channel.config).toBe(config);
    });

    it("reads unparseable config as empty instead of throwing", () => {
        const channel = parseChannelConfig({
            id: "nc1",
            type: "email",
            name: "Mail",
            config: "{not json",
            enabled: true,
        });

        expect(channel.config).toEqual({});
        expect(channel.name).toBe("Mail");
    });

    it("survives a channel with no config at all", () => {
        const channel = parseChannelConfig({ id: "nc1", type: "email", name: "Mail", enabled: true });
        expect(channel.name).toBe("Mail");
    });
});

describe("channelDisplayValue", () => {
    it("shows the recipients for an email channel", () => {
        expect(channelDisplayValue({ to: "oncall@example.com" })).toBe("oncall@example.com");
    });

    it("prefers the recipients over a webhook URL left behind by a type change", () => {
        expect(
            channelDisplayValue({ to: "oncall@example.com", webhookUrl: "https://hooks.slack.com/x" })
        ).toBe("oncall@example.com");
    });

    it("strips the scheme from a webhook URL", () => {
        expect(channelDisplayValue({ webhookUrl: "https://hooks.example.com/x" })).toBe("hooks.example.com/x");
        expect(channelDisplayValue({ webhookUrl: "http://hooks.example.com/x" })).toBe("hooks.example.com/x");
    });

    it("truncates a long recipient list rather than stretching the row", () => {
        const many = "a@example.com, b@example.com, c@example.com, d@example.com";
        const shown = channelDisplayValue({ to: many });

        expect(shown).toHaveLength(38); // 35 characters plus the ellipsis
        expect(shown.endsWith("...")).toBe(true);
        expect(many.startsWith(shown.slice(0, -3))).toBe(true);
    });

    it("does not truncate a list that fits", () => {
        const fits = "a@example.com";
        expect(channelDisplayValue({ to: fits })).toBe(fits);
    });

    it("falls back to a word rather than an empty cell", () => {
        expect(channelDisplayValue({})).toBe("Configured");
    });
});
