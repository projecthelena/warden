# Warden Status Page Redesign

## Problem Statement

The current status page is functional but minimal — a simple list of monitor names with colored status dots and an "All Systems Operational" banner. It lacks the features modern teams expect from a public-facing status page.

**What's missing:**
- No uptime history visualization (90-day bars)
- No overall uptime percentage per monitor
- No incident history (only active incidents shown)
- No branding/customization (logo, colors, description)
- No subscriber notifications
- No configurable display options
- No theme selection (always dark)

The status page is the public face of reliability. It builds trust, reduces support tickets during outages, and satisfies SOC2 availability requirements.

---

## Current Architecture

### Frontend (`web/src/components/status-page/StatusPage.tsx`)
- Single-page component with inline sub-components
- Auto-refreshes every 60s with countdown timer
- Shows: status header banner, scheduled maintenance, critical outages, groups with monitors
- Each monitor shows: name, external URL link (on hover), status text + colored dot
- Status states: Operational (green), Degraded (yellow), Down (red), Maintenance (blue)

### Backend (`internal/api/handlers_status_pages.go`)
- `GET /api/s/{slug}` — public endpoint returning groups, monitors, active incidents
- Monitor data comes from live uptime manager (in-memory history)
- Incidents merged from auto-detected outages + manual incidents

### Database (`internal/db/store_status_pages.go`)
- `status_pages` table: slug, title, group_id, public, enabled
- No branding columns, no subscriber infrastructure

### Data Available
- `monitor_checks` table stores every check result (monitor_id, status, latency, timestamp)
- `GetUptimeStats()` already computes 24h/7d/30d uptime from raw checks
- Data retention configurable up to 3650 days
- Can GROUP BY date to get daily uptime for 90-day bars

---

## Implementation Phases

```
Phase 1 (Uptime Bars + Redesign)     <- No dependencies, start here
    |
Phase 2 (Incident History)           <- Builds on Phase 1 layout
    |
Phase 3 (Configuration)              <- Makes Phase 1+2 features configurable
    |
Phase 4 (Subscribers)                <- Requires Phase 3 for config, Phase 2 for incidents
```

Each phase is independently shippable and valuable.

---

## Phase 1: 90-Day Uptime Bars + Visual Redesign [COMPLETED]

**Goal:** Transform the status page from a basic monitor list into a professional, trust-building page with the industry-standard uptime visualization.

### New Layout

```
+---------------------------------------+
|        [Icon]  Title                   |
+---------------------------------------+
|  [icon] All Systems Operational   52s  |
+---------------------------------------+
|  SCHEDULED MAINTENANCE (if any)        |
|  ACTIVE INCIDENTS (if any)             |
+---------------------------------------+
|  GROUP NAME                            |
|  +-----------------------------------+ |
|  | Monitor Name          [dot]       | |
|  | ||||||||||||||||||||||||||| 99.9%  | |
|  | Monitor Name          [dot]       | |
|  | ||||||||||||||||||||||||||| 99.8%  | |
|  +-----------------------------------+ |
|                                        |
|  GROUP NAME                            |
|  +-----------------------------------+ |
|  | Monitor Name          [dot]       | |
|  | ||||||||||||||||||||||||||| 100%   | |
|  +-----------------------------------+ |
+---------------------------------------+
|        Powered by Warden               |
+---------------------------------------+
```

### What Was Built

#### Backend: `GetDailyUptimeStats(monitorID, days)`
- **File:** `internal/db/store_monitors.go`
- Queries `monitor_checks` grouped by `DATE(timestamp)`
- Returns `[]DailyUptimeStat` with date, total checks, up count, uptime percentage
- Fills gaps for days with no data (returns -1 uptime = "no data")
- Supports both SQLite and PostgreSQL
- Input validation (1-365 days)

#### Backend: Extended Public Status API
- **File:** `internal/api/handlers_status_pages.go`
- `MonitorDTO` now includes `uptimeDays` (90-day array) and `overallUptime`
- Each monitor in `/api/s/{slug}` response carries its 90-day history
- Overall uptime computed from raw check counts across the 90-day window

