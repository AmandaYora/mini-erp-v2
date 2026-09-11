# Data Model Legacy — Modul 08 Order Purchasing

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi (`001`, `006`–`008`, `010`–`011`, `016`, `018`, `041`, `049`, `036`).

---

## 1. Tabel milik modul (7)

### `orders` — header order cabang

Konsep: satu baris = satu PO/SO cabang. Finansial tersimpan final (subtotal, diskon, pajak,
total); bayar dihitung live (bukan kolom); retur menyesuaikan tampilan (bukan kolom).
`dueDate` tersimpan selalu tetapi bermakna hanya bila net. `paymentTerms` kolom + logika
warisan `creditApproved` (resolve: kolom valid → kolom; else approved → net; else default).
Arsip = soft tanpa restore.

Kolom: `id_order` · `id_branch` · `order_number` (100) · `order_kind` · `source`
(manual/pos) · `id_related_party` (null) · ship-to 5 kolom (purchase selalu kosong) ·
`id_current_status` · `id_assigned_user` (null) · `order_date` · `due_date` (null) ·
`supplier_invoice_number` (100)/`supplier_invoice_date`/`payment_term_days` (pembelian) ·
`payment_terms` (default cod di kolom!) · uang (subtotal, diskon, taxable, includes, rate,
before, tax, after, faktur no/tgl, total) · `notes` · `metadata_json` · `id_created_by` ·
timestamps · `goods_received_at`/`goods_delivered_at` (null) · `creditApproved/By/At` ·
`archived_at`. Relasi: cabang, pihak, status, assigned, creator, items (cascade), history.

### `order_items` — baris + snapshot final

Konsep: satu baris = satu produk × qty dalam UOM transaksi + seluruh jejak (produk, varian,
UOM ganda + base, pricing 7 kolom, harga penuh/bersih/diskon/persen, pajak, total,
`stockTrackedSnapshot`, alasan, catatan). Ditulis-utuh tiap simpan (`lineNo` ulang);
tanpa arsip per baris (hidup-mati ikut header). Detail kolom di business-rules modul 05/07
dan §BR-15.

### Status (3): `order_status_definitions`, `order_status_transitions`, `order_status_history`

Definisi: kode unik perusahaan + label + grup (pending/active/completed/cancelled) +
kind (sales/purchase/all) + awal/terminal + urut + warna. Transisi: dari → ke + label +
aktif (tanpa cek siklus/duplikat di API). History: order + cabang + dari(null di awal)/ke +
pengubah + alasan + waktu (tambah-tulis; tanpa ubah).

### Penerimaan (2): `goods_receipts`, `goods_receipt_items`

Header: perusahaan + cabang + order + lokasi (null bila campur) + no SJ supplier (100) +
`received_at` + catatan + pencatat + timestamps + arsip (kolom ada, tanpa endpoint).
Item: penerimaan + order + baris-order + produk + varian + lokasi (null) + qty + qty-base +
snapshot UOM + referensi sumber (`purchase_order_item` + id baris) + movement (null untuk
non-stok). Tanpa status/total sendiri (turunan order).

## 2. Relasi (konsep)

```
branches 1──* orders ──* order_items (snapshot; varian wajib)
orders *──1 status_definitions; *──* via transitions; 1──* history
orders 1──* goods_receipts ──* goods_receipt_items (→ movement → balance)
orders 1──* payments / allocations (modul 14; dihitung live)
orders 1──* purchase_returns completed (modul 13; badge + penyesuaian angka)
business_parties 1──* orders (wajib supplier u/ purchase; live by-id)
branch_document_sequences ──kunci──> nomor PO/PB + nomor bayar COD
```

Cabang = batas (seluruh query purchase melingkup `id_branch` + guard); perusahaan = batas
(nomor urut, status config, arsip lintas-semua-baris untuk kode pihak — bukan order).

## 3. Jejak baca-tulis

| Fitur | Baca | Tulis |
|---|---|---|
| Daftar | order + status + pihak + creator (+ agregat bayar + retur batch) | — |
| Buat/ubah | status awal, pihak+member, produk+varian, sequence | order + items (transaksi) + history(buat) + audit |
| Pindah status | transisi + target + ringkasan bayar | status + history + audit |
| Terima | order+item, penerimaan lama (sisa), status-selesai, bayar, lokasi | penerimaan + item + saldo + movement (+ bayar / net) + status + history + 1–3 audit (transaksi) |
| Export | agregat read-only batas 5000 | berkas (tanpa tulis DB) |

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Kolom `payment_terms` default `cod`** tetapi default logis purchase = net (resolve menimpa
   bila kolom invalid/kosong — kolom tak pernah kosong praktiknya; warisan vs niat).
2. **`summary_only` mematikan badge** yang UI-nya ada (E-30) — API tahu, FE tak tahu.
3. **Arsip penerimaan ada kolom, tanpa endpoint** — mati setengah (beda dengan order).
4. **Update baris hapus-tulis** (bukan delta) — `lineNo` dan id baris tak stabil antar edit
   (rujukan luar ke `id_order_item` — mis. penerimaan — menunjuk baris lama pasca-edit;
   penerimaan lama tetap valid karena menyimpan id-nya sendiri).
5. **Transisi tanpa cek siklus/duplikat/keberadaan status** (KI-31 warisan) — graph bisa rusak
   via API.
6. **`DATETIME(6)` konsisten** di tabel modul ini (lebih baru dari baseline pihak/produk).
