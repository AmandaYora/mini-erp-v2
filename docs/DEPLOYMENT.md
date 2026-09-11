# Deployment — Mini ERP (Revamp)

## 1. Prasyarat Lokal

| Tool | Versi minimum | Catatan |
|---|---|---|
| Node.js | 20+ | Untuk `apps/web` dan tooling root |
| Go | 1.21+ | `apps/api/go.mod` menyatakan `go 1.21` — samakan dengan versi yang benar-benar terpasang lokal (cek `go version`), jangan biarkan Go mencoba mengunduh toolchain baru saat offline |
| MySQL | 8.0+ | Berjalan di host/OS, **bukan** container (lihat §4) |
| `air` | terbaru | Watcher dev backend — `go install github.com/air-verse/air@latest` |
| `migrate` (golang-migrate CLI) | terbaru | `go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| `sqlc` | terbaru | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |

## 2. Setup Awal

```bash
cp .env.example .env      # isi kredensial lokal, jangan commit .env
npm install                # root + apps/web (npm workspaces)
mysql -u root -e "CREATE DATABASE IF NOT EXISTS mini_erp_db"
npm run migrate:up         # jalankan migrasi awal (setelah modul pertama menulis migrasinya)
```

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
npm run migrate:create  # buat pasangan migrasi baru bernomor
npm run sqlc:generate   # generate kode akses DB dari internal/modules/**/infrastructure/queries
```

## 4. Docker (Produksi)

Default: **satu container aplikasi**, database MySQL tetap di level host/OS — bukan container
terpisah.

```txt
1 VPS / host
├── MySQL terpasang di level OS/sistem
└── Docker
    └── satu container aplikasi
        ├── build statis frontend (apps/web)
        └── backend API (apps/api, hasil `go build`)
```

```bash
docker compose up --build
```

Container mengekspos satu port (default `8080`):

- `/api/v1/*` → backend API
- selain itu → berkas statis SPA frontend

Koneksi DB dari container ke MySQL host (Linux VPS):

```env
DB_HOST=host.docker.internal
DB_PORT=3306
DB_USER=root
DB_PASSWORD=<isi-asli>
DB_NAME=mini_erp_db
DB_DSN=root:<isi-asli>@tcp(host.docker.internal:3306)/mini_erp_db
```

`docker-compose.yml` sudah menyertakan `extra_hosts: host.docker.internal:host-gateway` agar
resolusi ini bekerja di Linux VPS (bukan hanya Docker Desktop).

**Jangan** menambahkan service `mysql:` ke `docker-compose.yml` secara default — ini menyalahi
standar Docker Dimas (satu app container, DB di host).

## 5. Migrasi di Produksi

- Jalankan `npm run migrate:up` sebagai langkah deploy terpisah **sebelum** container baru mulai
  menerima trafik — jangan menjalankan migrasi otomatis di setiap start container (ini salah satu
  penyebab masalah di sistem lama: migration runner tanpa tabel pelacak, migrasi dijalankan ulang
  tiap start).
- `golang-migrate` sudah menyimpan versi migrasi di tabel `schema_migrations` — migrasi yang sudah
  jalan tidak akan diulang.

## 6. Variabel Environment Produksi

Lihat [API_CONTRACT.md §7](API_CONTRACT.md#7-konfigurasi-environment) untuk daftar lengkap.
Poin yang wajib diperhatikan khusus produksi:

- `CORS_ALLOWED_ORIGINS` **harus** diisi domain frontend produksi yang eksplisit — jangan `*`.
- `JWT_SECRET` wajib nilai acak yang kuat, berbeda dari `.env.example`.
- `APP_TIMEZONE=Asia/Jakarta` dan `APP_LOCALE=id-ID` adalah konstanta aplikasi — tidak perlu (dan
  tidak boleh) dijadikan setelan yang bisa diubah dari UI (lihat
  [ADR-0006](../knowledge/decisions/ADR-0006-locale-timezone-locked.md)).

## 7. Referensi

- [SYSTEM_DESIGN.md](SYSTEM_DESIGN.md) — arsitektur & stack
- [API_CONTRACT.md](API_CONTRACT.md) — konfigurasi environment lengkap
- `infra/docker/Dockerfile`, `infra/nginx/nginx.conf`, `docker-compose.yml` — implementasi konkret
