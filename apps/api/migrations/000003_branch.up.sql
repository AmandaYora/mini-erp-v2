-- Modul `branch` (L1): direktori cabang + penghitung nomor dokumen per cabang.
-- Cabang bisa ditutup (status), bukan permanen aktif (KI-36).

CREATE TABLE branches (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code       VARCHAR(10)  NOT NULL,
  name       VARCHAR(150) NOT NULL,
  address    VARCHAR(255) NULL,
  city       VARCHAR(100) NULL,
  phone      VARCHAR(30)  NULL,
  status     VARCHAR(20)  NOT NULL DEFAULT 'active',
  is_head    TINYINT(1)   NOT NULL DEFAULT 0,
  created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by BIGINT UNSIGNED NULL,
  updated_by BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_branches_code (code)
);

CREATE TABLE branch_document_sequences (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id     BIGINT UNSIGNED NOT NULL,
  doc_kind      VARCHAR(30) NOT NULL,
  current_value BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uq_branch_sequences (branch_id, doc_kind),
  CONSTRAINT fk_branch_sequences_branch FOREIGN KEY (branch_id) REFERENCES branches (id)
);
