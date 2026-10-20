# Source Priority — mini-erp

Ketika dua sumber mendeskripsikan fakta yang sama lalu bertentangan, yang lebih tinggi menang —
tapi konflik yang nyata selalu dilaporkan sebagai temuan, tidak pernah diselesaikan diam-diam.

1. `knowledge/decisions/ADR-*` — keputusan yang sudah diratifikasi
2. Standar monorepo Dimas + `.claude/rules/`
3. Kode, konfigurasi, migrasi aktif
4. Dokumen referensi: `knowledge/*`, `docs/STATUS.md`, `docs/PRD.md`, `docs/SYSTEM_DESIGN.md`,
   `docs/API_CONTRACT.md`, `docs/DB_SCHEMA.md`, `docs/DEPLOYMENT.md`, `docs/RUNBOOK.md`, `docs/PARITY_AUDIT.md`
5. Dokumen riwayat: `docs/plan/**`, audit bertanggal (`docs/ANALISIS-PARITAS-*.md`, `docs/PARITY_AUDIT.prev.md`),
   `analysis/**` — benar pada tanggalnya, bukan keadaan hari ini
6. `docs/legacy-reference/` — menggambarkan sistem **lama**; sumber perilaku yang ditiru, bukan
   deskripsi revamp
7. Observasi & hipotesis AI

Kode menunjukkan apa yang **benar-benar terjadi**, bukan apa yang **seharusnya**.
Bila (3) dan (4) bertentangan, itu konflik yang wajib dilaporkan — bukan pilihan yang diambil
sendiri. Bila (4) tertinggal dari kode karena perubahan yang memang disengaja, perbarui dokumennya
di commit yang sama.
