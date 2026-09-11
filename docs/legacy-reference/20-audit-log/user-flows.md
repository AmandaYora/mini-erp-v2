# User Flows — Modul 20 Audit Log

**Kelompok A.** Aktor: Pemilik/Admin (`audit_log.view`), Staff (ditolak), Sistem (penulis).

---

## F-01 Meninjau Riwayat (alur utama)

1. Buka menu Pantauan → Riwayat Aktivitas (`/audit-logs`).
2. Kartu menampilkan `Memuat…` lalu `{total} aktivitas tercatat.`
3. Baca tabel terbaru-dulu (25/halaman): waktu, siapa, aktivitas apa, data apa.
4. Pindah halaman via paginasi (server-side, 25 per halaman).
5. Bila kosong → empty-state `Belum ada aktivitas`.

## F-02 Menelusuri Kejadian (pola E2E 10 + 08 — via API, karena UI tanpa filter!)

1. Ingat aksi + entitas (mis. `order.create` + `order`).
2. `POST audit-logs/list` dengan `{ action_key, entity_type, limit }`.
3. Cocokkan `id_entity` / waktu dengan kejadian:
   - E2E 10 (`10-observability-permission-hardening.spec.ts:102-119`): setelah buat order
     + `deliver-goods` (pay_now cash), asersi `order.create`/`order` > 0,
     `stock.adjust`/`inventory_balance` > 0, `payment.create`/`payment` > 0 — adjust
     lahir dari helper `addStock` (`helpers/business.ts:569-576` → `stock/adjust`),
     bukan dari alur order itu sendiri!
   - E2E 08 (`08-inventory-return-transfer-adjustment.spec.ts:365-370`): setelah 3×
     `stock/adjust` (in/out/adjustment), asersi `stock.adjust`/`inventory_balance` > 0.
4. Catatan: filter substring — `action_key: 'order'` mengembalikan semua aksi order.

## F-03 Jejak Tertulis Otomatis (tanpa aktor)

1. User melakukan write bisnis (buat order, posting jurnal, ubah role, ...).
2. Service pemanggil menulis baris audit dalam/pasca transaksi yang sama
   (pola DALAM vs SETELAH — lihat F-01d; append-only, stempel server,
   sebelum/sesudah/metadata sesuai pemanggil; satu aksi bisa mengipas 2-3+ baris —
   lihat algorithms §1).
3. Baris muncul di F-01 pada muat berikutnya (tanpa realtime/refresh otomatis!).

## F-04 Akses Ditolak

1. Staff tanpa `audit_log.view` membuka `/audit-logs` → ditolak penjaga rute
   (pola global); panggil API langsung → 403. Tanpa token → 401.
2. Smoke menu: `/audit-logs` terdaftar di `01-sidebar-and-menu-smoke.spec.ts:52`
   (`{ path: '/audit-logs', text: /Riwayat Aktivitas/i }`) — halaman dikunjungi +
   dicek bebas runtime-error bersama `/dashboard` dkk. di E2E 10
   (`10-observability-permission-hardening.spec.ts:137-141`).
