// The shape of an email channel's config, as the form holds it. Text fields stay strings
// (including the port), while the insecure-relay opt-in is an explicit boolean.
export interface EmailConfig {
    host: string;
    port: string;
    username: string;
    password: string;
    from: string;
    to: string;
    allowInsecure: boolean;
}

export const emptyEmailConfig: EmailConfig = {
    host: "",
    port: "587",
    username: "",
    password: "",
    from: "",
    to: "",
    allowInsecure: false,
};

// isEmailConfigured reports whether there is enough here to attempt a send. The server
// validates properly — this only decides whether the Send Test button is clickable.
export function isEmailConfigured(config: EmailConfig): boolean {
    return config.host.trim() !== "" && config.from.trim() !== "" && config.to.trim() !== "";
}

// emailConfigFromChannel reads a stored channel back into the form's shape. Anything
// missing falls back to the defaults, which is what a channel of another type looks like
// here.
//
// The password comes back with the rest: the form preloads it so that saving an unrelated
// change does not blank it. A channel that stopped authenticating would not fall back to
// sending in the clear, it would stop sending at all.
export function emailConfigFromChannel(config: Record<string, string | boolean | undefined>): EmailConfig {
    const stringValue = (value: string | boolean | undefined) => typeof value === "string" ? value : "";
    return {
        host: stringValue(config.host),
        port: stringValue(config.port) || emptyEmailConfig.port,
        username: stringValue(config.username),
        password: stringValue(config.password),
        from: stringValue(config.from),
        to: stringValue(config.to),
        allowInsecure: config.allowInsecure === true,
    };
}
