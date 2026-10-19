# ADR-0017: Design System v2 "Bingkai Navy" on shadcn/ui (Radix)

## Status
Accepted (2026-10-16). Plan and execution notes: `docs/plan/redesain-ui/PLAN.md`.

## Context
The UI was judged dated, cluttered and uncomfortable. The diagnosis (PLAN §1) showed the
problem was systemic, not per page: an old-looking frame (near-black flush sidebar, gradients,
two `<h1>` per page), missing primitives (popover, sheet, tabs, skeleton, checkbox) that every
page re-assembled differently, and no placement rules (actions in three places, four "back"
styles, create flows split between pages and xl modals).

## Decision
1. **Foundation: shadcn/ui with Radix primitives**, generated into
   `apps/web/src/shared/components/primitives/` and treated as implementation detail.
   Modules and `app/` never import `primitives/*` (ESLint `no-restricted-imports`); they use the
   facade (`shared/components/ui`, names and props kept) and `shared/components/patterns`.
2. **Tokens**: one source, `src/theme/theme.css`, OKLCH, shadcn naming (`background`, `card`,
   `primary`, `muted-foreground`, `sidebar-*`, `chart-1..5`) plus `success|warning|destructive|info`
   (+ `-soft`). Brand stays navy `#1e3a5f`. Components use semantic classes only; no hex, no Tailwind
   palette colors. All text/background pairs are measured against WCAG 2.2 AA (PLAN Appendix C).
   Tailwind preflight is ON; print pages restore UA defaults via `LEGACY_UA_CSS` so paper never moves.
3. **Frame**: dark-navy `inset` sidebar, content on a floating light canvas, breadcrumb header
   (the page `<h1>` lives only in `PageHeader`), command palette (Ctrl+K), quick-create.
4. **Typography**: Plus Jakarta Sans (titles, KPI numbers), Inter (UI), Geist Mono (document
   numbers); self-hosted via `@fontsource-variable/*` (the app runs in one container, often on a
   shop LAN). Type scale `text-2xs … text-4xl`, no arbitrary sizes.
5. **Status semantics** (one map, `statusTone`): neutral = not started, info = in progress,
   warning = needs attention, success = done, muted = stopped (cancelled/archived), danger = problem.
6. **Placement rules**: dialog for ≤ 6 fields in one step; side sheet for master-data CRUD and actions
   on a document; full page for line-item documents (returns, delivery, receipt, transfer, manual
   journal). Document pages show a `DocumentRail` (lifecycle + related documents).
7. **POS** is a full-screen "Mode Kasir" for landscape touch tablets outside the app shell; camera
   scanning (HTTPS required), on-screen numpad, 48px targets.
8. **Dark theme is deferred**; reference values are in PLAN Appendix E.
9. No new backend endpoints: saved state (density, recent documents, column choice) is
   `localStorage`.

## Consequences
- `scripts/ui-ratchet.mjs` (+ `scripts/ui-migration-status.json`, part of `npm run verify`) counts
  leftover legacy patterns per module and fails when a count rises.
- A shadcn upgrade is a diff against `primitives/`; customisations live in the facade/patterns.
- Native `<select>` and native date inputs stay (keyboard speed, mobile, test stability); searchable
  selects are Radix Popover based so they work inside dialogs.
