// The shape of an email channel's config, as the form holds it. The API takes and returns
// these as plain strings, which is why the port is one too.
export interface EmailConfig {
    host: string;
    port: string;
    username: string;
    password: string;
    from: string;
    to: string;
}

export const emptyEmailConfig: EmailConfig = {
    host: "",
    port: "587",
    username: "",
    password: "",
    from: "",
    to: "",
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
export function emailConfigFromChannel(config: Record<string, string | undefined>): EmailConfig {
    return {
        host: config.host || "",
        port: config.port || emptyEmailConfig.port,
        username: config.username || "",
        password: config.password || "",
        from: config.from || "",
        to: config.to || "",
    };
}
