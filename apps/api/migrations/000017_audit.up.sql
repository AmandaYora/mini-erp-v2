-- Modul `audit` (L0): jejak siapa-mengubah-apa. Tulis best-effort setelah
-- mutasi komit (tak pernah menggagalkan operasi). Tanpa company_id (ADR-0009).

CREATE TABLE audit_logs (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  action_key  VARCHAR(100) NOT NULL,
  entity_type VARCHAR(50)  NOT NULL,
  entity_id   BIGINT UNSIGNED NULL,
  branch_id   BIGINT UNSIGNED NULL,
  actor_id    BIGINT UNSIGNED NULL,
  note        VARCHAR(255) NULL,
  created_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY ix_audit_logs_branch_time (branch_id, created_at),
  KEY ix_audit_logs_action (action_key)
);
