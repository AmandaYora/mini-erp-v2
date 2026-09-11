# ADR-0005: Money stored as integer rupiah

## Status
Accepted

## Context
The legacy system had two money-rounding standards living side by side (whole rupiah in some
places, 2-decimal in others — `docs/legacy-reference/00-overview.md` §6), which the legacy team
itself later had to patch with a dedicated migration
(`043_round_money_to_whole_rupiah` in the old repo). Indonesian Rupiah has no practical subunit in
day-to-day ERP usage.

## Decision
All monetary columns are stored as whole-rupiah integers (no decimal places). This is the single
standard from the first migration — not something patched in later.

## Consequences
No rounding ambiguity between modules. If a future requirement genuinely needs sub-rupiah
precision (e.g. weighted unit cost before rounding), that precision lives in a clearly-named
intermediate calculation column, never in the stored transaction amount.
