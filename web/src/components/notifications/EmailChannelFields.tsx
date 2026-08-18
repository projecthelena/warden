import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { EmailConfig } from "@/lib/emailChannel";

// Both the create and the edit sheet render the same six fields, so they live here rather
// than in two copies that slowly disagree about placeholders and hints.
export function EmailChannelFields({
    config,
    onChange,
}: {
    config: EmailConfig;
    onChange: (config: EmailConfig) => void;
}) {
    const set = (key: keyof EmailConfig) => (e: React.ChangeEvent<HTMLInputElement>) =>
        onChange({ ...config, [key]: e.target.value });

    return (
        <>
            <div className="grid grid-cols-[1fr_100px] gap-3">
                <div className="grid gap-2">
                    <Label>SMTP Server</Label>
                    <Input
                        value={config.host}
                        onChange={set("host")}
                        required
                        className="font-mono text-xs"
                        placeholder="smtp.example.com"
                        data-testid="channel-smtp-host-input"
                    />
                </div>
                <div className="grid gap-2">
                    <Label>Port</Label>
                    <Input
                        value={config.port}
                        onChange={set("port")}
                        className="font-mono text-xs"
                        placeholder="587"
                        data-testid="channel-smtp-port-input"
                    />
                </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-2">
                    <Label>Username</Label>
                    <Input
                        value={config.username}
                        onChange={set("username")}
                        autoComplete="off"
                        placeholder="Optional"
                        data-testid="channel-smtp-username-input"
                    />
                </div>
                <div className="grid gap-2">
                    <Label>Password</Label>
                    <Input
                        type="password"
                        value={config.password}
                        onChange={set("password")}
                        autoComplete="new-password"
                        placeholder="Optional"
                        data-testid="channel-smtp-password-input"
                    />
                </div>
            </div>

            <div className="grid gap-2">
                <Label>From</Label>
                <Input
                    value={config.from}
                    onChange={set("from")}
                    required
                    className="font-mono text-xs"
                    placeholder="Warden &lt;alerts@example.com&gt;"
                    data-testid="channel-smtp-from-input"
                />
            </div>

            <div className="grid gap-2">
                <Label>Send To</Label>
                <Input
                    value={config.to}
                    onChange={set("to")}
                    required
                    className="font-mono text-xs"
                    placeholder="oncall@example.com, cto@example.com"
                    data-testid="channel-smtp-to-input"
                />
                <p className="text-[0.8rem] text-muted-foreground">
                    Separate several recipients with commas. Port 465 connects over TLS; anything
                    else upgrades with STARTTLS.
                </p>
            </div>
        </>
    );
}
