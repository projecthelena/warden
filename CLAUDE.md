# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ClusterUptime is a self-hosted uptime monitoring application. Go 1.24 backend with embedded React/TypeScript frontend, SQLite database. Ships as a single binary with no external dependencies.

## Commands

### Development (two terminals)
```bash
make dev-backend          # Go server on :9096 (sets ADMIN_SECRET for local dev)
make dev-frontend         # Vite dev server on :5173, proxies /api to :9096
```

### Build
```bash
make build                # Full production build (frontend + backend → bin/clusteruptime)
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

# Single E2E test
cd web && npx playwright test tests/e2e/auth.spec.ts
```

## Architecture

### Backend (`cmd/`, `internal/`)

Three layers: **Handlers → Store → Database/Uptime Engine**

- **Entry point:** `cmd/dashboard/main.go` — initializes config, DB, uptime manager, and Chi router
- **Router:** `internal/api/router.go` — Chi with middleware (logger, recoverer, auth). Public routes: `/api/auth/login`, `/api/setup`, `/api/s/{slug}`. All other `/api/*` routes require auth middleware.
- **Handlers:** `internal/api/handlers_*.go` — each domain (CRUD, uptime, incidents, maintenance, status pages, settings, notifications, admin) in its own file. Response helpers: `writeJSON()`, `writeError()`
- **Store:** `internal/db/store_*.go` — one file per domain (monitors, groups, users, incidents, status_pages, api_keys, settings). SQLite with embedded migrations (`internal/db/migrations/`). Tests use `:memory:` database.
- **Uptime Manager:** `internal/uptime/manager.go` — 50 worker goroutines consuming from a buffered job queue (1000). Results batch-written every 2s or 50 items. Handles latency threshold detection, notifications (Slack), and data retention cleanup.
- **Auth:** Session-based. First admin created via `ADMIN_SECRET` env var during setup flow. Passwords hashed with bcrypt.
- **Static assets:** `internal/static/` — production frontend embedded into the binary via Go embed.

### Frontend (`web/`)

React 18 + TypeScript + Vite SPA.

- **State:** Zustand store at `web/src/lib/store.ts`
- **Data fetching:** TanStack React Query via custom hooks in `web/src/hooks/`
- **API client:** `web/src/lib/api.ts`
- **UI:** shadcn/ui components (Radix primitives + Tailwind CSS) in `web/src/components/ui/`
- **Routing:** React Router v6 — dashboard, login, setup, public status pages (`/status/{slug}`)
- **Path alias:** `@/*` maps to `web/src/*`

### Key Environment Variables
- `LISTEN_ADDR` — server bind address (default `:9090`, dev uses `:9096`)
- `DB_PATH` — SQLite file path (default `/data/clusteruptime.db`)
- `ADMIN_SECRET` — required for initial setup flow
- `COOKIE_SECURE` — set `true` for HTTPS deployments

### E2E Test Setup
Playwright tests in `web/tests/e2e/` use page object models from `web/tests/pages/`. Tests run sequentially (1 worker, Chromium only). The backend must be running with `ADMIN_SECRET=clusteruptime-e2e-magic-key`. Playwright auto-starts the Vite dev server locally.
