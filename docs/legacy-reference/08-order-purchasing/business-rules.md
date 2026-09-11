# Business Rules — Modul 08 Order Purchasing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula, kondisi
khusus. Dari `order.service.ts`, `goods-receipt.service.ts`, `order-pricing.service.ts`,
`order-status.service.ts`, form FE, E2E 05. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Jenis & pihak (BR-01…BR-05)

| ID | Aturan |
|---|---|
| BR-01 | `order_kind` ∈ {sales, purchase} (create, update, filter list) else `400 'order_kind hanya mendukung sales atau purchase.'` |
| BR-02 | PO wajib supplier: tanpa pihak → `400 'Purchase order wajib memilih supplier.'`; tipe ≠ supplier → `400 'Pihak terkait untuk purchase order harus bertipe supplier, bukan {t}.'`; tak dikenal/arsip → `400 'Supplier tidak ditemukan atau sudah diarsipkan.'` (berlaku create + setiap update yang menyentuh pihak/jenis/termin) |
| BR-03 | Status awal = cocok-kind-awal lalu `all`-awal; tanpa keduanya → `400 'Status awal order belum dikonfigurasi'` |
| BR-04 | `source`: `'pos'` bila dikirim pos else `manual` (PO selalu manual via UI) |
| BR-05 | Update boleh ganti `order_kind` (pihak/termin divalidasi ulang terhadap jenis baru); tanpa pihak saat jadi purchase → ditolak |

## 2. Finansial PO (BR-06…BR-12)

| ID | Aturan |
|---|---|
| BR-06 | Termin default purchase = `net`; tersimpan selalu eksplisit (prepaid/cod/net); sisi = `payable` |
| BR-07 | `payment_term_days`: hanya bila net + purchase (else null, input diabaikan); integer 0–3650 else `400 'Termin hari pembayaran harus berupa angka 0 sampai 3650.'`; FE default 30 |
| BR-08 | Jatuh tempo: non-net → selalu null (dibersihkan); net → input eksplisit, else nilai lama (update), else invoice + hari (purchase), else `400 'Jatuh tempo wajib diisi untuk termin tempo.'`. Tampil efektif hanya bila net (`getEffectiveDueDate`, list/detail menimpa field) |
| BR-09 | Invoice supplier (nomor trim-or-null ≤100 + tanggal) bebas; FE: tanggal invoice + hari menghitung jatuh tempo otomatis |
| BR-10 | Pajak: `taxable` hanya bila true eksplisit; tarif 0–100 else `400 'Tarif pajak harus bernilai 0 sampai 100.'`; `priceIncludesTax` hanya bila taxable; per baris: kena = baris ?? order, tarif = baris ?? order |
| BR-11 | Uang ≥ 0 (`'{label} harus bernilai 0 atau lebih.'`); semua uang → rupiah utuh (`roundRupiah`); persen & tarif → 4 desimal |
| BR-12 | Rumus baris: `total_sebelum_diskon = penuh × qty`; `diskon_total = perunit × qty` (purchase selalu 0); `bersih = (penuh − perunit) × qty`; kena & tarif>0: termasuk → `dasar = bersih/(1+r)`, `pajak = bersih − dasar`, `total = bersih`; ditambah → `pajak = bersih×r`, `total = bersih + pajak`. Order = jumlah baris; `total_amount = subtotal_after_tax` |

## 3. Baris & snapshot (BR-13…BR-17)

