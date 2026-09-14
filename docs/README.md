# Documentation

Use this page as the starting point for installing, operating, and understanding Warden.

## Get started

- [Database configuration](database.md) — run Warden with SQLite or PostgreSQL.
- [Application configuration](configuration.md) — configure the listener, database, cookies, and reverse proxy handling.
- [Monitor types](monitor-types.md) — configure HTTP, TCP, ping, and DNS checks.
- [Notifications](notifications.md) — deliver alerts through Slack, webhooks, or email.
- [Password recovery](recovery.md) — regain access to an installation.

## Understand Warden

- [Why Warden](why-warden.md) — product focus and the gap Warden is designed to fill.
- [Adaptive latency](adaptive-latency.md) — how each monitor learns its normal response time.
- [Notification fatigue](notification-fatigue.md) — confirmation, reminders, flapping detection, correlation, and alert damping.
- [Patterns](patterns.md) — recurring behavior and longer-term changes found in monitor history.
- [Roadmap](roadmap.md) — product themes and planned areas of investment.

## Automate and integrate

- [REST API](api.md) — API keys, endpoints, and Swagger documentation.
- [MCP server](mcp.md) — safely inspect and operate Warden through an AI assistant.

## Operate and migrate

- [Move PostgreSQL to another instance](postgresql-migration.md) — dump, restore, verify, roll back, and relearn latency after moving networks.
- [Load testing](load-testing.md) — exercise Warden under sustained check volume.

For local development, tests, and Markdown formatting, see [Contributing](../CONTRIBUTING.md).
