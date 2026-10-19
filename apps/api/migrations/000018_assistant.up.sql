-- Baseline modul assistant (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `assistant_authorizations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `phone` varchar(20) NOT NULL,
  `name` varchar(150) NOT NULL,
  `access_level` varchar(20) NOT NULL DEFAULT 'authorized_party',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `is_primary_owner` tinyint(1) NOT NULL DEFAULT '0',
  `last_seen_at` datetime(6) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_assistant_authorizations_phone` (`phone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `assistant_channel` (
  `id` tinyint unsigned NOT NULL DEFAULT '1',
  `state` varchar(20) NOT NULL DEFAULT 'disconnected',
  `phone` varchar(20) DEFAULT NULL,
  `last_error` varchar(255) DEFAULT NULL,
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  CONSTRAINT `chk_assistant_channel_single` CHECK ((`id` = 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `assistant_config` (
  `id` tinyint unsigned NOT NULL DEFAULT '1',
  `mode` varchar(20) NOT NULL DEFAULT 'rule_based',
  `rate_limit_per_minute` int unsigned NOT NULL DEFAULT '10',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  CONSTRAINT `chk_assistant_config_single` CHECK ((`id` = 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `assistant_messages` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `thread_id` bigint unsigned NOT NULL,
  `direction` varchar(10) NOT NULL,
  `body` text NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'received',
  `run_id` bigint unsigned DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `ix_assistant_messages_thread` (`thread_id`),
  KEY `ix_assistant_messages_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `assistant_runs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `thread_id` bigint unsigned DEFAULT NULL,
  `phone` varchar(20) DEFAULT NULL,
  `branch_id` bigint unsigned DEFAULT NULL,
  `actor_id` bigint unsigned NOT NULL DEFAULT '0',
  `intent` varchar(50) NOT NULL,
  `mode` varchar(20) NOT NULL DEFAULT 'rule_based',
  `status` varchar(20) NOT NULL DEFAULT 'running',
  `answer` mediumtext,
  `duration_ms` int unsigned NOT NULL DEFAULT '0',
  `failure_reason` varchar(255) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `ix_assistant_runs_created` (`created_at`),
  KEY `ix_assistant_runs_intent` (`intent`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `assistant_threads` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `phone` varchar(20) NOT NULL,
  `branch_id` bigint unsigned DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'open',
  `last_message_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `ix_assistant_threads_phone` (`phone`),
  KEY `ix_assistant_threads_last` (`last_message_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `assistant_tool_executions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `run_id` bigint unsigned NOT NULL,
  `seq` int unsigned NOT NULL DEFAULT '1',
  `tool` varchar(50) NOT NULL,
  `input_json` text NOT NULL,
  `output_json` text,
  `duration_ms` int unsigned NOT NULL DEFAULT '0',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `ix_assistant_tool_executions_run` (`run_id`),
  KEY `ix_assistant_tool_executions_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
