# Test Cases — Modul 06 Business Party (Customer / Supplier)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Skenario input→output nyata dari
kode/data. **Sudah ada** = `business-party.service.spec.ts` (479 baris),
`customer-address.service.spec.ts` (168 baris), `address-picker.test.tsx`,
`use-business-party-module.test.ts`, E2E 20/21/22. **GAP** = belum ada test.

---

## TC-L — List & detail (TC-L-01…TC-L-07)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-L-01 | `list {}` | `{items, meta}`; `archived_at IS NULL` | Ya (spec) |
| TC-L-02 | `list {search:'Jaya'}` | `andWhere` LIKE `%Jaya%` | Ya (spec) |
| TC-L-03 | `list {search:'0812'}` | Klausa mencakup `bp.name/code/phone_e164/email/address_text` | Ya (spec) |
| TC-L-04 | `list {status:'archived'}` / `{}` | `IS NOT NULL` / `IS NULL` | Ya (spec ×2) |
| TC-L-05 | `detail(42)` ada + relasi | Pihak + `memberType`; where `{id, company, type}` | Ya (spec) |
| TC-L-06 | `detail(999)` / tipe salah / terarsip | `404 'Data tidak ditemukan'` | Ya (spec, sebagian) + E2E 22 |
| TC-L-07 | Cari `"Santoso Budi"` vs `"Budi Santoso"` | Tidak ketemu (frasa, bukan token) | GAP → KI-65 |

## TC-P — Create/update/archive/restore pihak (TC-P-01…TC-P-16)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-P-01 | `create {code:'CUST-001', name}` baru | Tersimpan + audit `customer.create {id,name,idMemberType}` | Ya (spec) |
| TC-P-02 | `create` kode dipakai aktif | `409 ConflictException` | Ya (spec) |
| TC-P-03 | `create` kode milik Sampah | `409` + pesan mengandung `Sampah`; `save` tak dipanggil; cek tanpa filter arsip | Ya (spec) |
| TC-P-04 | `create {attributes:{level:'gold'}}` | `metadata` tersimpan | Ya (spec) |
| TC-P-05 | `create` + `id_member_type:7` aktif (customer) | `idMemberType=7` | Ya (spec) |
| TC-P-06 | `create` + member untuk supplier | `400 BadRequestException` | Ya (spec) |
| TC-P-07 | `create {name:'Bu Sari'}` (tanpa kode; ada CUS-001/003 + CUST-9999) | `code='CUS-004'`, tanpa cek duplikat | Ya (spec) |
| TC-P-08 | `create {code:'   ', name:'Walk-in'}` (kosong) | `CUS-001` | Ya (spec) |
| TC-P-09 | Supplier tanpa kode (ada SUP-005) | `SUP-006` | Ya (spec) |
| TC-P-10 | `update {id:1, name}` | Tersimpan + audit `customer.update` (before penuh) | Ya (spec) |
| TC-P-11 | `update {id:999}` | `404` | Ya (spec) |
| TC-P-12 | `update {attributes_json:{region:'Jawa'}}` | `metadata` tertimpa | Ya (spec) |
| TC-P-13 | `update {id_member_type:8}` (aktif) | `idMemberType=8` | Ya (spec) |
| TC-P-14 | `archive(1)` / `(999)` / supplier | `archivedAt=Date` + audit / `404` / audit `supplier.archive` | Ya (spec ×3) |
| TC-P-15 | `restore` tanpa bentrok / bentrok aktif / tak di Sampah | `archivedAt=null` + audit / `409` tanpa save / `404` | Ya (spec ×3) |
| TC-P-16 | `create {name:''}`; `update` string kosong; update kode | Lolos kosong / hapus nilai / kode diabaikan | GAP → KI-66/62 |

