# Business Rules — Modul 12 Sales Return (Retur Penjualan)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula,
kondisi khusus. Dari service 1400 baris + controller + FE + E2E 18. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

---

## 1. Kelayakan & baris retur (BR-01…BR-07)

| ID | Aturan |
|---|---|
| BR-01 | Alasan wajib non-kosong (`Alasan retur/tukar wajib diisi`); ≥1 baris (`Minimal satu barang yang dikembalikan harus dipilih`) |
| BR-02 | Order: ada-cabang-non-arsip (`Order asal tidak ditemukan` 404); sales saja (`Retur/tukar hanya berlaku untuk order penjualan`); cabang≈perusahaan (`Cabang tidak sesuai company aktif` 404); sudah-serah ATAU completed (`...harus sudah selesai/diserahkan...`); punya item; urut `lineNo` |
| BR-03 | Baris: unik (`Item retur tidak boleh duplikat`); milik order (`...tidak ditemukan pada order asal`); qty > 0 (`{label} harus lebih dari 0`); ≤ sisa + 0.0001 (`Qty retur '{n}' melebihi sisa yang bisa diretur. Sisa: {s} {u}`); sisa = order − retur-completed (`line_type='returned'` saja) |
| BR-04 | Kondisi ∈ normal/damaged/not_returned (`Kondisi barang retur wajib dipilih`); tracked + normal wajib lokasi (`Lokasi masuk stok wajib dipilih untuk '{n}'`) |
| BR-05 | Nilai retur = proporsional rasio qty × harga/diskon/pajak baris-order (uang rupiah; `lineTotal` baris = total-order × rasio) |
| BR-06 | Lokasi: ada-cabang-non-arsip (`Lokasi stok tidak ditemukan` 404!) + aktif (`...harus aktif`) + daun (`Pilih lokasi stok paling bawah, bukan grup gudang`) |
| BR-07 | Efek stok: normal → lokasi-pilih `in`; damaged → lokasi RUSAK auto (buat `RUSAK`/`Barang Rusak`/9999/`damaged` bila belum ada; paksa status bila ada); not_returned → tanpa gerak; non-tracked → tanpa gerak; saldo buat-0; movement `in`/`sales_return` alasan-`Retur barang dari {nomor}` + `returnUnitCost` (avg-cost else beli) + tanggal-retur |

## 2. Pengganti (BR-08…BR-13)

| ID | Aturan |
|---|---|
| BR-08 | Produk ada-perusahaan-non-arsip (`Produk pengganti tidak ditemukan` 404) + aktif (`'{n}' tidak aktif`); varian pola-order (wajib/aktif/default); qty > 0; harga = input ?? jual ?? 0 (negatif/bukan-angka → `{label} tidak valid`) |
| BR-09 | Harga ≥ minimum bila ada (`Harga jual pengganti '{n}' tidak boleh di bawah harga minimum`) — tanpa pengecualian member |
| BR-10 | Satuan = jual (faktor jual; `|| baseUom`, faktor `|| 1`); pajak mengikuti order (kena + tarif + termasuk); tanpa diskon (0/0) |
| BR-11 | Lokasi keluar: eksplisit (daun-aktif + cukup: `Stok pengganti tidak cukup di lokasi {nama}`) else otomatis (cukup + daun-aktif, urut petik→primer→default→sort; `Stok pengganti tidak cukup di cabang aktif`); kurang saat-keluar → `Stok pengganti '{n}' tidak cukup di lokasi {nama}` |
| BR-12 | Modal-guard (avg-cost else beli, > 0) saat keluar-sekarang (non-tunda) + saat dispatch (tunda); pesan modal-panjang |
| BR-13 | Snapshot penuh (nama/kode/varian/UOM/harga/pajak); `idOriginalOrderItem` null; kondisi null; catatan trim-or-null |

## 3. Selisih & settlement (BR-14…BR-19)

