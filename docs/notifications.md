# Notification Channels

Warden delivers alerts through channels you configure in **Settings → Notifications**. A
channel is a destination, not a rule: every enabled channel receives every alert the
[fatigue rules](notification-fatigue.md) let through, plus the daily digest.

Three types are available:

| Type | Goes to | Needs |
| :--- | :--- | :--- |
| Slack | A Slack channel, as a formatted attachment | An incoming webhook URL |
| Webhook | Any HTTP endpoint, as JSON | A URL that accepts `POST` |
| Email | One or more mailboxes | An SMTP server |

## Email

Email is the channel that reaches people who don't live in Slack — the client who wants to
know their site is back up, the colleague on call this weekend, the shared `alerts@` inbox
that everything else is already piped into.

### Settings

| Field | Notes |
| :--- | :--- |
| SMTP Server | Hostname of your mail server, e.g. `smtp.resend.com` |
| Port | `587` by default. `465` connects over TLS immediately; everything else upgrades with STARTTLS |
| Username / Password | Optional. A password requires a username; credentials are only sent over TLS |
| From | `alerts@example.com`, or `Warden <alerts@example.com>` |
| Send To | One address, or several separated by commas |
| Allow insecure local relay | Off by default. Enable only for a trusted, unauthenticated relay that cannot offer STARTTLS |

Alerts arrive as both plain text and HTML, so they read correctly in a terminal mail client
and in Gmail alike. The subject line carries the state and the monitor, so it is legible
from a phone's lock screen without opening anything:

```
[Warden] Monitor Down: API Gateway
[Warden] Monitor Recovered: API Gateway
[Warden] Daily Summary — all systems operational (March 14, 2026)
```

### About encryption

Warden will not send your password or alert contents over an unencrypted connection by
default. On port 465 the whole
conversation is encrypted from the first byte. On any other port Warden upgrades the
connection with STARTTLS when the server offers it — and if the server does **not** offer
it, the send fails with an error rather than putting the password or alert body on the wire
in the clear.

For a trusted local relay that offers no STARTTLS and needs no credentials, explicitly
enable **Allow insecure local relay**. This exception only permits the alert contents to be
sent in plaintext; Warden still refuses to send SMTP credentials without TLS.

### Common providers

| Provider | Server | Port |
| :--- | :--- | :--- |
| Resend | `smtp.resend.com` | 465 |
| Postmark | `smtp.postmarkapp.com` | 587 |
| SendGrid | `smtp.sendgrid.net` | 587 |
| Gmail (app password) | `smtp.gmail.com` | 587 |
| Local Postfix relay | `localhost` | 25 |

For Gmail you need an [app password](https://support.google.com/accounts/answer/185833);
your normal account password will be rejected.

### When it doesn't work

Use **Send Test** on the channel — it delivers a sample alert through the same code path as
a real one, and reports the server's own error message rather than a generic failure.

- *"does not offer STARTTLS"* — the server won't encrypt the connection. Try port 465 or
  the provider's documented submission port. Only for a trusted, unauthenticated local
  relay, enable **Allow insecure local relay**.
- *"authenticating as …"* — wrong username or password. Many providers want an API key as
  the password and a fixed string as the username.
- *"connecting to …: i/o timeout"* — the port is blocked. Several hosting providers block
  outbound port 25 by default; use 587 or 465.
- Mail is accepted but never arrives — check that the **From** address belongs to a domain
  the provider is allowed to send for. Most will accept the message and then drop it.

## Webhook payload

Webhook channels receive a `POST` with this body:

```json
{
  "event": "down",
  "monitorId": "mon-abc123",
  "monitorName": "API Gateway",
  "monitorUrl": "https://api.example.com",
  "message": "Connection refused after 10s timeout",
  "timestamp": "2026-03-14T15:09:26Z"
}
```

The daily digest arrives on the same endpoint with `"type": "digest"` and a summary of the
day's events.
