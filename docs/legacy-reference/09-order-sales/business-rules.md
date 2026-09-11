# Business Rules — Modul 09 Order Sales

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula, kondisi
khusus sisi sales. Mekanika bersama (uang, pajak, UOM, guard tiga-lapis, daftar, export,
status-config) = modul 08. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Pihak & termin (BR-01…BR-06)

| ID | Aturan |
|---|---|
| BR-01 | Sales non-net boleh tanpa pihak (walk-in null); net wajib customer (`Penjualan tempo wajib memilih customer.`) + HP non-kosong (`...wajib mencatat nomor HP customer.`) |
| BR-02 | Pihak terisi harus customer (pesan menyebut "tempo" walau berlaku umum — → KI baru); tak dikenal/arsip → `Customer tidak ditemukan atau sudah diarsipkan.` |
| BR-03 | Default sales = `cod`; sisi = `receivable`; `payment_term_days`/invoice supplier selalu null (input purchase diabaikan) |
| BR-04 | Ship-to hanya sales: id divalidasi milik + aktif (`Alamat kirim tidak ditemukan.` / `...bukan milik pelanggan ini.`); ad-hoc trim; kosong = fallback cetak |
| BR-05 | Approve-credit: purchase ditolak; sudah-net ditolak; terminal ditolak; tempo wajib (input else lama else 400); hasil net + tempo + approve + audit |
| BR-06 | Tombol Tempo (ghost, non-purchase + non-terminal + non-net); dialog tempo +30-hari-default; toleransi teks tanpa-izin hanya di dialog terima purchase (serah langsung 403 murni) |

## 2. Harga & diskon (BR-07…BR-12)

| ID | Aturan |
|---|---|
| BR-07 | Saran FE: `sellingPrice ?? purchasePrice ?? minSellingPrice ?? 0`; sisi JUAL (satuan + faktor) |
| BR-08 | Guard minimum (create + update): sales + non-`member_rule` + minimum ≠ null + bersih < minimum → `400` dua-nominal id-ID (boleh pas) |
| BR-09 | Diskon: input TOTAL-baris → klem `(penuh − floor) × qty` (floor 0 member else minimum) → simpan per-unit rupiah; tampil total; persen 0–100; total > harga ditolak |
| BR-10 | Member: timpa penuh + sebelum-diskon; `priceEdited` lestari; requote klem-ulang; quote-error blokir submit (sales + customer) |
| BR-11 | Edit: harga-penuh + `priceEdited` (anti-ganda); ganti jenis saran-ulang kecuali manual |
| BR-12 | Harga cetak = penuh (`max(sebelum, bersih+diskon, basis-member-bila-lebih-tinggi)`); Disc tampil bila ada-diskon-baris; sembunyi → bruto + sisa-riil |

## 3. Status sales (BR-13…BR-17)

| ID | Aturan |
|---|---|
| BR-13 | Selesai wajib serah: manual pre-serah ditolak (`Order manual tidak boleh langsung selesai tanpa proses serah barang.`); semua pre-serah ditolak (`Sales order harus diserahkan terlebih dahulu sebelum selesai.`); pengecualian source-pos lapis-1 **mati efektif** oleh lapis-2 (→ KI baru) |
| BR-14 | Pasca-serah hanya ke selesai; aktifkan-prepaid-berutang ditolak (`...harus lunas sebelum dapat diproses.` — seksi bayar tampil sejak Draft) |
| BR-15 | Selesai-terminal prabayar/COD berutang ditolak (2 pesan); net bebas |
| BR-16 | FE menyembunyikan: selesai pre-serah, non-selesai pasca-serah, selesai berutang, Terbitkan-SJ pre-lunas-prepaid |
| BR-17 | Seed + transisi = modul 08 (kind `all`; cari awal kind lalu `all`) |

## 4. Serah langsung (BR-18…BR-24)

| ID | Aturan |
|---|---|
| BR-18 | Sekali tembak: sales + belum-pernah + non-terminal + punya-item + status-selesai + (transisi aktif kecuali pos) — 6 pesan |
| BR-19 | Prabayar lunas-dulu; COD-berutang wajib mode; mode hanya COD (pesan sama purchase) |
| BR-20 | `pay_now`: izin bayar (403 customer); sisa-penuh; metode default tunai; tunai: bulat + ≥ tagihan else 400; non-tunai/kosong = null; nomor `PAY-...`; sisi receivable; bukti opsional tak membatalkan |
| BR-21 | `switch_to_net`: termin + tempo (input else lama) + approve + audit sumber-serah |
| BR-22 | Stok: tracked per baris via alokasi (+ reservasi bila kunci); non-tracked lewati; tanggal = eksplisit ?? tanggal-bayar ?? kini; selalu selesai + history otomatis + audit `order.goods_delivered`; respons cermin-receive |
| BR-23 | `amount_tendered` hanya tunai-serah-langsung (kembalian tampil, tak tersimpan) |
| BR-24 | POS memakai jalur ini (E2E 02: `deliver-goods` + `pay_now` tunai) — alasan pengecualian transisi pos |

## 5. Bayar & cetak (BR-25…BR-30)

| ID | Aturan |
|---|---|
| BR-25 | Seksi tampil: non-pending ATAU (sales + prepaid) — Draft prepaid membayar dulu |
| BR-26 | Form bayar: tanggal* kini-default; jumlah bulat (pecahan dibuang, bukan digabung); referensi auto `REF-YYYYMMDD-XXXX` bisa-ubah; bukti JPG/PNG 5MB kompres; gagal-bukti tak membatalkan |
| BR-27 | Hapus bayar disembunyikan bila terminal + non-net (backend menolak — milik 14); hapus = arsip + toast + dialog tak-batal |
| BR-28 | Judul nota = status bayar (Lunas/Sebagian/Tagihan); tombol detail = kebalikannya; jatuh-tempo `-` bila lunas/tanpa; Staff = pembuat-mentah; USER = pencetak |
| BR-29 | Opsi cetak via URL (kertas/preprinted/diskon) + Notes-hanya-cetak + ganjal-A4-6 + tanpa-ganjal-kontinu |
| BR-30 | Bukti bayar: satu pembayaran + sisa-sesudah; tanpa tanda tangan (disengaja) |

## 6. Aturan lintas modul (BR-31…BR-33)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-31 | Serah mengurangi stok base + movement alasan-SO + tanggal-serah (dashboard net-sales memakai tanggal ini) | Stock (16), Dashboard (18) |
| BR-32 | Bayar/tempo/snapshot menjadi sumber posting + saldo | Payment (14), Finance (17) |
| BR-33 | Serah-penuh membuka retur/tukar; SJ jalan bila dipakai (modul 11/12 membaca `goodsDeliveredAt`) | Delivery (11), Sales Return (12) |

## 7. Aturan yang TIDAK ada (verifikasi)

Tidak ada: serah parsial langsung; kembalian tersimpan; cicilan COD; tempo tanpa tanggal;
member/purchase-diskon di sales (ada — kebalikan purchase); minimum untuk member; tanda tangan
cetak; batas tender atas; tolak referensi ganda; arsip bayar terminal-net (boleh — hanya
non-net disembunyikan).