| ID | Aturan |
|---|---|
| BR-14 | `difference = pengganti − retur` (rupiah); total per sisi = subtotal + pajak (masing-masing dibulatkan) |
| BR-15 | Aturan pilih: + (>0.009) → eksplisit-valid else `collect_payment`; − (<−0.009) → eksplisit-valid-kecuali-reduce else `reduce_receivable`; ±0.009 → `none` |
| BR-16 | Negatif: potong = min(\|selisih\|, sisa-terutang-efektif-sebelumnya); eksternal = selisih − potong; eksternal ≤ 0.009 + paksa-non-otomatis → `400 ...belum ada nilai...`; eksternal > 0 + `reduce_receivable` → berpihak ? `customer_credit` : `refund` |
| BR-17 | Paksa + bukan-collect → `400 ...tambah bayar customer`; paksa − bukan-tiga → `400 ...mengurangi piutang, refund, atau saldo customer`; kredit tanpa pihak → `400 Saldo customer hanya bisa dipakai...`; refund tanpa izin → 403 |
| BR-18 | Metode tunai-default hanya collect/refund; referensi/catatan trim-or-null; baris settlement hanya tipe-riil + jumlah > 0 (tanggal = tanggal-retur); `none`/`reduce_receivable` tanpa baris (efek implisit via selisih di query finansial!) |
| BR-19 | **Tanpa baris `payments`** untuk collect/refund (kas di luar sistem; piutang-efektif langsung terkoreksi via efek — → KI baru) |

## 4. Penundaan & SJ pengganti (BR-20…BR-24)

| ID | Aturan |
|---|---|
| BR-20 | Tunda ⟺ exchange + source ≠ pos + ada-pengganti-fisik (fungsi teruji 4 kasus); tunda: tanpa gerak + tanpa guard + lokasi-null + status `pending`; else `not_required` |
| BR-21 | Mode: tanpa-pengganti → `return_only`; else `exchange`. Sumber: pos → `pos` else `sales_delivery`. Status lahir `completed` (selalu!) |
| BR-22 | Dispatch: retur ada + completed (`Retur ini sudah dibatalkan` bila bukan!) + `pending` (`...tidak memiliki barang pengganti yang menunggu dikirim`); sopir/gudang wajib; lokasi per-fisik wajib (`Lokasi pengambilan barang pengganti '{n}' wajib dipilih`); guard modal semua-dulu; tanggal default hari-ini (`YYYY-MM-DD`); SJ `replacement` + nomor-SJ + stok-keluar (movement `out`/`sales_return_exchange` alasan-pengganti) + tulis lokasi+movement ke baris + `dispatched` + audit |
| BR-23 | Confirm: status ∈ 2 (`Status tanda tangan penerima wajib dipilih`); ada + belum-pernah; **signed wajib nama** (`Nama penerima wajib diisi` — ketat vs SJ order!); not_signed wajib alasan; drop opsional; `dispatched` → `delivered` (hanya bila masih dispatched); audit; tanpa arsip-TTD-wajib |
| BR-24 | Nomor: retur `RTR-...` (lock, fallback darurat) + SJ pengganti deret-SJ normal (detail numbering doc) |

## 5. Daftar & DTO (BR-25…BR-27)

| ID | Aturan |
|---|---|
| BR-25 | List: cabang + perusahaan + non-arsip; status-bebas (tanpa validasi nilai!); tanggal mentah (`new Date()` — invalid → ? [PERLU KONFIRMASI] perilaku tanggal-sampah); cari 3 kolom; urut tanggal-DESC + id-DESC; limit jepit 1–50 default 20 (SATU-SATUNYA list dengan batas-bawah!); page ≥ 1 |
| BR-26 | Detail: + order + pihak + item (+lokasi) + settlement-non-arsip + SJ-pengganti + lokasi-daun (hanya bila `pending`); item urut (tipe, nomor); `404 Dokumen retur/tukar tidak ditemukan` |
| BR-27 | Konteks: order layak + sisa-per-baris + lokasi-daun (default→sort→nama); dipakai inisialisasi form (qty = sisa, kondisi normal, lokasi default bila tracked) |

## 6. Lintas modul (BR-28…BR-29)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-28 | Selisih + settlement + tanggal menjadi penyesuaian piutang order (total & bayar-efektif) + sumber posting | Order (08/09), Finance (17) |
| BR-29 | Stok-masuk (RUSAK/normal) + keluar-pengganti + SJ-pengganti + modal-guard menjadi saldo/HPP/arsip | Stock (16), Finance (17), Delivery-cetak (11) |

## 7. Aturan yang TIDAK ada (verifikasi)

Tidak ada: edit/batal/hapus retur; validasi nilai status-list; arsip-TTD pengganti; pajak-input
pengganti; diskon pengganti; pengecualian-minimum-member pengganti; cicilan; baris-payments;
restore; cetak-khusus; batas selisih; tolak pengganti-tanpa-stok-saat-buat-tunda (disengaja).