#### Frontend: `UptimeBar` Component
- **File:** `web/src/components/status-page/UptimeBar.tsx`
- 90 thin vertical bars, flex-sized to fill available width
- Color scale: green (100%) -> yellow-green (99-99.9%) -> yellow (95-99%) -> orange (90-95%) -> red (<90%) -> gray (no data)
- Hover tooltip: date, uptime %, check count
- Overall uptime % displayed right-aligned with color coding
- Responsive: bars flex naturally to container width

#### Frontend: Redesigned StatusPage
- **File:** `web/src/components/status-page/StatusPage.tsx`
- Compact status banner (icon + label + description + countdown)
- Monitors in card sections with uptime bars below each name
- Cleaner maintenance and incident alerts
- Better spacing, typography, and staggered enter animations
- All existing functionality preserved

#### Tests
- **File:** `internal/db/store_monitors_test.go`
- `TestGetDailyUptimeStats_Empty` — no checks returns all -1 days
- `TestGetDailyUptimeStats_WithChecks` — 8 up + 2 down = 80% verified
- `TestGetDailyUptimeStats_MultipleMonitors` — isolates per-monitor data
- `TestGetDailyUptimeStats_InvalidDays` — rejects 0 and 366
- `TestGetDailyUptimeStats_GapFilling` — 1 day with data, 6 filled as no-data

---

## Phase 2: Incident History on Status Page

**Goal:** Show resolved incidents from the last 7-14 days grouped by date. This is a trust signal — users can see the incident response pattern.

### Tasks

#### 2.1 — Backend: Resolved Incidents Query
- **File:** `internal/db/store_incidents.go`
- Add `GetResolvedIncidents(since time.Time) ([]Incident, error)`
- Fetch incidents where status = 'resolved' or 'completed', end_time > since
- Also fetch resolved outages from `monitor_outages` where end_time > since
- Order by start_time DESC

#### 2.2 — Backend: Past Incidents in API Response
- **File:** `internal/api/handlers_status_pages.go`
- Extend `GetPublicStatus()` response with `pastIncidents` field
- Group by date (ISO date string key)
- Include: id, title, description, type, severity, status, startTime, endTime, duration
- Limit to last 14 days

#### 2.3 — Frontend: Incident History Section
- New section below monitor groups in `StatusPage.tsx`
- Heading: "Past Incidents"
- Each day: date header (e.g., "Feb 14, 2026") + incident cards below
- Incident card: severity badge, title, duration (e.g., "2h 15m"), description snippet
- Days with no incidents: "No incidents reported." (explicit trust signal)
- Show last 7 days by default, "Show more" expands to 14

#### 2.4 — Tests
- Unit test for `GetResolvedIncidents` in `internal/db/store_incidents_test.go`

### API Response Shape
```json
{
  "title": "Global Status",
  "groups": [...],
  "incidents": [...],
  "pastIncidents": [
    {
      "id": "inc-123",
      "title": "API Latency Spike",
      "description": "Elevated response times on /api/v2",
      "type": "incident",
      "severity": "major",
      "status": "resolved",
      "startTime": "2026-02-13T14:30:00Z",
      "endTime": "2026-02-13T16:45:00Z",
      "duration": "2h 15m"
    }
  ]
}
```

---

## Phase 3: Status Page Configuration (Branding & Display Options)

**Goal:** Make each status page configurable — logo, colors, description, theme, and display toggles. This transforms a generic page into "your company's" status page.

### Tasks

#### 3.1 — Database Migration
- **Files:** `internal/db/migrations/sqlite/00008_*.sql`, `internal/db/migrations/postgres/00008_*.sql`
- Add columns to `status_pages`:

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `description` | TEXT | `''` | Subtitle/tagline |
| `logo_url` | TEXT | `''` | URL or base64 data URI |
| `accent_color` | TEXT | `''` | Hex color for theming |
| `theme` | TEXT | `'system'` | 'light', 'dark', or 'system' |
| `show_uptime_bars` | BOOLEAN | `TRUE` | Toggle uptime bar visibility |
| `show_uptime_percentage` | BOOLEAN | `TRUE` | Toggle percentage display |
| `show_incident_history` | BOOLEAN | `TRUE` | Toggle incident history |
| `incident_history_days` | INTEGER | `7` | Days of history to show |
| `custom_css` | TEXT | `''` | Custom CSS injection |

