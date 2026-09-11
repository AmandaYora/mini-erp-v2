-- Modul `assistant` (L9): BOT WhatsApp (whatsmeow) + tooling operasional.
-- Tanpa RAG/AI/knowledge (keputusan skop L9). Tanpa company_id (ADR-0009):
-- standalone single-tenant, satu sesi WA untuk seluruh instalasi.
-- Tanpa FK (konvensi modul): relasi thread→message→run hanya ID primitif.

CREATE TABLE assistant_channel (
  id            TINYINT UNSIGNED NOT NULL DEFAULT 1,
  state         VARCHAR(20)  NOT NULL DEFAULT 'disconnected',
  phone         VARCHAR(20)  NULL,
  last_error    VARCHAR(255) NULL,
  updated_at    DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  CONSTRAINT chk_assistant_channel_single CHECK (id = 1)
);

CREATE TABLE assistant_config (
  id                   TINYINT UNSIGNED NOT NULL DEFAULT 1,
  mode                 VARCHAR(20)  NOT NULL DEFAULT 'rule_based',
  rate_limit_per_minute INT UNSIGNED NOT NULL DEFAULT 10,
  updated_at           DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  CONSTRAINT chk_assistant_config_single CHECK (id = 1)
);

CREATE TABLE assistant_authorizations (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  phone             VARCHAR(20)  NOT NULL,
  name              VARCHAR(150) NOT NULL,
  access_level      VARCHAR(20)  NOT NULL DEFAULT 'authorized_party',
  status            VARCHAR(20)  NOT NULL DEFAULT 'active',
  is_primary_owner  TINYINT(1)   NOT NULL DEFAULT 0,
  last_seen_at      DATETIME(6)  NULL,
  created_at        DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at        DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  created_by        BIGINT UNSIGNED NULL,
  updated_by        BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_assistant_authorizations_phone (phone)
);

CREATE TABLE assistant_threads (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  phone           VARCHAR(20)  NOT NULL,
  branch_id       BIGINT UNSIGNED NULL,
  status          VARCHAR(20)  NOT NULL DEFAULT 'open',
  last_message_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_at      DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY ix_assistant_threads_phone (phone)
);

CREATE TABLE assistant_messages (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  thread_id  BIGINT UNSIGNED NOT NULL,
  direction  VARCHAR(10)  NOT NULL,
  body       TEXT         NOT NULL,
  status     VARCHAR(20)  NOT NULL DEFAULT 'received',
  run_id     BIGINT UNSIGNED NULL,
  created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY ix_assistant_messages_thread (thread_id)
);

CREATE TABLE assistant_runs (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  thread_id      BIGINT UNSIGNED NULL,
  phone          VARCHAR(20)  NULL,
  branch_id      BIGINT UNSIGNED NULL,
  actor_id       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  intent         VARCHAR(50)  NOT NULL,
  mode           VARCHAR(20)  NOT NULL DEFAULT 'rule_based',
  status         VARCHAR(20)  NOT NULL DEFAULT 'running',
  answer         MEDIUMTEXT   NULL,
  duration_ms    INT UNSIGNED NOT NULL DEFAULT 0,
  failure_reason VARCHAR(255) NULL,
  created_at     DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY ix_assistant_runs_created (created_at),
  KEY ix_assistant_runs_intent (intent)
);

CREATE TABLE assistant_tool_executions (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id      BIGINT UNSIGNED NOT NULL,
  seq         INT UNSIGNED NOT NULL DEFAULT 1,
  tool        VARCHAR(50)  NOT NULL,
  input_json  TEXT         NOT NULL,
  output_json TEXT         NULL,
  duration_ms INT UNSIGNED NOT NULL DEFAULT 0,
  created_at  DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY ix_assistant_tool_executions_run (run_id)
);
