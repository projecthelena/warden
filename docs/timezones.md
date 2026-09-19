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

Public status pages currently display dates in **UTC**. This gives every visitor the same published time, regardless of their device or location.

A future update will let you choose a timezone for each status page. All visitors will continue to see the same time chosen by the page owner.

## Notifications and scheduled reports

Warden currently uses the primary administrator's timezone for shared scheduled messages, such as digests and reports.

A future update will add a separate workspace timezone for these shared schedules.

## Recommended setup

1. Open **Settings → General**.
2. Select your city or region, for example `America/Bogota`.
3. Save the changes.
4. Check the timezone shown when scheduling maintenance.

Use a city or region instead of a short abbreviation such as `EST`, because regional timezones automatically follow local clock changes.
