# Roadmap

Warden's goal is to be the quiet, intelligent, self-hosted reliability monitor for small teams, agencies, and homelabs. It will prioritize trustworthy diagnosis and low operational burden over matching every feature of a general-purpose monitoring platform.

This document describes direction, not release commitments. GitHub issues are the source of truth for implementation scope and progress; roadmap items should link to an issue when work is ready to be designed.

## Now: make the intelligence clear and dependable

- Keep every alert explainable: why it fired, which threshold was crossed, and why related alerts were suppressed.
- Strengthen adaptive latency, pattern detection, correlation, and notification-fatigue controls.
- Keep the REST API, MCP tools, dashboard, and documentation consistent.
- Make migrations and changes of network location safe through explicit baseline relearning.

## Next: close adoption blockers

- **Push monitors:** accept heartbeats from cron jobs, backups, and short-lived workloads.
- **Monitor dependencies:** suppress cascades and identify the upstream service most likely to be responsible.
- **HTTP assertions:** validate response text and structured JSON, not only the status code.
- **Declarative provisioning:** import, export, and reconcile monitors from version-controlled configuration.
- **Generic OIDC:** integrate with self-hosted and managed identity providers beyond one vendor.
- **Status-page subscriptions:** notify interested users about incidents, resolutions, and scheduled maintenance.

## Later: build the hard-to-copy advantage

- **Remote probes:** run checks from a homelab, cloud region, private network, or customer environment.
- **Multi-location confirmation:** distinguish a service outage from a problem affecting only one observer.
- **Location-aware baselines:** learn normal latency independently for every monitor and probe pair.
- **Probable cause:** combine dependencies, timing, location, and correlation into an evidence-based explanation.
- **Agency workflows:** provide client-scoped status pages, reports, branding, access, and service-level views.

## Product principles

1. Preserve evidence even when an alert is suppressed.
2. Explain automated decisions in plain language.
3. Keep safe defaults and require explicit authority for destructive operations.
4. Prefer a small coherent feature over a broad but shallow integration.
5. Keep SQLite simple while supporting PostgreSQL for larger installations.
6. Treat API, MCP, UI, and documentation as equal product surfaces.

## How work moves from roadmap to release

1. Open a focused GitHub issue with the user problem and acceptance criteria.
2. Link related issues under the appropriate roadmap theme or milestone.
3. Design and deliver each issue independently where possible.
4. Update this document only when product direction changes, not for routine task status.
