---
paths:
  - "apps/api/**"
---

# Rule: Database

- A repository accesses only the tables its own module owns.
- No cross-module joins; no cross-module foreign keys. Store cross-module relations as primitive IDs.
- Go: golang-migrate for migrations, sqlc for access, database/sql + MySQL driver. Not GORM.
  Modules listed `migrated` in `scripts/sqlc-migration-status.json` use no raw SQL outside
  `scripts/raw-sql-allow.json` (ADR-0012).
- One migration touches one module's tables; its file name `NNNNNN_<module>[_<change>]` names the owner.
  Never edit a released migration. After the table list changes, run `node scripts/db-schema-doc.mjs`.
- A transaction holds a single connection: fully read and close rows before
  issuing any other query on the same tx, or the connection deadlocks forever.

Details: `knowledge/DATABASE_GUIDE.md`.
