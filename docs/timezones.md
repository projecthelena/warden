# Timezone model

Time has two different meanings in an uptime monitor:

- An **instant** is an event that happened at one exact point in time: a check, an outage, an incident update, or the beginning and end of a maintenance window.
- A **wall-clock schedule** is a human instruction such as “send the digest at 09:00 Bogotá time” or “run this recurring maintenance every Sunday at 02:00.” It needs both a local date or recurrence rule and an IANA timezone.

Warden must not treat those as the same kind of value.

## Product decision

Warden uses the following contract:

1. Store every instant in UTC.
2. Exchange instants through the API as RFC 3339 timestamps with an explicit offset, normally normalized to `Z`.
3. Render the authenticated application in the signed-in user's configured IANA timezone.
4. Give each public status page its own fixed IANA timezone. Every visitor sees the same published schedule.
5. Use a workspace timezone for shared automation such as digests and scheduled reports. Do not derive shared automation from whichever user happens to be first in the database.
6. Store recurring wall-clock schedules with their IANA timezone and rule, then calculate each occurrence as a UTC instant.

IANA identifiers such as `America/Bogota` and `Europe/Berlin` are required. Fixed labels such as `EST` are not sufficient because they do not reliably encode daylight-saving rules.

## Why public status pages are different

An authenticated dashboard belongs to a person, so two operators can view the same outage in their own timezones without changing its meaning.

A public status page is a publication. A maintenance announcement must have one canonical interpretation across visitors, screenshots, support conversations, RSS, and notification messages. Warden will therefore use the status page's configured timezone rather than silently changing the time according to each visitor's browser.

The page should display the timezone next to absolute dates. A future convenience feature may also show the visitor's local equivalent, but it must be secondary to the page's canonical time.

## Current implementation

The following describes the current maintenance lifecycle implementation.

| Surface | Current behavior | Target behavior |
| --- | --- | --- |
| Monitor checks and events | Check timestamps are generated as UTC instants. APIs return RFC 3339 values. | Keep this behavior and audit every database write path. |
| Outages and incidents | Most application-owned writes use UTC. Incident and maintenance API inputs require RFC 3339 and maintenance inputs are normalized to UTC. | Guarantee UTC at every write boundary and test SQLite and PostgreSQL equally. |
| Authenticated dashboard | Dates generally use the signed-in user's `users.timezone`. | Use that timezone consistently on every dashboard surface. |
| Maintenance creation and editing | The form treats entered date and time as wall time in the creator's configured timezone, then sends a UTC instant. | Keep this behavior. Another operator may see the same instant in a different timezone. |
| Maintenance lifecycle | Active state is calculated from UTC instants. It does not depend on display timezone. | Keep one shared lifecycle predicate across dashboard, monitor cards, and status pages. |
| Public status pages | Maintenance and incident dates currently fall back to UTC because status pages have no timezone setting. Some date grouping also uses UTC/browser defaults. | Add a timezone to each status page and use it for all dates, grouping, “Today/Yesterday,” RSS, and subscriber communication. |
| Daily digests and weekly insights | The manager loads the first user's timezone and treats it as the notification timezone. | Replace this implicit behavior with an explicit workspace timezone. |
| API clients and MCP | Instants are represented as RFC 3339 values and should be treated as UTC. | Document the UTC contract and reject timestamps without an offset. |

### Database reality

SQLite `DATETIME` columns are not timezone-aware, and Warden's PostgreSQL schema currently uses `TIMESTAMP` rather than `TIMESTAMPTZ`. Consequently, the schema does not enforce UTC by itself. The application already stamps critical outage paths with `time.Now().UTC()` and normalizes maintenance inputs, but the remaining default-generated and application-generated timestamps need an audit.

The target invariant is that every instant read into Go represents UTC. A future paired SQLite/PostgreSQL migration should harden PostgreSQL instant columns to `TIMESTAMPTZ` after verifying existing values. SQLite will continue storing normalized UTC values because it has no equivalent timezone-aware type.

## Industry patterns

There is no single UI choice across monitoring products, but the durable architecture is consistent: separate stored instants from display timezone.

