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

## 4. Docker (Produksi) — VPS elcodelabs

Target: VPS **elcodelabs** (`ssh elcodelabs`, 103.189.235.79), mengikuti
`/opt/elcodelabs/SERVER_PLAYBOOK.md` di server ([ADR-0019](../knowledge/decisions/ADR-0019-deploy-elcodelabs-ghcr.md)).
Satu container aplikasi; MySQL di level host, dipakai bersama aplikasi lain dengan database + user
sendiri. Image: `infra/docker/Dockerfile` (Go 1.26.7 + Node 22, user non-root 10001, HEALTHCHECK
`/readyz`).

```txt
VPS elcodelabs
├── MySQL 8 di host (bersama) ── minierp_db / minierp_user@'172.%'
├── nginx host: situs <domain> -> 127.0.0.1:8084, TLS certbot
└── Docker
    ├── elkasir-app :8081 · elproof-app :8082 · elwedding-app :8083   (aplikasi lain)
    └── minierp-app :8084 -> 8080   (frontend statis + API Go)
```

| Konvensi | Nilai mini-erp-v2 |
|---|---|
| Folder server | `~/minierp` (`docker-compose.prod.yml`, `.env` 600, `deploy.sh`, `data/storage`, `backups/`) |
| Container / port | `minierp-app` / `127.0.0.1:8084` -> 8080 |
| Database / user | `minierp_db` / `minierp_user@'172.%'` (`mysql_native_password`) |
| Image | `ghcr.io/amandayora/mini-erp-v2:<git-sha>` (+ `:latest`), dibangun `release-image.yml` |
| Domain | `<domain>` dengan `server_name` sendiri, **tanpa** `default_server` (milik elkasir) |

**Build di CI, jalan di server.** `.github/workflows/release-image.yml` berjalan setelah `ci` hijau
untuk push ke `main` dan mendorong image `linux/amd64`. VPS (2 GB RAM) **tidak pernah** membangun
image. `docker-compose.yml` di akar repo hanya untuk uji lokal (membangun dari sumber).

### Setup pertama di server (sekali)

```bash
# 1. Database + user (host MySQL); password: openssl rand -hex 24
sudo mysql -e "CREATE DATABASE minierp_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
  CREATE USER 'minierp_user'@'172.%' IDENTIFIED WITH mysql_native_password BY '<hex>';
  GRANT ALL PRIVILEGES ON minierp_db.* TO 'minierp_user'@'172.%'; FLUSH PRIVILEGES;"

# 2. Folder app — salin dari repo: infra/elcodelabs/{docker-compose.prod.yml,deploy.sh}
#    dan .env.production.example (sebagai .env), lalu isi nilai asli.
mkdir -p ~/minierp/data/storage ~/minierp/backups
chmod 600 ~/minierp/.env && chmod +x ~/minierp/deploy.sh
sudo chown -R 10001:10001 ~/minierp/data/storage

# 3. Deploy pertama (database masih kosong -> backup dilewati), lalu seed awal:
#    `init` = katalog izin, role, chart, cabang kepala; `admin` = user owner pertama
#    (password acak dicetak SEKALI — arahkan ke berkas 600, ganti setelah login, hapus berkasnya)
cd ~/minierp && SKIP_BACKUP=1 ./deploy.sh <git-sha>
docker compose -f docker-compose.prod.yml run --rm --no-deps --entrypoint ./seed app init
(umask 077; docker compose -f docker-compose.prod.yml run --rm --no-deps --entrypoint ./seed app admin > ADMIN_AWAL.txt)

# 4. nginx + TLS (templat: infra/nginx/nginx.conf)
sudo cp nginx.conf /etc/nginx/sites-available/minierp && sudo ln -s ../sites-available/minierp /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx && sudo certbot --nginx -d <domain>
```

Lalu perbarui tabel *App registry* di `SERVER_PLAYBOOK.md` server (port 8084, `minierp_db`).
Paket GHCR baru defaultnya **privat**: login `docker login ghcr.io` dengan PAT `read:packages` yang
sudah tersimpan di server (dipakai aplikasi `amandayora/*` lain), atau jadikan paket publik.

### Deploy rutin & rollback

```bash
cd ~/minierp && ./deploy.sh <git-sha>
# = cek awal -> pull -> backup DB (sudo mysqldump -> backups/) -> migrate up -> up -d -> /readyz
```

Rollback = `./deploy.sh <git-sha-lama>` (detail: `docs/RUNBOOK.md` §4). `deploy.sh` menolak tag
`latest` dan menulis `IMAGE_TAG` ke `.env`; compose produksi menolak jalan tanpa `IMAGE_TAG`.

### Isi container

