# Deployment — Mini ERP (Revamp)

## 1. Prasyarat Lokal

| Tool | Versi minimum | Catatan |
|---|---|---|
| Node.js | 20+ | Untuk `apps/web` dan tooling root |
| Go | 1.26 | `apps/api/go.mod`: `go 1.26.0`, `toolchain go1.26.7` (dikunci agar semua mesin/CI sama). Pasang Go >= 1.26 sebelum offline; image build harus memakai Go 1.26 juga |
| MySQL | 8.4 | Berjalan di host/OS, **bukan** container (lihat §4). CI dan DB uji (`mini_erp_test`, Laragon) memakai 8.4.x |
| `air` | terbaru | Watcher dev backend — `go install github.com/air-verse/air@latest` |
| `migrate` (golang-migrate CLI) | terbaru | `go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| `sqlc` | v1.31.1 (dipin) | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1` — akses data modul `migrated` (ADR-0012); `npm run sqlc:generate` |
| `golangci-lint` | v2.14.0 (dipin) | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0` |

## 2. Setup Awal

```bash
cp .env.example .env      # isi kredensial lokal, jangan commit .env
npm install                # root + apps/web (npm workspaces)
mysql -u root -e "CREATE DATABASE IF NOT EXISTS mini_erp_db"
npm run migrate:up         # 18 baseline per modul + migrasi bernomor sesudahnya (DB_SCHEMA §8)
```

Verifikasi dan DB uji:

```bash
node scripts/test-db.mjs up     # pastikan MySQL Laragon + DB/user mini_erp_test
node scripts/verify.mjs --fast  # tanpa integrasi; tanpa argumen = semuanya
```

Catatan Windows: bila `go build` gagal `detected dubious ownership`, jalankan
sekali `git config --global --add safe.directory H:/dimasprasetio/SAAS/mini-erp-revamp`.
`scripts/verify.mjs` juga menyetel `GOFLAGS=-buildvcs=false` agar sama dengan image.

## 3. Development (jalankan terpisah dari root)

```bash
npm run dev:web   # Vite dev server, apps/web
npm run dev:api   # air, apps/api — reload otomatis saat file .go berubah
```

Perintah lain dari root:

```bash
npm run build:web       # build statis apps/web
npm run build:api       # go build -> dist/api
npm run migrate:up      # golang-migrate up
npm run migrate:down    # golang-migrate down 1
npm run migrate:create  # buat pasangan migrasi baru bernomor (mulai 000019; format: <modul>_<perubahan>)
npm run sqlc:generate   # sync sqlc.yaml lalu generate (11 modul sudah migrated, lihat scripts/sqlc-migration-status.json)
npm run arch:check      # penjaga arsitektur; npm run arch:selftest menguji penjaganya
npm run verify          # semuanya: build, vet, test, tsc, lint, vitest, arch, sqlc-sync, schema-doc, contract
```

## 4. Docker (Produksi)

Default: **satu container aplikasi**, database MySQL tetap di level host/OS — bukan container
terpisah. Image: `infra/docker/Dockerfile` (Go 1.26.7 + Node 20, `npm ci`,
`ca-certificates`, user non-root 10001, HEALTHCHECK ke `/readyz`).

```txt
1 VPS / host
├── MySQL terpasang di level OS/sistem
├── nginx di host (terminasi TLS, lihat infra/nginx/nginx.conf)
└── Docker
    └── satu container aplikasi
        ├── build statis frontend (apps/web)
        └── backend API (apps/api, hasil `go build`)
