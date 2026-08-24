import { NotificationChannel } from "./store";

// Reading a channel back from the API, and turning what it holds into the one line the
// table shows. Both live here rather than inside the store and the view because both are
// pure, and both were places where a wrong answer is invisible: a config that failed to
// parse looks like an empty form, and a display value that falls through looks like a
// channel nobody configured.

// The API stores a channel's config as a JSON string, because each channel type keeps a
// different set of keys. The UI wants an object, so it is parsed once on the way in —
// otherwise every read of channel.config.<key> is silently undefined.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function parseChannelConfig(channel: any): NotificationChannel {
    if (typeof channel?.config !== "string") {
        return channel;
    }
    try {
        return { ...channel, config: JSON.parse(channel.config) };
    } catch {
        // Unparseable config is treated as empty rather than left as a string: the sheets
        // read config.<key>, and a string would make every field undefined anyway while
        // looking like it held something.
        return { ...channel, config: {} };
    }
}

const MAX_DISPLAY_LENGTH = 35;

function truncate(value: string): string {
    return value.length > MAX_DISPLAY_LENGTH ? value.substring(0, MAX_DISPLAY_LENGTH) + "..." : value;
}

// channelDisplayValue is the line under a channel's name in the table — the thing that
// tells two channels of the same type apart. Recipients come first because an email
// channel has no URL to show.
export function channelDisplayValue(config: NotificationChannel["config"]): string {
    if (config.to) {
        return truncate(config.to);
    }
    if (config.webhookUrl) {
        return truncate(config.webhookUrl.replace("https://", "").replace("http://", ""));
    }
    return "Configured";
}
