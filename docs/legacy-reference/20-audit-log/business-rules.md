# Business Rules — Modul 20 Audit Log

**Kelompok A.**

---

## 1. Aturan Tulis

| ID | Aturan |
|---|---|
| BR-01 | Setiap write bisnis di luar modul exempt HARUS memanggil `auditLog.log` (aturan knowledge — ditegakkan review, bukan kode!). Cakupan aktual: 102 situs di 28 berkas, 10 modul menulis (lihat feature-inventory F-01a/F-01b). |
| BR-02 | Exempt (tanpa jejak): assistant, whatsapp, tools, auth, reporting, dashboard, storage, audit-log sendiri (hindari rekursi!). Terbukti via grep `auditLog\|AuditLog\|actionKey` → nol berkas di 8 modul itu. |
| BR-03 | Field wajib: `idCompany`, `actionKey`, `entityType`; sisanya boleh null (cabang/aktor/entitas-id/before/after/metadata). |
| BR-04 | `happenedAt` = waktu server saat tulis (tanpa parameter — tak bisa backdate!). |
| BR-05 | Baris tak pernah diubah/dihapus via API (append-only; koreksi = baris baru!). |
| BR-06 | Kunci aksi stabil selamanya — filter UI + riwayat bergantung padanya (jangan rename!; lihat KI-127 untuk yang sudah terlanjur basi: 14 label mati + 14 kunci statis/12 dinamis/1 skrip tanpa label — feature-inventory NF-05/NF-06). |

## 2. Aturan Baca (`audit-logs/list`)

| ID | Aturan |
|---|---|
| BR-07 | Wajib JWT + `audit_log.view`; **tanpa** BranchGuard (selalu lingkup perusahaan!). |
| BR-08 | Filter opsional: `id_branch` (`=`), `entity_type` (`=` persis), `action_key` (**LIKE `%v%`** substring!). |
| BR-09 | `page` default 1, `limit` default 20, **cap 100 diam-diam**. |
| BR-10 | Urutan tetap terbaru-dulu (`happenedAt` DESC); respons `{ items, meta: { page, limit, total } }`. |
| BR-11 | Indeks DB `(id_company, id_branch, happened_at)` — filter perusahaan+cabang+waktu cepat; filter aksi/entitas tanpa indeks ([PERLU KONFIRMASI] performa di volume besar — lihat KI-129!). |

## 3. Aturan Tampil

| ID | Aturan |
|---|---|
| BR-12 | Halaman memakai limit **25** (konstanta `AUDIT_LOG_PAGE_LIMIT`), slice prefetch **100** — dua angka berbeda, dipertahankan sebagaimana adanya. |
| BR-13 | Nama aktor/cabang dari store (100 user + daftar cabang); gagal cocok → `User #{id}` / `Branch #{id}` / `System`. |
| BR-14 | Label aksi/entitas dari peta + fallback format (tak pernah blank!). |
| BR-15 | Kolom Keterangan selalu `Aktivitas tercatat otomatis.` (konsekuensi adapter — lihat KI-128). |
| BR-16 | Gagal muat/refresh ditelan diam-diam (catch → kosong), tanpa pesan ke user. |
| BR-17 | Hook hanya reload saat `idle` + login (`loading`/`ready` dilewati — dijaga 5 test!). |

## 4. Katalog Pesan Lengkap (teks apa adanya)

Lihat feature-inventory §4 (14 string UI + `System`/`User #`/`Branch #`/`-`).
Backend modul ini **tanpa pesan khas** (hanya 401/403 global).

## 5. Aturan Pola Tulis (hasil baca seluruh situs)

