-- Modul `finance` (L7) — dimensi analitik pada baris jurnal.
--
-- Tanpa ini, buku hanya bisa menjawab "berapa saldo akun X"; tidak bisa
-- menjawab "margin produk mana yang tertinggi" atau "siapa yang berhutang".
-- Sistem lama menaruh 6 kolom + metadata_json di baris jurnal; di sini hanya
-- dimensi yang benar-benar BERVARIASI DI DALAM satu entry yang disimpan di
-- baris. Dokumen sumber (order/payment/mutasi stok) sudah ada di level entry
-- lewat `source_doc_type` + `source_doc_id`, jadi menyalinnya ke tiap baris
-- hanya menduplikasi fakta yang sama. `metadata_json` sengaja tidak dibawa:
-- blob yang tidak bisa di-query bukan dimensi, itu tempat sampah.
--
-- Relasi lintas modul disimpan sebagai ID primitif tanpa FK fisik, sesuai
-- `.claude/rules/backend-modular-monolith.md`.

ALTER TABLE finance_journal_lines
  ADD COLUMN product_id  BIGINT UNSIGNED NULL AFTER account_id,
  ADD COLUMN variant_id  BIGINT UNSIGNED NULL AFTER product_id,
  ADD COLUMN party_id    BIGINT UNSIGNED NULL AFTER variant_id,
  ADD COLUMN description VARCHAR(255) NULL AFTER credit;

-- Indeks mengikuti bentuk query laporan, bukan bentuk tabel:
-- margin per produk memfilter akun lalu mengelompokkan produk; piutang/utang
-- memfilter akun lalu mengelompokkan party. Akun didahulukan di kedua indeks
-- karena selektivitasnya paling tinggi (satu akun dari belasan).
CREATE INDEX ix_finance_lines_account_product ON finance_journal_lines (account_id, product_id);
CREATE INDEX ix_finance_lines_account_party   ON finance_journal_lines (account_id, party_id);
