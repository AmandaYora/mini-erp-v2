# ADR-0019: Deploy produksi ke VPS elcodelabs — image dari CI (GHCR), server hanya pull & run

## Status
Accepted (2026-10-21). Melengkapi ADR-0001/0002 (satu container, MySQL di host); menggantikan alur
lama `scripts/ops/deploy.sh` yang membangun image di server.

## Context
mini-erp lama berjalan di VPS `mini-erp` (103.55.38.123) tanpa Docker: NestJS lewat pm2, nginx,
MySQL host, domain `minierp.my.id`. Revamp akan berjalan di VPS **elcodelabs** (103.189.235.79)
sebagai aplikasi ke-4, di samping elkasir, elproof, dan elwedding. Server itu punya konvensi baku
(`/opt/elcodelabs/SERVER_PLAYBOOK.md`):
- 1 aplikasi = 1 container;
- MySQL di host dipakai bersama (1 database + 1 user `<app>_user@'172.%'` per aplikasi);
- image dibangun di CI lalu didorong ke GHCR, server hanya `pull` (RAM 2 GB);
- port `127.0.0.1` unik per aplikasi di belakang nginx host + certbot;
- rahasia hanya di `~/<app>/.env`.

Rancangan deploy revamp sebelumnya membangun image di server dan memakai port 8080 serta nama
DB/user sendiri.

## Decision
- Nama aplikasi di server **`minierp`**: folder `~/minierp`, container `minierp-app`, port
  `127.0.0.1:8084` (port container tetap 8080), database `minierp_db`, user
  `minierp_user@'172.%'` (`mysql_native_password`).
- `.github/workflows/release-image.yml` membangun `linux/amd64` setelah workflow `ci` lulus untuk
  push ke `main`, lalu mendorong `ghcr.io/amandayora/mini-erp-v2:<git-sha>` dan `:latest`.
- Server memakai `infra/elcodelabs/docker-compose.prod.yml` (image saja, tanpa `build:`) dan
  `infra/elcodelabs/deploy.sh <git-sha>`, yang menjalankan:
  cek awal → pull → backup DB (`sudo mysqldump`) → `migrate up` → `up -d` → tunggu `/readyz`.
- `IMAGE_TAG` wajib di compose (tanpa default `latest`) dan dicatat `deploy.sh` ke `.env`, supaya
  `docker compose up` manual tidak pernah turun ke tag `latest` lokal yang lebih tua. Ini celah
  yang ditemukan pada tiga aplikasi lain di server yang sama.
- `TRUSTED_PROXIES` mencakup `172.16.0.0/12`, karena nginx host terlihat dari bridge docker.
- Media tetap di bucket S3 IDCloudHost yang sama dengan sistem lama, dengan prefix unggahan baru.
  Prefix lama dilindungi lewat `STORAGE_PROTECTED_PREFIXES` (F1-9).
- `docker-compose.yml` di akar repo dipertahankan khusus uji lokal (build dari sumber).
  `scripts/ops/deploy.sh` dihapus.

## Consequences
- Rilis = merge ke `main` → CI hijau → image terbit → `./deploy.sh <sha>` di server.
  Rollback = `./deploy.sh <sha-lama>`; migrasi maju saja (RUNBOOK §4).
- Paket GHCR baru defaultnya privat: server butuh PAT `read:packages` (sudah ada untuk
  `amandayora/*`) atau paket dibuat publik.
- Domain cutover: bila tetap `minierp.my.id`, DNS dipindah ke elcodelabs dan APK kasir tidak
  perlu dibangun ulang (DEPLOYMENT §7). Domain staging sebelum cutover diputuskan pemilik.
- Belum terbukti sampai dijalankan: build image di CI dan deploy pertama di server. Yang sudah
  diuji lokal tanpa Docker: CLI `migrate` (versi yang sama dengan image) dan biner server dalam
  mode produksi dengan user `mysql_native_password`.
