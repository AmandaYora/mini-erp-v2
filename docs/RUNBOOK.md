# RUNBOOK — Operasi mini-erp (F6-7)

Satu container app + MySQL di host, di VPS **elcodelabs** (`ssh elcodelabs`, folder `~/minierp`,
container `minierp-app`, `127.0.0.1:8084`, ADR-0019). Skrip server: `infra/elcodelabs/` (disalin ke
`~/minierp`); skrip backup/restore: `scripts/ops/`. Detail deploy penuh: `docs/DEPLOYMENT.md`. Pedoman ETL/cutover: `docs/plan/menuju-pengganti-production/PLAN.md` §9.

## 1. Deploy normal (setiap rilis)

Image dibangun CI (`release-image.yml`, setelah `ci` hijau di `main`) dan didorong ke
`ghcr.io/amandayora/mini-erp-v2:<git-sha>`. VPS **tidak pernah** membangun image.

```bash
ssh elcodelabs
cd ~/minierp && ./deploy.sh <git-sha-lengkap>
# = cek awal -> pull -> backup DB (sudo mysqldump) -> migrate up -> up -d -> tunggu /readyz
```

Urutan manual bila skrip tak dipakai (di `~/minierp`):

```bash
export IMAGE_TAG=<git-sha>
docker compose -f docker-compose.prod.yml pull app
sudo mysqldump --single-transaction minierp_db | gzip > backups/pre-deploy-$(date +%F-%H%M).sql.gz
docker compose -f docker-compose.prod.yml --profile ops run --rm migrate up
docker compose -f docker-compose.prod.yml up -d app
curl -fsS http://127.0.0.1:8084/readyz
```

`deploy.sh` menulis `IMAGE_TAG` yang sedang berjalan ke `.env`, sehingga `docker compose up` manual
tetap di versi itu (compose menolak jalan bila `IMAGE_TAG` kosong — tidak ada jatuh ke `latest`).

Jangan menjalankan migrasi otomatis saat start (keputusan F6-3). Biner
menolak start bila schema lebih tua dari biner (`schema_migrations.version`
vs berkas di `/app/migrations`); pesannya menyebut perintah migrate di atas.
`dirty=1` berarti migrasi terakhir gagal di tengah — perbaiki manual,
jangan `up` ulang membabi buta.

