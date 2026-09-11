-- Modul `company` (L1): profil + pengaturan perusahaan tunggal (singleton).
-- Tanpa tabel `companies`, tanpa company_id (ADR-0009). Tanpa zona waktu/mata uang/
-- bahasa/feature-flag (konstanta aplikasi — ADR-0006, KI-28/KI-33).

CREATE TABLE company_profile (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name       VARCHAR(150) NOT NULL,
  legal_name VARCHAR(150) NULL,
  address    VARCHAR(255) NULL,
  city       VARCHAR(100) NULL,
  phone      VARCHAR(30)  NULL,
  email      VARCHAR(255) NULL,
  tax_id     VARCHAR(50)  NULL,
  created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by BIGINT UNSIGNED NULL,
  updated_by BIGINT UNSIGNED NULL,
  PRIMARY KEY (id)
);

CREATE TABLE company_settings (
  setting_key   VARCHAR(100) NOT NULL,
  setting_value TEXT NULL,
  updated_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_by    BIGINT UNSIGNED NULL,
  PRIMARY KEY (setting_key)
);