| ID | Aturan |
|---|---|
| BR-13 | Produk wajib ada (`404 'Produk ID {id} tidak ditemukan'`); qty > 0; varian: eksplisit harus ada+aktif, produk bervarian wajib pilih, else default-aktif (4 pesan §E-07) |
| BR-14 | Sisi BELI: satuan = purchaseUom, faktor = purchaseToBaseFactor (ternormalkan), qty-base = `toBaseQuantity`; harga selalu manual (tanpa member, tanpa guard minimum, tanpa diskon) |
| BR-15 | Snapshot per baris (final): produk (nama/kode), varian (null bila hidden), qty + UOM ganda + base, pricing (manual + 7 kolom null), harga penuh/bersih/diskon/persen, pajak (kena/tarif/dasar/nilai), total, `stockTrackedSnapshot`, alasan diskon (null), catatan. `lineNo` ulang dari 1 tiap tulis |
| BR-16 | Update baris = hapus + tulis ulang (bukan delta); syarat: items non-kosong + lolos guard gerak/bayar/finance (BR-18); header tanpa items selalu boleh (non-terminal) |
| BR-17 | Saran FE: `purchasePrice ?? sellingPrice ?? minSellingPrice ?? 0`; ganti jenis memakai saran baru kecuali baris manual |

## 4. Guard perubahan (BR-18…BR-20)

| ID | Aturan |
|---|---|
| BR-18 | Ubah-baris / batal / arsip diblokir bila: movement (`referenceType='order'`) ATAU bayar aktif ATAU sumber finance (`posted/ready/pending/failed/reversed`) — 3 pesan `/diubah barangnya\|dibatalkan\|diarsipkan/` yang menunjuk retur/adjustment/refund/reversal |
| BR-19 | Terminal (`isTerminal`) menolak update apa pun + pindah status (`Order terminal tidak bisa diubah` / `Order sudah berada di status terminal`) |
| BR-20 | Audit: create `{orderNumber}`; update before+after 8 field header (tanpa baris! — perubahan baris tak tercatat rinci); status `{dari→ke}`; arsip id; terima kaya (§F-04.5) |

## 5. Pindah status purchase (BR-21…BR-24)

| ID | Aturan |
|---|---|
| BR-21 | Target wajib transisi aktif dari status kini (`Transisi status tidak diizinkan`); history selalu ditulis (dari, ke, pengubah, alasan-or-null, kini) |
| BR-22 | Selesai wajib `goodsReceivedAt` (`Purchase order tidak dapat diselesaikan sebelum barang diterima`); pasca-terima hanya ke selesai (`...hanya dapat dilanjutkan ke status selesai.`) — FE menyembunyikan tombol yang sama |
| BR-23 | Grup batal: lolos guard BR-18; grup selesai-terminal: prabayar/COD wajib lunas (`Order dengan termin prabayar harus lunas sebelum diselesaikan.` / `...bayar saat serah...`), net bebas |
| BR-24 | Seed bawaan (perusahaan baru): 5 status `all` (Draft/pending/awal → Dikonfirmasi/Diproses/active → Selesai/completed/terminal → Dibatalkan/cancelled/terminal) + 6 transisi (Konfirmasi, Mulai Proses, Selesaikan, Batalkan ×3). Status custom via `/settings/order-status` (tambah = Menunggu non-terminal — KI-35; kode unik; kind ∈ sales/purchase/all) |

## 6. Penerimaan (BR-25…BR-34)

