# UI/UX Spec — Modul 20 Audit Log

**Kelompok A.** Satu halaman + pola pemuatan slice. Sumber: `audit-log-pages.tsx`
(penuh, 263 baris), `use-audit-log-module.ts` (18 baris), `audit-log.slice.ts` (30 baris),
`toAuditLog` (`org.adapter.ts:36-57`), `formatDateTime` (`utils.ts:25-37`), registry
(`module-registry.tsx:735-742`), izin (`role-access-config.ts:217-218`), test adapter
(`adapters.test.ts:597-644`), test registry/route (`module-registry.test.ts:37,156-159`,
`route-access.test.ts:107-108`).

---

## 1. Halaman Riwayat Aktivitas — `/audit-logs` (izin `audit_log.view`)

- **Akses:** menu grup **Pantauan** → **Riwayat Aktivitas** (ikon `audit_logs`;
  `module-registry.tsx:741`; judul route `Riwayat Aktivitas`, `access: protected`,
  `shell: true`). Tanpa data workspace → halaman kosong (render `null`).
- **Header:** breadcrumb `Dashboard > Riwayat Aktivitas`; judul `Riwayat Aktivitas`;
  deskripsi `Jejak aktivitas penting yang tercatat dari modul operasional dan konfigurasi.`
- **Kartu `Aktivitas Terbaru`:** deskripsi `Memuat…` saat loading, else
  `{total} aktivitas tercatat.` (total = seluruh perusahaan, bukan halaman ini!).
- **Tabel kolom persis:** Waktu | Pengguna | Aktivitas | Data Terkait | Keterangan.
  - Waktu: `formatDateTime` (`04 Sep 2026, 14:30` gaya `id-ID`, tanpa detik; kosong → `-`).
  - Pengguna: nama display user, atau `User #{id}`, atau `System`.
  - Aktivitas: label Indonesia (`Buat Order`, `Balikkan Jurnal`, ...) atau fallback
    `Kunci - Lain` untuk 27 nilai runtime tanpa label (NF-05/NF-06).
  - Data Terkait: dua baris — tebal `{entityType} #{id}` + redup label entitas
    (`Jurnal Finance`, `Penyesuaian Pajak`, ...).
  - Keterangan: selalu `Aktivitas tercatat otomatis.`
- **Paginasi:** 25/halaman (`Pagination` bersama; `isLoading` saat muat).
- **Kosong:** `EmptyState` judul `Belum ada aktivitas` + deskripsi
  `Aktivitas penting akan muncul setelah ada perubahan data di modul operasional atau pengaturan.`
- **Gagal:** diam — tabel kosong tanpa pesan (catch → `[]`).
- **Tanpa di halaman:** filter, pencarian, urutan, ekspor, drill-down baris,
  penampil before/after.

## 2. Pemuatan Latar (slice + hook)

- Kunjungan pertama (status `idle`, sudah login) memicu `reloadAuditLogs`:
  paralel `users/list` (100) + `audit-logs/list` (100); status `loading`/`ready`
  tidak memicu ulang (dijaga test!).
- Hasil disimpan ke store (`userRecords` bila tak-kosong, else pertahankan lama;
  `auditLogs` dipetakan via `toAuditLog`); halaman memetakan ulang 25 barisnya
  sendiri dari API (bukan dari store!).
- Catatan: prefetch 100 vs halaman 25 — dua sumber angka berbeda (total halaman
  dari API, nama dari store; lihat KI-131).
- Tiap request slice punya `.catch(() => ({ items: [] }))` sendiri
  (`audit-log.slice.ts:12-13`) — `users/list` gagal (mis. tanpa `user.view`)
  tak menggagalkan daftar audit, hanya mendegradasi nama aktor ke `User #`.
- `toAuditLog` menerima DUA ejaan field (`idActor ?? id_actor`, dst.,
  `org.adapter.ts:37-56`) + fallback `happenedAt = new Date().toISOString()`
  (`:47`) bila respons tanpa stempel — lihat temuan baru business-rules.
- Izin `user.view` menentukan kualitas nama: store `userRecords` hanya terisi bila
  sesi boleh memanggil `users/list` (komentar `org.adapter.ts:16-23`); kasir/staf
  tanpa izin tetap melihat namanya sendiri? cek! — fallback akun-aktif hanya ada di
  `resolveUserName` (dipakai struk/nota), BUKAN di `toAuditLog` (yang memakai
  `User #`/`System` saja, `:38-39`).