#### 3.2 — Backend: Config in Store and API
- **File:** `internal/db/store_status_pages.go`
- Extend `StatusPage` struct with new fields
- Update `UpsertStatusPage()` to handle new fields
- Include config fields in `GetPublicStatus()` response

#### 3.3 — Admin UI: Status Page Settings
- **File:** `web/src/components/status-pages/StatusPageSettings.tsx` (new)
- Dialog/drawer when clicking a status page in admin
- Sections:
  - **Branding:** Title, description, logo URL, accent color picker
  - **Theme:** Radio group for light/dark/system
  - **Display:** Toggles for uptime bars, uptime %, incident history
  - **Advanced:** Incident history days dropdown, custom CSS textarea
- Update `StatusPagesView.tsx` to add "Configure" button

#### 3.4 — Frontend: Apply Config to Public Page
- **File:** `web/src/components/status-page/StatusPage.tsx`
- Apply logo: show image in header if set, fallback to Activity icon
- Apply accent color: override CSS custom property
- Apply theme: force dark/light class on root
- Conditionally show/hide: uptime bars, percentages, incident history
- Inject custom_css via `<style>` tag (sanitized)

#### 3.5 — Tests
- Unit test for config persistence
- E2E: verify config changes apply to public page

### Config API Shape
```json
{
  "title": "Acme Status",
  "description": "Real-time system status for Acme Corp",
  "logoUrl": "https://acme.com/logo.png",
  "accentColor": "#10b981",
  "theme": "dark",
  "showUptimeBars": true,
  "showUptimePercentage": true,
  "showIncidentHistory": true,
  "incidentHistoryDays": 7,
  "customCss": ""
}
```

---

## Phase 4: Subscriber Notifications

**Goal:** Let visitors subscribe to status updates via email. This is the industry standard for proactive incident communication and a SOC2 checkbox.

### Tasks

#### 4.1 — Database: Subscribers Table
- **Files:** `internal/db/migrations/sqlite/00009_*.sql`, `internal/db/migrations/postgres/00009_*.sql`

```sql
CREATE TABLE status_page_subscribers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    status_page_slug TEXT NOT NULL,
    email TEXT NOT NULL,
    confirmed BOOLEAN DEFAULT FALSE,
    token TEXT UNIQUE NOT NULL,
    components TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(status_page_slug, email),
    FOREIGN KEY(status_page_slug) REFERENCES status_pages(slug) ON DELETE CASCADE
);
```

#### 4.2 — Backend: Subscription Endpoints
- **File:** `internal/api/handlers_subscribers.go` (new)
- **Public routes** (no auth):
  - `POST /api/s/{slug}/subscribe` — accepts `{email, components?}`, generates token, sends confirmation email
  - `GET /api/s/{slug}/confirm/{token}` — confirms subscription
  - `GET /api/s/{slug}/unsubscribe/{token}` — removes subscription
- **Admin routes** (auth required):
  - `GET /api/status-pages/{slug}/subscribers` — list subscribers
  - `DELETE /api/status-pages/{slug}/subscribers/{id}` — remove subscriber
- Rate limiting on subscribe endpoint

#### 4.3 — Backend: Email Notifier
- **File:** `internal/notifications/email.go` (new)
- SMTP config via env vars: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`
- HTML email templates:
  - Confirmation email (with confirm link)
  - Incident notification (new/updated)
  - Resolution notification
  - Scheduled maintenance reminder
- Every email includes one-click unsubscribe link (CAN-SPAM compliance)

#### 4.4 — Backend: Trigger Subscriber Notifications
- **File:** `internal/notifications/notifications.go`
- Dispatch to subscribers when:
  - New incident created (or auto-detected outage starts)
  - Incident status updated (investigating -> identified -> monitoring)
  - Incident resolved
  - Scheduled maintenance upcoming (24h before)
- Filter by component subscription preferences

#### 4.5 — Frontend: Subscribe Widget
- **File:** `web/src/components/status-page/SubscribeButton.tsx` (new)
- "Subscribe to updates" button in status page header
- Click opens dialog: email input, optional component checkboxes, submit
- Success: "Check your email to confirm your subscription"
- Error handling: duplicate email, invalid format, rate limited

#### 4.6 — Admin UI: Subscriber Management
- New tab/section in status page settings
- Table: email, confirmed status, subscribed components, date
- Delete button per subscriber
- Subscriber count badge on admin row

#### 4.7 — Tests
- Unit tests: subscriber store, email template rendering
- Integration test: full subscription flow

### Subscription Flow

```
User clicks "Subscribe" on status page
    |
    v
