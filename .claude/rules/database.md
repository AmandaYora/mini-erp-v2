# Rule: Database

Applies to: `apps/api/**`.

- A repository accesses only the tables its own module owns.
- No cross-module joins; no cross-module foreign keys. Store cross-module relations as primitive IDs.
- Go: golang-migrate for migrations, sqlc for access, database/sql + MySQL driver. Not GORM.
- A transaction holds a single connection: fully read and close rows before
  issuing any other query on the same tx, or the connection deadlocks forever.
