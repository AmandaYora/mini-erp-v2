# Rancangan F0 + F1 + F6 — Fondasi Verifikasi, Keamanan/Konfigurasi, Deploy/Ops

Target: `H:/dimasprasetio/SAAS/mini-erp-revamp` (Go modular monolith + MySQL, React 19).
Legacy (read-only referensi): `H:/dimasprasetio/SAAS/mini-erp` (NestJS).
Disusun 2026-10-08 dari **pembacaan kode sendiri** (bukan sekadar klaim audit). Read-only: tidak ada
berkas repo yang diubah. Satu-satunya tindakan non-baca: menjalankan `go test ./...`, `tsc`, `eslint`,
`vitest`, `arch-check` dan `npm run verify` (hanya menyentuh database `mini_erp_test`), serta
`SELECT` ke MySQL Laragon.

Konvensi ukuran: S = <= 0,5 hari, M = 0,5-2 hari, L = > 2 hari.
Kode baru memakai gaya `database/sql` yang ada (sqlc = fase terakhir). Perubahan schema = migrasi tambahan
(`000032_...` dst; nomor 000026-000029 memang tidak terpakai — pakai nomor bebas berikutnya saat implementasi,
koordinasikan dengan paket fase lain), di-squash di akhir.

---------------------------------------------------------------------------------------------------

## 0. Ringkasan temuan: terbukti / terbantah / dikoreksi

| # | Klaim audit | Status | Bukti singkat |
|---|---|---|---|
| F0 | `go test` butuh `-buildvcs=false` | **Sebagian benar (dikoreksi)**. Yang gagal: `go build ./...`, `go vet ./cmd/...`, **`npm run verify` (gagal di langkah 1)**, dan **`npm run dev:api`** (air memanggil `go build ./cmd/server`). `go test ./...` sendiri LULUS tanpa flag | Dijalankan: `go build ./cmd/server` -> `error obtaining VCS status: exit status 128`; `git` -> `detected dubious ownership` (safe.directory global berisi `H:/dimasprasetio/SAAS/mini-erp` tetapi **tidak** `.../mini-erp-revamp`) |
| F0 | `TestMain` `os.Exit(1)` tanpa MySQL; `truncateAll` tanpa guard | **Terbukti** | `internal/integration/integration_test.go:119-135` (Exit), `:80-106` (`truncateAll`), `:127` (`TESTDB_REBUILD` = `DROP DATABASE` tanpa guard); `TEST_DB_DSN` bebas apa saja |
| F0 | tambahan: server MySQL Laragon yang sama berisi DB lain (`elkasir_db`, `elproof_db`, `indobraga_*`, `mini_erp_db`, ...) | **Baru** | `SHOW DATABASES`: salah ketik `TEST_DB_DSN=.../mini_erp_db` akan mem-TRUNCATE 59 tabel DB dev |
| F0 | tambahan: "reusing migrated test db" tidak mendeteksi migrasi baru/berubah | **Baru** | `integration_test.go:158-163` hanya menghitung tabel > 0 |
| F0 | tambahan: integration test 196 dtk (39 test x `freshDB` yang TRUNCATE 59 tabel) | **Baru (terukur)** | `go test ./... -count=1` = 3m29s, 196 dtk di paket `integration` |
| F1a | `/uploads/` menyajikan seluruh `StorageDir` tanpa auth, termasuk `whatsapp_session.db` | **Terbukti, kritis** | `server.go:83-97` (`http.FileServer(http.Dir(dir))` + listing default), `main.go:140`, `assistant/infrastructure/gateway.go:50` (`filepath.Join(storageDir,"whatsapp_session.db")`). `GET /uploads/whatsapp_session.db` = unduh kunci sesi WA; `GET /uploads/` = listing direktori |
| F1b | default `APP_ENV=development` melonggarkan validasi secret | **Terbukti** | `config.go:96-107`; `.env` lokal juga `APP_ENV=development` |
| F1c | `X-Real-IP` dipercaya mentah | **Terbukti** | `auth/presentation/handler.go:24-26`; + limiter di-reset total saat >10k key (`service.go:Allow`) sehingga banjir key menghapus semua hitungan |
| F1d | ganti sandi/nonaktif tidak mencabut sesi; Refresh tak cek status; sesi hidup lagi saat diaktifkan | **Terbukti** (sebagian diperhalus) | `user/application/service.go` `SetStatus` (hanya `invalidateUser`, tidak `revoked_at`), `ChangePassword` (tanpa cabut/invalidasi). Perhalus: `Verify` -> `resolveSession` memang menolak user `inactive` selama nonaktif, jadi akses tertutup; risikonya di **reaktivasi** (semua sesi lama hidup lagi) dan **ganti sandi** (sesi pencuri tetap hidup). `Refresh` (`auth/service.go:247-290`) tidak membaca user. Legacy mencabut sesi pada keduanya (`user.service.ts` `updateStatus`/`changePassword`) = **regresi paritas** |
| F1e | sandi > 72 byte -> 500; cost 10 vs legacy 12; tanpa rehash | **Terbukti** | `service.go:66-72` `bcrypt.GenerateFromPassword` -> `ErrPasswordTooLong` (x/crypto v0.55) dibungkus `apperror.Internal`; `bcrypt.DefaultCost`=10; legacy `bcrypt.hash(...,12)` (bcryptjs; bcryptjs memotong 72 byte tanpa error). Tambahan: user tak dikenal kembali cepat tanpa bcrypt (**timing enumeration username**) |
| F1f | seed: tak boleh di production, password hardcode; DB baru tanpa permission/role = fail-closed | **Terbukti** | `cmd/seed/main.go:101-111` `Admin123!`; permission hanya masuk via `cmd/seed` (server tidak pernah memanggil `EnsurePermissions`) |
| F1f | role default "gudang, keuangan sesuai legacy" | **Terbantah** | Legacy hanya punya `superadmin, owner, admin, staff` (sistem, `permission-code.ts DEFAULT_ROLE_PERMISSIONS`) + `kasir` (custom di `seed-runner.ts`). "gudang/keuangan" hanya nama contoh di `role.service.spec.ts`. Jangan di-seed |
| F1g | `is_system` tak pernah di-set 1; tak ada proteksi admin terakhir | **Terbukti** | `user/application/service.go` `CreateRole` tidak mengisi `IsSystem` (repo `:343-356` mendukung); role `owner` dibuat seed dengan `is_system=0` -> bisa dihapus bila tak terpakai, permission-nya bisa dikosongkan lewat `SetRolePermissions` (tanpa guard); `UpdateUser` mengganti semua role tanpa cek; tak ada kebijakan `superadmin` (legacy `reserved-role-policy.ts`, user id 1) |
| F1h | approver koreksi stok: tanpa rate limit; bcrypt di dalam tx | **Terbukti** (nomor baris bergeser) | `stock/application/transactions.go:323` memanggil `checkApprover` (`:358-362` `VerifyCredentials`) **setelah** `repo.Begin` (`:300`) dan setelah `GetBalanceTx` |
| F1i | kredensial S3 sama dengan legacy | **Terbukti** (dibandingkan via hash, nilai tak ditampilkan) | `.env` revamp vs `mini-erp/apps/api/.env`: `STORAGE_BUCKET`, `STORAGE_ACCESS_KEY_ID`, `STORAGE_SECRET_ACCESS_KEY`, `STORAGE_ENDPOINT` **identik**; legacy prefix `vioni/production/upload/...`, revamp dev `vioni/development/upload/...` (bucket & kunci sama). `S3Driver.Remove` menghapus objek apa pun yang ada di baris `media_files` |
| F1j | reuse refresh-token mencabut semua sesi | **Terbukti; race antar-tab nyata** | `Rotate` (`auth/infrastructure/repository.go`) `UPDATE ... WHERE id=?` tanpa CAS; dua tab membaca token yang sama -> dua rotasi -> token tab A menjadi `prev_refresh_hash` -> pemakaian berikutnya dianggap replay -> `RevokeUserSessions` (semua sesi user). Frontend: single-flight per tab saja (`http-client.ts:62-87`), token di `localStorage` bersama antar-tab |
| F1k | tidak ada audit assistant/whatsapp | **Terbukti, tapi paritas legacy** | `assistant/application/service.go:24-26` komentar "exempt (legacy rule)"; legacy `modules/assistant` & `modules/whatsapp` memang tanpa `auditLog`. Tetap ditambahkan (nilai keamanan; mengubah whitelist nomor & reset sesi WA = tindakan sensitif) |
| F6 | Dockerfile `golang:1.22` vs `go 1.26.0`+`toolchain go1.26.7`; `npm install` tanpa lock; tanpa HEALTHCHECK/non-root | **Terbukti** | `infra/docker/Dockerfile:2-8,11`; `package-lock.json` ada di root tetapi tidak di-COPY. **Tambahan:** `alpine:3.20` tanpa `ca-certificates` -> panggilan HTTPS ke S3 gagal verifikasi sertifikat; `timeutil` aman (fallback WIB) |
| F6 | compose tanpa volume/restart/healthcheck, 8080 terekspos; nginx tanpa TLS/body size | **Terbukti** | `docker-compose.yml` (10 baris), `infra/nginx/nginx.conf` |
| F6 | `/health` tanpa ping DB | **Terbukti** | `server.go:25-27` |
| F6 | tak ada request-id, user/cabang di log, stack panic, metrics | **Terbukti** | `middleware.go` `recoverer` (tanpa `debug.Stack`), `requestLog` (method/path/status/durasi saja); `statusWriter` & `gzipResponseWriter` tanpa `Unwrap()` (menghalangi `http.ResponseController`) |
| F6 | timeout seragam 15s/30s | **Terbukti** | `main.go:143-147`; upload 5 MB dan ekspor lewat server yang sama; tak ada `ReadHeaderTimeout`/`IdleTimeout` |
| F6 | graceful shutdown tidak menghentikan WA; Boot pakai Background | **Terbukti** + **temuan baru** | `main.go:105-109,161-163`. Baru: `Service.Connect(r.Context())` (`assistant/presentation/handler.go:114`) meneruskan ctx request ke `gateway.GetQRChannel(ctx)`; whatsmeow menyimpan ctx itu di `qrChannel` (`qrchan.go:235`) -> kanal QR berhenti saat request selesai. **Diduga** pairing via UI rusak; perlu uji manual dengan ponsel (tandai: hipotesis, belum terbukti) |
| F6 | `docs/DEPLOYMENT.md` usang | **Terbukti** | "Go 1.21+", `go 1.21`, tak menyebut `APP_ENV`, `/uploads`, volume, backup |
| Koreksi dok | PARITY_AUDIT §11.2 "C0 ditunda" | **Terbukti keliru** | `.git` ada, branch `main`, remote `AmandaYora/mini-erp-v2`, `.gitignore` sudah memuat `apps/api/storage/` (`PARITY_AUDIT.md:712-715`) |
| Koreksi dok | `api-standard.md` `total_pages` vs kode `totalPages` | **Terbukti** | `response.go:15` `json:"totalPages"`; `API_CONTRACT.md:57` & `API_GUIDE.md:17` & `http-client.ts:18` semua `totalPages`; hanya `.claude/rules/api-standard.md` yang `total_pages` |

