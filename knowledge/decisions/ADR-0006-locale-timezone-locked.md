# ADR-0006: Locale, timezone, and currency are locked application constants

## Status
Accepted

## Context
The legacy system exposed timezone, currency, and system language as editable company settings,
but no part of the codebase actually read them — date formatting, currency formatting, and report
timezone were all hardcoded elsewhere. Changing the setting silently did nothing
(`docs/legacy-reference/known-issues.md` KI-28). This was already recognized and decided against
before this revamp started.

## Decision
`Asia/Jakarta`, `id-ID`, and IDR are application constants (`APP_TIMEZONE`, `APP_LOCALE` in
`.env.example`), applied consistently across every module — not a per-company database setting,
and not a UI-editable field.

## Consequences
One less settings surface to build, validate, and keep in sync with actual behavior. If genuine
multi-timezone or multi-currency operation is ever required, that is a new architectural decision
requiring its own ADR — not a toggle added to `company_settings`.
