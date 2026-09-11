# Business Rules — Modul 06 Business Party (Customer / Supplier)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula, kondisi
khusus. Diturunkan dari `business-party.service.ts`, `customer-address.service.ts`, kedua controller,
form FE, dan E2E 20/21/22. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Tipe & identitas (BR-01…BR-06)

| ID | Aturan |
|---|---|
| BR-01 | `party_type` ∈ {`customer`, `supplier`}; ditentukan oleh endpoint (`customers/*` vs `suppliers/*`), **tidak bisa diubah** setelah dibuat (tidak ada field di update). Tipe salah di URL → `404 'Data tidak ditemukan'` (mis. customer dibuka via `suppliers/detail`) |
| BR-02 | Kode unik per `(perusahaan, kode, tipe)` **termasuk baris terarsip** (unique key `uq_business_parties_code` mencakup arsip). Cek create tanpa filter arsip → bentrok dengan Sampah = `409 "Kode '{k}' sudah digunakan (ada di Sampah)"`, bukan 500. Kode arsip **tidak bisa dipakai ulang** (beda dengan produk yang bisa) |
| BR-03 | Kode input di-`trim`; kosong → auto (lihat §2). Kode **tidak bisa diubah** (DTO update tanpa field kode; nilai FE diabaikan — → KI-62) |
| BR-04 | Nama: **tanpa validasi server** (bisa `""` via API langsung). Wajib hanya di UI: `required` HTML form menu + cek JS `trim()` di shortcut Order/POS (`"Nama pelanggan wajib diisi"`) |
| BR-05 | Telepon (`phone_e164`), email, alamat, catatan: tanpa validasi server (format bebas, boleh kosong/null). `type=email` hanya di browser. Nama kolom `phone_e164` menjanjikan format E.164 yang tidak ditegakkan (M2-Q4 tetap terbuka) |
| BR-06 | Update: tiap field hanya bila `!== undefined` (string kosong = hapus nilai). `attributes` menang atas `attributes_json`; `attributes: null` eksplisit = hapus seluruh metadata; keduanya undefined = tak tersentuh |

## 2. Kode otomatis (BR-07…BR-10)

| ID | Aturan |
|---|---|
| BR-07 | Prefix `CUS-` (customer) / `SUP-` (supplier); suffix = angka tertinggi + 1 di antara kode berpola murni `PREFIX+digit` **termasuk arsip** (kode tak berpola diabaikan — `CUST-9999` tidak menggeser `CUS-`; manual `CUS-007` menggeser ke `CUS-008`); `padStart(3,'0')` (tumbuh alami: `CUS-1000`); tanpa kandidat → `CUS-001` |
| BR-08 | Karena arsip ikut dihitung, kode auto **tidak pernah dipakai ulang** — restore tak pernah bentrok dengan auto-code |
| BR-09 | Tanpa retry konkuren: dua create kosong bersamaan dapat menghasilkan kode sama → satu gagal 500 unique-key (→ KI-64) |
| BR-10 | Kode manual bebas bentuk (≤50 char, case mengikuti collation `utf8mb4_unicode_ci`); manual berpola ikut menggeser auto berikutnya (BR-07) |

## 3. Member (BR-11…BR-14; tulis milik modul 07, baca di sini)

| ID | Aturan |
|---|---|
| BR-11 | `id_member_type`: `undefined` → null (create) / lestari (update); `null`/`0` → lepas (null) |
| BR-12 | Non-null untuk supplier → `400 'Jenis member hanya dapat dipakai untuk pelanggan.'` (dicek sebelum keberadaan) |
| BR-13 | Harus milik perusahaan + non-arsip (`400 'Jenis member tidak ditemukan.'`) + status active (`400 'Jenis member tidak aktif.'`) |
| BR-14 | FE: field hanya bila `member_type.view`; opsi = member aktif + member terpasang (walau nonaktif); tanpa izin → field tak dikirim → nilai lama lestari diam-diam (→ KI-67). Guard arsip member yang dipakai (modul 07, dikunci E2E 22-B) membaca relasi ini |

## 4. Daftar & cari (BR-15…BR-18)

| ID | Aturan |
|---|---|
| BR-15 | `status`: `archived` → Sampah (`archived_at IS NOT NULL`, urut arsip terbaru); selain itu → aktif (urut nama A→Z). Selalu dilingkup perusahaan + tipe + join member |
| BR-16 | Cari = satu frasa `LIKE %s%` di nama/kode/telepon/email/alamat (OR). **Bukan tokenized**: urutan kata harus pas (→ KI-65) |
| BR-17 | Paginasi: `page ?? 1`, `limit = min(??20, 100)` — tanpa batas bawah (→ KI-70) |
| BR-18 | Detail by-id mensyaratkan non-arsip + `relations: memberType` selalu; dipakai form edit (anti-duplikat E2E 21) dan resolve picker |