Baseline terukur (2026-10-08, mesin ini): `go test ./... -count=1` hijau (integration 196 dtk, total 3m29s);
`tsc --noEmit` 25 dtk bersih; `eslint` 63 dtk bersih; `vitest` 55 dtk (17 file) hijau; `arch-check` OK.

---------------------------------------------------------------------------------------------------

# F0 — Fondasi verifikasi

## F0-1 Menyalakan MySQL Laragon untuk test + membuat `mini_erp_test` (S)

**Fakta terverifikasi**
- Biner: `C:/laragon/bin/mysql/mysql-8.4.3-winx64/bin/{mysqld,mysql,mysqladmin,mysqldump}.exe`.
- Konfigurasi: `.../mysql-8.4.3-winx64/my.ini` (`port=3306`, `datadir="C:/laragon/data/mysql-8.4"`, `secure-file-priv=""`).
- Saat dibaca, server **sudah berjalan** (PID 17628 listen `0.0.0.0:3306` & `33060`; `mysqld.exe` kedua PID 17468 juga ada).
  `root` @ `localhost` **tanpa password** diterima lewat TCP `127.0.0.1:3306`. Versi 8.4.3, collation
  `utf8mb4_0900_ai_ci`, `sql_mode` strict (cocok dengan migrasi). `mini_erp_test` sudah ada (59 tabel). User lain: `indobraga`.
- `data/mysql-8.4/DESKTOP-6SU19HQ.err` berakhir dengan `Failed to initialize DD Storage Engine ... Aborting`:
  pola kegagalan saat **instance kedua** dijalankan pada datadir yang sama. Skrip WAJIB mengecek port 3306 dulu.
- Server dipakai bersama proyek lain (`elkasir_db`, `elproof_db`, `elwedding_db`, `indobraga_*`, `jwswedding_*`, `stockflow_test`, ...).

**Rancangan**
- `scripts/test-db.ps1` (utama, mesin Windows pemilik) dan `scripts/test-db.sh` (Git Bash/CI-lokal), subperintah `up|status|down|reset|dsn`:
  1. `status`: `Get-NetTCPConnection -LocalPort 3306 -State Listen` -> jika ada, lewati start; ping `mysqladmin.exe --protocol=tcp -h127.0.0.1 -uroot ping`.
  2. `up`: bila port bebas -> `Start-Process mysqld.exe --defaults-file=<my.ini> --console -WindowStyle Hidden`, tunggu <= 60 dtk dengan loop ping; cetak 20 baris terakhir `.err` jika gagal.
     Lokasi dapat di-override `LARAGON_MYSQL_DIR`.
  3. Buat DB + user terbatas (idempotent, dijalankan sebagai root):
     `CREATE DATABASE IF NOT EXISTS mini_erp_test CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;`
     `CREATE USER IF NOT EXISTS 'mini_erp_test'@'localhost' IDENTIFIED BY 'mini_erp_test'; GRANT ALL ON mini_erp_test.* TO ...;`
     (`@'127.0.0.1'` tidak perlu: klien TCP ke 127.0.0.1 dicocokkan sebagai `localhost` pada instalasi ini — **uji**; bila tidak, buat keduanya.)
     User ini **tidak dapat** menyentuh `mini_erp_db` / DB proyek lain: lapisan pengaman kedua di luar guard kode (F0-2).
  4. `reset`: `DROP DATABASE mini_erp_test` lalu buat ulang (hanya nama ini, hardcode, tanpa parameter).
  5. `dsn`: mencetak `TEST_DB_DSN=mini_erp_test:mini_erp_test@tcp(127.0.0.1:3306)/mini_erp_test`.
- Default `testDBDSN()` diganti ke DSN user terbatas tersebut (bukan `root:@localhost`).
- `down`: hanya menghentikan proses yang **dimulai skrip** (simpan PID di `%TEMP%\mini-erp-test-mysql.pid`); jangan membunuh `mysqld` Laragon pemilik.
- Dokumentasi di `docs/DEPLOYMENT.md` bagian "Testing" + `knowledge/` (lihat F6-8).

**Reuse**: tidak ada skrip serupa di `scripts/` (hanya `arch-check.mjs`, `migrate.mjs`, `scaffold-modules.sh`). `migrate.mjs` pola pembaca `.env` dapat ditiru.
**Test**: jalankan skrip di keadaan (a) server hidup, (b) server mati, (c) `.err` berisi kegagalan; pastikan idempotent 2x.
**Volume/performa**: tidak relevan. **Dependensi**: F0-2 (default DSN), F0-4 (CI memakai DSN CI sendiri).

## F0-2 Pengaman `integration_test.go` (M)

**Masalah** (`internal/integration/integration_test.go`): `TestMain` `os.Exit(1)` tanpa pesan yang menolong saat MySQL mati;
`truncateAll` dan `TESTDB_REBUILD` bekerja pada nama DB apa pun; tabel lama dipakai ulang walau migrasi berubah; lambat.

**Rancangan** (semua di berkas test itu; tanpa kode produksi):
1. `mustBeTestDSN(dsn)`: parse server & nama DB (`splitDSN` sudah ada). **Tolak** (exit 2 dengan pesan Indonesia) bila nama DB tidak berakhiran `_test`
   atau host bukan `localhost|127.0.0.1|::1|[::1]` (override `ALLOW_REMOTE_TEST_DB=1` untuk service container CI).
   Dipanggil di awal `TestMain` sebelum `sql.Open`, sebelum `DROP`/`CREATE`.
2. `truncateAll`: tambah `SELECT DATABASE()` dan `strings.HasSuffix(name,"_test")` sebagai pertahanan kedua (`t.Fatalf`).
3. Tanpa MySQL: dua mode. Default = gagal keras dengan pesan "jalankan `scripts/test-db.ps1 up`" (cegah hijau palsu). `INTEGRATION_OPTIONAL=1` -> cetak peringatan & `os.Exit(0)` (untuk laptop tanpa DB; **CI tidak boleh** memakainya). Probing dengan `PingContext` timeout 3 dtk.
4. Deteksi migrasi basi: tabel penanda `_it_schema(hash CHAR(64))` berisi `sha256` gabungan seluruh `*.up.sql` terurut. Cocok -> pakai ulang; beda/absen -> `DROP` semua tabel DB tersebut (bukan DROP DATABASE; FK check off) lalu terapkan ulang. `TESTDB_REBUILD=1` tetap ada. `truncateAll` mengecualikan tabel berawalan `_it_`.
5. Percepatan `freshDB`: sebelum `TRUNCATE`, `SELECT EXISTS(SELECT 1 FROM t)`; hanya TRUNCATE tabel berisi. Catatan risiko: tabel kosong dengan AUTO_INCREMENT maju tak di-reset; bila ada test bergantung ID mulai 1, tambahkan reset khusus tabel itu. Ukur sebelum/sesudah (target < 90 dtk dari 196 dtk). Opsi lanjutan: `t.Parallel` tidak aman (satu DB) — jangan.
6. Hapus `t := &testing.T{}` yang tak berguna (`:149-150`).

**Test**: test kecil `TestMustBeTestDSN` (tabel kasus: `mini_erp_db`, `x_test`, `root@10.0.0.5/x_test`, `.../x_test?parseTime=true`) — fungsi murni, dapat dijalankan tanpa DB (letakkan di berkas `_test.go` terpisah yang tidak terikat `TestMain`? `TestMain` per-paket; maka pindahkan fungsi murni ke `internal/integration/dsnguard_test.go` namun `TestMain` memanggilnya; uji dengan subproses `go test -run` tidak perlu — cukup unit test fungsi sebelum `TestMain` bergantung DB: jadikan guard fungsi murni yang dipanggil `TestMain` dan diuji dengan `TestMustBeTestDSN` yang tetap butuh `TestMain` lulus -> akibatnya test guard hanya jalan bila DB ada. Alternatif bersih: letakkan guard di paket kecil `internal/testutil/dsnguard` (non-test .go, murni, ber-unit-test sendiri) — **pilih ini**; arch-check aturan A tidak berlaku di luar `internal/modules`).
**Volume**: 39 test x 59 tabel. **Dependensi**: F0-1. **Ukuran**: M.

## F0-3 Perintah `verify` yang benar-benar jalan di mesin ini (S)

**Masalah**: `npm run verify` gagal di langkah pertama (`go build ./...` -> VCS stamping; git menolak repo "dubious ownership"). `.air.toml`
(`cmd = "go build -o ./tmp/main ./cmd/server"`) dan `build:api` terkena hal yang sama. Akar masalah = `safe.directory`, bukan kode.

**Rancangan**
- Akar masalah (tindakan pemilik, satu kali): `git config --global --add safe.directory H:/dimasprasetio/SAAS/mini-erp-revamp`. Dokumentasikan.
- Ketahanan di repo (agar tidak bergantung pengaturan mesin): 
  - `.air.toml`: `go build -buildvcs=false -o ./tmp/main ./cmd/server`.
  - `package.json` `build:api`: `go build -buildvcs=false ...`. (Produksi tetap boleh menyematkan VCS lewat `-ldflags -X main.version=` dari build arg, lihat F6-1/F6-5; di Docker `.git` dikecualikan `.dockerignore`, jadi `-buildvcs=false` memang diperlukan di sana juga.)
  - `scripts/verify.mjs` menggantikan satu baris `verify` yang rapuh (`&&` berantai + `cd`): menjalankan langkah berurutan dengan `spawnSync`, mengatur `GOFLAGS=-buildvcs=false` (hormati bila sudah ada), mencetak ringkasan waktu per langkah, `--fast` (lewati integration & vitest), `--api`, `--web`. Urutan: preflight DB test (ping; saran `scripts/test-db.ps1 up`) -> `go build ./...` -> `go vet ./...` -> `go test ./... -count=1` -> `tsc --noEmit` -> `npm run lint` -> `npm run test` -> `arch-check`. `package.json`: `"verify": "node scripts/verify.mjs"`, `"verify:fast": "node scripts/verify.mjs --fast"`.
- Tambah `govulncheck ./...` (terpasang di `~/go/bin`) sebagai langkah opsional `--vuln` (tidak menghalangi).

**Test**: jalankan `npm run verify` penuh di mesin ini (harus hijau; perkiraan 5-6 menit); jalankan dengan MySQL mati (pesan jelas, kode keluar != 0).
**Dependensi**: F0-1, F0-2. **Ukuran**: S.

## F0-4 CI GitHub Actions (M)

`.github/` belum ada. Repo remote `AmandaYora/mini-erp-v2` (branch `main`). Berkas: `.github/workflows/ci.yml` (+ `.github/dependabot.yml` untuk `gomod`, `npm`, `github-actions`, `docker`).

Pemicu: `pull_request` dan `push` ke `main`; `concurrency` per ref dengan `cancel-in-progress`; `permissions: contents: read`.

