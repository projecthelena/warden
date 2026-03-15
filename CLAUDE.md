# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Warden is a self-hosted uptime monitoring application by Project Helena. Go 1.24 backend with embedded React/TypeScript frontend, dual-database support (SQLite and PostgreSQL). Ships as a single binary with no external dependencies.

## Commands

### Development (two terminals)
```bash
make dev-backend          # Go server on :9096 (sets ADMIN_SECRET for local dev)
make dev-frontend         # Vite dev server on :5173, proxies /api to :9096
```

### Build
```bash
make build                # Full production build (frontend + backend → bin/warden)
make build-frontend       # Vite build, copies output to internal/static/dist/
make build-backend        # Go binary only
make docker               # Docker image build
```

### Test
```bash
make test                 # Go unit tests (go test ./...)
make lint                 # Both frontend (ESLint) and backend (golangci-lint)

# Single Go test
go test ./internal/api -run TestUpdateMonitor -v

# Frontend unit tests
cd web && npm run test

# E2E (requires dev-backend running with ADMIN_SECRET)
make e2e                  # Playwright headless (starts Vite automatically)
make e2e-ui               # Playwright with UI
make e2e-fresh            # Wipes DB, restarts backend, runs E2E

# Single E2E test
cd web && npx playwright test tests/e2e/auth.spec.ts
```

### Other
```bash
make stop                 # Kill dev servers on :9096/:5173
make check                # Run lint + tests + security (same as pre-push hook)
make docs                 # Regenerate Swagger docs (requires swag)
```

## Architecture

### Backend (`cmd/`, `internal/`)

Three layers: **Handlers → Store → Database/Uptime Engine**

- **Entry point:** `cmd/dashboard/main.go` — initializes config, DB, uptime manager, and Chi router
- **Router:** `internal/api/router.go` — Chi with middleware (logger, recoverer, auth). Public routes: `/api/auth/login`, `/api/setup`, `/api/s/{slug}`. All other `/api/*` routes require auth middleware.
- **Handlers:** `internal/api/handlers_*.go` — each domain (CRUD, uptime, incidents, maintenance, status pages, settings, notifications, admin) in its own file. Response helpers: `writeJSON()`, `writeError()`
- **Store:** `internal/db/store_*.go` — one file per domain (monitors, groups, users, incidents, status_pages, api_keys, settings). Tests use `:memory:` database via `NewTestConfig()`.
- **Uptime Manager:** `internal/uptime/manager.go` — 50 worker goroutines consuming from a buffered job queue (1000). Results batch-written every 2s or 50 items. Handles latency threshold detection, per-monitor request configuration (method, headers, body, timeout, retries, accepted status codes), SSL cert expiry warnings, flap detection, and data retention cleanup.
- **Monitor:** `internal/uptime/monitor.go` — per-monitor goroutine with confirmation thresholds, notification cooldowns, recovery confirmation, and flap detection state. All mutable state protected by `sync.RWMutex`.
- **Notifications:** `internal/notifications/` — pluggable notification service (Slack, webhooks). Supports per-event-type toggles, daily digest, and notification cooldowns.
- **Auth:** Session-based with cookie auth. SSO (Google) support. First admin created via `ADMIN_SECRET` env var during setup flow. Passwords hashed with bcrypt.
- **Static assets:** `internal/static/` — production frontend embedded into the binary via Go embed.

### Database

Dual-database support: SQLite (default) and PostgreSQL.

- **Migrations:** `internal/db/migrations/sqlite/` and `internal/db/migrations/postgres/` — paired goose migrations, one per directory. Both must be kept in sync when adding new migrations.
- **Query differences:** Use `s.IsPostgres()` to branch SQL syntax (e.g., `datetime('now')` vs `NOW()`, `MAKE_INTERVAL` vs string concatenation). Use `s.rebind()` for parameter placeholders (`?` → `$1`).
- **Config:** `DB_TYPE=sqlite|postgres`, `DB_PATH` for SQLite, `DB_URL` for PostgreSQL connection string.
- **JSON columns:** Complex config stored as JSON TEXT columns (e.g., `notification_channels.config`, `monitors.request_config`). Marshal/unmarshal in store layer with `sql.NullString`.

### Frontend (`web/`)

React 18 + TypeScript + Vite SPA.

- **State:** Zustand store at `web/src/lib/store.ts` — types and API actions for all domains
- **Data fetching:** TanStack React Query via custom hooks in `web/src/hooks/` (e.g., `useMonitors.ts`). Zustand store still used for some legacy actions.
- **UI:** shadcn/ui components (Radix primitives + Tailwind CSS) in `web/src/components/ui/`. Sheet components used for create/edit forms (e.g., `CreateMonitorSheet.tsx`, `MonitorDetailsSheet.tsx`).
- **Routing:** React Router v6 — dashboard, login, setup, public status pages (`/status/{slug}`)
- **Path alias:** `@/*` maps to `web/src/*`

### Key Environment Variables
- `LISTEN_ADDR` — server bind address (default `:9090`, dev uses `:9096`)
- `DB_TYPE` — `sqlite` (default) or `postgres`
- `DB_PATH` — SQLite file path (default `/data/warden.db`)
- `DB_URL` — PostgreSQL connection string
- `ADMIN_SECRET` — required for initial setup flow and enables DB reset endpoint
- `COOKIE_SECURE` — set `true` for HTTPS deployments
- `TRUST_PROXY` — set `true` behind reverse proxy for real IP rate limiting

### E2E Test Setup
Playwright tests in `web/tests/e2e/` use page object models from `web/tests/pages/`. Tests run sequentially (1 worker, Chromium only). The backend must be running with `ADMIN_SECRET=warden-e2e-magic-key`. Playwright auto-starts the Vite dev server locally.

### Testing Patterns

**Go unit tests:** Use `newTestStore(t)` for store tests (`:memory:` SQLite). For integration tests needing the uptime manager started, use `db.NewTestConfigWithPath("file:unique_name_<counter>?mode=memory&cache=shared")` with unique names to avoid `UNIQUE constraint` failures across `-count=N` runs.

**E2E tests:** Page object models in `web/tests/pages/` (`DashboardPage`, `LoginPage`). Use `data-testid` attributes for stable selectors. Sheet content that extends beyond viewport needs `overflow-y-auto` on `SheetContent` for Playwright to scroll into view.