`docker-compose.prod.yml` mengikat port hanya ke loopback, me-mount `./data/storage` ke
`/data/storage` (`media/` bila `STORAGE_DRIVER=local`; `_private/` sesi WA, tak disajikan), rootfs
read-only, batas memori 768 MB, log 10m×3, `stop_grace_period: 30s`, dan service sekali-pakai
`migrate` (`--profile ops`). Container mengekspos satu port:

- `/api/v1/*` → backend API (+ `/healthz`, `/readyz`, `/api/v1/metrics` ber-token)
- `/media/*?exp&sig` → berkas media (HMAC + expiry, tanpa listing; `/uploads/*` = 404)
- selain itu → berkas statis SPA frontend

Koneksi DB dari container ke MySQL host lewat `host.docker.internal` (`extra_hosts:
host-gateway`). **Jangan** menambahkan service `mysql:` ke compose — melanggar standar server.

## 5. Migrasi di Produksi

- `deploy.sh` menjalankan `docker compose -f docker-compose.prod.yml --profile ops run --rm migrate up` sebagai langkah terpisah
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

Salin `.env.production.example` ke `~/minierp/.env` di server, isi nilai asli, `chmod 600`. Poin yang
wajib diperhatikan khusus produksi (gagal start bila dilanggar):

- `APP_ENV=production` wajib; `CORS_ALLOWED_ORIGINS` domain eksplisit — jangan `*`.
- `JWT_SECRET` acak kuat (>=32 char) + `MEDIA_SIGNING_KEY` terpisah dan beda.
- `DB_DSN` user aplikasi (`minierp_user`), bukan root tanpa password; password hex (dipakai juga di
  URL `migrate`, karakter seperti `@`/`/` merusaknya).
- `TRUSTED_PROXIES=172.16.0.0/12,…`: nginx host terlihat dari bridge docker; tanpa ini semua
  pengguna tampak satu IP bagi pembatas login.
- S3: `STORAGE_UPLOAD_ROOT_PREFIX` eksplisit dan tak bertumpuk
  `STORAGE_PROTECTED_PREFIXES`; `STORAGE_DRIVER=local` butuh `STORAGE_DIR` absolut.
- `APP_TIMEZONE=Asia/Jakarta` dan `APP_LOCALE=id-ID` adalah konstanta aplikasi — tidak perlu (dan
  tidak boleh) dijadikan setelan yang bisa diubah dari UI (lihat
  [ADR-0006](../knowledge/decisions/ADR-0006-locale-timezone-locked.md)).

## 7. APK Kasir Android (`android-pos-shell`) dan cutover tablet (OP-05, D5)

POS di tablet kasir berjalan lewat **cangkang Android tipis** (`android-pos-shell`, paket `id.minierp.kasir`):
`WebView` layar penuh yang memuat POS web dan menyediakan jembatan cetak `window.AndroidPrinter`
(raw ESC/POS ke printer thermal Epson TM-T82X di LAN, `IP:9100`) — hal yang tak bisa dilakukan browser.
Keputusan D5 ([ADR-0018](../knowledge/decisions/ADR-0018-keputusan-paritas.md)): **dokumentasi saja**, tanpa perubahan kode;
sumber APK tetap di repo legacy (`H:\dimasprasetio\SAAS\mini-erp\android-pos-shell`; di Mac
`~/Documents/dimas-project-individu/mini-erp/android-pos-shell`, lihat `README.md` di sana).

**Kapan APK harus dibangun ulang.** Kode POS **tidak** ada di dalam APK; ia dimuat dari server saat dijalankan.
APK membuka `POS_URL` (`https://minierp.my.id`, terkunci saat build) tanpa pembatasan host, dan kontrak jembatannya
sama dengan `apps/web/src/modules/print/lib/native-printer.ts`. Jadi deploy web biasa **dan cutover yang
mempertahankan domain `minierp.my.id`** (DNS dialihkan dari VPS legacy ke VPS elcodelabs) **tidak** butuh APK baru.
APK hanya dibangun ulang bila:

- domain/URL aplikasi berubah — tanpa APK baru tablet tetap membuka `POS_URL` lama;
- logika jembatan/printer native berubah (versi bridge, ePOS2 SDK, kontrak `native-printer.ts`).

Syaratnya: `https://minierp.my.id` sudah melayani v2 dengan sertifikat valid di server baru **sebelum** DNS dialihkan —
APK memasang `usesCleartextTraffic=false`, jadi HTTP polos atau sertifikat salah = layar kosong di tablet.

