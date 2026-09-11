# Rule: Backend modular monolith

Applies to: `apps/api/**`.

- Each module is a boundary. Only `contracts/` is public to other modules.
- Forbidden: importing another module's application/infrastructure/domain internals.
- Forbidden: cross-module DB joins and cross-module foreign keys.
- Cross-module relations are primitive IDs; resolve via module clients.
- Transactions are scoped inside a single module; orchestrate cross-module flows via app services.
- `shared/` holds only technical utilities (errors, response, logger, validator, pagination, utils) — never domain logic.
