-- AS-02: a tool that fails is recorded as failed with its error text (legacy
-- tool_execution.status/error_text). Existing rows are successes.
ALTER TABLE `assistant_tool_executions`
  ADD COLUMN `status` varchar(20) NOT NULL DEFAULT 'success' AFTER `output_json`,
  ADD COLUMN `error_text` text NULL AFTER `status`;