**Service worker legacy di perangkat.** Aplikasi legacy mendaftarkan service worker `/sw.js` (lingkup `/`, cache
`mini-erp-pwa-v1-app`/`-runtime`, navigasi network-first, `/offline.html` dipra-cache) di setiap WebView/browser yang
pernah membukanya. v2 tidak memakai service worker, tetapi registrasi lama tetap hidup setelah domain dialihkan.
Karena itu v2 menyajikan **`/sw.js` pembersih** (`apps/web/public/sw.js`, `Content-Type: text/javascript`,
`Cache-Control: no-store`): saat app dibuka dan online, browser memeriksa ulang `/sw.js`, memasang versi pembersih,
yang menghapus semua cache origin, mencabut registrasinya, lalu memuat ulang tab sekali — sesudahnya v2 berjalan tanpa
service worker. Tanpa berkas itu (`/sw.js` jatuh ke fallback SPA HTML) pembaruan selalu gagal dan saat offline tablet
menampilkan halaman offline legacy. Berkas ini dipertahankan di v2 (perangkat yang lama tak dibuka baru dibersihkan saat
kembali online); v2 sendiri **tidak** mendaftarkannya (dijaga `src/test/legacy-sw-kill-switch.test.ts`, e2e E18).

Memastikan service worker sudah hilang di tablet:

- APK build debug (WebView otomatis dapat di-*inspect*): sambungkan USB debugging, buka `chrome://inspect/#service-workers`
  di Chrome desktop — tidak boleh ada entri `https://minierp.my.id/`. APK release tidak dapat di-*inspect*.
- Gejala: setelah app dibuka sekali saat online, matikan Wi-Fi lalu buka lagi — halaman "Mini ERP sedang offline" legacy
  tidak boleh muncul lagi (yang tampil halaman galat jaringan WebView).
- Upaya terakhir: Setelan Android → Aplikasi → Kasir Mini ERP → Penyimpanan → **Hapus data**. Ini juga menghapus
  konfigurasi printer (IP/port, auto-cut, laci) yang tersimpan di tablet — isi ulang lewat Pengaturan Printer.

**Langkah per tablet sesudah DNS dialihkan** (uji satu tablet dulu, baru semua tablet):

1. **Tutup paksa** app Kasir Mini ERP (Setelan → Aplikasi → Kasir Mini ERP → Paksa berhenti, atau geser dari daftar app
   terbaru), lalu buka lagi dari ikonnya (bukan Chrome). Tablet harus online; halaman boleh memuat ulang sekali sendiri
   (service worker legacy dibersihkan).
2. Pastikan halaman login **v2** yang tampil (bukan legacy). Bila masih legacy: DNS tablet/router belum berpindah —
   tunggu TTL atau mulai ulang Wi-Fi, lalu ulangi langkah 1.
3. **Login ulang dengan akun pribadi kasir** (mis. `widia`, `luluk`, `nurma`, `selvi`). Sesi legacy tidak berlaku di v2,
   dan hash kata sandi akun pribadi ikut termigrasi, jadi sandinya sama dengan di legacy. Akun bawaan seed (`kasir`,
   `admin`, `staff`, …) yang masih bersandi bawaan dikunci ETL (temuan `USER_SEED_PASSWORD`) dan tidak dipakai di tablet.
4. Uji cetak: Tes Cetak di Pengaturan Printer, satu struk penjualan tunai uji, dan laci kas bila dipakai (rincian di
   "Uji jembatan" di bawah). Konfigurasi printer tersimpan di tablet, jadi tetap ada setelah pengalihan.

**Langkah bangun ulang** (hanya bila domain/URL atau jembatan native berubah).

1. Prasyarat: Android Studio (Ladybug/Koala+) atau CLI (Android SDK + JDK 17 + Gradle 8.7).
2. Ubah URL di `app/build.gradle.kts`:
   `buildConfigField("String", "POS_URL", "\"https://<domain-revamp>\"")` (gunakan HTTPS; WebView memuat URL ini sebagai halaman awal).
3. Naikkan `versionCode`/`versionName` pada `defaultConfig` agar tablet menerima sebagai pembaruan.
4. Bangun: `./gradlew assembleRelease` (tandatangani untuk distribusi) atau `assembleDebug`
   (`app/build/outputs/apk/debug/app-debug.apk`). Bila `gradle-wrapper.jar` belum ada: `gradle wrapper --gradle-version 8.7` sekali.
5. Pasang ke tiap tablet: `adb install -r <apk>` atau salin APK ke tablet dan buka (izinkan "Install dari sumber tak dikenal").
   Konfigurasi printer (IP/port, auto-cut, laci kas) tersimpan di SharedPreferences tablet — `install -r` tidak menghapusnya.
6. Setiap tablet: buka ikon **Kasir Mini ERP** (bukan Chrome), lalu ikuti "Langkah per tablet" di atas mulai butir 2.

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
- `.env.production.example`, `infra/docker/Dockerfile`, `infra/elcodelabs/` (compose produksi + `deploy.sh`), `infra/nginx/nginx.conf`, `.github/workflows/release-image.yml` — implementasi konkret; `docker-compose.yml` akar = uji lokal