Job:
1. `api` (ubuntu-latest, timeout 20 mnt): `services.mysql` image `mysql:8.4` (env `MYSQL_ROOT_PASSWORD=ci`, `MYSQL_DATABASE=mini_erp_test`, `--health-cmd "mysqladmin ping -h127.0.0.1 -pci" --health-interval 5s --health-retries 20`, port 3306). `actions/setup-go` dengan `go-version-file: apps/api/go.mod` (toolchain 1.26.7 ter-resolve) + cache. Env `TEST_DB_DSN=root:ci@tcp(127.0.0.1:3306)/mini_erp_test`, `ALLOW_REMOTE_TEST_DB=1` tidak perlu (127.0.0.1 lolos). Langkah: `go build ./...`, `go vet ./...`, `gofmt -l` (gagal bila ada keluaran), `go test ./... -count=1 -race` (cek `-race` butuh cgo; ubuntu ada gcc; bila waktu 2x lipat, pindah ke job nightly), `govulncheck ./...` (continue-on-error awal).
   `.gitattributes`/`autocrlf=false` aman. `actions/checkout` memberi `safe.directory` otomatis (tidak perlu `-buildvcs=false`, tetapi `GOFLAGS=-buildvcs=false` tetap diset agar sama dengan lokal).
2. `web` (ubuntu, node 20 + cache npm): `npm ci` (memakai `package-lock.json` root), `npx tsc --noEmit -p apps/web`, `npm run lint -w apps/web`, `npm run test -w apps/web` (vitest.config sudah mengunci `NODE_ENV=development`, lihat `PARITY_AUDIT.md:716`). `npm run build:web`.
3. `arch` (ubuntu, node 20): `node scripts/arch-check.mjs`.
4. `docker` (hanya setelah 1-3 hijau): `docker build -f infra/docker/Dockerfile .` tanpa push; pada `main`, opsional push ke GHCR (butuh keputusan; default = build saja).
5. `migrations` (pagar tambahan): terapkan semua `up` ke MySQL kosong dengan **golang-migrate CLI** (bukan loader test) lalu `down` seluruhnya — memverifikasi file `down` dan kecocokan CLI-vs-test-loader. `go install -tags mysql github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1` (versi lokal `v4.19.1`).

Uji sebelum dikomit: jalankan setara lokal lewat `scripts/verify.mjs`; validasi YAML dengan `actionlint` (opsional).
**Dependensi**: F0-2 (guard), F0-3, F6-1 (job docker). **Volume**: 3-6 menit tanpa `-race`.

---------------------------------------------------------------------------------------------------

# F1 — Keamanan & konfigurasi

## F1-1 Upload: pisahkan sesi WA, sajikan hanya `media/`, URL lokal bertanda tangan (L)

**Bukti**: lihat §0 (F1a). Seluruh URL lokal dikeluarkan satu pintu: `LocalDriver.URLFor` (`media/infrastructure/driver.go:64-67`),
dipanggil via `media.GetURL` dari `delivery/application/service.go:428,446`, `payment/application/service.go:856,874`, `product/presentation/media.go:31,70`.
Frontend tidak menyebut `/uploads` (grep `apps/web/src` kosong); URL dipakai sebagai `src`/`href` apa adanya (tak bisa menyertakan header Authorization -> perlu tanda tangan di URL).

**Rancangan**
1. **Config** (`config.go`): `StorageDir` (default `./storage`), baru `SessionDir` = `SESSION_DIR` (default `<StorageDir>/_private`), `MediaDir` = `<StorageDir>/media`,
   `MediaSigningKey` = `MEDIA_SIGNING_KEY` (opsional; bila kosong turunkan `HMAC-SHA256(JWT_SECRET, "media-url-v1")` agar tak menambah rahasia wajib; di production dianjurkan diisi terpisah). `Storage.LocalURLTTLSecs` memakai `SignedURLTTLSecs` yang sudah ada.
   Validasi: `SessionDir` tidak boleh berada di dalam `MediaDir` (dan sebaliknya) — gagal start dengan pesan jelas.
2. **Gateway**: `assistant.NewModule(db, cfg.SessionDir, ...)` (parameter `storageDir` -> `sessionDir`); `NewGateway` membuat folder `0o700`, berkas `whatsapp_session.db` `0o600` (setelah dibuat: `os.Chmod`). `main.go:96` ganti argumen.
3. **LocalDriver**: `NewLocalDriver(mediaDir)`; `Store` menulis ke `mediaDir/<key>` (key tetap tanpa awalan `media/`, sehingga kolom `media_files.file_key` **tidak berubah**); mode file `0o640`/dir `0o750`. Tambah proteksi traversal defensif `filepath.Rel` (key server-generated, tetapi baris DB hasil ETL tidak dipercaya mutlak).
4. **Penyaji**: ganti `Server.MountUploads(dir)` -> `Server.MountMedia(dir, signer)` di rute `GET /media/{path...}` (bukan `/uploads/`; `/uploads/` dihapus -> 404). Handler:
   - tolak bila `path` kosong atau diakhiri `/` (listing mati) -> 404;
   - verifikasi `?exp=<unix>&sig=<base64url>`: `sig = HMAC-SHA256(key, "GET\n"+path+"\n"+exp)`; bandingkan `hmac.Equal`; `exp` lewat -> 403 (`410` tidak perlu); 
   - buka lewat `http.Dir` dibungkus `noDirFS` (FileSystem yang `Open` mengembalikan error untuk direktori) atau `http.ServeContent` setelah `os.Open`+`Stat` (cek `IsRegular`); `Content-Type` dari ekstensi whitelist (`.jpg .jpeg .png .webp .pdf`), `X-Content-Type-Options: nosniff`, `Content-Disposition: inline` + `Content-Security-Policy: default-src 'none'` untuk PDF/gambar; `Cache-Control: private, max-age=<sisa exp, maks 3600>`.
   - Paket baru `internal/shared/signedurl` (utilitas teknis: `Sign(path, exp)`/`Verify(path, exp, sig)`), diuji unit. Jam & kunci diinjeksi agar bisa di-test.
5. **URL stabil agar cache browser bekerja**: `exp` dibulatkan ke batas jendela: `exp = (floor(now/step)+1)*step + ttl` dengan `step = 1 jam`; dalam satu jendela URL identik -> cache `private` efektif (tanpa pembulatan, tiap respon daftar produk menghasilkan URL baru dan cache 1 tahun `immutable` yang ada kini akan mubazir).
   `LocalDriver.URLFor` menerima `signer`; hasil `"/media/"+key+"?exp=..&sig=.."`.
6. **Vite dev**: tambah proxy `"/media"` ke target API di `apps/web/vite.config.ts` (sekarang hanya `/api`, sehingga gambar lokal tak tampil di dev).
7. **Migrasi berkas lama** (dev saja; produksi memakai S3): `LocalDriver`/modul media menjalankan **migrasi sekali jalan idempoten saat start** di `NewLocalDriver`: bila `<StorageDir>/whatsapp_session.db*` ada -> pindah ke `SessionDir` (juga `-wal`,`-shm`); bila ada entri lain di `StorageDir` selain `media`, `_private`, `.gitkeep` -> `os.Rename` ke `media/` (pindahan satu direktori; log jumlah). Dilindungi: jangan jalan bila `SessionDir`==`StorageDir`. Isi `apps/api/storage/` saat ini: `product/2026/09/...` + `whatsapp_session.db` (0 byte, belum pernah dipasangkan).
8. **S3 tetap**: `S3Driver` tidak berubah di bagian baca (presigned `ttl` dari config). Lihat F1-9 untuk pagar `Remove`.
9. `.env.example`, `.gitignore` (`apps/api/storage/` sudah ada), `docs/API_CONTRACT.md:306` (`/uploads/*` -> `/media/*` bertanda tangan), `docker-compose.yml` volume (F6-2).

**Reuse terverifikasi**: `http.StripPrefix`+`FileServer` pola `server.go:83`; `config.Storage.SignedURLTTLSecs` (env `STORAGE_SIGNED_URL_TTL_SECONDS`, default 300) sudah ada dan tidak terpakai oleh driver lokal; `uuid` key sudah server-generated (`media/application/service.go:buildKey`).
**Test**: `signedurl` (valid, exp lewat, path berubah, sig salah, panjang tak sama, jam mundur); handler: `/media/` -> 404, `/media/vioni` (dir) -> 404, tanpa sig -> 403, sig valid -> 200 + header, `../` -> 404/403, `/uploads/x` -> 404, **`GET /media/..%2f_private%2fwhatsapp_session.db`** -> bukan 200; test integrasi: unggah foto produk lalu ambil URL dari API lalu `GET` -> 200 byte sama; test migrasi folder (tmp dir berisi sesi + `product/`).
**Volume/performa**: HMAC per URL ~µs; daftar produk 100 foto = 100 HMAC (diabaikan). Penyajian file tetap `http.ServeContent` (Range, ETag dari modtime).
**Dependensi**: F6-2 (volume), F1-2 (config). **Ukuran**: L.

## F1-2 Konfigurasi aman: `APP_ENV` wajib, default tidak melonggarkan (S)

**Bukti**: `config.go:96` `get("APP_ENV","development")`; hanya `development` yang melewati validasi `JWT_SECRET`.
**Rancangan** (`config.Load`):
- `APP_ENV` **wajib** dan salah satu dari `development|test|staging|production`; kosong/salah ketik -> gagal start (`APP_ENV wajib: development|test|staging|production`). Untuk `go test`/integration yang memakai `config.Config` literal, tidak terpengaruh. `cmd/server`/`cmd/seed` terpengaruh -> `.env.example` & `.env.production.example` mengisi nilai.
- Aturan `JWT_SECRET`: di luar `development` & `test`: panjang >= 32, bukan `change-me`; di `production` tambah: entropi minimal (bukan satu karakter berulang), `MEDIA_SIGNING_KEY` terisi & beda dari JWT_SECRET (peringatan -> galat di production).
- `production`: tolak `CORS_ALLOWED_ORIGINS` kosong bila `VITE_API_BASE_URL` diset; tolak `*`; tolak `STORAGE_DRIVER=local` tanpa `STORAGE_DIR` absolut; tolak `DB_DSN` memakai `root:` tanpa password (`root:@`); tolak `STORAGE_ACCESS_KEY_ID` yang sama dengan nilai legacy tidak mungkin dicek -> diganti pagar prefix (F1-9).
- Prefix S3 default `joinKey(root, appEnv, "upload")` tetap (production -> `vioni/production/upload`!). Lihat F1-9: di production **wajib** `STORAGE_UPLOAD_ROOT_PREFIX` eksplisit (tak boleh diturunkan otomatis, agar tidak diam-diam menimpa prefix legacy).
- Konstanta aplikasi tetap (ADR-0006): `APP_TIMEZONE`/`APP_LOCALE` tidak dibaca (sudah demikian).
**Test**: tabel `config_test.go` (sudah ada paket `internal/config` dengan test): kombinasi env x galat. **Ukuran**: S. **Dependensi**: F1-1, F1-3, F1-9, F6-8 (`.env.production.example`).

## F1-3 Trusted proxy untuk IP klien + rate limit login (M)