## 5. Arsip & restore (BR-19…BR-22)

| ID | Aturan |
|---|---|
| BR-19 | Arsip = soft **tanpa syarat**: boleh berpiutang, berorder, bermember. Efek: hilang dari daftar/picker; relasi tetap (saldo, ledger, pelunasan, order lama, nama live) — dikunci E2E 22. Order baru ditolak **modul Order**, bukan di sini |
| BR-20 | Restore mensyaratkan sedang terarsip, else `404 'Data tidak ditemukan di Sampah'` |
| BR-21 | Restore memeriksa bentrok kode vs pihak aktif → `409 "Kode '{k}' sudah dipakai data aktif lain. Ubah kode itu dulu sebelum memulihkan."`. Normalnya mustahil (BR-02 reserve lintas arsip); bila terjadi, saran itu **tak bisa dijalankan** (BR-03, → KI-62) |
| BR-22 | Audit: create `{id,name,idMemberType}`; update `before` = seluruh baris + `after` `{id,name,idMemberType}`; archive/restore = id saja. Semua `idBranch: null`, `entityType: 'business_party'` (+ varian tipe di actionKey: `customer.*`/`supplier.*`) |

## 6. Buku alamat (BR-23…BR-31)

| ID | Aturan |
|---|---|
| BR-23 | Hanya customer aktif non-arsip (list & create menegaskan; supplier/arsip/tak dikenal → `404 'Pelanggan tidak ditemukan.'`). Update/archive **tidak** menegaskan (inkonsisten — → KI-63) |
| BR-24 | Teks alamat wajib non-kosong setelah trim (create & update) → `400 'Alamat wajib diisi.'`; label/penerima/telepon kosong → `null` |
| BR-25 | Invarian "tepat satu Utama" (selama ≥1 alamat aktif): pertama otomatis Utama; `is_primary: true` (create/update) menurunkan yang lain **dalam satu transaksi**; `false` di update **diabaikan** (→ KI-69) |
| BR-26 | Arsip Utama mempromosikan aktif lain paling awal (`sortOrder ASC, id ASC`) dalam transaksi yang sama; bila tak tersisa, pelanggan boleh tanpa alamat/prrimer |
| BR-27 | Urutan list: primer dulu, lalu `sortOrder`, lalu id. `sort_order` default 0, bebas diisi (tanpa validasi; tanpa UI sortir) |
| BR-28 | Alamat tak berpindah pelanggan (update tanpa field pihak) dan tak bisa dipulihkan (tanpa endpoint restore — arsip final, tidak seperti pihak) |
| BR-29 | `findOwned` melingkup `(id, perusahaan, non-arsip)` → `404 'Alamat tidak ditemukan.'` untuk milik perusahaan lain/arsip |
| BR-30 | Backfill migrasi 044: tiap customer ber-`address_text` non-kosong tanpa alamat mendapat satu `Utama` dari teks itu (idempoten via `NOT EXISTS`); kolom snapshot order ditambah per-kolom |
| BR-31 | Audit `customer_address.create/update/archive` (`entityType: 'business_party_address'`, `idBranch: null`); update mencatat `before{label,address,isPrimary}`. Snapshot order (`id_ship_to_address` + 4 teks) ditulis modul Order dengan filosofi sama seperti snapshot item (stabil walau buku berubah) |

## 7. Aturan lintas modul yang dijaga dari sini (BR-32…BR-34)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-32 | Relasi order→pihak by-id dengan nama live (rename mengikuti, tanpa duplikat) | Order list/detail/cetak; E2E 22-A |
| BR-33 | Pihak arsip tetap di saldo & ledger, tetap bisa dilunasi; order baru ditolak | Payment (saldo/ledger), Order (validasi pihak) |
| BR-34 | `customer-addresses/create` memakai `order.create` (bukan `update`) agar kasir penambah pelanggan bisa tambah alamat inline; arsip alamat memakai `order.update` (bukan `archive`) | POS, order form (izin pintu) |

## 8. Aturan yang TIDAK ada (verifikasi)

Tidak ada: validasi nama/telepon/email di server; batas panjang selain kolom (kode 50, nama 255,
telepon 30, label 120, penerima 150); unik telepon/email; larangan arsip bertransaksi; larangan rename
bertransaksi; approval untuk perubahan master (berlaku seketika); rate-limit; cek konkuren kode auto;
demote primer; restore alamat; pencarian gabungan customer+supplier; filter/sortir daftar selain
nama/arsip.
