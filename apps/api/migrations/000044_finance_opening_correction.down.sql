-- Hanya dilepas bila belum ada jurnal yang memakai akun 3100.
DELETE m FROM `finance_account_mappings` m
  JOIN `finance_accounts` a ON a.`id` = m.`account_id`
 WHERE m.`mapping_key` = 'opening_correction'
   AND NOT EXISTS (SELECT 1 FROM `finance_journal_lines` l WHERE l.`account_id` = a.`id`);

DELETE a FROM `finance_accounts` a
 WHERE a.`code` = '3100'
   AND NOT EXISTS (SELECT 1 FROM `finance_journal_lines` l WHERE l.`account_id` = a.`id`)
   AND NOT EXISTS (SELECT 1 FROM `finance_account_mappings` m WHERE m.`account_id` = a.`id`);
