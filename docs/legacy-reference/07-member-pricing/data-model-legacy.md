# Data Model Legacy — Modul 07 Member Type & Member Pricing

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi `040_member_pricing`.

---

## 1. Tabel milik modul (1)

### `member_types` — rule harga global per perusahaan

Konsep: satu baris = satu jenis member = satu rule untuk **semua** produk. Tanpa periode, tanpa
cabang, tanpa pengecualian produk. Arsip = soft **tanpa restore**; status aktif/nonaktif terpisah
dari arsip (guard penetapan membaca keduanya: nonaktif ATAU terarsip = tak bisa dipakai baru).

Kolom: `id_member_type` (PK) · `id_company` (FK) · `code` (50, uppercase) · `name` (120) ·
`description_text` · `price_basis` (purchase/min_selling/selling) · `price_adjustment_direction`
(add/subtract) · `price_adjustment_type` (nominal/percent) · `price_adjustment_value`
(`DECIMAL(18,4)`, ≥0) · `rounding_mode` (none/up/nearest/down) · `rounding_increment`
(`DECIMAL(18,2)`, null) · `status` (active/inactive) · timestamps + `archived_at`. Unik:
`(id_company, code)` — **mencakup arsip** (akar KI-71); index `(company, status, archived)`.

## 2. Relasi (konsep)

```
companies 1──* member_types (unik kode lintas arsip)
member_types 1──* business_parties (via id_member_type; hanya customer; guard arsip di sini)
member_types ──snapshot──> order_items (7 kolom: source, id, nama, basis, arah, tipe, nilai, basis_price)
products ──basis──> quote (selling/min/purchase + faktor UOM; tanpa FK, baca live saat hitung)
```

Tanpa tabel histori rule; tanpa tabel pengecualian; `order_items.pricing_source` default `'manual'`
untuk baris pra-040. Isolasi perusahaan manual; tanpa cascade.

## 3. Jejak baca-tulis

| Fitur | Baca | Tulis |
|---|---|---|
| Daftar | rule non-arsip + `meta.total` | — |
| Create/update/archive | cek kode; hitung pemakai aktif (guard) | baris rule + audit snapshot penuh |
| Quote | rule (+ relasi member pihak) + produk | — (read-only, tanpa audit) |
| Order/POS | rule live per baris | 7 snapshot per `order_items` (ditulis modul Order) |

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Unik mencakup arsip** tanpa pesan ramah (cek create hanya non-arsip) → 500. Pertahankan sifat
   anti-pakai-ulang atau tambah pesan + reserve seperti pihak.
2. **`DATETIME(6)` sejak lahir** (040 sudah presisi 6 — beda dengan baseline modul 01/05/06).
3. **Primer invarian di aplikasi** (guard hitung, bukan constraint) — race langka dapat mengarsip
   saat penetapan bersamaan.
4. **Dua representasi enum** (snake DB vs camel entity) + dua nama deskripsi (`description`/
   `description_text`) — seragamkan di kontrak baru.
