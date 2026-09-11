# Business Rules — Modul 15 POS / Kasir

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula,
kondisi khusus kasir. Mekanika order/bayar/member/stok milik 07/09/14/16; di sini aturan
kasirnya. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Katalog & stok-tampil (BR-01…BR-06)

| ID | Aturan |
|---|---|
| BR-01 | Daftar: `status=active` + cari + kategori + stok-tersedia-dulu, 36/halaman tambah-gulir (observer 320px, request-seq, gagal-mode-ganti/append, meta-tanpa-total-diturunkan) |
| BR-02 | Saldo: produk tampil + cart, 100/batch `include_location_breakdown`, 60-detik, gagal = lama; sinkronisasi cart per kunci (tak-berubah = array-sama!) |
| BR-03 | Tampil UOM-jual (dual bila beda); badge: Habis-merah / kritikal (≤ minimum-base!) / sukses; tanpa-badge non-tracked |
| BR-04 | Batas: tracked = tersedia (0 = tanpa-tambah) else ∞; varian per-kunci; scan-induk → pilih |
| BR-05 | Harga awal = jual-master (`standard`, tersimpan-ganda); member via quote (pilih-pelanggan; lepas = reset); varian preselect-bila-1 |
| BR-06 | Langkah-qty = 1-bulat vs 0.0001-pecahan (faktor); stepper/input/`max`/scan dijepit sama |

## 2. Cart (BR-07…BR-11)

| ID | Aturan |
|---|---|
| BR-07 | `subtotal = Σ harga×qty`; `diskon = Σ perunit×qty`; `total = max(0, subtotal−diskon)`; `itemCount = Σ qty`; kunci `produk:varian` |
| BR-08 | Tambah-ada = +1-dijepit + segarkan (harga/sumber/member/stok/label/lokasi); qty-0 = hapus; `clearCart` reset-semua (metode-tunai!) |
| BR-09 | Diskon nominal-total: klem `(harga − floor) × qty` (floor-nol-member!); tak-habis-bagi per-unit; quote-menurunkan-diskon (min-diskon-harga-baru!) |
| BR-10 | Invalid bila non-member + (bersih < minimum); member selalu-valid; tombol + submit-diam |
| BR-11 | Reservasi-kirim = tracked saja (qty + faktor-jual + base); tanpa-tracked tanpa-kunci |

## 3. Checkout (BR-12…BR-18)

| ID | Aturan |
|---|---|
| BR-12 | Blokir: kosong / tanpa-`order.create` / harga-invalid / quote-loading / quote-error / tempo-tanpa-HP / tunai-kurang (7!) |
| BR-13 | Meterai + tertunda: sama → pakai-ulang; beda → error-ke-Order; tanpa-pelanggan-tunai lolos; catatan → `notes` |
| BR-14 | Order: sales + tempo→`net` (+ `due_date` bila diisi) else `cod` + pelanggan-null-boleh + kini + alamat-opt-in + baris (harga-penuh + diskon!) + `source='pos'`; total = backend-bila-hingga (else cart!) |
| BR-15 | Serah: tunai → `pay_now` (+ tender-tunai-saja + tanggal-kini + referensi/catatan-null); tempo → tanpa-mode (+ kunci!); abaikan-sudah-serah (sukses!); sukses = rotasi-kunci + saldo + lepas-pelanggan |
| BR-16 | Tunai: digit-saja; saran pas + 4-bulat-unik; kurang-merah + tombol-mati; kembalian = max(0, terima − total-tampil!) |
| BR-17 | Tempo: tanggal-min-hari-ini (opsional!); teks-piutang; HP-wajib-dua-lapis; sukses-kuning-lunasi-Order |
| BR-18 | Tanpa-`payment.create` + tunai = order-yatim + 403-serah (→ KI baru; guard halaman hanya `order.create`!) |

## 4. Struk + native (BR-19…BR-22)

| ID | Aturan |
|---|---|
| BR-19 | Terima = bayar-terakhir ?? serah ?? order; kasir = pembuat-jatuh-pencetak; metode gabung-koma (tanpa-bayar-tempo = `Tempo`); status LUNAS/PIUTANG |
| BR-20 | Hide-diskon simetris-nota (penuh + bruto + sisa-riil; kembalian-dari-bruto!); pajak bila > 0; rekening/kebijakan/`Terima kasih` default |
| BR-21 | Payload-angka-final (web-hitung, Kotlin-cetak; field-tak-relevan omit); gagal → alert + manual; 20-detik-timeout; `NO_BRIDGE/TIMEOUT` |
| BR-22 | Fisik: 48-kolom + logo + UPPERCASE + `[ STATUS ]` + potong/laci-konfig; `TES CETAK` tetap; pindai-LAN; pengaturan 3-jari + auto-pertama |

## 5. Lintas modul (BR-23…BR-24)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-23 | Order-POS + serah-bayar + tender menjadi order + bayar + stok + cetak | Order (09), Payment (14), Stock (16) |
| BR-24 | Quote + snapshot-member + diskon menjadi harga-final + cetak-hemat | Member (07), cetak (09) |

## 6. Aturan yang TIDAK ada (verifikasi)

Tidak ada: backend-POS; edit/hapus; persen/ke-3/cicilan; tempo-tanpa-HP; cetak-tanpa-order;
kembalian-simpan; tender-non-tunai; blokir-saldo-gagal/bunyi-gagal; kirim-default; izin-cetak;
stok-negatif; guard-tanpa-`order.create`-halaman (tombol-mati saja!).
