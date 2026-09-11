# API Guide — mini-erp

Ringkasan gateway. Konvensi penuh ada di **[../docs/API_CONTRACT.md](../docs/API_CONTRACT.md)** —
baca itu sebelum menulis handler baru, dan tegakkan `.claude/rules/api-standard.md` saat menyunting
kode.

## Ringkas

- Semua endpoint bisnis di `/api/v1/*`, metode HTTP REST standar (`GET` baca, `POST`/`PUT`/`PATCH`/`DELETE` tulis).
- Sukses: `{ "success": true, "message": "...", "data": {...} }`.
- Gagal: `{ "success": false, "message": "...", "errors": [...] }` — `message` **selalu** pesan asli
  yang bisa ditampilkan ke pengguna, tidak boleh dibuang di frontend.
- Scope (`branchId`/`userId`/role/permission) selalu dari sesi (`auth`), tidak pernah
  dari body/query.
- Setiap endpoint tulis wajib validasi payload eksplisit + deklarasi permission eksplisit
  (fail-closed: tanpa deklarasi = ditolak).
- Paginasi: `{ data: [...], meta: { page, limit, total, totalPages } }`, `limit` dijaga dari nilai
  tidak wajar di `shared/pagination`.

## Kenapa Berbeda dari Sistem Lama

Sistem lama menyeragamkan semua endpoint jadi `POST` + body `{ data }`, envelope `{code, info,
data}`, dan validasi hanya di 1 dari ~220 endpoint. Alasan perubahan ada di
[../docs/API_CONTRACT.md §1, §4](../docs/API_CONTRACT.md).
