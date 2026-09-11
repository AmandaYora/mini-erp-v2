-- Modul `media` (L0): metadata file + driver penyimpanan lokal.
-- Tanpa company_id (ADR-0009). Key dibuat server (uuid) — anti traversal.

CREATE TABLE media_files (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  owner_type    VARCHAR(50)  NOT NULL,
  owner_id      BIGINT UNSIGNED NOT NULL,
  file_key      VARCHAR(255) NOT NULL,
  original_name VARCHAR(255) NOT NULL,
  mime          VARCHAR(100) NOT NULL,
  size_bytes    BIGINT NOT NULL DEFAULT 0,
  is_primary    TINYINT(1)   NOT NULL DEFAULT 0,
  created_at    DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by    BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_media_files_key (file_key),
  KEY ix_media_files_owner (owner_type, owner_id)
);
