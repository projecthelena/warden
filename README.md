# <img src="assets/favicon.svg" width="28" height="28" alt="PH" /> Warden

[![CI](https://github.com/projecthelena/warden/actions/workflows/ci.yml/badge.svg)](https://github.com/projecthelena/warden/actions/workflows/ci.yml) [![Docker](https://github.com/projecthelena/warden/actions/workflows/docker.yml/badge.svg)](https://github.com/projecthelena/warden/actions/workflows/docker.yml) [![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

Warden is an open-source, self-hosted uptime monitor that learns what is normal, reduces alert noise, and can be operated directly or through an AI assistant.

It monitors HTTP endpoints, TCP ports, ICMP hosts, and DNS records; publishes status pages; and sends alerts through Slack, webhooks, or email. Adaptive latency baselines, failure confirmation, flapping detection, and incident correlation help distinguish a real problem from a transient spike.

<div align="center">
  <img src="assets/dashboard-overview.png" alt="Warden dashboard" width="100%" />
</div>

## Why Warden

- **Quiet by design:** sustained failures, cooldowns, flapping detection, and correlation prevent one problem from becoming dozens of alerts.
- **Learns each service:** every monitor can build its own latency baseline instead of sharing an arbitrary global threshold.
- **AI-ready and safe:** the role-scoped MCP server lets an assistant investigate and manage monitors without exposing destructive operations.
- **Simple to own:** run one container with SQLite, or connect PostgreSQL when you need an external database.

Read [Why Warden](docs/why-warden.md) for the product direction and comparison with conventional uptime monitoring.

## Quick start

```bash
docker run -d -p 9090:9090 \
  -v warden_data:/data \
  ghcr.io/projecthelena/warden:latest
```

Open `http://localhost:9090` and create your admin account.

Production options are covered in the [configuration guide](docs/configuration.md). Ready-to-use SQLite and PostgreSQL Compose files live in [`deploy/`](deploy/).

## Documentation

Start with the [documentation index](docs/README.md), or read the [public roadmap](docs/roadmap.md).

## License

[AGPL-3.0](LICENSE) — Project Helena
