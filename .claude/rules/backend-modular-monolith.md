---
paths:
  - "apps/api/**"
---

# Rule: Backend modular monolith

- Each module is a boundary. Only `contracts/` is public to other modules.
- Forbidden: importing another module's application/infrastructure/domain internals.
- Forbidden: cross-module DB joins and cross-module foreign keys.
- Cross-module relations are primitive IDs; resolve via module clients.
- Transactions are scoped inside a single module; orchestrate cross-module flows via app services,
  with idempotent steps + CAS (ADR-0016). Audit writes join the caller's transaction (ADR-0011).
- `shared/` holds only technical utilities (errors, response, logger, validator, pagination, utils) — never domain logic.
  The one exception is `shared/money` (ADR-0010).
- `scripts/arch-check.mjs` must stay green; exceptions go in `scripts/arch-allow.json` with a reason.