Enters email + optional component selection
    |
    v
POST /api/s/{slug}/subscribe
    |
    v
Server generates unique token, stores unconfirmed subscriber
    |
    v
Sends confirmation email with link
    |
    v
User clicks confirm link -> GET /api/s/{slug}/confirm/{token}
    |
    v
Subscriber marked as confirmed
    |
    v
Receives email notifications on incidents affecting subscribed components
    |
    v
Can unsubscribe via link in any email -> GET /api/s/{slug}/unsubscribe/{token}
```

---

## Files Modified/Created Per Phase

| File | Phase | Change |
|------|-------|--------|
| `internal/db/store_monitors.go` | 1 | `GetDailyUptimeStats()` |
| `internal/db/store_monitors_test.go` | 1 | 5 new tests |
| `internal/api/handlers_status_pages.go` | 1, 2, 3 | Extended API response |
| `web/src/components/status-page/UptimeBar.tsx` | 1 | New component |
| `web/src/components/status-page/StatusPage.tsx` | 1, 2, 3 | Full redesign |
| `internal/db/store_incidents.go` | 2 | `GetResolvedIncidents()` |
| `internal/db/store_status_pages.go` | 3 | Extended struct + queries |
| `internal/db/migrations/sqlite/00008_*.sql` | 3 | Config columns |
| `internal/db/migrations/postgres/00008_*.sql` | 3 | Config columns (PG) |
| `web/src/components/status-pages/StatusPageSettings.tsx` | 3 | New admin config UI |
| `web/src/components/status-pages/StatusPagesView.tsx` | 3 | Configure button |
| `internal/db/store_subscribers.go` | 4 | New subscriber CRUD |
| `internal/db/migrations/sqlite/00009_*.sql` | 4 | Subscribers table |
| `internal/db/migrations/postgres/00009_*.sql` | 4 | Subscribers table (PG) |
| `internal/api/handlers_subscribers.go` | 4 | New subscriber endpoints |
| `internal/api/router.go` | 4 | New routes |
| `internal/notifications/email.go` | 4 | SMTP email notifier |
| `internal/notifications/notifications.go` | 4 | Subscriber dispatch |
| `web/src/components/status-page/SubscribeButton.tsx` | 4 | Subscribe widget |

---

## Competitive Context

This redesign draws from patterns established by:

- **Atlassian Statuspage** — 90-day uptime bars, subscriber management, component groups
- **BetterStack** — Free tier, dark mode, historical data display
- **Instatus** — Fast loading, aggressive customization, multi-theme support
- **incident.io** — Incident workflow integration, pre-approved templates
- **Uptime Kuma** — Beautiful self-hosted UI (open source benchmark)

### Industry-Standard Features Being Added

| Feature | Phase | Industry Expectation |
|---------|-------|---------------------|
| 90-day uptime bars | 1 | THE most expected element |
| Overall uptime % | 1 | Trust signal, SOC2 evidence |
| Incident history | 2 | Shows response patterns |
| Custom branding | 3 | "Your company's" page |
| Theme selection | 3 | Dark/light/system |
| Email subscribers | 4 | Proactive communication |
| Component subscriptions | 4 | Targeted notifications |

### SOC2 Relevance

| SOC2 Requirement | Feature |
|-----------------|---------|
| Availability monitoring | Uptime bars + percentages |
| Incident documentation | Past incidents timeline |
| Proactive notifications | Subscriber emails |
| Change management | Maintenance window visibility |
| Audit trail | Incident lifecycle history |
