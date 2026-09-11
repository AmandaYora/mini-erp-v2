# Feature Inventory — Modul 10 Goods Receipt (Penerimaan Barang)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode (`goods-receipt.service.ts`, `order.controller.ts` terkait, entity, migrasi 018/036,
dialog + kartu di `order-detail-page.tsx`, slice, E2E 05). Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = **dokumen penerimaan**: satu penerimaan = satu header
(`goods_receipts`) + N baris (`goods_receipt_items`) + efek stok + keputusan COD final.
Mekanika PO (buat/ubah/status/nomor) milik modul 08 dan hanya dirujuk; dialog Terima Barang
didokumentasikan sisi-dokumen di sini (spesifikasi widget penuh di modul 08).

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 4 — `orders/receive-goods` (pintu operasional, dipakai FE), `goods-receipts/{list,detail,create}` (API dokumen; `create` = alias kompatibel **tanpa pemanggil** — → KI-88) |
| Halaman | **Tanpa halaman sendiri.** Tampil di `/orders/:orderId`: dialog Terima Barang + kartu Riwayat Penerimaan Barang |
| Permission | `order.view` (list/detail penerimaan), `order.update` (terima + alias create). COD `pay_now` menuntut `payment.create` di sesi (403 khusus) |
| Tabel yang dimiliki | `goods_receipts`, `goods_receipt_items` |
| Tabel yang ditulis | `inventory_balances` (+), `inventory_movements` (`in`/`goods_receipt`), `payments` (COD bayar-kini), `orders` (tanggal + status final), `order_status_history`, `branch_document_sequences` (nomor bayar) |
| Penomoran | **Tidak ada nomor penerimaan** — identitas = id internal + no SJ supplier (opsional) + tanggal. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | `order.goods_received` (kaya, tiap penerimaan) + `payment.create` (sumber `receive_goods_cod`) + `order.approve_credit` (sumber `receive_goods_cod`, bila alih-tempo) |
| Aksi yang TIDAK ada | Edit/hapus/batal penerimaan; restore; cetak dokumen penerimaan; nomor penerimaan; halaman daftar penerimaan; terima tanpa PO; terima tanpa jalur ke selesai |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Dokumen penerimaan (header + baris)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Header | id + perusahaan + cabang + id order + lokasi-header (null bila baris campur) + no SJ supplier (trim-or-null, ≤100) + `received_at` + catatan (trim-or-null) + pencatat (`id_received_by` = user aplikasi, bukan pekerja gudang — lihat modul 08) + timestamps + `archived_at` (kolom ada, tanpa endpoint — → KI-91) |
| F-01.2 | Baris | id penerimaan + id order + id baris-order + produk + varian + lokasi + qty (transaksi) + qty-base + snapshot UOM ganda + referensi sumber (`purchase_order_item` + id baris) + id movement (null untuk non-stok) |
| F-01.3 | Lokasi-header | = lokasi bila seluruh baris seragam, else null (aturan tampil: header else item-pertama else `-`) |
| F-01.4 | Keterkaitan movement | Baris tracked menunjuk movement-nya (`id_inventory_movement` ditulis balik setelah movement tersimpan); baris non-stok null permanen |

### F-02 — Terima via `orders/receive-goods` (pintu operasional)

Satu transaksi: validasi tahap → hitung sisa kumulatif → keputusan COD (bila final-berutang) →
tulis penerimaan + stok + movement → (bayar / alih-tempo) → finalisasi order → 1–3 audit →
respons. Aturan validasi lengkap = modul 08 (E-15…E-23, BR-25…BR-34); sisi-dokumen yang mengikat
di sini: items kosong = isi-otomatis sisa; tetap kosong = `Tidak ada sisa item yang dapat
diterima`; final = tanggal + selesai + history otomatis; parsial = ketiganya tidak;
`requires_payment_completion` selalu false; `auto_completed` ≡ `fully_received`.

### F-03 — API dokumen `goods-receipts/*` (kompatibilitas)

| Endpoint | Perilaku | Pemakai terverifikasi |
|---|---|---|
| `list` (`order.view`) | Saring cabang (+ order opsional), non-arsip, relasi (item, lokasi, pencatat), urut `receivedAt DESC, id DESC`, limit min(20,100), `{items, meta}` | Hanya E2E 05 (verifikasi ≥2 batch). **FE tidak memakai** (riwayat dibaca via `orders/detail`) |
| `detail` (`order.view`) | By id + cabang + non-arsip + relasi (+ order); `404 'Penerimaan barang tidak ditemukan'` | **Tanpa pemanggil** (FE/E2E/helper nol) |
| `create` (`order.update`) | Alias: `order_id`, `stock_location_id` (header+baris), `notes`→catatan, `order_item_id`, `quantity_received`; wajib (`Order penerimaan wajib diisi`, `Item order penerimaan wajib diisi`) → delegasi penuh ke `receiveGoods` | **Tanpa pemanggil** + tanpa spec (→ KI-88) |

### F-04 — Efek stok per baris

Hanya baris `stockTrackedSnapshot`: saldo dibuat bila belum ada (0/0/0); `on_hand += base`;
`available = on_hand − reserved` (reservasi tak tersentuh penerimaan); movement `in` /
`goods_receipt` (ref = id penerimaan, `movedAt` = tanggal terima, alasan `Penerimaan barang dari
PO {nomor}`, metadata order/baris/penerimaan/konversi). Non-stok: tercatat, tanpa saldo/movement.

### F-05 — Keputusan COD di penerimaan terakhir

`pay_now` (butuh `payment.create`; bayar = sisa penuh; metode default tunai; nomor `PAY-...`;
bukti opsional tak membatalkan) vs `switch_to_net` (termin net + tempo + approve + audit
sumber-penerimaan). Di luar COD-final: mode ditolak (2 pesan); final-COD-berutang tanpa mode
ditolak. Detail = modul 08 (BR-27…BR-28, UF-05).

