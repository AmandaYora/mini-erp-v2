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
`000023`, `000031` (nomor 000024–000030 sengaja kosong). Sesudah `migrate up`, `schema_migrations.version` harus **31** dengan
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
6. `etl rehearse --company-id N --opening-date YYYY-MM-DD --workdir DIR --media-src ...
   --media-dst ...` dengan `ETL_LEGACY_DSN`/`ETL_TARGET_DSN`. Jalankan **dua kali** dan
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
8. **Keputusan pemilik/akuntan** sebelum cutover nyata: tanggal saldo awal
   (`--opening-date`; gladi memakai 2026-01-01), perlakuan jurnal inklusif pajak dan
   selisih Rp1 (lihat `adjustments.csv`).

## 5c. Checklist cutover (ringkas, urut hari-H)

Rincian ada di §5 (jalur balik), §5b (ETL) dan [DEPLOYMENT.md](DEPLOYMENT.md). Centang tiap butir sebelum menyatakan cutover selesai.

- [ ] Keputusan pemilik/akuntan §5b butir 8 terjawab (tanggal saldo awal, jurnal inklusif pajak).
- [ ] Backup DB revamp dan dump legacy final terverifikasi (`gzip -t`, sha256); legacy di-freeze (read-only).
- [ ] Gladi ETL lulus pada dump terbaru (§5b butir 6): `rehearse pass=true`, hash dua kali jalan sama, `load --resume` `inserted=0`.
- [ ] `migrate up` (versi 31, `dirty=0`), `seed init`, ETL final, `etl verify` hijau; `GET /readyz` OK; rekonsiliasi saldo & stok (Readiness/rollups) cocok.
- [ ] Urutan saldo: impor stok awal -> saldo awal keuangan (Persediaan dihitung server dari ledger biaya, K-05) -> dokumen berjalan; `GET /finance/close/readiness` tanpa blocker `inventory_recon`, dan produk berstok tanpa harga modal dilengkapi lewat `POST /finance/opening/complete`.
- [ ] `GET /api/v1/ops/integrity` (keadaan antara alur multi-langkah) kosong; tak ada dokumen macet di antrean posting yang belum dijurnal atau diabaikan dengan alasan (`finance/posting-sources`).
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

Alat ini adalah perintah operasi (bukan endpoint), jalan di kontainer app terhadap DB host; biner `costrebuild` ada di
image bersama `api` dan `seed`:

```bash
# 1. DRY-RUN (bawaan): selisih per produk + status gerbang; TIDAK menulis apa pun.
docker exec minierp-app ./costrebuild --branch-id 1
# 2. Periksa laporan. Bila setuju, jalankan COMMIT (setelah backup DB, bagian 2):
docker exec minierp-app ./costrebuild --branch-id 1 --commit [--date 2026-10-31] [--actor-id 5]
# Lokal: cd apps/api && go run ./cmd/costrebuild --branch-id 1 [--commit] [--json]
```

Yang dilakukan: (1) menarik ledger terbaru; (2) memutar ulang tiap posisi (produk x varian) berurut id mutasi; harga masuk
(penerimaan, saldo awal, baris biaya-saja) adalah bukti dan tidak diubah, setiap baris keluar dihitung ulang pada rata-rata
bergerak pada saat itu; mutasi transfer antar-cabang tidak disentuh (cabang lawan membawa biaya yang sama, tanpa jurnal);
posisi yang berakhir negatif dilewati dan dilaporkan; (3) untuk commit: menulis ulang baris ledger yang berubah dan memposting
SATU jurnal koreksi seimbang (`cost_restatement`: Dr Persediaan / Cr HPP bila nilai persediaan naik, sebaliknya bila turun,
satu pasang baris per produk) dalam satu transaksi, lalu mencatat audit `finance.cost_restatement`.

Gerbang (dilaporkan juga saat dry-run; commit ditolak bila ada yang gagal):

| Gerbang | Arti |
|---|---|
| `trial_balance` | total debit = total kredit dan tidak ada jurnal timpang di cabang; diperiksa lagi **di dalam transaksi** setelah jurnal koreksi (gagal = rollback total) |
| `inventory_recon` | rekonsiliasi K-05 sebelum commit konsisten (selisih dalam toleransi): koreksi menggeser buku dan ledger sebesar sama, jadi selisih bawaan tidak boleh disembunyikan |
| `pending_documents` | tidak ada dokumen/mutasi yang belum dijurnal (nilainya dibaca dari ledger yang akan dikoreksi; jurnalkan atau abaikan dulu di antrian posting) |
| `period_open` | periode tanggal jurnal koreksi belum ditutup (`--date`, bawaan hari ini WIB) |

Setelah commit alat mencetak ulang rekonsiliasi (harus `konsisten=true`, selisih dalam toleransi) dan menjalankan ulang
dry-run harus melaporkan 0 baris. Pemulihan bila keliru: restore backup DB pra-commit (bagian 2). Jangan hanya membalikkan jurnal koreksi: baris ledger tidak ikut kembali dan buku-vs-ledger akan menyimpang.
