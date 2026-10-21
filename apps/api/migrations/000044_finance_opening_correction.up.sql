-- Koreksi saldo awal (ADR-0024): akun ekuitas 3100 dan kunci pemetaan
-- opening_correction untuk koreksi stok awal yang salah hitung. Hanya untuk
-- database yang sudah punya bagan akun (ada pemetaan opening_equity); basis
-- data baru mendapatkannya dari `seed init` (DefaultChart) atau dari ETL.
INSERT INTO `finance_accounts` (`code`, `name`, `type`, `is_cash`, `status`)
SELECT '3100', 'Koreksi Saldo Awal', 'modal', 0, 'active' FROM DUAL
 WHERE EXISTS (SELECT 1 FROM `finance_account_mappings` WHERE `mapping_key` = 'opening_equity')
   AND NOT EXISTS (SELECT 1 FROM `finance_accounts` WHERE `code` = '3100');

INSERT INTO `finance_account_mappings` (`mapping_key`, `account_id`)
SELECT 'opening_correction', a.`id` FROM `finance_accounts` a
 WHERE a.`code` = '3100' AND a.`type` = 'modal'
   AND NOT EXISTS (SELECT 1 FROM `finance_account_mappings` WHERE `mapping_key` = 'opening_correction');