**Bukti**: `auth/presentation/handler.go:24-35 clientIP`; `docker-compose.yml` mengekspos `8080:8080`; `nginx.conf` hanya `X-Real-IP`/`Host`. Limiter `auth/service.go` memakai kunci `ip|username` — pemalsu header `X-Real-IP` memperoleh hitungan baru tiap permintaan; banjir kunci acak > 10.000 mengosongkan seluruh peta.
**Rancangan**
- Config `TRUSTED_PROXIES` (daftar CIDR/IP dipisah koma; default kosong = tidak ada proxy dipercaya) dan `CLIENT_IP_HEADER` (`X-Forwarded-For` default bila TRUSTED_PROXIES diisi, atau `X-Real-IP`).
- Paket `internal/shared/clientip`: `Resolve(r, trusted []*net.IPNet) string`: ambil `RemoteAddr` via `net.SplitHostPort` (versi sekarang memotong manual dan salah untuk IPv6 `[::1]:1234`); **hanya bila** peer langsung ada di `trusted`, baca header: untuk `X-Forwarded-For` telusuri dari kanan, lewati IP trusted, ambil yang pertama bukan-trusted (anti spoof kiri); validasi `net.ParseIP`; jika tidak valid -> pakai peer.
- `server.New` memasang middleware yang menaruh IP hasil resolve ke `reqctx` (lihat F6-5) sehingga `auth` dan log memakai satu sumber; `handler.go` `clientIP` dihapus -> `reqctx.FromContext(r.Context()).IP` (auth boleh mengimpor `shared`).
- Limiter: pindahkan `rateLimiter` ke `internal/shared/ratelimit` (dipakai juga F1-8), ganti strategi "reset total di 10k" dengan **penghapusan kadaluarsa (sweep)** + batas keras per kunci (bila penuh, tolak kunci baru berdasarkan IP yang sudah tercatat, bukan hapus semua). Tambah kunci kedua **per username saja** (mis. 30/menit) agar serangan terdistribusi satu akun tetap dibatasi; keduanya mengembalikan 429 `apperror.RateLimited()`.
- Deploy: compose `ports: "127.0.0.1:8080:8080"` (F6-2); nginx meneruskan `X-Forwarded-For $proxy_add_x_forwarded_for`, `X-Forwarded-Proto`, `X-Real-IP`. `TRUSTED_PROXIES` default contoh `.env.production.example`: `127.0.0.1/32,172.16.0.0/12`.
**Test**: `clientip` (tanpa proxy: header diabaikan; proxy dipercaya + XFF berantai; XFF palsu di kiri; IPv6; header sampah), limiter (sweep, banjir kunci tidak menghapus kunci aktif), integrasi login 11x dengan `X-Real-IP` berbeda -> tetap 429.
**Dependensi**: F6-5 (reqctx), F6-2. **Ukuran**: M.

## F1-4 Pencabutan sesi: nonaktif, ganti sandi, reaktivasi, Refresh (M)

**Bukti**: lihat §0 (F1d). Tabel sesi `user_sessions` milik modul auth; modul `user` tidak boleh menyentuhnya (aturan database.md) -> perlu kontrak.
**Rancangan**
1. `auth/contracts`: antarmuka baru
   ```go
   // SessionRevoker dicabut dari modul auth; diteruskan ke user lewat composition root.
   type SessionRevoker interface {
       RevokeUserSessions(ctx context.Context, userID, exceptSessionID int64) (int64, error)
   }
   ```
   `auth.Service` mengimplementasikan: `repo.RevokeUserSessionsExcept(ctx, userID, exceptID)` (`UPDATE user_sessions SET revoked_at=UTC_TIMESTAMP(6) WHERE user_id=? AND revoked_at IS NULL AND id<>?`; `exceptID=0` = semua; pakai kolom ada `ix_user_sessions_user`), lalu `cache.dropUser(userID)`. `auth.Module.Revoker()`; `main.go`: `userMod.SetSessionRevoker(authMod.Revoker())` (pola sama `SetAuthzInvalidator`).
2. `user/application/service.go`:
   - `SetStatus`: setelah update status berhasil, **cabut semua sesi** user itu untuk kedua arah (inactive dan active; reaktivasi tidak menghidupkan sesi lama). Catat jumlah di `Note` audit.
   - `ChangePassword`: cabut semua sesi user kecuali sesi aktor bila aktor == user (ambil `SessionFromContext(ctx).ID`; paritas legacy `retainedCurrentSession`).
   - `UpdateUser`: bila himpunan role/cabang berubah -> `invalidateUser` (sudah ada), tanpa cabut.
   - revoker `nil` = no-op (seed/test) sebagaimana `authz`.
3. `auth.Service.Refresh`: setelah `rec` valid, baca `s.users.GetByID` dan `GetRoleIDs`; user nil/`inactive`/tanpa role -> `RevokeSession(rec.ID)` + `Unauthorized`. (Satu query tambahan per refresh; jarang -> aman.)
4. Aturan "aktor tidak boleh menonaktifkan diri" (`SetStatus` KI-19) dipertahankan.
**Reuse**: `AuthzInvalidator`/`sessionCache.dropUser`, `Repository.RevokeUserSessions` (sudah ada, dipakai untuk replay), audit entry `users.status`/`users.password`.
**Test**: unit auth (Refresh user nonaktif -> 401 + sesi revoked; reaktivasi -> `Verify` sesi lama gagal), integrasi `user_roles_test.go` pola: login -> nonaktif -> aktif -> token lama 401; ganti sandi oleh admin -> sesi target 401, sesi admin hidup; ganti sandi diri sendiri -> sesi lain mati, sesi sendiri hidup.
**Volume**: UPDATE per user_id berindeks; ratusan baris max. **Dependensi**: F1-5. **Ukuran**: M.

## F1-5 Kebijakan sandi: batas 72 byte, cost, rehash, anti timing (S)

**Rancangan** (`user/application/service.go`, domain `permissions.go`/baru `password.go`):
- `domain.MaxPasswordBytes = 72`; `validatePassword(pw)`: `len([]byte(pw)) < 8` -> 400 "minimal 8 karakter"; `> 72` -> 400 `{"field":"password","message":"maksimal 72 byte (sekitar 72 karakter)"}`. Dipakai di `CreateUser` & `ChangePassword`; `hashPassword` tetap menangkap `bcrypt.ErrPasswordTooLong` dan memetakannya ke `apperror.Validation` (pagar kedua; tidak lagi 500). Login/`VerifyCredentials` dengan sandi > 72 byte: ditolak sebagai kredensial salah (tanpa memotong).
  Keputusan teknis: tidak memakai pra-hash SHA-256 agar hash legacy bcrypt tetap kompatibel (hash legacy `$2a$/$2b$` cost 12 dari bcryptjs bisa diverifikasi `golang.org/x/crypto/bcrypt`).
- `const bcryptCost = 12` (paritas legacy); biaya per hash ~250 ms -> hanya di jalur ganti/buat sandi dan login.
- **Rehash saat login**: di `VerifyCredentials` setelah cocok, `bcrypt.Cost(hash) < bcryptCost` -> `repo.SetPasswordHash` best-effort (`updated_by` tidak diubah; tulis fungsi repo `UpgradePasswordHash(userID, hash)` yang hanya mengubah `password_hash` jika hash lama masih sama: `WHERE id=? AND password_hash=?`).
  Hash hasil ETL legacy sudah cost 12 -> tidak memicu.
- **Anti timing**: bila user tak ada/nonaktif, jalankan `bcrypt.CompareHashAndPassword(dummyHash, password)` dengan `dummyHash` konstan cost 12 dibuat sekali saat init, lalu kembalikan `Unauthorized` yang sama.
- Sandi minimum tetap 8 (paritas legacy `new_password.length < 8`).
**Test**: tabel (71/72/73 byte, multibyte 25 huruf "é" = 50 byte, 40 huruf CJK = 120 byte -> 400); `Cost()==12` setelah buat; hash cost 10 lama -> login berhasil dan hash ter-upgrade; timing kasar (selisih < 3x antara user ada/tidak ada; tidak flakey: cukup pastikan jalur dummy dipanggil lewat hook).
**Perf**: login naik ~4x biaya bcrypt (10->12): ~60 ms -> ~250 ms/login; tidak masalah (rate limit melindungi). **Dependensi**: F1-4, F1-8. **Ukuran**: S.

## F1-6 Seeding katalog permission + role default di server start; tanpa password hardcode (L)

**Bukti**: lihat §0 (F1f). Katalog tersebar: `userdomain.Catalog()` (10 kode) + `PermissionCatalog()` di 15 paket `contracts` (total 74 kode unik; pencocokan: semua kode yang dipakai `perm("...")` di rute ada di katalog — **0 kode rute tak terkatalog**; `stock.approve` dipakai di `checkApprover`, bukan rute).
`go test` integrasi membuat role/izin sendiri; produksi DB baru -> `permissions` kosong -> `RequirePermission` menolak semua (fail-closed, benar tetapi tak ada jalan masuk).
**Rancangan**
1. Paket komposisi baru `internal/bootstrap` (di luar `internal/modules`; setara `main.go`, boleh mengimpor `*/contracts` semua modul + `user.Module` service):
   - `catalog.go`: `AllPermissionSeeds() []usercontracts.PermissionSeed` = gabungan `userdomain.Catalog()` (diekspor lewat `user/contracts` agar tak mengimpor internal: pindahkan `Catalog()` ke `user/contracts.PermissionCatalog()`), + 15 katalog modul. Satu daftar; modul baru dari fase lain **wajib** menambah satu baris di sini (tambah test `TestCatalogCoversRoutes` yang mem-parse kode sumber rute: lihat Test).
   - `roles.go`: definisi role sistem + default (kosakata **baru**), idempotent:
     * `superadmin` (is_system, reserved) & `owner` (is_system): seluruh kode, **disinkron tiap start** (kode baru langsung melekat; izin mereka tidak dapat diedit lewat API, lihat F1-7).
     * `admin`, `staff` (is_system) dan `kasir` (**bukan** sistem, paritas legacy `is_system_role=0`): izin awal diberikan **sekali per kode** dan dicatat di tabel baru `authz_seed_applied(role_code, permission_code, applied_at, PK(role_code, permission_code))` (milik modul `user`, migrasi `0000xx_authz_seed`). Kode yang kemudian dicabut admin tidak diberikan ulang; kode katalog baru yang masuk daftar default diberikan sekali. Menghindari "seed menimpa kustomisasi" tanpa mengubah `roles`.
   - `EnsureAuthz(ctx, users *userapp.Service) error`: `EnsurePermissions` (upsert `ON DUPLICATE KEY UPDATE name`), buat role sistem bila belum ada dengan `is_system=1` (`UPDATE roles SET is_system=1 WHERE code IN (...)` untuk DB lama), lalu sinkron di atas. Dijalankan **di `main.go` sebelum `ListenAndServe`** dengan `context` timeout 30 dtk dan di `cmd/seed`. Biaya: ~75 upsert + <200 insert pada start (ms).
