# Business Rules — Modul 10 Goods Receipt

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Aturan sisi-dokumen; tahap PO,
finansial umum, dan guard = modul 08 (BR-18…BR-38). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Dokumen (BR-01…BR-08)

| ID | Aturan |
|---|---|
| BR-01 | Satu penerimaan: header (order + lokasi-header + SJ trim-or-null + tanggal + catatan trim-or-null + pencatat = user aplikasi) + N baris (order + baris-order + produk + varian + lokasi + qty + base + snapshot UOM + referensi `purchase_order_item`) |
| BR-02 | Lokasi-header = lokasi bila seragam else null; tampil header → item-pertama → `-` |
| BR-03 | Baris tracked ↔ movement 1:1 (`id_inventory_movement` ditulis balik); non-tracked null permanen, tetap tercatat |
| BR-04 | Alias create: `id_order`/`order_id` (wajib), `id_stock_location`/`stock_location_id` (header + baris), `receipt_notes`/`notes`, `id_order_item`/`order_item_id` (wajib), `quantity`/`quantity_received`; lalu delegasi penuh (tanpa logika sendiri) |
| BR-05 | List: cabang + non-arsip (+ order opsional); `receivedAt DESC, id DESC`; limit min(20,100); relasi penuh; `{items, meta}` |
| BR-06 | Detail: id + cabang + non-arsip (+ relasi order); `404 'Penerimaan barang tidak ditemukan'` |
| BR-07 | Tanpa arsip/batal/edit (kolom arsip mati — KI-91); tanpa nomor (KI-89); tanpa cetak/halaman |
| BR-08 | Audit `order.goods_received` tiap penerimaan: before `{status, goodsReceivedAt: null}` + after (nomor, id penerimaan, SJ, jumlah baris, status, tanggal, auto, parsial, aksi-settlement) |

## 2. Sisa & penyelesaian (BR-09…BR-12)

| ID | Aturan |
|---|---|
| BR-09 | Sisa = order − kumulatif (toleransi 0.0001); baris ≤ sisa; kosong → isi-otomatis; tetap kosong → `Tidak ada sisa item yang dapat diterima` |
| BR-10 | Final (semua ≥ qty − 0.0001): tanggal = tanggal-terima + status selesai + history otomatis; parsial: ketiganya tidak |
| BR-11 | Items kosong di request ≠ tanpa sisa: auto-fill dulu, tolak hanya bila tetap kosong |
| BR-12 | Tanpa lock konkuren sisa (beda dengan retur-beli) — E-18 feature (→ KI-92) |

## 3. Stok & lokasi (BR-13…BR-17)

| ID | Aturan |
|---|---|
| BR-13 | Lokasi per baris else header else default-cabang (aktif-daun; 4 pesan lokasi) |
| BR-14 | Saldo (cabang, produk, varian, lokasi) dibuat-0 bila belum ada; `on_hand += base`; `available = on_hand − reserved` |
| BR-15 | Movement `in`/`goods_receipt`, ref = id penerimaan, `movedAt` = tanggal-terima, alasan `Penerimaan barang dari PO {nomor}`, metadata (order, baris, penerimaan, item, qty-UOM, base, faktor) |
| BR-16 | Non-stok: tanpa saldo/movement (tercatat saja) |
| BR-17 | Lokasi default cabang: aktif + daun (dua pesan khusus); induk/nonaktif ditolak per baris |

## 4. Settlement di penerimaan (BR-18…BR-20)

| ID | Aturan |
|---|---|
| BR-18 | `pay_now`: izin bayar (403) + sisa-penuh + metode-tunai-default + nomor `PAY-...` + bukti-opsional-tak-membatalkan + audit bayar sumber-penerimaan |
| BR-19 | `switch_to_net`: termin + tempo (input else lama) + approve + audit approve sumber-penerimaan |
| BR-20 | Mode hanya COD-final-berutang (3 pesan); prabayar-lunas-dulu; respons `settlement_action` + `payment_id/number` + `requires_payment_completion: false` |

## 5. Lintas modul (BR-21…BR-22)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-21 | Stok-masuk + movement + tanggal menjadi dasar saldo, HPP (basis biaya), dan posting | Stock (16), Finance (17) |
| BR-22 | Tanggal-final + status-selesai membuka retur-beli; badge + angka memakai kumulatif | Purchase Return (13), daftar/detail (08) |

## 6. Aturan yang TIDAK ada (verifikasi)

Tidak ada: nomor/edit/batal/cetak/halaman penerimaan; cicilan COD; terima tanpa jalur-selesai;
validasi stok tersedia; lock sisa; cek SJ ganda; batas umur tanggal (backdate bebas — KI-80);
pemanggil alias; spec list/detail/alias.

