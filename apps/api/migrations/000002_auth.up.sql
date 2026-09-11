-- Modul `auth` (L1): sesi aktif + hash refresh token yang dirotasi tiap pakai.
-- user_id / branch / role disimpan sebagai primitive ID tanpa FK lintas modul.

CREATE TABLE user_sessions (
  id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id             BIGINT UNSIGNED NOT NULL,
  refresh_hash        CHAR(64)    NOT NULL,
  prev_refresh_hash   CHAR(64)    NULL,
  id_active_branch    BIGINT UNSIGNED NULL,
  id_active_role      BIGINT UNSIGNED NULL,
  expires_at          DATETIME(6) NOT NULL,
  absolute_expires_at DATETIME(6) NOT NULL,
  last_activity_at    DATETIME(6) NOT NULL,
  revoked_at          DATETIME(6) NULL,
  created_at          DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_user_sessions_refresh (refresh_hash),
  KEY ix_user_sessions_user (user_id)
);
