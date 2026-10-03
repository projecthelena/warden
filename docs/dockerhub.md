# Warden

Warden is an open-source uptime monitor that learns each service's normal latency and reduces noisy alerts. Run it on your own infrastructure with SQLite or PostgreSQL.

- Monitor HTTP endpoints, TCP ports, ICMP hosts, and DNS records.
- Publish status pages and send alerts through Slack, webhooks, or email.
- Confirm failures and group related incidents to avoid repeated notifications.
- Investigate incidents and manage monitors through the API or an AI assistant using MCP.

## Quick start

```bash
docker run -d \
  --name warden \
  --restart unless-stopped \
  -p 127.0.0.1:9090:9090 \
  -v warden_data:/data \
  jesuspaz/warden:latest
```

Open <http://localhost:9090> and create your administrator account. The volume keeps the SQLite database across container restarts and upgrades.

`latest` follows the latest stable release. For production, choose a versioned tag from the [tags list](https://hub.docker.com/r/jesuspaz/warden/tags) and pin it in your deployment. Release candidate tags do not update `latest`.

Images support `linux/amd64` and `linux/arm64`. The same images are also available from [GitHub Container Registry](https://github.com/projecthelena/warden/pkgs/container/warden).

## Configuration

SQLite is the default. PostgreSQL is available for deployments that need an external database. Before upgrading, back up your database and read the [release notes](https://github.com/projecthelena/warden/releases).

For public access, use an HTTPS reverse proxy. See the [configuration guide](https://github.com/projecthelena/warden/blob/main/docs/configuration.md) for cookies, proxy settings, persistence, and database options. [Docker Compose examples](https://github.com/projecthelena/warden/tree/main/deploy) cover SQLite and PostgreSQL.

## Links

- [Source code](https://github.com/projecthelena/warden)
- [Documentation](https://github.com/projecthelena/warden/blob/main/docs/README.md)
- [Report an issue](https://github.com/projecthelena/warden/issues)
- [Helm chart](https://github.com/projecthelena/helm-charts/tree/main/charts/warden)

Warden is licensed under [AGPL-3.0](https://github.com/projecthelena/warden/blob/main/LICENSE).
