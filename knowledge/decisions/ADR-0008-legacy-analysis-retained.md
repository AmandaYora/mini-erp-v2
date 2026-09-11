# ADR-0008: Legacy analysis retained in-repo as `docs/legacy-reference/`

## Status
Accepted

## Context
Before this revamp started, the legacy system (`../mini-erp`) was analyzed exhaustively:
21 modules, ~26,100 lines of documentation, 149 known issues, 130 open questions
(`../mini-erp/docs/legacy-analysis/`). Every future module design needs this material — feature
inventories, business rules, and explicitly which behaviors are bugs vs. intentional habits.

## Decision
Copy the full legacy analysis into this repo as `docs/legacy-reference/` (read-only reference,
not re-generated here) instead of only linking to the old repo path. It is small (~2.2 MB) and
becomes stale-proof once the old repo is eventually archived or removed.

## Consequences
Every module's design phase has one required first stop:
`docs/legacy-reference/<module>/known-issues.md` (via `docs/legacy-reference/known-issues.md`) and
`open-questions.md`, cross-referenced from `knowledge/MODULE_MAP.md`. This is reference material
only — it describes the old system's behavior, not a spec for the new one. Do not edit these files;
if the old repo's analysis is later updated, re-copy rather than diverge.