2. **Peta kode legacy -> baru** (untuk default dan ETL stage K; turunan dari `DEFAULT_ROLE_PERMISSIONS` + `seed-runner.ts`; di legacy semua rute pihak-bisnis memakai `order.*`: `business-party.controller.ts` -> `order.view/create/update/archive`):
   `dashboard.view`->`dashboard.view`; `product.*`->`products.*` (+`product_categories.manage`,`product_import.manage` bila `product.update`); `member_type.manage`->`member_types.manage` (+`pricing.quote` bagi pemegang `order.create`); `order.view|create|update|archive`->`sales.*`+`purchasing.*`+`customers.*`+`suppliers.*`+`goodsreceipt.*`+`delivery.*` (view/create/update/archive sesuai); `order.export`->`reporting.view`;
   `sales_return.view|create|cancel`->`salesreturn.view|create|archive`; `purchase_return.*`->`purchasereturn.*` (view/create/cancel->archive; `refund` -> `payment.create`); `stock.view`->`stock.view`, `stock.update|transfer`->`stock.manage`, `stock.adjust`->`stock.adjust`, `stock.adjust.approve`->`stock.approve`; `reporting.view`->`reporting.view`;
   `user.*`->`users.*` (`view/create/update/archive`) dan `role.manage`->`roles.view`+`roles.manage`; `company_config.*`->`company.*`; `branch.*`->`branches.*`; `whatsapp.view|manage|simulate`->`assistant.view|manage`; `audit_log.view`->`audit.view`; `payment.*` sama; `finance.view`->`finance.view`; `finance.manage|expense.*|tax_adjustment.manage|close.force|journal.reverse`->`finance.manage`; `finance.posting.post`->`finance.post`; `finance.*.view|report.view|tax_report.view`->`finance.view`; `knowledge.*` -> **dibuang** (by-design RAG).
   Peta ini disimpan sebagai `bootstrap/legacymap.go` (tabel data + test) dan dipakai ETL (jangan duplikasi; ETL stage K mengimpor paket yang sama).
   Default hasil peta: `admin` = peta dari daftar admin legacy (view user/company/branch, **tanpa** `users.create/update/archive`, `branches.manage`, `company.manage`, `finance.manage/post`; **dengan** `roles.manage` seperti legacy — lihat Pertanyaan P2); `staff`/`kasir` sesuai daftar legacy (kasir legacy tidak punya `payment.*`: dipertahankan; Pertanyaan P2b tidak perlu — cukup catatan).
3. **Admin awal tanpa password hardcode**: `cmd/seed` dipecah subperintah (`flag`/`os.Args[1]`):
   - `seed init` : `EnsureAuthz` + `EnsureChart` + cabang kepala (butuh `BOOTSTRAP_BRANCH_CODE`/`NAME`, default `UTM`/`Utama` hanya bila bukan production) + profil perusahaan minimal. Idempotent.
   - `seed admin` : membuat user pertama **hanya bila tabel `users` kosong** (atau `--force-new` dengan `--username`). Password dari `BOOTSTRAP_ADMIN_PASSWORD` (>=12 karakter, lolos `validatePassword`) atau, bila tidak ada, **dibangkitkan acak 20 karakter dan dicetak sekali ke stdout** (tidak ke log/audit). Username dari `BOOTSTRAP_ADMIN_USERNAME` (default `admin`). Role `owner`. Audit actor 0 `bootstrap.admin`.
   - `seed demo` : data demo/dev (cabang `UTM`, profil "Perusahaan Demo", `admin` dengan password dari env **atau acak**), **menolak** berjalan bila `APP_ENV` in (`production`,`staging`) kecuali `--i-know-this-is-not-production`.
   - Seluruh subperintah menolak `APP_ENV` kosong (F1-2). `Admin123!` dihapus dari kode dan dokumentasi (`grep -rn "Admin123" .` harus kosong, termasuk `docs/`, `knowledge/`, README).
4. `integration_test.go` tetap membuat data sendiri; tambahkan satu test yang memanggil `bootstrap.EnsureAuthz` dua kali pada DB uji kosong dan memeriksa idempotensi dan jumlah izin.
**Reuse terverifikasi**: `Service.EnsurePermissions`/`EnsurePermissionCatalog`/`AllPermissionCodes`/`GetRoleByCode`/`CreateRole`/`SetRolePermissions` (`user/application/service.go`), `Repository.EnsurePermission` (upsert), `financeMod.Service().EnsureChart` (dipakai seed lama), pola wiring `cmd/seed/main.go:48-61`.
**Test**: `TestEnsureAuthzIdempotent`; `TestCatalogCoversRoutes` (parse `perm("...")` pada `internal/modules/**/presentation/*.go` dengan regex dan pastikan tiap kode ada di `AllPermissionSeeds()` — menutup celah "kode rute baru lupa dikatalog"; pada pembacaan hari ini lulus); `TestLegacyMap` (tiap kode legacy memetakan ke kode ada atau ke daftar "dibuang"); `seed admin` menolak bila ada user; password acak lolos validator; `seed demo` menolak di production.
**Volume**: kecil. **Dependensi**: F1-2, F1-5, F1-7 (`is_system`), migrasi `authz_seed_applied`. **Ukuran**: L.

## F1-7 Proteksi role sistem, admin terakhir, kebijakan superadmin (M)

**Bukti**: lihat §0 (F1g). Legacy: `reserved-role-policy.ts` (`superadmin`, user id 1, role tak dapat ditetapkan lewat API, tersembunyi bagi non-superadmin; `user.service.ts:assignRoles` memasang ulang role pada user id 1).
**Rancangan** (`user` module; sesi aktor dibaca dari `authcontracts.SessionFromContext(ctx)` yang sudah diimpor modul ini — tanpa mengubah tanda tangan servis):
1. **Role sistem**: `CreateRole` internal menerima `isSystem`; jalur API tetap `is_system=0`. `UpdateRole`, `DeleteRole`, `SetRolePermissions` pada `IsSystem` -> 403 `apperror` dengan pesan ("Role sistem tidak dapat diubah") kecuali `bootstrap` (jalur internal `ensureSystemRole`, bukan HTTP). `roles.update` audit. Untuk `admin`/`staff` sistem: izin **dapat** diubah lewat API (paritas legacy: role sistem boleh diubah izinnya kecuali superadmin) — kecuali `owner`/`superadmin` (selalu semua). **Keputusan teknis**: sistem = tak dapat dihapus/rename-kode; hanya `owner` & `superadmin` terkunci izin.
2. **Superadmin**: konstanta `ReservedSuperadminRole="superadmin"`, `DefaultSuperadminUserID=1` di `user/domain`.
   - Tak dapat ditetapkan lewat `CreateUser`/`UpdateUser` (validasi 400 "Role Super Admin hanya untuk akun developer default"); `UpdateUser` untuk user id 1 memasang ulang role itu (paritas `assignRoles`).
   - Sesi non-superadmin: `ListRoles` menyembunyikan role itu; `ListUsers/GetUser/UpdateUser/SetStatus/ChangePassword` atas user pemegangnya -> 404. Query tambahan `NOT EXISTS (SELECT 1 FROM user_roles ...)` pada `ListUsers/CountUsers` (indeks `user_roles` PK `(user_id, role_id)`; subquery per baris pada halaman 20-100 baris — murah).
   - Dibuat hanya lewat ETL (user id 1 legacy) atau `seed admin --superadmin` (Pertanyaan P1).
3. **Admin terakhir (anti lock-out)**: definisi "pengelola" = user `active` yang lewat salah satu role memegang **`roles.manage` DAN `users.update`**. Repo `CountManagers(ctx, q execer)` (SQL: `SELECT COUNT(DISTINCT ur.user_id) FROM user_roles ur JOIN users u ON u.id=ur.user_id AND u.status='active' JOIN role_permissions rp ON rp.role_id=ur.role_id JOIN permissions p ON p.id=rp.permission_id AND p.code='roles.manage' WHERE EXISTS(... 'users.update' ...)`).
   Operasi yang dapat menurunkan hitungan dibungkus transaksi: ubah -> `CountManagers` dalam tx yang sama -> 0 -> rollback dan `apperror.Conflict("Operasi ditolak: harus tersisa minimal satu pengelola aktif")`. Berlaku untuk: `SetStatus(inactive)`, `UpdateUser` (ganti role), `SetRolePermissions`, `DeleteRole` (hanya bila role dipakai pemilik terakhir — sudah ditolak `RoleInUse`, jadi tak perlu).
   Repo saat ini membuka tx sendiri di `UpdateUser`/`SetRolePermissions`; tambahkan `postCheck func(tx) error` opsional. `SetStatus` repo saat ini tanpa tx -> bungkus.
4. **Eskalasi hak** (lihat Pertanyaan P2): tidak dikunci default; opsi `users.update` tidak boleh menetapkan role yang izinnya bukan himpunan bagian dari izin aktor.
**Test** (integrasi pada DB uji, pola `user_roles_test.go`): hapus izin `roles.manage` dari satu-satunya role pengelola -> 409; nonaktifkan pengelola terakhir -> 409 (dua pengelola -> boleh); `UpdateUser` mengganti role pengelola terakhir -> 409; role `owner` `PUT permissions` -> 403; `DELETE owner` -> 403; non-superadmin tak melihat user id 1 di list dan mendapat 404 di GET; assign `superadmin` lewat API -> 400; user id 1 setelah `UpdateUser` tetap `superadmin`.
**Perf**: `CountManagers` join 4 tabel kecil, dijalankan hanya pada tulis admin. **Dependensi**: F1-6. **Ukuran**: M.

## F1-8 Approver koreksi stok: rate limit + bcrypt di luar transaksi (S)

**Bukti**: `stock/application/transactions.go:300` `Begin`, `:323` `checkApprover` -> `VerifyCredentials` (bcrypt ~60-250 ms) menahan koneksi tx (pool 25) dan kunci baris saldo; tanpa pembatas laju -> penebak sandi pengguna lain via form koreksi (butuh `stock.adjust` saja).
**Rancangan**
- `internal/shared/ratelimit` (dari F1-3) diinjeksi ke `stock.Service` (`NewModule(..., limiter)`; default `ratelimit.New(5, time.Minute)` per kunci `actorID|approverUsername`, plus per `approverUsername` 10/menit). Gagal -> `apperror.RateLimited()`.
- Urutan baru: (1) validasi input; (2) bila `in.Approver != ""`: `approver, err := s.verifyApprover(ctx, ...)` **sebelum** `repo.Begin` (VerifyCredentials + GetRoleIDs + cek `stock.approve`); hasil disimpan `approverOK bool`; (3) di dalam tx, bila `abs > threshold` dan !approverOK -> 403 seperti sekarang. Biaya: bcrypt dijalankan walau ambang tak terlewati (hanya jika approver dikirim) — wajar.
- Audit: catat `stock_adjustments.create` dengan `Note` berisi username approver bila dipakai; kegagalan approver dicatat `stock.approver_failed` (tanpa sandi).
**Test**: unit dengan fake `UserClient` yang menghitung panggilan: tidak ada panggilan sebelum/selama tx (urutan); 6 kali salah dalam semenit -> 429; approver tanpa `stock.approve` -> 403; threshold tak terlewati + approver terkirim -> tetap sukses. Integrasi `stock_flows_test.go` pola ada.
**Dependensi**: F1-3 (paket ratelimit). **Ukuran**: S.

## F1-9 Pemisahan S3 (bucket/prefix/kunci) dan larangan Archive menghapus objek bersama (M)