| ID | Aturan |
|---|---|
| BR-25 | Syarat tahap: purchase + non-terminal + punya item + status-selesai terkonfigurasi + transisi aktif kini→selesai (`Purchase order belum berada pada tahap penerimaan barang.`) |
| BR-26 | Baris: qty > 0; tanpa duplikat item; milik order ini; ≤ sisa + 0.0001 (`...melebihi sisa...`); kosong → isi-otomatis sisa; tetap kosong → `Tidak ada sisa item...` |
| BR-27 | Prabayar wajib lunas (`...harus lunas sebelum barang diterima.`); `settlement_mode` hanya COD; hanya penerimaan terakhir; final-COD-berutang wajib pilih (`...harus memilih bayar saat terima atau diubah ke tempo...`) |
| BR-28 | `pay_now`: butuh `payment.create` (403 khusus); bayar = sisa penuh (bukan cicilan), metode default tunai, nomor `PAY-...`; `switch_to_net`: termin net + jatuh tempo (input else lama) + `creditApproved` + audit approve_credit sumber penerimaan |
| BR-29 | Lokasi: per baris else header else default cabang (aktif-daun; induk/nonaktif ditolak); header tercatat hanya bila seragam; non-stok tanpa movement |
| BR-30 | Stok tracked: saldo dibuat bila belum ada; `on_hand += base`, `available = on_hand − reserved`; movement `in`/`goods_receipt` (ref = id penerimaan, `movedAt` = tanggal terima, alasan `Penerimaan barang dari PO {nomor}`, metadata order/item/penerimaan/konversi) |
| BR-31 | Final: `goodsReceivedAt` = tanggal terima + status selesai + history otomatis; parsial: ketiganya tidak (tanggal tetap null) |
| BR-32 | Alias `goods-receipts/create`: `order_id`, `stock_location_id` (header+baris), `notes`→catatan, `order_item_id`, `quantity_received`; `id_order` wajib (`Order penerimaan wajib diisi`), item wajib (`Item order penerimaan wajib diisi`) |
| BR-33 | Daftar penerimaan: cabang + arsip-null, saring order opsional, `receivedAt DESC, id DESC`, limit min(20,100); detail + relasi (item, lokasi, penerima, order), `404 'Penerimaan barang tidak ditemukan'` |
| BR-34 | Respons: `auto_completed` ≡ `fully_received`; `requires_payment_completion` selalu false; `settlement_action` ∈ auto_completed/paid_now/switched_to_net; `payment_id/number` bila bayar |

## 7. Daftar & angka (BR-35…BR-38)

| ID | Aturan |
|---|---|
| BR-35 | Daftar: cabang + non-arsip; cari frasa (nomor + 4 identitas pihak); saring grup status/id status/jenis/termin/tanggal/pihak; urut tanggal DESC; limit min(20,100); `summary_only` mematikan item/riwayat + peta receipt/return |
| BR-36 | Dibayar = bayar-langsung + alokasi non-arsip; retur completed penyesuaian total (`−returned_total`) + pengurang bayar (refund/supplier_credit); terutang = max(0, total − bayar); status bayar unpaid/partial/paid; status finansial settled/overdue (net + tempo lewat + berutang)/partial/open; `dueDate` tampil = efektif-net-else-null |
| BR-37 | Badge retur purchase: ada retur completed → sebagian; ada `cancels_order` → penuh (satu query per halaman; sales dilewati; daftar summary mematikan — E-30) |
| BR-38 | `createdBy` dibuang dari respons (hindari bocor password/hash) → hanya `createdByName` (Dibuat Oleh / Staff SJ); export: 366 hari, 5000 order, tanggal `YYYY-MM-DD`, file `report-order_{jenis}_{dari}_{sampai}.{ext}` |

## 8. Aturan lintas modul (BR-39…BR-41)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-39 | Terima menambah stok base + movement alasan PO; saldo per (cabang, produk, varian, lokasi) | Stock (16) |
| BR-40 | Bayar COD + jatuh tempo net + snapshot pajak/UOM menjadi sumber posting | Payment (14), Finance (17) |
| BR-41 | Retur penuh (`cancels_order`) → status batal via `findCancelledStatusForOrder` (kind lalu `all`, null bila tak ada) | Purchase Return (13) |

## 9. Aturan yang TIDAK ada (verifikasi)

Tidak ada: hapus permanen; edit/batal penerimaan; restore PO; diskon/guard-minimum/member di PO;
cicilan COD di penerimaan (selalu sisa penuh); terima tanpa jalur ke selesai; nomor penerimaan;
pindah cabang; validasi stok tersedia saat terima (selalu masuk); batas qty selain sisa; batas
umur jatuh tempo; tolak invoice duplikat (nomor invoice supplier bebas ganda — [PERLU KONFIRMASI]
disengaja?).
