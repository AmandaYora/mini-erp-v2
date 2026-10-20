---
paths:
  - "apps/api/**"
---

# Rule: API standard

- Version endpoints under `/api/v1`.
- Success: `{ success: true, message, data }`. Error: `{ success: false, message, errors }`.
- Paginated: include `meta: { page, limit, total, totalPages }`.
- Write envelopes only through `shared/response` (never a hand-written `{"success":` literal — arch-check Rule M).
- Parse `page`/`limit` with the `shared/pagination` helpers (`Parse`, `ParseDisplayLimit`), not ad hoc `strconv.Atoi`.
- A new or changed route must be added to `docs/API_CONTRACT.md` §6 in the same change, by hand —
  `npm run contract:check` matches frontend calls to the router but does not read that table.