Rantai migrasi saat ini: baseline per modul `000001`–`000018` ditambah migrasi bernomor `000019`, `000020`, `000021`, `000022`,
`000023`, `000031`–`000037`, `000039`–`000041` (nomor 000024–000030 dan 000038 sengaja kosong). Sesudah `migrate up`,
`schema_migrations.version` harus **41** dengan
`dirty=0`; riwayat kolom/tabel baru dan dampak ETL ada di [DB_SCHEMA.md §8](DB_SCHEMA.md#8-riwayat-migrasi-bernomor-sesudah-baseline).
Migrasi baru memakai nomor berikutnya yang bebas (`ls apps/api/migrations`) dan wajib punya `.down.sql`.

## 2. Backup

```bash
./scripts/ops/backup.sh                 # DB + storage -> ./backups/
BACKUP_DIR=/mnt/offhost BACKUP_DIR=... ./scripts/ops/backup.sh --db-only
```

- DB: `mysqldump --single-transaction ... | gzip`, nama memuat timestamp UTC
  dan versi schema (`db-<ts>-v<ver>.sql.gz` + `.sha256`). Kredensial via berkas
  sementara 0600, tidak di argv.
- Storage: `tar` atas `data/storage` (`media/` + `_private/`). `_private`
  berisi sesi WhatsApp: mode 0600, simpan terpisah/terenkripsi di luar host.
  Objek S3 tidak dibackup skrip (diversi penyedia); pastikan versinya aktif.
- Retensi yang disarankan: harian 14, mingguan 8, bulanan 6; satu salinan di
  luar host (input pemilik §10 PLAN: tujuan backup).
- Windows dev: `powershell -File scripts/ops/backup.ps1 -DbName mini_erp_db`.

## 3. Restore + uji pulih

```bash
./scripts/ops/restore.sh backups/db-<ts>-v<ver>.sql.gz            # ke DB baru
./scripts/ops/restore.sh backups/db-<ts>-v<ver>.sql.gz mini_erp_db --yes-overwrite  # timpa (bahaya)
./scripts/ops/verify-backup.sh <db>   # dipakai restore.sh otomatis
```

`verify-backup.sh` memeriksa: tabel `schema_migrations` ada dan `dirty=0`,
tabel kunci ada, `SUM(debit)=SUM(credit)` jurnal, dan menghitung saldo stok
negatif. Restore selalu ke database **baru** dulu; arahkan `DB_DSN` setelah
SEHAT. Wajib uji restore ke DB uji tiap bulan dan sebelum cutover (gladi L).

## 4. Rollback aplikasi (rilis bermasalah)

```bash
cd ~/minierp && ./deploy.sh <git-sha-lama-yang-baik>   # image lama dari GHCR
# Bila migrasi sudah naik dan harus turun (hanya bila *.down.sql terverifikasi
# di CI + backup pra-deploy di ~/minierp/backups sudah ada):
IMAGE_TAG=<git-sha-baru> docker compose -f docker-compose.prod.yml --profile ops run --rm migrate down 1
# Bila ragu: restore backup pra-deploy (ayat 3), jangan down.
```

Deploy yang gagal sehat mencetak perintah rollback ke versi sebelumnya.

## 5. Rollback cutover ke legacy (jalur balik)

Legacy dibiarkan hidup **read-only** selama jendela yang disepakati (input
pemilik). Kriteria kembali: rekonsiliasi saldo/stok gagal, galat kritis,
atau perintah pemilik.

1. Hentikan trafik revamp (`cd ~/minierp && docker compose -f docker-compose.prod.yml stop app`).
2. Kembalikan DNS/proxy ke legacy, buka tulis legacy.
3. Konsekuensi: transaksi yang sudah dicatat di revamp setelah cutover tidak
   ada di legacy — ekspor delta (CSV dari tabel dokumen) untuk entri ulang
   manual. Tidak ada sinkronisasi dua arah.
4. Objek S3 aman: revamp tidak menghapus objek ber-`origin=legacy` (F1-9).

## 5b. Gladi / cutover ETL (urutan terbukti pada dump produksi 2026-10-16)

Gladi L1 pada dump produksi nyata (company 1): `rehearse pass=true`, 86 pemeriksaan
verify lulus. Urutan yang **harus** diikuti, dan jebakan yang sudah ketahuan:

1. **Dump legacy** dari server lama: `mysqldump --single-transaction --routines
   --triggers --events --hex-blob --set-gtid-purged=OFF`; `gzip -t` dan cocokkan sha256
   setelah diunduh. Muat ke DB **salinan** lokal (mis. `mini_erp_legacy_copy`), lalu buat
   tabel penanda: `CREATE TABLE etl_copy_marker (note VARCHAR(100), created_at DATETIME
   DEFAULT CURRENT_TIMESTAMP); INSERT INTO etl_copy_marker(note) VALUES ('...')`.
2. **Samakan collation tabel legacy di salinan** (bukan di produksi). Legacy punya
   `payments`, `delivery_notes`, `delivery_note_items` ber-`utf8mb4_0900_ai_ci` sementara
   tabel lain `utf8mb4_unicode_ci`; tanpa ini pemeriksa preflight Q-ORD-2 error 1271:
   `ALTER TABLE <t> CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`.
3. **Freeze legacy** (read-only) sebelum dump final; reservasi stok `active` harus nol
   (Q-STK-5). Pada salinan gladi: `UPDATE stock_reservations SET status='released'
   WHERE status='active'`.
4. **Target** `*_rehearsal` kosong, `migrate up`, lalu `seed init` agar katalog izin,
   peran, dan akun ada; **kosongkan** `company_profile`, `branch_document_sequences`,
   `audit_logs`, `user_branch_access`, `branches` (seed membuat cabang UTM) sebelum ETL.
5. **Media**: lampiran ada di S3 (`object_key` = path relatif). Unduh tiap `object_key`
   dari `media_files` ke folder yang meniru key (GET saja), lalu `--media-src`/`--media-dst`.
   Hati-hati: daftar key dari `mysql` Windows berakhiran `\r` — buang sebelum dipakai.
   Kunci yang diunduh = `media_files.object_key` **ditambah** `payments.proof_file_path` (bukti transfer;
   di ETL berjenis `payment_proof`, `legacy_id` negatif). Tanpa yang kedua, fase media melaporkan ±282
   `missing_source` yang sebenarnya ada di S3 (gladi 2026-10-21: 4.078 objek, 0 hilang). Unduh di server
   lama dengan kredensialnya sendiri (GET saja, `nice`), salurkan ke mesin gladi, hapus folder sementara.
6. `etl rehearse --company-id N --opening-date YYYY-MM-DD --freeze-at <RFC3339> --workdir DIR --media-src ...
   --media-dst ...` dengan `ETL_LEGACY_DSN`/`ETL_TARGET_DSN` (`--freeze-at` = saat legacy dibekukan; tanpa itu
   tanggal jurnal koreksi migrasi = `created_at` terakhir di dump). Jalankan **dua kali** dan
   bandingkan dengan `--compare-with` (hash keluaran harus sama); `load --resume` harus
   `inserted=0` (idempoten); `verify --self-test` harus membuktikan verifier menangkap data rusak.
7. Perlakuan data legacy yang kini ditangani ETL (muncul di `findings.csv`): pindah-lokasi
   tanpa dokumen jadi transfer sintetis `TRF-<cabang>/MIG/<id>` (TRF_SYNTH); kode varian
   dipakai-ulang setelah arsip diberi sufiks `~<id>` (VARIANT_CODE_DEDUPE); jurnal yang
   semua barisnya Rp0 tidak dimigrasi (JRN_ZERO_VALUE). Saldo awal biaya HPP legacy (baris biaya `opening`
   tanpa mutasi, satu per cabang x produk) **tidak** membuat mutasi stok baru: nilainya dibagi pro-rata qty ke
   mutasi `opening_stock` produk itu yang belum berbiaya, sebagai baris biaya asli (COST_OPENING_SPLIT); saldo stok
   tidak berubah. Bila qty biaya pembukaan != Σ qty mutasi `opening_stock` produk, transform berhenti
   (COST_OPENING_UNMATCHED, blocker): legacy keuangan dan stok tidak sepakat, keputusan pemilik (Q-HPP-2).
   Baris biaya tanpa mutasi bernilai nol (koreksi `manual_correction` legacy) dilewati (COST_NOOP).
   Ledger biaya HPP mengikuti konvensi revamp: `total_cost` bertanda (keluar negatif, V-14c). Mutasi legacy
   `adjustment` (stock opname) menimpa saldo, jadi qty mutasinya = |saldo sesudah − sebelum|, bukan `quantity`.
   Nilai per cabang x produk = `finance_inventory_cost_states` legacy (= akun Persediaan legacy, V-14d): nilai yang
   legacy ubah tanpa baris biaya (koreksi pra-ledger 2026-08-13) dan pembagian ulang antar-varian (legacy satu harga
   modal per produk) masuk sebagai baris biaya-saja qty 0, id ≥ 2^62, `ref_type=opening_cost` (COST_BASELINE,
   COST_VARIANT_REALIGN); bila karenanya harga modal revamp menyimpang >5% dari `average_cost` legacy, COST_BASELINE
   menjadi warn dan produknya masuk `report/cost_baseline_review.csv` (kode, nama, rata-rata legacy, harga modal revamp,
   qty, nilai) untuk ditelaah akuntan (COST_REVIEW_LIST). ETL membiayai **setiap** mutasi (aturan `costOf` revamp; Sync revamp mulai sesudah mutasi
   migrasi terakhir); barang kembali dari SJ yang dibatalkan dinilai sebesar biaya keluar SJ itu. Dokumen yang masuk
   antrean posting revamp tercatat POST_PENDING (alasan: belum dikandidatkan legacy / masih di antrean legacy); dokumen
   pending yang punya jurnal legacy hidup = blocker POST_PENDING_JOURNALED. **Zona waktu** (konvensi kolom:
   `knowledge/DATABASE_GUIDE.md`): legacy menulis tanggal dokumen dalam UTC (TypeORM `timezone:'Z'`), sedangkan
   `created_at`/`updated_at` diisi default DB server WIB. ETL menulis tanggal dokumen sebagai jam dinding WIB
   (`fmtWIB`: tanggal bayar/order/jatuh tempo/faktur/retur/pelunasan/penerimaan/transfer; `delivery_date` = tanggal
   WIB naif) dan membaca `created_at`/`updated_at` legacy sebagai WIB → instan UTC (`row.LocalTime`). Gladi
   2026-10-21: 11.929 pembayaran sama persis dengan tampilan legacy (sebelumnya 1.814 jatuh sehari lebih awal dan
   1.687 berbeda tanggal dari jurnalnya sendiri). Item SJ sintetis memakai satuan baris SO (V-28).
   **Kartu stok** dihitung ulang urut `moved_at`, lalu (pada detik yang sama) stok awal, penyesuaian, masuk, baru keluar,
   lalu id legacy; `balance_before/after` legacy diarsip utuh (`archive/raw/inventory_movements`). Saldo sementara negatif
   yang tersisa dan jumlah mutasi yang saldonya berbeda dari legacy dilaporkan STOCK_CARD_RECOMPUTED (gladi 2026-10-21:
   1.122 negatif sementara, tidak berkurang oleh urutan seketika: sebabnya SJ bertanggal mundur sebelum stok awal/opname
   masuk; saldo akhir tidak terpengaruh). **Pembersihan dokumen**: SJ yang sudah dikirim tetapi belum dikonfirmasi diterima
   di legacy dimigrasi terkonfirmasi dengan `confirmed_at` = waktu kirim dan dicatat DN_IN_TRANSIT (warn: operasional
   mengumpulkan SJ bertanda tangan); order penjualan yang semua barisnya terkirim oleh SJ terkonfirmasi tetapi di legacy
   masih berjalan dimigrasi `completed` (`completed_at` = konfirmasi SJ terakhir; ORDER_COMPLETED_DELIVERED, V-02
   memperhitungkannya); order draf yang sudah dibayar dicatat DRAFT_ORDER_PAID (warn: dikonfirmasi atau dibatalkan dan
   dikembalikan). Lampiran yang dihapus di legacy tidak dimigrasi (MEDIA_ARCHIVED; diarsip `archive/raw/media_files`).
   Penomoran biaya usaha melanjutkan seri legacy `EXP/<tahun>/n`: penghitung `EXP:<tahun>` revamp = nomor legacy
   tertinggi, jadi biaya baru bernomor `EXP-<cabang>/<tahun>/n+1` (V-18 juga mensimulasikan penghitung keuangan).
   **Jurnal koreksi migrasi** (ADR-0018 D11; `source_doc_type=migration_correction`, nomor `JV-<cabang>/<tahun>/n`,
   tanggal freeze di periode terbuka; rinciannya per baris di `report/migration_corrections.csv`): HPP SJ batal yang
   tidak pernah dibalik legacy (DN_CANCEL_COGS, Dr Persediaan / Cr HPP); keadaan biaya legacy bernilai tanpa qty
   (COST_STATE_NO_QTY, Dr selisih persediaan / Cr Persediaan, ledger dinolkan) atau ber-qty tanpa nilai padahal baris
   biayanya bernilai (COST_STATE_NO_VALUE, ditulis naik); mutasi yang tidak akan pernah dijurnal: barang SJ dari dokumen
   legacy yang terposting sebagian/diarsip/sebelum saldo awal (COGS_UNPOSTED, lawan HPP) dan koreksi drift, penyesuaian
   tanpa jurnal, penerimaan/retur sebelum saldo awal (STOCK_UNPOSTED, lawan selisih persediaan); pendapatan SJ order yang
   HPP-nya sudah dijurnal legacy tetapi penjualannya tidak (legacy menjurnal pendapatan saat order selesai, dan order itu
   belum selesai) sementara SJ-nya di bawah batas cutover sehingga tidak diantre revamp (REVENUE_UNBOOKED, satu jurnal per
   order: Dr Piutang per pelanggan / Cr Penjualan per produk, Cr PPN Keluaran bila berpajak; nilainya = jendela qty SJ itu
   persis seperti posting SJ revamp, jadi SJ berikutnya yang diantre hanya membukukan sisanya; order yang dibatalkan legacy
   tidak dikoreksi, hanya REVENUE_UNBOOKED_CANCELLED untuk telaah). Akun lawan diambil dari `system_key` legacy (`cogs`,
   `stock_difference`, `accounts_receivable`, `sales_revenue`, `output_tax`); tidak ada = blocker MIG_CORR_ACCOUNT. Verify:
   V-26 (Σ jurnal koreksi = Σ temuan = daftar), V-29 (tidak ada piutang pelanggan negatif sesudah antrean posting hari-H
   tanpa uang muka/pembayaran di depan pengiriman/kredit retur yang menjelaskannya), V-27 (buku Persediaan − ledger = selisih bawaan legacy buku − keadaan biaya; gladi 2026-10-21:
   −Rp297 pembulatan), V-14d (ledger per produk = keadaan biaya legacy ± koreksi). Pembulatan rupiah (ADR-0005) dan
   penyeimbang jurnal (≤ `--max-balance-delta`) tidak dikoreksi; ringkasannya di temuan ROUNDING_SUMMARY, buktinya V-11 &
   V-13a. Order inklusif pajak muncul sebagai INCL_ORDER (warn); jurnal GR inklusif yang sudah diposting legacy =
   blocker INCL_GR_POSTED (keputusan P2).
8. **Keputusan pemilik/akuntan** sebelum cutover nyata: tanggal saldo awal **diputuskan 2026-07-01**
   (`--opening-date 2026-07-01`, sama dengan saldo awal legacy `JRN/2026/00001`; jurnal legacy sejak itu dimigrasi
   apa adanya; jurnal saldo awal revamp hanya untuk jurnal legacy sebelum tanggal itu), cutover langsung; kesalahan
   legacy dikoreksi dengan jurnal koreksi migrasi (butir 7; ADR-0018 D11). Masih terbuka: perlakuan jurnal inklusif
   pajak bila muncul di dump cutover (INCL_GR_POSTED).

## 5c. Checklist cutover (ringkas, urut hari-H)

Rincian ada di §5 (jalur balik), §5b (ETL) dan [DEPLOYMENT.md](DEPLOYMENT.md). Centang tiap butir sebelum menyatakan cutover selesai.

- [ ] Keputusan pemilik/akuntan §5b butir 8 terjawab (tanggal saldo awal: 2026-07-01; koreksi migrasi ADR-0018 D11; jurnal inklusif pajak bila ada).
- [ ] Temuan ETL ditindaklanjuti (§5b butir 7): akuntan menelaah `report/cost_baseline_review.csv` dan REVENUE_UNBOOKED_CANCELLED; operasional mengumpulkan SJ bertanda tangan untuk DN_IN_TRANSIT dan menyelesaikan order DRAFT_ORDER_PAID.
- [ ] Backup DB revamp dan dump legacy final terverifikasi (`gzip -t`, sha256); legacy di-freeze (read-only).
- [ ] Gladi ETL lulus pada dump terbaru (§5b butir 6): `rehearse pass=true`, hash dua kali jalan sama, `load --resume` `inserted=0`.
- [ ] `migrate up` (versi 41, `dirty=0`), `seed init`, ETL final, `etl verify` hijau; `GET /readyz` OK; rekonsiliasi saldo & stok (Readiness/rollups) cocok.
- [ ] Urutan saldo: impor stok awal -> saldo awal keuangan (Persediaan dihitung server dari ledger biaya, K-05) -> dokumen berjalan; `GET /finance/close/readiness` tanpa blocker `inventory_recon`, dan produk berstok tanpa harga modal dilengkapi lewat `POST /finance/opening/complete`.
- [ ] `GET /api/v1/ops/integrity` (keadaan antara alur multi-langkah) kosong; tak ada dokumen macet di antrean posting yang belum dijurnal atau diabaikan dengan alasan (`finance/posting-sources`).
- [ ] **Sesudah restore, sebelum akses dibuka**: jalankan antrean posting dan rekonsiliasi (langkah di bawah) sampai readiness bulan freeze tanpa `failed`.

**Antrean posting & rekonsiliasi hari-H** (sesudah ETL/restore, sebelum akses dibuka; terbukti di gladi 2026-10-21):

```bash
API=https://<domain>; FREEZE=2026-10-21; Y=2026; M=10   # tanggal freeze WIB & bulannya
TOKEN=$(curl -s -X POST $API/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"<admin>","password":"<sandi>"}' | jq -r .data.accessToken)
H="Authorization: Bearer $TOKEN"
# 1. Readiness awal (arsipkan): unposted = antrean legacy yang belum dijurnal (POST_PENDING di findings.csv)
curl -s -H "$H" "$API/api/v1/finance/close/readiness?year=$Y&month=$M" > readiness-0.json
# 2. Tutup harian per tanggal freeze; ulangi sampai "remaining" 0 atau tidak ada kemajuan. Dokumen legacy
#    bertanggal SETELAH hari ini (mis. SJ dijadwalkan) tetap di antrean dan dijurnal oleh tutup harian tanggalnya
#    sendiri — tutup harian menolak tanggal masa depan dan jurnal tak boleh bertanggal ke depan; readiness
#    menampilkannya sebagai belum dijurnal dan inventory_recon menjelaskannya lewat pendingValue
curl -s -X POST -H "$H" -H 'Content-Type: application/json' -d "{\"date\":\"$FREEZE\"}" $API/api/v1/finance/posting/close-day | jq '.message, [.data.results[]|select(.error)]'
# 3. Dokumen yang tetap gagal: perbaiki sebabnya (mis. harga modal) lalu ulangi 2; data uji yang tak material
#    boleh diabaikan dengan alasan tertulis. (Gladi 2026-10-21 malam: 0 gagal — SJ order batal legacy tidak lagi
#    diantre, ADR-0018 D11f.)
curl -s -X POST -H "$H" -H 'Content-Type: application/json' \
  -d '{"docType":"delivery","docId":<id>,"reason":"<alasan>"}' $API/api/v1/finance/posting/ignore
# 4. negative_inventory dari histori legacy TIDAK diperbaiki costrebuild (histori migrasi tidak ditulis ulang,
#    ADR-0020; §6b): catat posisinya untuk akuntan dan telaah sebabnya (oversell/inflow hilang di legacy).
# 5. Bukti akhir (arsipkan): tidak ada status failed selain `unposted` untuk dokumen bertanggal setelah hari-H
#    (gladi 2026-10-21: SJ-BLR/2026/02446 bertanggal 25 Okt — dijurnal tutup harian 25 Okt; bulan Oktober memang
#    belum boleh ditutup sebelum berakhir); inventory_recon konsisten (ADR-0020: selisih tak terjelaskan
#    = selisih bawaan legacy V-27 di verify.md, dalam toleransi; gladi 2026-10-21 sesudah posting: Rp297).
#    Cocokkan juga buku vs dokumen per pihak (Piutang+Uang Muka, Hutang+Uang Muka vs saldo pihak modul pembayaran):
#    gladi 2026-10-21 malam: 87/87 pemasok cocok, 925/927 pelanggan cocok (2 sisanya waktu: SJ bertanggal ke depan,
#    barang pengganti retur tukar belum dikirim) — ADR-0018 D11h
curl -s -H "$H" "$API/api/v1/finance/close/readiness?year=$Y&month=$M" > readiness-1.json
curl -s -H "$H" "$API/api/v1/ops/integrity"
```
- [ ] DNS/proxy dialihkan ke revamp; jalur balik ke legacy (§5) siap dan diuji sebelumnya.
- [ ] **APK Kasir Android dibangun ulang bila domain berubah** (`POS_URL` di `android-pos-shell/app/build.gradle.kts`),
      dipasang di semua tablet kasir, lalu **uji jembatan `AndroidPrinter`** per tablet (Tes Cetak, struk POS, laci kas,
      gagal-aman) — langkah di [DEPLOYMENT.md §7](DEPLOYMENT.md#7-apk-kasir-android-android-pos-shell--bangun-ulang-saat-cutover-op-05-d5).
- [ ] Cetak fisik diverifikasi manual di perangkat nyata (struk thermal, nota, faktur, label QR).
- [ ] Kasir dan admin login, satu penjualan POS tunai + satu PO + satu penerimaan uji berhasil (lalu dibatalkan/ditandai uji).

## 6. Operasi harian

- Kesehatan: `GET /readyz` (DB + direktori + versi) untuk compose/healthcheck;
  `GET /api/v1/health` liveness murni. Metrics `GET /api/v1/metrics` butuh
  `Authorization: Bearer <METRICS_TOKEN>`; di nginx luar, deny all.
- Log membawa `request_id`, `user_id`, `branch_id`; panic mencatat stack,
  klien hanya menerima 500 envelope + header request-id (F6-5).
- Shutdown anggun 25 dtk (`SHUTDOWN_TIMEOUT`) < `stop_grace_period: 30s`
  compose; WhatsApp pairing perlu uji ponsel manual (F6-6).
- **SJ Langsung belum diorder (ADR-0022):** sebelum tutup harian/bulan, periksa panel "SJ menunggu order" di
  Order Jual (atau `GET /api/v1/deliveries/open-direct?limit=0`). Barang yang sudah keluar lewat SJ Langsung
  baru dijurnal setelah admin penjualan membuat ordernya (Tambah order → Dari surat jalan); antrean posting
  menampilkannya sebagai "SJ Langsung · menunggu order", **tidak bisa diabaikan**, dan kesiapan tutup bulan
  gagal pada butir `direct_unordered`. SJ yang tidak jadi dibatalkan beralasan (stok kembali).
- **Galat 5xx tak terduga (OP-06):** panic dan respons 5xx (selain 503 dan probe `/healthz`, `/readyz`) dicatat middleware
  `recoverer` ke jejak audit sebagai aksi `error.unexpected` (entitas `error`, "Galat server tak terduga") dan tampil di
  halaman Audit (filter Aksi). Catatan berisi status, pola rute (id dimasker) dan pesan panic yang dipotong 200 karakter dan
  disaring dari teks mirip kredensial; **tidak pernah** body, header, query string atau token. Dedupe per (rute, status, pesan)
  60 detik dan batas 20 baris per menit, sehingga badai galat tidak menjadi badai tulis. Rincian lengkap (stack) tetap di log
  server, dicari lewat `requestId` pada metadata baris audit.

## 6b. Restatement biaya persediaan (`costrebuild`, OP-04 — padanan `cost:rebuild` legacy)

Kapan dipakai: harga keluar (HPP) di ledger biaya masih memakai rata-rata lama karena rantai di bawahnya berubah (mutasi
yang terlambat masuk ledger, perbaikan harga masuk), sehingga jurnal HPP/Persediaan ikut basi. Pemeriksa yang melihatnya:
rekonsiliasi buku-vs-ledger (K-05) di Kesiapan Tutup Buku atau margin per produk yang janggal.

Lewat web: **Keuangan > Hitung Ulang HPP** (`/finance/cost-restatement`): dry-run otomatis saat halaman dibuka (izin
`finance.manage`), komit dengan alasan wajib (izin `finance.close.force`; alasan tercatat di audit), hasilnya nomor jurnal dan
rekonsiliasi sebelum/sesudah. Mesin, gerbang, dan kuncinya sama dengan perintah konsol di bawah, yang tetap ada untuk operator
(jalan di kontainer app terhadap DB host; biner `costrebuild` ada di image bersama `api` dan `seed`):

```bash
# 1. DRY-RUN (bawaan): selisih per produk + status gerbang; TIDAK menulis apa pun.
docker exec minierp-app ./costrebuild --branch-id 1
# 2. Periksa laporan. Bila setuju, jalankan COMMIT (setelah backup DB, bagian 2):
docker exec minierp-app ./costrebuild --branch-id 1 --commit [--date 2026-10-31] [--actor-id 5] [--reason "…"]
# Lokal: cd apps/api && go run ./cmd/costrebuild --branch-id 1 [--commit] [--json]
```

Yang dilakukan: (1) menarik ledger terbaru; (2) memutar ulang tiap posisi (produk x varian) berurut id mutasi; harga masuk
(penerimaan, saldo awal, baris biaya-saja, kaki cermin) adalah bukti dan tidak diubah; baris biaya-saja (lengkapi saldo
awal, revaluasi) diputar di titik ia ditulis, bukan di akhir rantai, sehingga koreksinya tidak dibatalkan; hanya baris keluar yang dijurnal dari
ledger (pengiriman, selisih stok kurang, hapus stok, retur beli) yang dihitung ulang pada rata-rata bergerak saat itu, dalam
rupiah bulat (keluar yang mengosongkan posisi mengambil tepat sisa nilainya); pasangan netral (transfer, pindah lokasi rusak)
dan dokumen yang dibatalkan tidak disentuh (ADR-0020); **histori migrasi tidak pernah ditulis ulang**: hanya baris keluar
dengan id mutasi di atas batas cutover (`finance_cutover` `stock_movement`, dilaporkan sebagai `cutoverMovementId`) yang
boleh berubah; posisi yang berakhir negatif dilewati dan dilaporkan; (3) untuk
commit: menulis ulang baris ledger yang berubah dan memposting SATU jurnal koreksi seimbang (`cost_restatement`: Persediaan
lawan akun yang dipakai jurnal aslinya: HPP, Selisih Persediaan, Hapus Stok, atau Selisih Retur Beli; Dr Persediaan bila
nilai persediaan naik, sebaliknya bila turun) dalam satu transaksi, lalu mencatat audit `finance.cost_restatement`.
Selama berjalan ia memegang kunci basis data `fincost:<cabang>` (`GET_LOCK`) yang sama dengan penghitungan biaya server,
jadi aman dijalankan saat aplikasi melayani: posting yang tiba menunggu (maks. 30 detik) lalu ditolak dengan pesan
"sedang dihitung ulang" bila restatement belum selesai.

Gerbang (dilaporkan juga saat dry-run; commit ditolak bila ada yang gagal):

| Gerbang | Arti |
|---|---|
| `trial_balance` | total debit = total kredit dan tidak ada jurnal timpang di cabang; diperiksa lagi **di dalam transaksi** setelah jurnal koreksi (gagal = rollback total) |
| `inventory_recon` | rekonsiliasi K-05 sebelum commit konsisten (selisih tak terjelaskan dalam toleransi): koreksi menggeser buku dan ledger sebesar sama, jadi selisih bawaan tidak boleh disembunyikan |
| `pending_documents` | tidak ada dokumen/mutasi yang belum dijurnal (nilainya dibaca dari ledger yang akan dikoreksi; jurnalkan atau abaikan dulu di antrian posting) |
| `period_open` | periode tanggal jurnal koreksi belum ditutup (`--date`, bawaan hari ini WIB) |

Setelah commit alat mencetak ulang rekonsiliasi (harus `konsisten=true`, selisih dalam toleransi) dan menjalankan ulang
dry-run harus melaporkan 0 baris. Pemulihan bila keliru: restore backup DB pra-commit (bagian 2). Jangan hanya membalikkan jurnal koreksi: baris ledger tidak ikut kembali dan buku-vs-ledger akan menyimpang.

## 6c. Revaluasi harga modal (harga modal non-nol yang salah)

Kapan dipakai: harga modal sebuah produk berstok sudah terisi tetapi salah (harga saldo awal salah ketik, barang masuk
dengan harga keliru). "Lengkapi harga modal" di Saldo Awal hanya mengisi posisi bernilai nol, dan restatement (6b) tidak
pernah mengubah barang masuk, jadi keduanya tidak bisa memperbaikinya.

Lewat web: **Keuangan > Revaluasi Harga Modal** (`/finance/inventory-revaluations`). Pilih produk (stok, rata-rata, harga beli
terakhir, dan harga master tampil), isi harga modal yang benar per satuan dasar, pratinjau (izin `finance.manage`), lalu
bukukan dengan alasan (izin `finance.close.force`). Yang terjadi dalam satu transaksi di bawah kunci biaya cabang: selisih
`round(qty × harga baru) − nilai` ditulis sebagai satu baris ledger nilai saja (`inventory_revaluation`, qty 0) dan satu
jurnal (`inventory_revaluation`): Dr Persediaan / Cr Selisih Stok bila naik, kebalikannya bila turun; catatan revaluasi
(sebelum/sesudah, alasan, jurnal) dan audit `finance.inventory_revaluation`. Barang keluar berikutnya memakai rata-rata baru;
rekonsiliasi buku-vs-ledger tidak berubah. Ditolak: stok 0, harga negatif, tanpa perubahan nilai, tanggal masa depan atau
periode tertutup, saldo awal keuangan belum diposting.

Sifatnya maju saja: HPP barang yang sudah keluar sebelum revaluasi tidak diubah (gunakan restatement bila perlu, sesudahnya).
Pemulihan bila keliru: revaluasi sekali lagi ke harga yang benar, jangan membalik jurnalnya (baris ledger tidak ikut kembali).
