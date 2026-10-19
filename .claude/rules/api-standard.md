# Rule: API standard

Applies to: `apps/api/**`.

- Version endpoints under `/api/v1`.
- Success: `{ success: true, message, data }`. Error: `{ success: false, message, errors }`.
- Paginated: include `meta: { page, limit, total, totalPages }`.
- Write envelopes only through `shared/response` (never a hand-written `{"success":` literal — arch-check Rule M).
- Parse `page`/`limit` with the `shared/pagination` helpers (`Parse`, `ParseDisplayLimit`), not ad hoc `strconv.Atoi`.
