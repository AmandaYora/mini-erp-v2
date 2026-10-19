-- N-41: pemakaian saldo uang muka. Kedua leg pasangan menyebut retur sumber saldo.
ALTER TABLE `payments`
  ADD COLUMN `credit_source_type` varchar(20) DEFAULT NULL AFTER `pair_id`,
  ADD COLUMN `credit_source_id` bigint unsigned DEFAULT NULL AFTER `credit_source_type`,
  ADD KEY `ix_payments_credit_source` (`credit_source_type`,`credit_source_id`);
