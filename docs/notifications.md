# Notification Channels

Warden delivers alerts through channels you configure in **Settings → Notifications**. A channel is a destination, not a rule: every enabled channel receives every alert the [fatigue rules](notification-fatigue.md) let through, plus the daily digest.

Five types are available:

| Type | Goes to | Needs |
| :-- | :-- | :-- |
| Slack | A Slack channel, as a formatted attachment | An incoming webhook URL |
| Webhook | Any HTTP endpoint, as JSON | A URL that accepts `POST` |
| Discord | A Discord channel, as an embed | An incoming webhook URL |
| Telegram | A Telegram chat, group, or channel | A bot token and numeric chat ID |
| Email | One or more mailboxes | An SMTP server |

## Delivery behavior

Warden sends each alert independently to every enabled channel. If Slack, email, Discord, and Telegram are enabled, all four receive the alert. One slow or failing destination does not block the others.

Channels are global and are not attached to individual monitors. Deleting a channel stops future delivery to that destination and does not modify any monitor. Disable a channel instead when you may want to turn it back on later.

Routing selected monitors to selected channels is intentionally outside the current channel model. It can be added later as a separate notification-policy feature without changing how provider credentials are stored.

## Choose which alerts are sent

The **Event Types** controls under **Settings → Notifications** apply globally. Enable or disable immediate notifications for **Down**, **Recovered**, **Degraded**, **Flapping**, **Stabilized**, and **SSL Expiring** events, then save the settings. Disabled event types remain in Warden's history but are not sent to any channel.

The daily digest has its own switch, delivery time, and event-type selection. Its selection controls what appears in the digest; it does not route an event to a different channel. Monitor-specific confirmation and cooldown settings control when an alert is ready to send, but every enabled channel still receives the same alert once it is ready.

## Add and verify a channel

1. Open **Settings → Notifications** and select **Add Channel**.
2. Choose the channel type, give it a **Friendly Name**, and enter the provider details described below.
3. Select **Send Test** before saving. Confirm that the message actually arrived at the intended destination; a success message in Warden alone is not enough.
4. Save the channel. New channels are enabled immediately.

Use **Edit** to send another test or disable a channel temporarily. A disabled channel receives neither immediate alerts nor daily digests. **Delete** permanently removes only that destination; it does not delete or change any monitor.

For final production validation, create a temporary monitor for an endpoint you control and let it generate one real down alert and one recovery alert. This tests monitor evaluation, notification timing, and delivery together. **Send Test** validates only the provider connection and message formatting.

## Slack

