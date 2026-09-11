# User Flows — Modul 15 POS / Kasir

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Tanpa endpoint sendiri
(`order.create`/`deliver-goods`/`pricing/quote`/stok/scan/reservasi + `order.create` halaman).
Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Buka `/pos` | `order.create` | Rute diblokir (tombol label tetap!) |
| Buat + serah + bayar | `order.create` (+ `payment.create` wajib via deliver `pay_now`!) | Tanpa `payment.create`: order JADI dibuat lalu serah 403 → order yatim (→ KI baru) |
| Quote member | `order.create` + `member_type.view` (opsi) | Tanpa opsi = tanpa member |
| Struk | `order.view` | Rute diblokir |
| Cetak native | Perangkat (tanpa izin app) | Fallback browser |

## UF-01 — Tunai walk-in (E2E 02/07)

Cari/klik/scan → cart → `Lanjut Pembayaran` → tunai + diterima (saran/kembalian) →
`Proses Pembayaran` → order-POS + serah-bayar + sukses → `Cetak Struk` (+ Nota/Bukti/Order)
→ `Transaksi Baru`. Tanpa pelanggan lolos; stok −= qty (E2E).

## UF-02 — Member + diskon + sembunyi (E2E 20-pola)

Pilih pelanggan → quote (badge + harga) → diskon-baris (klem-nol) → tunai → sukses →
checkbox-sembunyi → struk-penuh + nota-penuh. Tanpa-izin-member = harga-standard.

## UF-03 — Piutang (E2E 07/20)

`Bayar Nanti` → pelanggan + HP (wajib-dua-lapis!) + tempo-opsional → `Serahkan & Catat
Piutang` → `Barang Diserahkan` + kuning-lunasi-Order → saldo +50rb (5 order!) → bayar-FIFO
27rb → 23rb + alokasi-3 → lunasi → 0.

## UF-04 — Tambah pelanggan + kirim

Inline (nama + HP-hutang + member + langsung-terpilih-server) → opt-in alamat → snapshot →
struk/nota. Tanpa-pelanggan-tempo mustahil.

## UF-05 — Scan-gagal + stok-habis (E2E 07)

Sukses (+ bunyi + input-kosong) vs kosong (dialog + bunyi-tetap!) vs habis/lemah/koneksi
(dialog + `Mengerti`).

## UF-06 — Cetak-ganda + perangkat

Sukses → 5 tombol (Struk/Nota/Bukti/Order/Baru); native → ESC/POS-LAN (+ laci/potong) else
browser; gagal-native → alert + manual. Shell: instal → pengaturan-otomatis → URL → jual →
cetak/tes/pindai/laci.

## UF-07 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Tanpa-cabang / tanpa-izin / tanpa-stok | Layar / tombol-mati / dialog |
| Tempo-tanpa-HP / quote-gagal / tunai-kurang / harga-invalid | Blokir-4 (error + tombol) |
| Klik-ganda / serah-ganda / cart-berubah | Guard / abaikan-sukses / error-ke-Order |
| Cetak-gagal / saldo-gagal / bunyi-gagal | Alert / diam / diam |