**Bukti**: lihat §0 (F1i). Setelah ETL, baris `media_files` berisi key legacy `vioni/production/upload/...`; `Archive` -> `driver.Remove(key)` -> `DeleteObject` menghapus objek legacy yang masih dipakai sistem lama (jalur rollback!).
**Rancangan**
1. Kolom baru `media_files.origin ENUM('app','legacy') NOT NULL DEFAULT 'app'` (migrasi `0000xx_media_origin`); ETL mengisi `legacy`. `Archive` -> bila `origin='legacy'` hapus baris saja (**jangan** `driver.Remove`), catat `Note` "objek bersama dipertahankan". Repo `Delete` mengembalikan `(key, origin)`.
2. Pagar prefix pada `S3Driver.Remove`: hanya menghapus key yang diawali salah satu prefix upload milik deployment ini (`ProductPrefix`/`TransferPrefix`/`DeliveryPrefix`) **dan** bukan di `STORAGE_PROTECTED_PREFIXES` (daftar, mis. `vioni/production/upload/` saat deployment dev/staging). Di production revamp: set `STORAGE_UPLOAD_ROOT_PREFIX` ke prefix **terpisah** untuk unggahan baru (mis. `vioni/production-v2/upload`) sehingga file baru tak bercampur dengan file legacy; pembacaan key legacy tetap berfungsi (presign berdasarkan key, tak peduli prefix).
3. Config production: `STORAGE_UPLOAD_ROOT_PREFIX` wajib eksplisit (F1-2); jika prefix upload production sama dengan `STORAGE_PROTECTED_PREFIXES` -> gagal start.
4. Dev/staging: `STORAGE_DRIVER=local` default; bila S3 dipakai dev, **minta pemilik menerbitkan access key terpisah** (atau bucket dev) — `.env` lokal saat ini memakai bucket+kunci yang identik dengan production legacy (Pertanyaan P4). Sampai itu dilakukan, aturan: dev **tidak** menjalankan unggah/arsip ke S3 produksi; ubah `.env` lokal ke `STORAGE_DRIVER=local` (instruksi untuk pemilik; saya tidak mengubah berkas). Kunci tidak ditampilkan di dokumen ini.
5. Dokumentasikan IAM minimal: key revamp = `GetObject`+`PutObject` pada prefix revamp, `GetObject` saja pada prefix legacy, tanpa `DeleteObject` pada prefix legacy (bila IDCloudHost mendukung kebijakan per-prefix — tanyakan penyedia).
**Test**: unit `S3Driver.Remove` dengan fake `s3` API (interface kecil) untuk prefix terlindungi; `media.Archive` origin legacy -> `Remove` tak dipanggil (fake driver); migrasi up/down.
**Dependensi**: F1-2, ETL (stage K) mengisi `origin`. **Ukuran**: M.

## F1-10 Refresh token: toleransi race antar-tab (M)

**Bukti**: lihat §0 (F1j); `auth/service.go:247-290`, `infrastructure/repository.go Rotate`.
**Rancangan**
1. Migrasi: `user_sessions.rotated_at DATETIME(6) NULL`.
2. `Rotate` menjadi CAS: `UPDATE user_sessions SET prev_refresh_hash=refresh_hash, refresh_hash=?, rotated_at=UTC_TIMESTAMP(6), expires_at=?, last_activity_at=UTC_TIMESTAMP(6) WHERE id=? AND refresh_hash=? AND revoked_at IS NULL`; `RowsAffected==0` -> pemanggil membaca ulang.
3. `Refresh`: token cocok `prev_refresh_hash` **dan** `rotated_at` dalam `RefreshReuseGrace` (30 dtk) -> perlakukan sebagai race sah: balas `401` dengan `code: "refresh_raced"` **tanpa** mencabut sesi; di luar jendela -> replay sungguhan -> cabut semua sesi user (perilaku sekarang). Kalah CAS = kasus yang sama.
   (Pilihan "terbitkan pasangan baru lagi" ditolak: hash token hanya disimpan sebagai hash, token baru tak bisa diberikan ulang ke tab yang kalah.)
4. Frontend `apps/web/src/shared/services/http-client.ts`: bungkus rotasi dengan `navigator.locks.request("mini-erp-refresh", ...)` (fallback tanpa Web Locks); di dalam kunci baca ulang `tokenStore.refresh`; bila sudah berbeda dari yang dipakai request gagal -> jangan panggil `/refresh`, pakai token baru; pada `refresh_raced` coba sekali lagi dengan token terbaru dari `localStorage`.
5. Log `warn` replay menyertakan `ip` dan `ua` (dari reqctx) agar insiden dapat ditelusuri; audit `auth.refresh_replay`.
**Test**: unit (grace: prev dalam 5 dtk -> 401 tanpa cabut; prev 60 dtk -> cabut semua; dua `Refresh` konkuren dengan token sama -> tepat satu sukses, sesi tetap hidup — pakai `sync.WaitGroup` + DB uji); vitest untuk interleaving dua pemanggil `refreshTokens` (mock Web Locks). Paritas: legacy `auth.service.ts` (tidak dicek mendalam; perilaku legacy tak diwajibkan).
**Dependensi**: F1-3 (reqctx untuk IP). **Ukuran**: M.

## F1-11 Audit untuk assistant/WhatsApp (S)

**Bukti**: `assistant.NewModule` tak menerima `AuditClient` (`assistant.module.go:28-38`); `application/service.go:24-26` komentar pengecualian. Legacy tak mengaudit (paritas) — tetapi aksi berikut sensitif dan murah diaudit.
**Rancangan**: `NewModule(..., audit auditcontracts.AuditClient)`; `Service` menerima `audit`. Entri best-effort setelah sukses (pola `_ = s.audit.Log(...)` yang dipakai modul lain): `assistant.channel_connect`, `assistant.channel_disconnect`, `assistant.channel_reset`, `assistant.auth_create|update|revoke` (Note = 4 digit terakhir nomor, bukan nomor penuh), `assistant.config_update`. **Tidak** mengaudit `chat`/`simulate`/inbound (run history sudah menjadi jejaknya). Perbarui komentar `service.go:24`. Wiring `main.go:97` dan `cmd/seed` (tak memakai assistant).
**Test**: integrasi atau unit dengan fake audit: tiap handler menulis satu entri; `audit.view` melihatnya. **Ukuran**: S. **Dependensi**: tidak ada.

---------------------------------------------------------------------------------------------------

# F6 — Deploy & operasi

## F6-1 Dockerfile yang benar dan reproducible (M)

**Bukti**: `infra/docker/Dockerfile` — `golang:1.22-alpine` (go.mod `go 1.26.0`, `toolchain go1.26.7`); `COPY package.json` + `npm install` tanpa `package-lock.json`; tanpa `HEALTHCHECK`, user non-root, `ca-certificates`; hanya biner `api`; `.dockerignore` mengecualikan `*.md` (aman) dan `.git`.
**Rancangan** (multi-stage, satu image app):
```
FROM node:20-alpine AS web
WORKDIR /repo
COPY package.json package-lock.json ./
COPY packages ./packages
COPY apps/web/package.json ./apps/web/
RUN npm ci --workspace apps/web --include-workspace-root
COPY apps/web ./apps/web
RUN npm run build -w apps/web

FROM golang:1.26.7-alpine AS api          # samakan dengan `toolchain` di go.mod; verifikasi tag ada di Docker Hub saat implementasi
ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOTOOLCHAIN=local
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api ./
ARG VERSION=dev
RUN go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/server \
 && go build -trimpath -ldflags "-s -w" -o /out/seed ./cmd/seed
RUN go install -tags mysql github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1   # CLI migrasi, versi sama dengan lokal

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && addgroup -S app && adduser -S -G app -u 10001 app \
 && mkdir -p /data/storage && chown -R app:app /data
WORKDIR /app
COPY --from=api /out/api /out/seed ./
COPY --from=api /go/bin/migrate /usr/local/bin/migrate
COPY apps/api/migrations ./migrations
COPY --from=web /repo/apps/web/dist ./public
ENV APP_PORT=8080 PUBLIC_DIR=/app/public STORAGE_DIR=/data/storage
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD wget -qO- http://127.0.0.1:8080/api/v1/health/ready >/dev/null || exit 1
ENTRYPOINT ["./api"]
```
Catatan: `CGO_ENABLED=0` aman (sqlite `modernc`, whatsmeow murni Go); `-buildvcs=false` wajib karena `.git` dikecualikan; `ca-certificates` menutup bug TLS ke S3; `GOTOOLCHAIN=local` menjaga build offline-deterministik (gagal cepat bila tag image < toolchain). Cache npm via `--mount=type=cache` (BuildKit) opsional. Sertakan `.dockerignore`: tambah `apps/api/storage`, `apps/api/tmp`, `.github`, `analysis`, `docs`, `knowledge`, `*.md`, `.env*`.
**Test**: `docker build` lokal (Docker Desktop terpasang), `docker run --rm image ./seed` menolak tanpa env, `docker run` dengan `.env.production.example` terisi -> `healthy`; CI job `docker`; `docker history`/ukuran < 80 MB.
**Dependensi**: F6-4 (endpoint `/health/ready`), F6-3. **Ukuran**: M.

## F6-2 `docker-compose.yml` dan nginx (M)

