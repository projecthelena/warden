# Design — Warden

A locked design system for Warden's authenticated application. Product pages use the same visual language; hierarchy and density change with the operator's task, not with arbitrary themes.

## Genre

Modern-minimal, technical and operational. The interface should feel calm during normal operation and become explicit when something needs attention.

## Macrostructure family

- App pages: Workbench. A compact context header followed by the surface that answers the operator's current question.
- Content pages: Long Document, using the same tokens and controls.
- Marketing pages: outside the scope of this system.

## Theme

Warden's existing dark and light shadcn themes remain authoritative. Cyan is reserved for selection, focus, links and primary actions. Health states use semantic green, amber, blue and red.

## Typography

- Display and body: DM Sans.
- Technical values: IBM Plex Mono.
- Headings use normal style, tight tracking and restrained sizes.
- Metrics use tabular numerals.

## Spacing

Use the named 4-point scale from `tokens.css`. Dense controls may use the compact values; major workbench regions use the larger values.

## Motion

- No entrance choreography.
- State transitions use opacity and transform only.
- Reduced-motion removes spatial movement.

## Microinteractions stance

- Silent success for routine saves, accompanied by an accessible toast.
- Persistent controls have visible focus states and at least 44px touch targets.
- Destructive irreversible actions keep confirmation.

## CTA voice

- Primary actions use the existing shadcn primary button.
- Secondary and operational actions use outline or ghost variants.
- Actions use short verb-first labels.

## Per-page allowances

- App pages use no decorative enrichment. Live data and status are the visual anchor.
- Charts may use a restrained cyan area fill and semantic failure markers.

## What pages MUST share

- Sidebar, breadcrumbs, page width and base themes.
- Typography, control geometry and semantic status colours.
- Tab treatment and form rhythm.
- Role-aware access to mutable controls.

## What pages MAY differ on

- Density and column count according to the task.
- Which metrics are promoted.
- Whether secondary information is inline or progressively disclosed.

## Monitor workspace

- Every monitor has one canonical URL: `/monitors/:id`.
- Overview answers health, performance and emerging risk.
- Incidents answers what failed and when.
- Settings answers how the check behaves, and is visible only to editors and admins.
- Pause, mute and delete remain separate actions because their consequences differ.

