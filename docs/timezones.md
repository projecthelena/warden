# Timezones

Warden keeps every event at the correct moment and shows it in the timezone that applies to each screen.

## Your dashboard

The dashboard uses the timezone selected in **Settings → General → Timezone**.

This applies to monitor activity, incidents, maintenance windows, and history. Changing your timezone only changes how dates and times are displayed; it does not move or modify any event.

## Maintenance windows

Enter the start and end time using your configured timezone. Warden shows the timezone beside the fields so it is clear which time is being used.

Other team members may see a different clock time if they use another timezone, but everyone is viewing the same maintenance window.

When a maintenance window ends:

- Monitors stop showing the maintenance state.
- Public status pages stop showing it as active.
- The window moves to **History**, where it can be reviewed or deleted.

## Public status pages

Each status page has its own timezone, configured under **Status Pages → Configure Status Page → Timezone**.

Every visitor sees dates in that page's timezone, regardless of their device or location. For example, if a page uses `America/Bogota`, a visitor in Japan still sees the official Bogotá time.

This keeps maintenance announcements, screenshots, notifications, and support conversations consistent. Warden does not currently show a second conversion to the visitor's local timezone.

## Notifications and scheduled reports

Shared schedules use the timezone selected under **Settings → Notifications → Workspace timezone**.

This applies to daily digests and weekly summaries. It is separate from each user's dashboard timezone because these messages are shared by the workspace.

## Recommended setup

1. Open **Settings → General**.
2. Select your city or region, for example `America/Bogota`.
3. Save the changes.
4. Check the timezone shown when scheduling maintenance.
5. Choose the official timezone for each public status page.
6. Choose the workspace timezone for digests and weekly summaries.

Use a city or region instead of a short abbreviation such as `EST`, because regional timezones automatically follow local clock changes.