- [Uptime Kuma documents](https://github.com/louislam/uptime-kuma/wiki/Maintenance) a timezone on maintenance schedules, with server timezone as the default. Its maintainer also states that dates are stored in UTC and timezone conversion happens in the frontend in [the timezone support discussion](https://github.com/louislam/uptime-kuma/issues/40).
- [Better Stack's status-page API](https://betterstack.com/docs/uptime/api/list-all-existing-status-pages/) exposes a timezone as an attribute of each status page. This matches the need for one canonical public communication timezone.
- [Atlassian Statuspage](https://support.atlassian.com/statuspage/docs/read-the-statuspage-user-guide/) exposes timezone among the page's global settings. Its maintenance lifecycle can automatically move from scheduled to in progress and completed at the configured boundaries, as described in [its maintenance documentation](https://support.atlassian.com/statuspage/docs/schedule-maintenance/).
- [Grafana](https://grafana.com/docs/grafana-cloud/learn-and-build/visualizations/dashboards/build-dashboards/modify-dashboard-settings/) supports browser time, UTC, and fixed timezones, with profile, team, and organization fallbacks. Its reporting documentation warns that browser time is unsuitable for server-generated reports because the server cannot know the viewer's browser timezone.
- Better Stack groups some automatic reports by UTC day, according to its [status update documentation](https://betterstack.com/docs/uptime/creating-status-report-and-status-update/). This is useful for internal aggregation, but it is distinct from how published timestamps are presented.

The lesson for Warden is to avoid one global timezone that tries to serve storage, operators, public viewers, and automation simultaneously.

## Detailed behavior

### Authenticated operators

- Setup detects the browser timezone as an initial suggestion.
- Each user can change their timezone under **Settings → General**.
- The preference is stored as an IANA timezone on the user record.
- Checks, events, incidents, outages, and maintenance windows are converted only when rendered.
- Changing the preference changes presentation, not stored timestamps or maintenance boundaries.

### Maintenance windows

For a one-time maintenance window:

1. The operator enters a local date and time.
2. The UI labels the timezone being used.
3. The UI converts both boundaries to RFC 3339 UTC instants.
4. The API validates that both timestamps contain an offset and that the end is after the start.
5. The database stores the normalized instants.
6. Active state is evaluated by comparing the current UTC instant with those boundaries.
7. Every surface renders the same instants in its own presentation timezone.

One-time maintenance keeps the selected instant even if timezone rules later change. Recurring maintenance is different: it must retain the timezone and wall-clock rule so future occurrences continue to mean, for example, “02:00 Europe/Berlin.”

### Daylight-saving transitions

IANA timezone rules handle ordinary daylight-saving changes. Input still needs explicit behavior for exceptional local times:

- A nonexistent time during a spring-forward transition must be rejected with an explanation and a suggested valid time.
- A duplicated time during a fall-back transition must ask which offset the operator means.
- UTC and zones without daylight-saving changes, such as `America/Bogota`, are unambiguous.

Silently shifting an invalid maintenance time is not acceptable because it changes the announced window.

### Public status pages

Each status page will have a `timezone` configuration field.

- Existing pages initially remain on `UTC` to preserve their current published meaning.
- New pages default to the workspace timezone.
- All page timestamps use the page timezone, including maintenance, incidents, incident updates, history grouping, and “Today/Yesterday.”
- RSS and subscriber messages identify the timezone or include an unambiguous RFC 3339 timestamp.
- Status calculations remain UTC-based and are never affected by display formatting.

### Notifications and scheduled automation

Direct alerts describe an instant and can include UTC/RFC 3339 for machines plus a formatted workspace time for people. Daily digests, weekly insights, reminders, and future recurring maintenance use the explicit workspace timezone because they are shared automation, not personal UI.

If Warden later supports per-user notification delivery, personal notifications may use the recipient's timezone. Channel-wide notifications must remain on the workspace timezone.

## Delivery plan

### Phase 1: maintenance correctness

- Normalize maintenance boundaries to UTC.
- Use the user's timezone when creating, editing, and viewing maintenance in the authenticated UI.
- Make active/expired state independent of status text and display timezone.
- Remove expired maintenance from current monitor and public status state.
- Cover timezone selection, persistence, monitor state, status pages, history, and deletion with unit and E2E tests.

### Phase 2: canonical status-page timezone

- Add a paired SQLite/PostgreSQL migration for status-page timezone configuration.
- Add the setting to status-page administration.
- Return it in public status-page configuration.
- Apply it to maintenance, incidents, updates, date grouping, RSS, and subscription messages.
- Add E2E coverage using a browser timezone different from the page timezone.

### Phase 3: workspace automation timezone

- Add an explicit workspace timezone setting.
- Migrate the current first-user notification behavior to that setting without changing scheduled delivery unexpectedly.
- Use it for digests, weekly insights, reminders, and defaults for new status pages.

### Phase 4: storage hardening

- Inventory every persisted instant and database default.
- Ensure all Go write paths normalize to UTC.
- Convert PostgreSQL instant columns to `TIMESTAMPTZ` with a paired SQLite no-op or equivalent migration.
- Add cross-database tests around non-UTC PostgreSQL sessions and daylight-saving boundaries.

## Required regression coverage

Timezone work is complete only when tests prove:

- UTC-to-IANA conversion and round trips.
- A user can select a timezone, save it, reload, and retain it, including an SSO user.
- Two users in different timezones see different labels for the same stored instant.
- A public status page renders its configured timezone regardless of browser timezone.
- Maintenance enters and leaves active state at the same UTC boundaries on dashboard, monitor cards, status pages, and notifications.
- Invalid and ambiguous daylight-saving inputs are handled deliberately.
- SQLite and PostgreSQL return equivalent instants.
