# API

Warden exposes a REST API for managing monitors, incidents, status pages, and more. All endpoints live under `/api/`.

## Swagger

Interactive API docs are available at:

```
http://localhost:9090/api/docs/index.html
```

## Authentication

Most endpoints require a valid session or API key.

### Session

Login via `POST /api/auth/login` with email and password. The server sets a session cookie.

### API Keys

Create API keys from the dashboard under **Settings > API Keys**. Pass the key in the `X-API-Key` header:

```bash
curl -H "X-API-Key: sk_live_..." http://localhost:9090/api/monitors
```

## Monitors

A monitor carries a `type` alongside its target, which decides what check runs against
it — `http` (default), `tcp`, `ping` or `dns`. The target format differs per type, and
so do the options in `requestConfig`. See [Monitor Types](monitor-types.md).

```bash
curl -X POST http://localhost:9090/api/monitors \
  -H "X-API-Key: sk_live_..." -H 'Content-Type: application/json' \
  -d '{"name":"Postgres","type":"tcp","url":"db.internal:5432","groupId":"g-default","interval":60}'
```

Omitting `type` on a `PUT` keeps the type the monitor already has.

## Public Endpoints

These do not require authentication:

| Method | Path | Description |
| :--- | :--- | :--- |
| `POST` | `/api/auth/login` | Login |
| `POST` | `/api/setup` | Initial admin setup |
| `GET` | `/api/s/{slug}` | Public status page data |

## Automation

A helper script is included to bulk-create monitors:

```bash
python3 tools/create_stack.py --key "sk_live_..." --group "Google" --urls https://google.com
```
