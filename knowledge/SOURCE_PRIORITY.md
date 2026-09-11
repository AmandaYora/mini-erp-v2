# Source Priority — mini-erp

Ketika dua sumber mendeskripsikan fakta yang sama lalu bertentangan, yang lebih tinggi menang —
tapi konflik yang nyata selalu dilaporkan sebagai temuan, tidak pernah diselesaikan diam-diam.

1. `knowledge/decisions/ADR-*` — keputusan yang sudah diratifikasi
2. Standar monorepo Dimas + `.claude/rules/`
3. Kode, konfigurasi, migrasi aktif
4. `knowledge/*` dan `docs/*` (PRD, SYSTEM_DESIGN, API_CONTRACT, DB_SCHEMA, legacy-reference)
5. Observasi & hipotesis AI

Kode menunjukkan apa yang **benar-benar terjadi**, bukan apa yang **seharusnya**.
Bila (3) dan (4) bertentangan, itu konflik yang wajib dilaporkan — bukan pilihan yang diambil
sendiri.
