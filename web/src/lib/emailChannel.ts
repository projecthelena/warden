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