1. Create an [incoming webhook in Slack](https://api.slack.com/messaging/webhooks) and choose its destination channel.
2. In Warden, select **Slack**, enter a friendly name, and paste the HTTPS **Webhook URL**.
3. Select **Send Test**, verify the formatted message in Slack, and save the channel.

Treat the webhook URL as a secret: anyone who has it can post to that Slack destination. Rotate it in Slack and update Warden if it is exposed.

## Discord

1. Create a webhook from the Discord channel's **Integrations → Webhooks** settings and copy its URL. Discord's [webhook guide](https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks) covers the provider-side steps.
2. In Warden, select **Discord**, enter a friendly name, and paste the HTTPS **Webhook URL**.
3. Select **Send Test**, verify the embed in Discord, and save the channel.

Warden disables mentions in Discord messages, so monitor-controlled text cannot notify `@everyone`, roles, or users. The webhook URL is still a credential and should not be committed or shared.

## Telegram

1. Create a bot with [BotFather](https://core.telegram.org/bots/features#botfather) and copy its token.
2. Add the bot to the destination chat, group, or channel. For a channel, grant it permission to post messages.
3. Obtain the destination's numeric chat ID. Group and channel IDs are commonly negative; preserve the leading minus sign.
4. In Warden, select **Telegram**, enter a friendly name, the **Bot Token**, and the numeric **Chat ID**.
5. Select **Send Test**, verify the message in Telegram, and save the channel.

The bot token grants control of the bot and must be treated as a secret. Warden splits alerts that exceed Telegram's per-message length limit while preserving their order.

## Email

Email is the channel that reaches people who don't live in Slack — the client who wants to know their site is back up, the colleague on call this weekend, the shared `alerts@` inbox that everything else is already piped into.

### Settings

| Field | Notes |
| :-- | :-- |
| SMTP Server | Hostname of your mail server, e.g. `smtp.resend.com` |
| Port | `587` by default. `465` connects over TLS immediately; everything else upgrades with STARTTLS |
| Username / Password | Optional. A password requires a username; credentials are only sent over TLS |
| From | `alerts@example.com`, or `Warden <alerts@example.com>` |
| Send To | One address, or several separated by commas |
| Allow insecure local relay | Off by default. Enable only for a trusted, unauthenticated relay that cannot offer STARTTLS |

Alerts arrive as both plain text and HTML, so they read correctly in a terminal mail client and in Gmail alike. The subject line carries the state and the monitor, so it is legible from a phone's lock screen without opening anything:

```
[Warden] Monitor Down: API Gateway
[Warden] Monitor Recovered: API Gateway
[Warden] Daily Summary — all systems operational (March 14, 2026)
```

### About encryption

Warden will not send your password or alert contents over an unencrypted connection by default. On port 465 the whole conversation is encrypted from the first byte. On any other port Warden upgrades the connection with STARTTLS when the server offers it — and if the server does **not** offer it, the send fails with an error rather than putting the password or alert body on the wire in the clear.

For a trusted local relay that offers no STARTTLS and needs no credentials, explicitly enable **Allow insecure local relay**. This exception only permits the alert contents to be sent in plaintext; Warden still refuses to send SMTP credentials without TLS.

### Common providers

| Provider             | Server                 | Port |
| :------------------- | :--------------------- | :--- |
| Resend               | `smtp.resend.com`      | 465  |
| Postmark             | `smtp.postmarkapp.com` | 587  |
| SendGrid             | `smtp.sendgrid.net`    | 587  |
| Gmail (app password) | `smtp.gmail.com`       | 587  |
| Local Postfix relay  | `localhost`            | 25   |

For Gmail you need an [app password](https://support.google.com/accounts/answer/185833); your normal account password will be rejected.

### Production smoke test

Do this once with the real provider before depending on email alerts:

1. Verify the sending domain and `From` address with the provider. Publish the SPF and DKIM records it gives you; add DMARC when the provider is passing both.
2. Create the email channel with the provider's submission host, port and credentials. Never put the password in a compose file, shell history or repository.
3. Use **Send Test** and confirm the message reaches the intended inbox rather than only checking for a success toast. Inspect the received message headers and verify that SPF, DKIM and DMARC pass.
4. Create a temporary monitor for a URL you control, let it produce one real down alert and one recovery alert, then remove the monitor. This validates the scheduler and fatigue rules as well as SMTP; **Send Test** only validates direct delivery.

Repeat the direct test after rotating SMTP credentials or changing DNS authentication records.

### When it doesn't work

Use **Send Test** on the channel — it delivers a sample alert through the same code path as a real one, and reports the server's own error message rather than a generic failure.

- _"does not offer STARTTLS"_ — the server won't encrypt the connection. Try port 465 or the provider's documented submission port. Only for a trusted, unauthenticated local relay, enable **Allow insecure local relay**.
- _"authenticating as …"_ — wrong username or password. Many providers want an API key as the password and a fixed string as the username.
- _"connecting to …: i/o timeout"_ — the port is blocked. Several hosting providers block outbound port 25 by default; use 587 or 465.
- Mail is accepted but never arrives — check that the **From** address belongs to a domain the provider is allowed to send for. Most will accept the message and then drop it.

## Generic webhook

Select **Webhook**, enter a friendly name, and provide an HTTP or HTTPS endpoint that accepts `POST` requests. Use **Send Test** to inspect a sample request at the receiving service before saving. Use HTTPS for any endpoint outside a trusted private network.

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

The daily digest arrives on the same endpoint with `"type": "digest"` and a summary of the day's events.
