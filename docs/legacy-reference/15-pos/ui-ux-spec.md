# UI/UX Spec — Modul 15 POS / Kasir

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Rute `/pos`
(`order.create`, menu Operasional → POS): grid 2-kolom (katalog + cart 380px; ≤1200px
menyempit; ≤900px tumpuk + bar-mobile + bawah-92px). Tanpa cabang: layar toko. Bagian ambigu
ditandai **[PERLU KONFIRMASI]**.

---

## 1. Kolom katalog (kiri)

**Filter** (bar 720px-min, gulir-x): `Cari produk` + tombol-scan + `{n} produk ditemukan`.
**Drilldown** (`Kategori POS`): `Semua` + breadcrumb `/` + `Naik ←` + pil (badge anak).
**Grid** (`pos-product-grid`, kartu 160px): foto-square-lazy/fallback-2-huruf + nama-2-baris +
kode + varian-dropdown + harga-brand + uom + badge-stok + stepper-atau-tambah (klik/Enter/
Spasi/animasi). Keadaan: skeleton-8 / gagal + `Muat ulang` / kosong / sentinel
(`Memuat produk...` / `Muat ulang` / kosong-aria / `Semua produk tampil`).

## 2. Kolom cart (kanan, `pos-cart-panel`)

Ringkasan pelanggan + sukses-overlay (bila sukses) else header (`Transaksi Aktif` + badge +
`Kosongkan`) + baris (kartu: nama/kode/harga/badge-member/lokasi-3/hapus-`✕` + stepper +
Diskon + total + Hemat) + ringkasan (Subtotal/Diskon/Total-besar) + Termin-Metode
**TERSEMBUNYI** + error/quote + tombol-utama + bar-mobile (≤900px: `{n} item` + total +
tombol; `pos-mobile-checkout-bar`).

**Sukses-overlay**: lingkaran `Sukses` + judul-mode + nomor (+ piutang-kuning) + kotak Total +
checkbox-diskon (kondisional + sub) + 5 tombol (`Cetak Struk` primer + Nota/Tagihan + Bukti
(bila ada) + Lihat-Order-tab + `Transaksi Baru` brand-kustom).

## 3. Dialog checkout (680px)

Judul-mode + Total-besar-brand + proses-spinner (kunci-tutup) else: 2-kotak-mode +
pelanggan (walk-in + autocomplete + tambah + member + HP-hutang + alamat-opt-in) + quote
(kuning/merah) + catatan + tunai (metode + diterima-digit + saran-cepat + kurang-merah /
kembalian-hijau) / tempo (tanggal-min-hari-ini + teks + blokir-kuning) + footer Batal/Proses
(disabled-3).

## 4. Scan + pelanggan + struk + shell

**Scan**: modal + manual + kamera; dialog-hasil (`Mengerti`, merah/kuning, nama + kode).
**Pelanggan**: autocomplete-10 + tambah-inline + walk-in + ringkasan-badge.
**Struk** (`pos-receipt/print`): toolbar + kop + meta-5 + baris + ringkasan-8 + rekening +
kebijakan + terima-kasih + footer-print; native-vs-browser.
**Shell**: WebView-URL-server (tanpa-bundle!) + jembatan + kamera + window.open + 3-jari →
pengaturan (IP/port/MAC/potong/laci + pindai-LAN + `TES CETAK` + logo + auto-pengaturan).

## 5. Format yang mengikat

- Uang-brand-besar (Total 1.6rem; dialog 2rem); qty-tebal-brand; badge-stok 3-warna; member
  aksen (cart + ringkasan).
- `items` Inggris di badge (`{n} items` — bukan "item"! [PERLU KONFIRMASI] disengaja?).
- Tanggal-struk terima = bayar-terakhir ?? serah ?? order; metode gabung-koma; tanpa-bayar
  tempo = `Tempo`.
- Cetak-fisik: 48-kolom + `Rp .` + UPPERCASE-nama + `[ STATUS ]` + potong/laci.