## TC-A — Alamat (TC-A-01…TC-A-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-A-01 | `list` pelanggan tak ada | `404` | Ya (spec) |
| TC-A-02 | `list` ada | Urut `isPrimary DESC` | Ya (spec) |
| TC-A-03 | `create {address:'   '}` | `400` | Ya (spec) |
| TC-A-04 | Alamat pertama | `isPrimary:true` + audit `customer_address.create` | Ya (spec) |
| TC-A-05 | Alamat ke-3 tanpa flag | `isPrimary:false` | Ya (spec) |
| TC-A-06 | `create {is_primary:true}` | Demote lain (qb execute) dalam transaksi | Ya (spec) |
| TC-A-07 | `update` id tak ada / label / kosongkan alamat | `404` / label berubah + audit / `400` | Ya (spec ×3) |
| TC-A-08 | `archive` non-primer / primer (ada aktif lain id 6) | `archivedAt` + audit / id 6 naik primer | Ya (spec ×2) |
| TC-A-09 | `shipToSelectionToInput` tersimpan/ad-hoc/kosong | Hanya id / teks+field / `""` pengosong | Ya (FE test ×3) |
| TC-A-10 | `orderToShipToSelection` snapshot id/ad-hoc/kosong/null | Pemetaan tepat | Ya (FE test ×3) |
| TC-A-11 | `toCustomerAddress` snake_case (`is_primary:1` dst.) | `{id:'3', isPrimary:true, sortOrder:2, ...}` | Ya (FE test) |
| TC-A-12 | Update/archive alamat milik pelanggan arsip; `is_primary:false`; arsip primer terakhir | Lolos / diabaikan / tanpa primer | GAP → KI-63/69 |

## TC-E — E2E terkunci (TC-E-01…TC-E-12, E2E 20/21/22)

| ID | Skenario | Kunci | Sudah ada |
|---|---|---|---|
| TC-E-01 | Menu: create lengkap (kode kosong→`CUS-###`, member, alamat) + edit telepon | `code~/^CUS-\d{3,}$/`; update by-id (id sama) | Ya (E2E 20-1) |
| TC-E-02 | Order: nama spasi → `"Nama pelanggan wajib diisi"`; isi → tersimpan + terpilih `#relatedPartyId`; alamat kosong tersimpan kosong | Validasi JS + auto-code + seed picker | Ya (E2E 20-2) |
| TC-E-03 | POS cash: nama spasi → pesan; isi → autocomplete terisi; kode auto | Pintu POS konsisten | Ya (E2E 20-3a) |
| TC-E-04 | POS pay-later: `No. HP *` required + `"No. HP wajib diisi untuk customer yang berhutang"` | Aturan khusus POS | Ya (E2E 20-3b) |
| TC-E-05 | Konsistensi: 3 kode unik `CUS-###`, ketiganya di daftar via satu pencarian | Unik lintas pintu + cross-visibility | Ya (E2E 20-4) |
| TC-E-06 | Form: alamat tak-required di semua pintu; member di Pelanggan+Order; edit by-id terisi | Keseragaman + bukti perbaikan | Ya (E2E 20-5) |
| TC-E-07 | Edit di luar 100 pertama (110 filler + target Z) | UPDATE 1 record (id sama, telepon baru), tanpa duplikat | Ya (E2E 21-B) |
| TC-E-08 | Tambah dari Order saat >100 | Langsung terpilih + tersimpan backend | Ya (E2E 21-A) |
| TC-E-09 | Rename: order by-id tetap; cari nama baru ketemu, lama tidak; tanpa duplikat | Nama live | Ya (E2E 22-A) |
| TC-E-10 | Member dipakai → arsip ditolak `/pelanggan\|digunakan\|masih/`; lepas → arsip lolos | Guard relasi (modul 07 membaca modul ini) | Ya (E2E 22-B) |
| TC-E-11 | Arsip berpiutang 30rb: hilang dari daftar, saldo tetap, ledger 30rb terbuka | Soft-delete relasional | Ya (E2E 22-C) |
| TC-E-12 | Order baru pihak arsip → error `/diarsip\|ditemukan/`; lunasi 30rb via payment → saldo 0 | Blokir bisnis baru + tagih jalan | Ya (E2E 22-D) |

## GAP (G-01…G-08)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Validasi nama/telepon/email kosong-format via API | Atenuasi KI-66; tanpa ini data sampah masuk lewat integrasi |
| G-02 | Kode diabaikan saat update + restore-bentrok jalan buntu | Mengunci KI-62 (perbaiki atau kunci sebagai perilaku + pesan baru) |
| G-03 | Simpan form gagal → umpan balik | Mengunci KI-61 (toast/notice + anti-double-submit) |
| G-04 | Alamat: pemilik arsip, demote, primer-terakhir, tanpa-konfirmasi hapus | Mengunci KI-63/69 |
| G-05 | Auto-code konkuren + manual-berpola | Mengunci KI-64 + contoh BR-07 |
| G-06 | Tokenized vs frasa pencarian pihak | Mengunci KI-65 (seragamkan dengan produk atau tetapkan frasa) |
| G-07 | `member_type.view`-less edit mempertahankan member | Mengunci KI-67 (sengaja atau bocor) |
| G-08 | FE: submit ganda, format tanggal `Dihapus`, gate tombol alamat | Mengunci KI-68 + §7 ui-ux |
