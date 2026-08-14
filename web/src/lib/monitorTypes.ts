import { MonitorType } from "@/lib/store";

interface MonitorTypeInfo {
    label: string;
    targetLabel: string;
    placeholder: string;
    hint: string;
}

export const MONITOR_TYPE_INFO: Record<MonitorType, MonitorTypeInfo> = {
    http: {
        label: "HTTP(S)",
        targetLabel: "Target URL",
        placeholder: "https://api.example.com/health",
        hint: "Requests the URL and checks the response status code.",
    },
    tcp: {
        label: "TCP Port",
        targetLabel: "Target Host and Port",
        placeholder: "db.example.com:5432",
        hint: "Opens a TCP connection. Up when the port accepts it.",
    },
    ping: {
        label: "Ping (ICMP)",
        targetLabel: "Target Host",
        placeholder: "192.168.1.1",
        hint: "Sends an ICMP echo request. Up when a reply comes back.",
    },
    dns: {
        label: "DNS",
        targetLabel: "Name to Resolve",
        placeholder: "example.com",
        hint: "Resolves the name. Up when the lookup returns at least one record.",
    },
};

// Underscores are allowed because Docker's embedded resolver serves compose service
// names that contain them, and those are targets people really do monitor.
const HOSTNAME_LABEL = /[a-zA-Z0-9_]([a-zA-Z0-9_-]*[a-zA-Z0-9_])?/.source;
const HOSTNAME_RE = new RegExp(`^${HOSTNAME_LABEL}(\\.${HOSTNAME_LABEL})*\\.?$`);
const IPV4_RE = /^\d{1,3}(\.\d{1,3}){3}$/;

// isValidTarget mirrors the server-side rules so the form can reject a target without a
// round trip. The server stays the authority; this only makes the answer faster.
export function isValidTarget(type: MonitorType, target: string): boolean {
    if (!target) return false;

    if (type === "http") {
        try {
            const { protocol, host } = new URL(target);
            return (protocol === "http:" || protocol === "https:") && host !== "";
        } catch {
            return false;
        }
    }

    if (type === "tcp") {
        const match = /^(.+):(\d+)$/.exec(target);
        if (!match) return false;
        const port = Number(match[2]);
        return port >= 1 && port <= 65535 && isValidHost(match[1]);
    }

    // Resolving an IP literal never queries anything, so a DNS monitor pointed at one
    // would report up forever. It has to be a name.
    if (type === "dns" && isIPLiteral(target)) return false;

    return isValidHost(target);
}

function isIPLiteral(host: string): boolean {
    return IPV4_RE.test(host) || host.includes(":");
}

function isValidHost(host: string): boolean {
    // IPv6 literals arrive bracketed inside a host:port target and bare on their own.
    const bare = host.replace(/^\[(.+)\]$/, "$1");
    if (!bare || bare.length > 253) return false;
    if (bare.includes(":")) return /^[0-9a-fA-F:]+$/.test(bare);
    return HOSTNAME_RE.test(bare);
}