### F-06 — Kartu Riwayat Penerimaan Barang (di detail PO)

Kosong: `Belum ada penerimaan` / `Purchase order ini belum memiliki batch penerimaan barang.`;
tabel Waktu (+`oleh {nama}`) · Dokumen Supplier (`-`) · Lokasi (header → item-pertama → `-`) ·
Item Diterima (`{nama}: {qty} {uom}` per baris) · Catatan (`-`). Tanpa paginasi (seluruh batch
satu PO), tanpa aksi per baris, tanpa cetak.

### F-07 — Dialog Terima Barang (ringkas; widget penuh di modul 08 §F-04/ui-ux §3)

Badge `Terima Barang` + deskripsi dinamis (net vs COD-keputusan) + SJ supplier (`Opsional`) +
Tanggal Terima + Lokasi-cepat (required) + baris (qty `max=sisa`, dijepit UI; lokasi per baris)
+ kotak `Total item purchase order` (tracked) + catatan + keputusan COD (kondisional) + label
konfirmasi 4 varian + penguncian 3 syarat. Toast sukses 5 deskripsi / gagal + pesan (slice).

---

## 3. Edge Case (18, sisi-dokumen; validasi tahap lengkap di modul 08)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Alias tanpa `id_order`/`order_id` | `400 'Order penerimaan wajib diisi'` |
| E-02 | Alias baris tanpa id item | `400 'Item order penerimaan wajib diisi'` |
| E-03 | `quantity_received` vs `quantity` | `quantity ?? quantity_received` → `Number()` (NaN → ditolak `Quantity receipt harus lebih dari 0` di inti) |
| E-04 | `notes` vs `receipt_notes` | `receipt_notes ?? notes` → trim-or-null |
| E-05 | Lokasi header vs baris vs tanpa | Baris ?? header; tanpa keduanya → default cabang (wajib daun-aktif) |
| E-06 | Seluruh baris satu lokasi | Header = lokasi itu; campur → null |
| E-07 | Baris non-stok | Header/item tercatat; tanpa saldo/movement; `id_inventory_movement` null selamanya |
| E-08 | Saldo belum ada di (cabang, produk, varian, lokasi) | Dibuat 0/0/0 lalu ditambah (bukan error) |
| E-09 | Reservasi menumpuk di lokasi | `available` berkurang reservasi; penerimaan tak menambah `reserved` |
| E-10 | Tanggal terima masa lalu | Diterima: `received_at`, `goodsReceivedAt` (bila final), `movedAt`, history penyelesaian (→ KI-80 modul 08) |
| E-11 | SJ supplier ganda antar penerimaan | Diizinkan (fakta lapangan: satu faktur/surat gabungan — E2E 05 mencatat di notes) |
| E-12 | Catatan/SJ whitespace | null (trim-or-null), bukan string kosong |
| E-13 | `goods-receipts/list` tanpa `id_order` | Seluruh cabang (paginasi; relasi penuh) — dipakai E2E dengan `id_order` |
| E-14 | Detail id tak dikenal / cabang lain / (terarsip — tidak terjangkau, tanpa endpoint arsip) | `404 'Penerimaan barang tidak ditemukan'` |
| E-15 | Terima saat status terminal / tanpa item / tanpa status-selesai / tanpa transisi | 4 pesan tahap (modul 08 E-15) — dialog FE tak memeriksa tahap (hanya prepaid-lunas); API yang menolak |
| E-16 | Bukti bayar COD gagal | Penerimaan + pembayaran tetap sah |
| E-17 | `payment.create` hilang di tengah (sesi berubah) | 403 khusus supplier (403, bukan 400) |
| E-18 | Dua terima bersamaan melebihi sisa | Tanpa lock baris-terima (beda dengan retur-beli yang `pessimistic_write`): risiko over-receipt konkuren — [PERLU KONFIRMASI] pernah terjadi? (→ KI-92) |

---

## 4. Katalog Pesan (sisi-dokumen; tahap + PO di modul 08 §4)

`'Order penerimaan wajib diisi'` · `'Item order penerimaan wajib diisi'` ·
`'Penerimaan barang tidak ditemukan'` · `'Quantity receipt harus lebih dari 0'` ·
`'Item receipt tidak boleh duplikat'` · `'Item order {id} tidak ditemukan pada order ini'` ·
`"Quantity receipt untuk {n} melebihi sisa yang belum diterima"` ·
`'Tidak ada sisa item yang dapat diterima'` · `'Lokasi stok default belum dikonfigurasi untuk cabang ini'`
(+ varian daun/aktif) · toast `Barang berhasil diterima` + 5 deskripsi · `Gagal menerima barang`.

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Nomor penerimaan | Tanpa sequence/kolom nomor (identitas = id + SJ + tanggal) |
| NF-02 | Edit/hapus/batal penerimaan | Tanpa endpoint (kolom arsip mati — KI-91) |
| NF-03 | Halaman/cetak penerimaan | Hanya kartu riwayat; tanpa rute cetak |
| NF-04 | Pemanggil alias create + pemakai list/detail di FE | Grep nol (KI-88); tanpa spec ketiganya |
| NF-05 | Alokasi `stock_issue_allocations` di terima | Hanya sisi serah (delivery); terima menulis movement langsung |
| NF-06 | Cicilan COD / terima tanpa jalur selesai | Aturan inti (modul 08 NF-07/08) |
| NF-07 | Validasi stok tersedia | Selalu masuk (terima = tambah) |
| NF-08 | Lock konkuren sisa | Berbeda dengan retur-beli (E-18) |

