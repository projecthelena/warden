# Why Warden

Most uptime monitors are good at answering whether a target responded. The harder operational question is whether the result is unusual, related to a larger failure, and important enough to interrupt someone.

Warden is built for small teams, agencies, and homelabs that want that context without adopting a full observability platform.

## Quiet by design

A failed check is evidence, not automatically an incident. Warden confirms sustained failures, detects flapping, applies notification cooldowns, mutes repeat offenders, and correlates monitors that fail together. The history remains available even when an event does not deserve an immediate notification.

See [Notification fatigue](notification-fatigue.md) for the complete alert lifecycle.

## Normal is different for every service

A local health endpoint and a storefront in another region should not share the same latency threshold. Warden learns p50 and p95 latency for each monitor, detects meaningful regressions, and lets an explicit service-level threshold take precedence when one exists.

See [Adaptive latency](adaptive-latency.md) for learning, threshold precedence, and relearning after a migration.

## History should explain the present

Warden looks for recurring degradation, time-of-day failures, correlated monitors, and week-over-week slowdowns. Findings are available in the dashboard, weekly summaries, and MCP instead of remaining hidden in raw check records.

See [Patterns](patterns.md) for the detectors and their conservative reporting rules.

## Built to work with an assistant

Warden exposes a documented REST API and a role-scoped MCP server. A read-only assistant can investigate status, incidents, latency, and notification behavior. An editor can also create, organize, pause, and resume monitors, while destructive deletion and credential management remain unavailable through MCP.

This makes AI an operating interface, not an unchecked automation layer. See the [MCP guide](mcp.md) for the permission model and exposed data.

## Where Warden fits

| Need | Conventional uptime monitor | Full observability platform | Warden's focus |
| --- | --- | --- | --- |
| Basic availability checks | Strong | Strong | Strong |
| Low setup and ownership cost | Strong | Often complex | Strong |
| Metrics, logs, and traces | Limited | Strong | Not the goal |
| Context-aware alert reduction | Varies | Powerful but configurable | Built in |
| Per-monitor learned latency | Uncommon | Requires instrumentation or rules | Built in |
| Safe operation through AI | Uncommon | Product-dependent | API and role-scoped MCP |
| Small-team and agency workflow | General purpose | Often enterprise-oriented | Product focus |

Warden does not aim to replace Prometheus, Grafana, or an application performance monitoring platform. It aims to make availability monitoring more useful and less noisy while remaining simple to self-host.

Projects such as [Uptime Kuma](https://github.com/louislam/uptime-kuma) set a high standard for accessible self-hosted monitoring and breadth of integrations. Warden's opportunity is complementary: deeper operational context, distributed verification, declarative management, and safe AI-assisted diagnosis. The [roadmap](roadmap.md) describes how that focus develops.