**Rancangan compose** (tetap satu container app, DB di host):
```yaml
services:
  app:
    build: { context: ., dockerfile: infra/docker/Dockerfile, args: { VERSION: "${APP_VERSION:-dev}" } }
    image: mini-erp:${APP_VERSION:-dev}
    restart: unless-stopped
    env_file: [.env]
    ports: ["127.0.0.1:8080:8080"]       # hanya loopback; proxy TLS host yang mengekspos 443
    volumes:
      - ./data/storage:/data/storage     # bind mount agar mudah di-backup (bukan named volume tak terlihat)
    extra_hosts: ["host.docker.internal:host-gateway"]
    read_only: true
    tmpfs: [/tmp]
    security_opt: ["no-new-privileges:true"]
    stop_grace_period: 30s                # > shutdown timeout aplikasi (F6-6)
    deploy: { resources: { limits: { memory: 768M } } }
    logging: { driver: json-file, options: { max-size: "20m", max-file: "5" } }
    healthcheck: { test: ["CMD","wget","-qO-","http://127.0.0.1:8080/api/v1/health/ready"], interval: 30s, timeout: 5s, retries: 3, start_period: 30s }
  migrate:                                # langkah deploy terpisah, bukan otomatis saat start
    image: mini-erp:${APP_VERSION:-dev}
    profiles: ["ops"]
    entrypoint: ["migrate","-path","/app/migrations","-database","mysql://${DB_DSN}"]
    command: ["up"]
    env_file: [.env]
    extra_hosts: ["host.docker.internal:host-gateway"]
```
- Volume: **berkas unggahan dan sesi WA kini di `/data/storage`** (`media/` + `_private/`, F1-1) -> sebelumnya hilang saat container diganti. Pemilik direktori `10001`; `chown` pada host saat setup (dokumentasikan).
- `read_only: true` mensyaratkan `/data/storage` dan `/tmp` yang dapat ditulis — diuji (whatsmeow/sqlite menulis di `SESSION_DIR`).
- nginx **tidak ditambahkan ke compose** (standar: satu container app; Pertanyaan P3). `infra/nginx/nginx.conf` ditulis ulang sebagai templat untuk proxy **di host**:
  `listen 80` -> `return 301 https://$host$request_uri`; `listen 443 ssl http2` dengan `ssl_certificate` (Let's Encrypt/certbot path contoh), `ssl_protocols TLSv1.2 TLSv1.3`, HSTS `max-age=31536000`, `client_max_body_size 6m` (upload 5 MB + overhead multipart; ekspor adalah GET), `proxy_pass http://127.0.0.1:8080`, `proxy_set_header Host $host; X-Real-IP $remote_addr; X-Forwarded-For $proxy_add_x_forwarded_for; X-Forwarded-Proto $scheme`, `proxy_read_timeout 120s` (selaras timeout ekspor, F6-4), `proxy_request_buffering on`, `gzip off` (aplikasi sudah mengompres `compress.go`), `location = /api/v1/metrics { deny all; }` (F6-5), header keamanan (`X-Content-Type-Options nosniff`, `Referrer-Policy`, `X-Frame-Options DENY`; CSP disusun setelah audit asset frontend).
- `docker compose up --build` di dokumentasi diganti urutan deploy yang jelas (F6-3).
**Test**: `docker compose config` valid; `docker compose up` lalu `curl 127.0.0.1:8080/api/v1/health/ready`; matikan MySQL -> `unhealthy`; ganti image -> `data/storage` utuh; port tidak terjangkau dari IP LAN. **Dependensi**: F6-1, F1-1, F1-3. **Ukuran**: M.

## F6-3 Migrasi: langkah deploy eksplisit, bukan otomatis (S)

Keputusan yang sudah ada di `docs/DEPLOYMENT.md §5` (jangan migrasi otomatis tiap start; root masalah sistem lama) **dipertahankan**; yang kurang adalah jalan praktisnya di container.
- Image membawa CLI `migrate` + `/app/migrations` (F6-1). Langkah: `docker compose --profile ops run --rm migrate up` **sebelum** `docker compose up -d app`. `DB_DSN` Go tanpa skema; entrypoint menambah `mysql://` persis seperti `scripts/migrate.mjs` (`:21`).
- Pagar versi di aplikasi: saat start, `server` memeriksa tabel `schema_migrations` (`version`, `dirty`) vs versi terbesar yang disematkan saat build (`//go:embed migrations/*.up.sql` hanya untuk menghitung maksimum; tidak menjalankan). `dirty=1` atau versi < diharapkan -> **gagal start** dengan pesan "jalankan migrasi" (mencegah aplikasi baru berjalan di schema lama). Versi > diharapkan (rollback image) -> peringatan, tetap jalan.
- Rollback migrasi: `... run --rm migrate down 1` hanya setelah backup (F6-7); tiap `*.down.sql` diverifikasi di CI (F0-4 job `migrations`).
- Catatan fase schema: di akhir pengembangan migrasi di-squash menjadi baseline (keputusan 6); runbook menyatakan database lama **tidak** dapat di-`migrate up` ke baseline — selalu rebuild + ETL.
**Test**: `go test` kecil untuk pembanding versi (fungsi murni), CI job `migrations`. **Dependensi**: F6-1. **Ukuran**: S.

## F6-4 Health, readiness, timeout HTTP, pool DB (M)

1. **Health** (`server.go:25`): `GET /api/v1/health` tetap liveness murni (`{"status":"ok"}` + `version`). Tambah `GET /api/v1/health/ready`: `db.PingContext` timeout 2 dtk, cek `SessionDir`/`MediaDir` dapat ditulis (create+remove file kecil, di-cache 30 dtk), versi migrasi (F6-3); gagal -> 503 envelope `{success:false,message:"Database tidak tersedia"}`. `server.New(cfg, log, ready func(ctx) error)`; `main.go` memberikan closure (db, path). Tidak membocorkan detail internal ke klien (detail ke log). Rute tetap tanpa auth, di-whitelist tanpa rate limit.
2. **Timeout** (`main.go:143-147`): `ReadHeaderTimeout 10s`, `ReadTimeout 30s`, `WriteTimeout 60s`, `IdleTimeout 120s`, `MaxHeaderBytes 1<<16`. Rute berat memperpanjang sendiri lewat `http.NewResponseController`: middleware `server.Extend(read, write time.Duration)` yang memanggil `SetReadDeadline/SetWriteDeadline`; dipasang pada upload (`/proof`, `/media`, `product-imports`, delivery proof) dan ekspor/PDF/Excel (`.../export*`, laporan). **Prasyarat**: `statusWriter` (`middleware.go:9-17`) dan `gzipResponseWriter` (`compress.go:48`) harus mengimplementasikan `Unwrap() http.ResponseWriter` (dan meneruskan `Flush`), kalau tidak `ResponseController` gagal diam-diam. Nilai awal: upload read 120s; ekspor write 120s. Ukur durasi ekspor terbesar sebelum menetapkan (lihat Volume).
3. **Pool DB** (`database/db.go:36-38`: 25 open / 10 idle / 5 menit): MySQL default `max_connections=151`; satu container -> 25 aman. Jadikan konfigurasi: `DB_MAX_OPEN_CONNS` (25), `DB_MAX_IDLE_CONNS` (10), `DB_CONN_MAX_LIFETIME` (5m), `DB_CONN_MAX_IDLETIME` (2m); validasi `idle <= open`. Pool 25 sudah cukup untuk beban single-company; rekomendasi diukur dengan `db.Stats()` (`WaitCount`, `WaitDuration`) dari metrics F6-5 sebelum menaikkan. Tambah `readTimeout=30s&writeTimeout=30s&timeout=5s` pada DSN (pencegah tx menggantung): `ensureParams` menambahkan bila tak ada.
**Test**: ready 200/503 (sqlmock tak ada; pakai DB uji + `db.Close()` untuk 503), `Unwrap` (test `ResponseController.SetWriteDeadline` melewati stack `chain`), test bahwa timeout kustom diterapkan pada rute upload (httptest + server nyata dengan WriteTimeout kecil), config pool.
**Volume**: ukur ekspor terbesar (data produksi nanti dari dump legacy, keputusan 8) — tentukan nilai akhir setelah ETL.
**Dependensi**: F6-5. **Ukuran**: M.

## F6-5 Observability dasar: request-id, konteks log, stack panic, metrics (M)

1. Paket `internal/shared/reqctx`: `Info{ID string; IP string; UserID, BranchID int64}` disimpan sebagai **pointer mutabel** di context. Middleware `requestID` (paling luar): terima `X-Request-ID` bila cocok `^[A-Za-z0-9._-]{8,64}$` selain itu `uuid.NewString()` (`google/uuid` sudah di go.mod); set header respon; taruh `Info` di context.
2. `auth/contracts.Authenticate` setelah `Verify` sukses: `if info := reqctx.From(ctx); info != nil { info.UserID, info.BranchID = s.UserID, s.BranchID }` (auth boleh mengimpor `shared`). `switch-branch` memperbarui nilai pada token berikutnya — cukup.
3. `requestLog` mencatat `request_id, ip, method, route (pola mux, `r.Pattern` Go 1.23+, bukan path mentah agar kardinalitas rendah), path, status, bytes, duration_ms, user_id, branch_id, ua` dan level: 5xx=Error, 4xx=Warn (kecuali 401/404 = Info), lainnya Info; `/api/v1/health*` di-skip atau level Debug. Redaksi: tidak mencatat query `sig=`/`exp=` (media) dan tidak pernah body.
4. `recoverer`: `log.Error("panic", "request_id", id, "panic", rec, "stack", string(debug.Stack()))`; respon 500 envelope menyertakan `requestId` di header (bukan stacktrace). Panic setelah header terkirim -> `http.ErrAbortHandler` untuk memutus.
5. **Metrics** tanpa dependensi baru: `internal/server/metrics.go`, counter atomik: `http_requests_total{route,method,class}`, histogram durasi (bucket 5,25,100,250,1000,5000 ms), `http_in_flight`, `go_goroutines`, `db_pool_open/in_use/idle/wait_count/wait_seconds`, `process_start_time`. Format teks Prometheus ditulis tangan (~60 baris). Endpoint `GET /api/v1/metrics` hanya aktif bila `METRICS_TOKEN` terisi (Bearer) dan nginx `deny all` di luar loopback; kosong -> 404. Kardinalitas label dibatasi pada pola rute.
6. Level log lewat `LOG_LEVEL` (ada) + `LOG_FORMAT=text|json` tidak perlu. `logger.New()` tambahkan atribut tetap `service=mini-erp`, `version`, `env`.
**Reuse**: `statusWriter`, `requestLog`, `chain`; `Authenticate`; `slog`. **Test**: request-id (diterima/ditolak/dibuat), log berisi `user_id` setelah login (tangkap handler slog ke buffer), panic -> stack ada & 500, `Unwrap`, metrics (counter naik, token salah -> 401/404), cardinality.
**Perf**: overhead per request < 5 µs (atomik + uuid). **Dependensi**: F1-3. **Ukuran**: M.

## F6-6 Graceful shutdown & konteks gateway WhatsApp (M)

**Bukti**: `main.go:105-109` Boot `context.Background()`+60 dtk; `:161-163` shutdown hanya `httpSrv.Shutdown` (10 dtk) lalu `db.Close` via defer; `Gateway` tak punya `Close`; `Service.Connect(r.Context())` -> `GetQRChannel(ctx)` terikat ctx request (hipotesis rusak, lihat §0).
**Rancangan**
1. `main.go`: `appCtx, stopApp := context.WithCancel(...)` diturunkan dari `signal.NotifyContext`; Boot memakai `appCtx` (bukan Background); urutan shutdown: (a) tandai `readiness` gagal (503) agar proxy berhenti mengirim; (b) `httpSrv.Shutdown` (timeout dari `SHUTDOWN_TIMEOUT`, default 25 dtk < `stop_grace_period 30s`); (c) `assistantMod.Close(ctx)` -> `gateway.Close()` (`client.Disconnect()`, tutup `container.Close()`/`db.Close()` sqlite, tunggu goroutine `watchQR`); (d) `db.Close()` setelah semuanya (urutan defer saat ini sudah benar untuk db). Goroutine di `main` menunggu dengan `sync.WaitGroup` dan log "api stopped" setelah semua selesai; `ListenAndServe` error -> `stop()` tidak `os.Exit(1)` langsung (`defer` tak jalan).
2. `Gateway`: field `appCtx`; `Connect(ctx)` memakai `g.appCtx` untuk `GetQRChannel` dan menjadikan ctx request hanya pembatas tunggu; `ensureClient` idem. `Disconnect/Reset` tetap. Tambah `Close()` idempotent.
3. **Uji manual wajib** (hipotesis QR): pairing dengan ponsel asli sebelum/sesudah perbaikan; unit test tak dapat membuktikannya. Bila hipotesis salah, perubahan tetap tak merugikan.
4. In-flight job lain (ekspor, import produk) selesai karena `Shutdown` menunggu handler; WriteTimeout 60s > 25s -> request lama dapat dipotong; dokumentasikan.
**Test**: unit `Gateway.Close` idempotent (tanpa jaringan: tutup sebelum `ensureClient`), integrasi: kirim `SIGTERM` ke biner hasil build di proses anak (skip di Windows; jalan di CI Linux) -> keluar kode 0 dalam < 30 dtk dengan request lambat berjalan. **Dependensi**: F6-4. **Ukuran**: M.

## F6-7 Backup, restore, rollback — skrip + runbook (M)

Berkas: `scripts/ops/backup.sh`, `scripts/ops/restore.sh`, `scripts/ops/rollback.md` (runbook di `docs/RUNBOOK.md`), `scripts/ops/verify-backup.sh`.
- **Backup DB**: `mysqldump --single-transaction --routines --triggers --events --hex-blob --default-character-set=utf8mb4 --set-gtid-purged=OFF --no-tablespaces $DB | gzip > backups/db-<timestamp>-<gitsha>.sql.gz`; kredensial via `--defaults-extra-file` (tidak di argumen proses); sertakan `SELECT MAX(version) FROM schema_migrations` pada nama/metadata. `mysqldump` Laragon tersedia lokal (`bin/mysqldump.exe`, my.ini `[mysqldump] quick`).
- **Backup storage**: `tar` terkompresi `data/storage/media` + `data/storage/_private` (sesi WA **sensitif**: enkripsi `age`/`gpg` atau izin `0600` dan simpan terpisah). Objek S3 tidak dibackup oleh skrip (diversi oleh penyedia); catat kebijakan.
- **Retensi**: harian 14 hari, mingguan 8, bulanan 6 (parameter env); satu salinan di luar host (Pertanyaan P5).
- **Restore**: `restore.sh <dump>` -> **ke database baru** `mini_erp_restore_<ts>` dahulu, jalankan `verify-backup.sh` (jumlah baris tabel kunci, `schema_migrations`, checksum jurnal: `SUM(debit)=SUM(credit)`), baru `RENAME`/arahkan `DB_DSN`. Menolak menimpa DB yang berjalan tanpa `--yes-overwrite`.
- **Uji pulih terjadwal**: runbook mewajibkan restore ke DB uji tiap bulan dan sebelum cutover (gladi stage L).
- **Rollback aplikasi** (versi image): `APP_VERSION=<lama> docker compose up -d app`; bila migrasi sudah naik, `migrate down N` **hanya** jika `*.down.sql` terverifikasi dan backup tersedia; bila tidak, restore backup pra-deploy. Setiap deploy: backup dulu (skrip dipanggil oleh `scripts/ops/deploy.sh`: backup -> migrate up -> `up -d` -> tunggu healthy -> smoke `curl`).
- **Rollback cutover ke legacy** (jalur balik): legacy tetap hidup **read-only** (DB legacy tidak diubah; akses tulis dimatikan di tingkat aplikasi/ proxy) selama jendela rollback (Pertanyaan P6). Runbook: kriteria (rekonsiliasi saldo/stock gagal, galat kritis), langkah (hentikan trafik revamp -> kembalikan DNS/proxy ke legacy -> buka tulis legacy), **konsekuensi**: transaksi yang sudah dicatat di revamp setelah cutover tidak ada di legacy -> ekspor delta (CSV dari tabel dokumen) untuk entri ulang; jangan ada sinkronisasi dua arah. Objek S3: revamp tidak menghapus objek legacy (F1-9) sehingga legacy tetap utuh.
- Windows (mesin pemilik): padanan `.ps1` untuk backup dev (`mysqldump` lokal), tetapi produksi Linux -> `.sh` utama.
**Test**: jalankan backup->restore->verify terhadap `mini_erp_test` (data hasil test integrasi) di CI/ lokal; test skrip dengan `shellcheck`. **Dependensi**: F6-2, F6-3, ETL/cutover (stage K/L). **Ukuran**: M.

## F6-8 Dokumen & `.env.production.example`; koreksi dokumen yang salah (S)

- **`.env.production.example`** (tanpa rahasia; semua komentar Indonesia): `APP_ENV=production`, `APP_PORT=8080`, `DB_DSN=mini_erp_app:<ISI>@tcp(host.docker.internal:3306)/mini_erp_db` (user aplikasi khusus, **bukan root**; hak minimum `SELECT,INSERT,UPDATE,DELETE` + untuk migrasi user terpisah `mini_erp_migrate` dengan `ALTER/CREATE/DROP/INDEX`), `JWT_SECRET=<openssl rand -hex 32>`, `MEDIA_SIGNING_KEY`, `JWT_ACCESS_EXPIRES_IN=15m`, `JWT_REFRESH_EXPIRES_IN=7d`, `CORS_ALLOWED_ORIGINS=https://<domain>`, `TRUSTED_PROXIES`, `PUBLIC_DIR=/app/public`, `STORAGE_DIR=/data/storage`, `SESSION_DIR=/data/storage/_private`, `STORAGE_DRIVER=s3` + prefix eksplisit & `STORAGE_PROTECTED_PREFIXES`, `METRICS_TOKEN`, `LOG_LEVEL=info`, `SHUTDOWN_TIMEOUT=25s`, `BOOTSTRAP_ADMIN_*` (dihapus setelah admin pertama). `.env.example` dibersihkan (APP_ENV diisi, `JWT_SECRET=change-me` tetap contoh yang ditolak di luar development).
- **`docs/DEPLOYMENT.md`** ditulis ulang: Go 1.26.x (toolchain `go1.26.7`), `GOFLAGS`/`safe.directory`, `APP_ENV` wajib, `npm run verify`, test DB (F0-1), urutan deploy (backup -> migrate -> up -> health), TLS & proxy (F6-2), volume, backup/restore (F6-7), `/media` bertanda tangan, hapus klaim "go 1.21".
- **Koreksi**: (1) `docs/PARITY_AUDIT.md:712-715` "C0 (git init) — ditunda" -> "selesai: repo git ada (`main`, remote AmandaYora/mini-erp-v2), `.gitignore` memuat `apps/api/storage/`"; (2) `.claude/rules/api-standard.md`: `total_pages` -> `totalPages` (sesuai `response.go:15`, `API_CONTRACT.md:57`, `API_GUIDE.md:17`, `http-client.ts:18`) — koreksi di **dokumen**, bukan kode (kode + frontend sudah konsisten); (3) `docs/API_CONTRACT.md:306` rute `/uploads/*` -> `/media/*` + `/api/v1/health/ready` + `/api/v1/metrics`; (4) `CLAUDE.md` bagian "Commands": tambah `npm run verify`, `scripts/test-db.ps1`; jaga ringkas (aturan monorepo: gateway, detail ke `knowledge/`); (5) `knowledge/BACKEND_GUIDE.md`/`INDEX.md`: tautan ke `docs/RUNBOOK.md`; (6) hapus rujukan `Admin123!` dari seluruh dokumen.
**Test**: `grep -rn "Admin123\|go 1.21\|total_pages" .` harus kosong di dokumen aktif; lint tautan manual. **Dependensi**: seluruh F1/F6. **Ukuran**: S.

---------------------------------------------------------------------------------------------------

## Urutan pengerjaan yang disarankan

1. F0-1, F0-3 (agar bisa memverifikasi apa pun) -> F0-2 -> F0-4 (CI) paralel dengan F1-2.
2. F1-1 + F1-9 (kebocoran sesi WA = prioritas tertinggi; boleh dikirim lebih dulu sebagai hotfix kecil: ganti `MountUploads` dan folder sesi) -> F1-3 -> F1-5 -> F1-4 -> F1-10.
3. F1-6 -> F1-7 -> F1-8 -> F1-11.
4. F6-5 -> F6-4 -> F6-6 -> F6-1 -> F6-2 -> F6-3 -> F6-7 -> F6-8.
Rentetan migrasi baru: `authz_seed_applied` (F1-6), `media_files.origin` (F1-9), `user_sessions.rotated_at` (F1-10) — tiga file kecil, di-squash kemudian.

## Risiko

1. **Pairing WA via UI** (F6-6): perubahan konteks gateway tidak dapat diuji otomatis; butuh ponsel. Mitigasi: uji manual sebelum merge, flag rollback mudah (ctx request tetap tersedia).
2. **Perubahan URL media** (`/uploads` -> `/media` bertanda tangan): klien yang menyimpan URL lama (mis. pesan WA/ekspor yang sudah terkirim) putus; URL kedaluwarsa pada halaman terbuka lama. Mitigasi: jendela exp dibulatkan 1 jam + ttl, frontend memuat ulang data saat gambar gagal (tidak ditambahkan di sini; catat untuk fase frontend).
3. **`APP_ENV` wajib** dapat mematahkan deploy/skrip yang mengandalkan default -> semua `.env*`/compose/CI harus diisi; test `config` menutup.
4. **Cost bcrypt 12** menaikkan beban login/CPU kecil; **rehash** menulis ke `users` saat login (kunci baris singkat).
5. **Anti lock-out** menolak operasi administratif yang sah bagi pemilik yang sengaja ingin menurunkan hak; pesan jelas, tidak ada bypass lewat API (hanya lewat `seed admin`).
6. **Default role** dari peta legacy -> baru bersifat turunan (legacy tak punya izin per-modul setara `customers.*`, `delivery.*`, ...): kemungkinan ada izin berlebih/kurang; ETL stage K harus memakai peta yang sama dan hasilnya diverifikasi pemilik per role nyata.
7. **Percepatan `freshDB`** (hanya TRUNCATE tabel berisi) dapat mengubah urutan AUTO_INCREMENT; ukur dan perbaiki test yang bergantung ID.
8. **`read_only` rootfs + sqlite WA**: butuh uji; fallback = hapus `read_only`.
9. **Rotasi `rotated_at` grace 30 dtk** sedikit melemahkan deteksi replay (pencuri yang memakai token dalam 30 dtk sejak rotasi tak memicu pencabutan massal); diterima demi stabilitas multi-tab.
10. **Toolchain image**: tag `golang:1.26.7-alpine` harus ada; bila belum, pakai `golang:1.26-alpine` dengan `GOTOOLCHAIN=auto` (butuh jaringan saat build).
11. `.env` lokal berisi kunci S3 yang sama dengan production legacy: sebelum F1-9 selesai, risiko operasional (arsip/hapus dari dev menyentuh bucket produksi) tetap ada.

## Pertanyaan terbuka (hanya yang butuh keputusan pemilik)

- **P1** Apakah akun developer `superadmin` (user id 1 legacy) dipertahankan di production revamp (dimigrasi ETL dan tersembunyi dari non-superadmin), atau dihapus/dinonaktifkan saat cutover? Default rancangan: dipertahankan, tersembunyi, tak bisa ditetapkan lewat API.
- **P2** Role `admin` legacy memegang `role.manage` (bisa mengedit izin role mana pun, termasuk memberi dirinya sendiri hak lebih) — pertahankan paritas ini, atau tambahkan aturan "aktor hanya dapat menetapkan/memberi izin yang ia miliki sendiri"? Default rancangan: paritas (tanpa aturan) + dicatat sebagai risiko.
- **P3** Reverse proxy/TLS: apakah di host sudah ada nginx/Caddy/certbot yang terminasi TLS (rancangan: ya, di luar compose, standar "satu container app"), atau ingin nginx masuk compose (menyalahi standar)? Dan domain produksi apa (untuk `CORS_ALLOWED_ORIGINS`, HSTS)?
- **P4** Dapatkah Anda menerbitkan **access key S3 terpisah** (idealnya tanpa hak hapus pada prefix legacy) atau bucket dev untuk revamp? Sampai itu ada, dev lokal harus `STORAGE_DRIVER=local`. Dan untuk production revamp: unggahan baru ke prefix terpisah (`vioni/production-v2/upload` — rancangan) atau tetap di prefix legacy?
- **P5** Tujuan backup di luar host (lokasi/penyedia) dan retensi yang diinginkan (rancangan: 14 harian/8 mingguan/6 bulanan).
- **P6** Jendela rollback ke legacy: berapa lama legacy dibiarkan read-only setelah cutover, dan dapatkah diterima bahwa transaksi yang dicatat di revamp selama jendela itu dikembalikan ke legacy hanya lewat ekspor delta + entri ulang manual (tanpa sinkronisasi dua arah)?
- **P7** CI: apakah repo `AmandaYora/mini-erp-v2` boleh memakai GitHub Actions runner hosted (menit gratis/privat)? Dan apakah image perlu di-push ke GHCR pada `main` (rancangan: build saja)?