| ID | Aturan |
|---|---|
| BR-18 | 102/102 situs memakai `await this.auditLog.log({...})` telanjang — nol fire-and-forget, nol try/catch per situs; `AuditLogService.log` sendiri tanpa try/catch (`audit-log.service.ts:26-41`). Gagal tulis SELALU menular sebagai exception ke service pemanggil. `actorType` = `'user'` di 102/102 situs. |
| BR-19 | Dua pola penempatan (contoh terverifikasi di feature-inventory F-01d): (a) DALAM callback transaksi — gagal audit me-rollback tulis utama; (b) SETELAH komit / tanpa transaksi — tulis utama tetap tersimpan tetapi API melempar error. `AuditLogService` memakai repositori sendiri tanpa menerima `EntityManager` transaksi ([PERLU KONFIRMASI] atomisitas silang: tulis audit tak ikut rollback/commit transaksi pemanggil — bukti tak langsung: tanda tangan `log(input: LogInput)` tanpa parameter manager + tak ada pemanggil yang meneruskan `em`). |
| BR-20 | `company.settings.update`/`company.feature.update` hanya menulis audit `if (session)` (`company.service.ts:123-141,167-179`) — pemanggil tanpa sesi (seed/internal) mengubah pengaturan tanpa jejak, by-design-di-kode tetapi tanpa komentar penjelas. |

## Temuan baru & pertanyaan terbuka (belum ber-ID)

- Sebagian besar baris KI-149 tidak lagi sesuai kode: 6 dari 7 aksi finance sensitif yang diklaim tanpa audit ternyata SUDAH diaudit (`finance.journal.post`, `finance.journal.reverse`, `finance.posting_source.sync/ignore/restore/cancel_posting`, `finance.posting.close_day`, `finance_tax_adjustment.create/update/archive` — lihat tabel verifikasi feature-inventory F-01c dengan lokasi baris). Satu-satunya yang benar-benar tanpa jejak: unduh paket berkas pajak (`FinanceExportService.buildTaxPackage`, `finance-export.service.ts:45-109` tanpa injeksi audit; endpoint `finance-close.controller.ts:38-46`). Perlukah KI-149 direvisi/ditutup sebagian? (tak mengubah KI-149 di sini sesuai aturan!)
- Kunci `finance.cost_ledger.restatement` ditulis skrip SQL mentah (`rebuild-cost-ledger.ts:416-430`, actor `system`, `NOW(6)`) — melewati `AuditLogService`, tanpa label UI, tanpa validasi panjang (`action_key` VARCHAR(100) vs string ini 31 char — aman, tetapi pola tulis-langsung-SQL tak terdokumentasi di mana pun).
- Satu aksi operasional (`closeDay`) mengipas jadi N+2 baris audit: 1× `finance.posting_source.sync` (via `syncSources` di `finance-posting.service.ts:901`) + N× `finance.journal.post` (satu per source, `:927-933`) + 1× `finance.posting.close_day` (`:944-952`). Deliver-goods menulis 3 baris berurutan (`payment.create` → `order.approve_credit` → `order.goods_delivered`, `delivery.service.ts:570-625`); receive-goods analog (`goods-receipt.service.ts:373-424`). Bila audit ke-2/ke-3 gagal di pola DALAM-transaksi, audit ke-1 yang sudah terlanjur tersimpan terpisah tak ikut rollback (cek! mengikuti BR-19 — perlu test kegagalan untuk membuktikan).
- `company.profile.update` (kunci nyata, `company.service.ts:70`) vs label `company.update` (mati): profil perusahaan selalu tampil sebagai fallback `Company - Profile - Update` di UI. Sama untuk `delivery.proof_uploaded` (nyata, `delivery-proof.service.ts:111`) vs label `payment.upload_proof` (mati, nama mirip-menyesatkan!).
- `toAuditLog` mengisi `happenedAt` dengan waktu browser (`new Date().toISOString()`) bila respons tak membawa kedua varian field (`org.adapter.ts:47`) — baris tanpa stempel terlihat "baru saja terjadi" di UI, bukan `-` seperti kolom Waktu halaman (yang memakai `-` hanya untuk falsy — `utils.ts:25-28`). Kapan respons tanpa stempel terjadi? cek!
- Test `toAuditLog` tanpa-aktor mengandalkan waktu-sistem-palsu global (`adapters.test.ts:137-138` — cek! cakupan `describe`-nya sebelum mengandalkan bahwa fallback waktu selalu deterministik).
- `list()` tak memvalidasi `page`/`limit` selain cap-atas (`audit-log.service.ts:44-45`): `page: 0` → `skip()` negatif, `limit: 0` → `take(0)` + `meta.limit: 0`, `limit` negatif lolos `Math.min` — cek! respons MySQL untuk OFFSET negatif (kemungkinan 500) dan apakah frontend/slice pernah mengirim nilai non-positif (halaman memakai 25, slice 100 — keduanya aman).
