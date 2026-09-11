# Data Model Legacy (Konsep) — Modul 20 Audit Log

**Kelompok B — konsep saja**, bukan skema persis. DDL: `001_baseline.sql:428-443`;
entity: `audit-log.entity.ts` (penuh, 41 baris).

## 1. Tabel Milik Sendiri (1)

`audit_logs` — 12 kolom (11 data + PK), 1 indeks sekunder, tanpa FK:

| Kolom | DDL (`001_baseline.sql`) | Entity (`audit-log.entity.ts`) | Keterangan |
|---|---|---|---|
| `id_audit_log` | `INT NOT NULL AUTO_INCREMENT`, PK | `@PrimaryGeneratedColumn` → `id: number` (`:6-7`) | ID teknis, tak tampil di UI (kunci baris tabel = id internal) |
| `id_company` | `INT NOT NULL` | `idCompany: number` (`:9-10`) | Scope wajib — semua baca difilter perusahaan |
| `id_branch` | `INT DEFAULT NULL` | `idBranch: number \| null`, nullable (`:12-13`) | NULL untuk master lintas-cabang; cabang sesi untuk operasional |
| `actor_type` | `VARCHAR(30) NOT NULL` | `actorType: ActorType` (`:15-16`) | Praktiknya selalu `'user'` (102/102 situs); `'system'` hanya dari skrip SQL |
| `id_actor` | `INT DEFAULT NULL` | `idActor: number \| null`, nullable (`:18-19`) | NULL → UI `System`; id tak dikenal → `User #{id}` |
| `action_key` | `VARCHAR(100) NOT NULL` | `actionKey: string`, length 100 (`:21-22`) | 103 nilai runtime (90 statis + 12 dinamis + 1 skrip) |
| `entity_type` | `VARCHAR(50) NOT NULL` | `entityType: string`, length 50 (`:24-25`) | 32 nilai aktual (lihat §3) |
| `id_entity` | `INT DEFAULT NULL` | `idEntity: number \| null`, nullable (`:27-28`) | NULL untuk aksi tanpa satu entitas (mis. sync/close_day) |
| `before_json` | `JSON DEFAULT NULL` | `before`, json nullable (`:30-31`) | Snapshot sebelum (update/arsip sensitif) |
| `after_json` | `JSON DEFAULT NULL` | `after`, json nullable (`:33-34`) | Ringkasan sesudah (nomor/nama/jumlah/id) |
| `metadata_json` | `JSON DEFAULT NULL` | `metadata`, nullable (`:36-37`) | Konteks tambahan (alasan batal, jumlah sesi dicabut, nomor jurnal) |
| `happened_at` | `DATETIME(3) NOT NULL` | `happenedAt: Date`, `datetime precision: 6` (`:39-40`) | Stempel server `new Date()` (`audit-log.service.ts:38`); anomali presisi (3 vs 6) tak berpengaruh perilaku |
| Indeks | `KEY idx_audit_logs (id_company, id_branch, happened_at)` (`:442`) | — (tanpa dekorator indeks di entity!) | Filter aksi/entitas tanpa indeks (KI-129) |

## 2. Relasi (tidak ada FK!)

- Tanpa FK ke tabel mana pun — `id_actor`/`id_entity`/`id_branch` hanya angka
  (pengguna terhapus pun baris tetap valid; nama di-resolve di runtime dengan
  fallback `User #`/`Branch #`). Ini **sengaja**: jejak harus lestari melewati
  penghapusan/arsip master.
- Anomali kecil: `happened_at` = `DATETIME(3)` di migrasi vs `precision: 6` di
  entity ([PERLU KONFIRMASI] nilai berjalan — tak berpengaruh perilaku).
- Anomali kecil kedua: entity mendeklarasikan TANPA indeks (`audit-log.entity.ts`
  tanpa `@Index`) — indeks hanya ada di DDL migrasi; sinkronisasi skema via
  `synchronize` (cek! apakah dipakai) akan menghilangkan `idx_audit_logs`.

## 3. Nilai `entity_type` Aktual (32, hasil enumerasi situs produksi)

`order`(10 situs) `payment`(7) `delivery_note`(5) `finance_posting_source`(5)
`inventory_balance`(5) `business_party`(4, templat dinamis) `role`(4) `user`(4)
`product`(4) `stock_location`(4) `stock_transfer`(4) `media_file`(4: 3 foto produk +
1 bukti serah) `product_category`(3) `finance_account`(3) `finance_cash_account`(3)
`finance_opening_balance`(3) `finance_period`(3) `finance_tax_adjustment`(3)
`member_type`(3) `business_party_address`(3) `knowledge_document`(3)
`business_expense`(2) `branch`(2) `finance_journal_entry`(2) `finance_tax_period`(2)
`company`(1) `company_settings`(1) `company_feature`(1) `finance_account_mapping`(1)
`purchase_return`(1) `sales_return`(1) `stock_opening_import`(1). 10 di antaranya tanpa label UI
(`business_party`, `business_party_address`, `company_feature`, `company_settings`,
`finance_journal_entry`, `finance_opening_balance`, `media_file`, `member_type`,
`purchase_return`, `stock_opening_import`); 7 label entitas tak pernah dipakai
(`product_media`, `category`, `stock_adjustment`, `goods_receipt`,
`order_status_definition`, `order_status_transition`, `finance_journal`).

## 4. Catatan Konseptual untuk Rebuild

- Pertahankan: append-only + tanpa-FK + resolusi-nama-dengan-fallback.
- Putuskan di KI-128/129: tampilkan JSON detail + filter UI, atau kunci minimalis.
- Normalisasi satu dari dua: `media_file` vs label `product_media`, dan
  `finance_journal_entry` vs label `finance_journal` (KI-127).
- Putuskan nasib kunci skrip `finance.cost_ledger.restatement` (satu-satunya penulis
  di luar `AuditLogService` — `rebuild-cost-ledger.ts:416-430`): dipertahankan sebagai
  kunci resmi (tambah label + kontrak) atau dipindah ke `system_event_logs`?