```

Urutan deploy (jangan dibalik; detail + rollback: `docs/RUNBOOK.md`):

```bash
cp .env.production.example .env   # isi nilai asli di host, jangan commit
chown -R 10001:10001 ./data/storage
APP_VERSION=<gitsha> ./scripts/ops/deploy.sh
# = backup DB -> build -> migrate up -> up -d app -> tunggu /readyz
```

`docker-compose.yml` mengikat port hanya ke loopback (`127.0.0.1:8080:8080`),
me-mount `./data/storage` ke `/data/storage` (`media/` disajikan lewat URL
bertanda tangan `/media/*`; `_private/` sesi WA tak disajikan), read-only
rootfs, `stop_grace_period: 30s`, dan service sekali-pakai `migrate`
(`--profile ops`) yang menjalankan `migrate up` sebelum container baru start.
TLS/body-limit/header diteruskan nginx host (`client_max_body_size 6m`,
`proxy_read_timeout 120s`, `/metrics` di-deny).

Container mengekspos satu port (hanya loopback `127.0.0.1:8080`):

- `/api/v1/*` → backend API (+ `/healthz`, `/readyz`, `/api/v1/metrics` ber-token)
- `/media/*?exp&sig` → berkas media (HMAC + expiry, tanpa listing; `/uploads/*` = 404)
- selain itu → berkas statis SPA frontend

Koneksi DB dari container ke MySQL host (Linux VPS): pakai user aplikasi
khusus (`mini_erp_app`, bukan root) — lihat `.env.production.example`:

```env
DB_HOST=host.docker.internal
DB_PORT=3306
DB_USER=mini_erp_app
DB_PASSWORD=<isi-asli>
DB_NAME=mini_erp_db
DB_DSN=mini_erp_app:<isi-asli>@tcp(host.docker.internal:3306)/mini_erp_db
```

`docker-compose.yml` sudah menyertakan `extra_hosts: host.docker.internal:host-gateway` agar
resolusi ini bekerja di Linux VPS (bukan hanya Docker Desktop).

**Jangan** menambahkan service `mysql:` ke `docker-compose.yml` secara default — ini menyalahi
standar Docker Dimas (satu app container, DB di host).

## 5. Migrasi di Produksi

- Jalankan `docker compose --profile ops run --rm migrate up` sebagai langkah deploy terpisah
  **sebelum** container baru mulai menerima trafik — jangan menjalankan migrasi otomatis di setiap
  start container (ini salah satu penyebab masalah di sistem lama: migration runner tanpa tabel
  pelacak, migrasi dijalankan ulang tiap start).
- `golang-migrate` sudah menyimpan versi migrasi di tabel `schema_migrations` — migrasi yang sudah
  jalan tidak akan diulang. Biner membawa pagar versi (F6-3): start gagal dengan pesan "jalankan
  migrasi" bila `schema_migrations.version` lebih tua dari `/app/migrations`, atau `dirty=1`.
- Baseline 18 berkas HANYA untuk database baru (atau DB yang dibangun ulang). Database yang dibangun dari
  rantai lama (000001-000031 dst.) tidak boleh di-`migrate up` dengan berkas baseline: buang dan buat ulang
  (data masuk lewat ETL, lihat F5), atau tandai versinya secara sengaja — jangan dibiarkan menebak.

## 6. Variabel Environment Produksi

Salin `.env.production.example` ke `.env` di host, isi nilai asli. Poin yang
wajib diperhatikan khusus produksi (gagal start bila dilanggar):

- `APP_ENV=production` wajib; `CORS_ALLOWED_ORIGINS` domain eksplisit — jangan `*`.
- `JWT_SECRET` acak kuat (>=32 char) + `MEDIA_SIGNING_KEY` terpisah dan beda.
- `DB_DSN` user aplikasi (`mini_erp_app`), bukan root tanpa password.
- S3: `STORAGE_UPLOAD_ROOT_PREFIX` eksplisit dan tak bertumpuk
  `STORAGE_PROTECTED_PREFIXES`; `STORAGE_DRIVER=local` butuh `STORAGE_DIR` absolut.
- `APP_TIMEZONE=Asia/Jakarta` dan `APP_LOCALE=id-ID` adalah konstanta aplikasi — tidak perlu (dan
  tidak boleh) dijadikan setelan yang bisa diubah dari UI (lihat
  [ADR-0006](../knowledge/decisions/ADR-0006-locale-timezone-locked.md)).

## 7. APK Kasir Android (`android-pos-shell`) — bangun ulang saat cutover (OP-05, D5)

POS di tablet kasir berjalan lewat **cangkang Android tipis** (`android-pos-shell`, paket `id.minierp.kasir`):
`WebView` layar penuh yang memuat POS web dan menyediakan jembatan cetak `window.AndroidPrinter`
(raw ESC/POS ke printer thermal Epson TM-T82X di LAN, `IP:9100`) — hal yang tak bisa dilakukan browser.
Keputusan D5 ([ADR-0018](../knowledge/decisions/ADR-0018-keputusan-paritas.md)): **dokumentasi saja**, tanpa perubahan kode;
sumber APK tetap di repo legacy (`H:\dimasprasetio\SAAS\mini-erp\android-pos-shell`, lihat `README.md` di sana).

**Kapan APK harus dibangun ulang.** Kode POS **tidak** ada di dalam APK; ia dimuat dari server saat dijalankan.
Jadi deploy web biasa tidak butuh APK baru. APK **wajib dibangun ulang** bila:

- domain/URL aplikasi berubah — `POS_URL` terkunci ke domain legacy (`https://minierp.my.id`) saat build.
  **Cutover ke domain/host baru = bangun ulang APK**; tanpa itu tablet tetap membuka sistem legacy;
- logika jembatan/printer native berubah (versi bridge, ePOS2 SDK).

**Langkah bangun ulang.**

1. Prasyarat: Android Studio (Ladybug/Koala+) atau CLI (Android SDK + JDK 17 + Gradle 8.7).
2. Ubah URL di `app/build.gradle.kts`:
   `buildConfigField("String", "POS_URL", "\"https://<domain-revamp>\"")` (gunakan HTTPS; WebView memuat URL ini sebagai halaman awal).
3. Naikkan `versionCode`/`versionName` pada `defaultConfig` agar tablet menerima sebagai pembaruan.
4. Bangun: `./gradlew assembleRelease` (tandatangani untuk distribusi) atau `assembleDebug`
   (`app/build/outputs/apk/debug/app-debug.apk`). Bila `gradle-wrapper.jar` belum ada: `gradle wrapper --gradle-version 8.7` sekali.
5. Pasang ke tiap tablet: `adb install -r <apk>` atau salin APK ke tablet dan buka (izinkan "Install dari sumber tak dikenal").
   Konfigurasi printer (IP/port, auto-cut, laci kas) tersimpan di SharedPreferences tablet — `install -r` tidak menghapusnya.
6. Setiap tablet: buka ikon **Kasir Mini ERP** (bukan Chrome), pastikan halaman login revamp tampil (bukan legacy).

**Uji jembatan `AndroidPrinter` (per tablet, setelah pasang/rebuild).**

1. Tablet dan printer satu LAN; port 9100 printer terbuka (DHCP reservation agar IP tetap).
2. Buka Pengaturan Printer native: tekan-lama **3 jari** di layar POS (otomatis muncul saat printer belum diatur) →
   **Cari printer** atau isi IP/port → **Tes Cetak** (kertas harus keluar) → **Simpan**.
3. Dari POS (login kasir): lakukan satu penjualan tunai uji → buka struk → **Cetak**. Struk thermal harus tercetak
   dan **tidak** membuka dialog cetak browser (bila dialog muncul, `window.AndroidPrinter` tidak terdeteksi → web
   jatuh ke `window.print()`; periksa bahwa yang dibuka adalah APK, bukan Chrome).
4. Uji laci kas (bila dipakai): cetak struk tunai dengan opsi laci aktif, atau tombol uji laci di Pengaturan Printer.
5. Uji gagal-aman: matikan printer lalu cetak — POS harus menampilkan toast "Gagal mencetak struk" dengan pesan
   dari bridge (mis. `NOT_CONFIGURED`, timeout), bukan membeku.
6. Kontrak bridge yang dipakai web ada di `apps/web/src/modules/print/lib/native-printer.ts`
   (`isAvailable`, `getConfig`, `saveConfig`, `scan`, `testPrint`, `printReceipt`, `openDrawer`; callback
   `window.onPrintResult` / `window.onPrinterScanResult`). Bila kontrak berubah, APK dan file itu harus diubah bersama.

Checklist cutover (RUNBOOK §5c) memuat butir ini.

## 8. Referensi

- [SYSTEM_DESIGN.md](SYSTEM_DESIGN.md) — arsitektur & stack
- [API_CONTRACT.md](API_CONTRACT.md) — konfigurasi environment lengkap
- [RUNBOOK.md](RUNBOOK.md) — backup/restore/rollback operasi harian, checklist cutover (§5c)
- `.env.production.example`, `infra/docker/Dockerfile`, `infra/nginx/nginx.conf`, `docker-compose.yml` — implementasi konkret
