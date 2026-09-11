# Data Model Legacy — Modul 06 Business Party (Customer / Supplier)

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Skema & relasi setingkat
konsep. Presisi mengikuti entity + migrasi (`001_baseline`, `040_member_pricing`,
`044_customer_delivery_addresses`).

---

## 1. Tabel milik modul (2)

### `business_parties` — pelanggan & pemasok satu tabel

Konsep: satu baris = satu pihak; peran dibedakan kolom `party_type` (bukan tabel terpisah).
Lahir di baseline (kode/nama/telepon/email/alamat/catatan/metadata + arsip), lalu 040 menambah
`id_member_type` (nullable, FK member_types) + index-nya. Arsip = soft; restore didukung (satu dari
sedikit modul yang punya — kontras dengan produk/kategori).

Kolom: `id_business_party` (PK) · `id_company` (FK) · `party_type` (customer/supplier) ·
`id_member_type` (nullable, FK; bermakna hanya untuk customer) · `code` (50) · `name` (255) ·
`phone_e164` (30, tanpa validasi format) · `email` (255) · `address_text` (teks bebas) · `notes` ·
`metadata_json` (map string→string; dua nama tulis `attributes`/`attributes_json`) · timestamps +
`archived_at`. Unik: `(id_company, code, party_type)` — **mencakup baris terarsip** (alasan seluruh
aturan Sampah di BR-02/BR-08/BR-21).

### `business_party_delivery_addresses` — buku alamat kirim (044)

Konsep: banyak baris per pelanggan; tepat satu aktif ber-`is_primary` (default order) selama ada
alamat; supplier tidak punya. Lahir di 044 beserta 5 kolom snapshot di `orders` dan backfill
(`address_text` lama → satu `Utama`). Arsip = soft **tanpa restore**.

Kolom: `id_address` (PK) · `id_company` · `id_business_party` (FK) · `label` (120, null) ·
`recipient_name` (150, null) · `phone_e164` (30, null) · `address_text` (wajib) · `is_primary` ·
`sort_order` · timestamps + `archived_at`. Index: `(party, archived)`, `(company)`.

## 2. Relasi (konsep)

```
companies 1──* business_parties (per tipe; unik kode lintas arsip)
member_types 1──* business_parties (via id_member_type; hanya customer; guard arsip di modul 07)
business_parties 1──* business_party_delivery_addresses (hanya customer; 0..1 primer aktif)
business_parties 1──* orders (id_related_party; live by-id, bukan snapshot)
business_party_delivery_addresses 1──* orders (id_ship_to_address opsional + 4 snapshot teks)
business_parties 1──* payments / payment_allocations (idBusinessParty; saldo & ledger milik modul 14)
```

Isolasi perusahaan manual per query (tanpa RLS). Tanpa cascade (semua soft). FK tidak mencegah arsip
yang masih dirujuk — arsip justru dirancang boleh dirujuk (BR-19).

## 3. Jejak baca-tulis per fitur

| Fitur | Baca | Tulis |
|---|---|---|
| Daftar/cari | parties (+join member) + `meta.total` | — |
| Detail/edit | satu pihak + member (by-id) | baris pihak + audit |
| Create | cek kode lintas arsip; max suffix auto; validasi member | baris pihak + audit |
| Restore | baris Sampah + cek bentrok aktif | `archived_at=null` + audit |
| Buku alamat | alamat per pelanggan (primer-dulu) | baris alamat (transaksi bila sentuh primer) + audit |
| Order/POS | via endpoint modul ini (picker server-side, seed instan) | snapshot ship-to di `orders` (ditulis modul Order) |

## 4. Anomali & catatan presisi (untuk arsitek, bukan untuk ditiru)

1. **Unik lintas arsip** adalah keputusan sadar (bukan kelalaian): create me-reserve kode Sampah
   agar pesan ramah + restore auto-code tak pernah bentrok. Sistem baru harus mempertahankan sifat
   ini atau mengganti dengan pesan yang setara baiknya.
2. **`DATETIME(3)` baseline vs presisi 6 di entity** (pola modul 01/05). **[PERLU KONFIRMASI]**
   nilai live sebelum menetapkan presisi baru.
3. **Primer tanpa constraint**: "tepat satu Utama" ditanggung transaksi aplikasi (demote/promote),
   bukan partial-unique index — race bersamaan bisa ganda (seperti KI-64).
4. **Snapshot ganda**: alamat hidup di buku + disalin ke 5 kolom order + `address_text` lama tetap
   ada (tiga representasi; backfill hanya sekali). Pemilik kebenaran per konteks: buku (pilihan),
   snapshot (histori), `address_text` (kontak cepat/kompatibilitas).
5. **`metadata_json` dua nama tulis** (`attributes`/`attributes_json`, menang yang pertama) —
   seragamkan satu nama di kontrak baru.
6. **Restore tanpa kode-edit** (KI-62): skema mengizinkan bentrok yang tak bisa diperbaiki lewat API.
