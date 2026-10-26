# Analisis 003 — Foto produk di kartu katalog POS

**Tanggal:** 2026-10-26 · **Jenis:** brief desain (skill `riset-desain`) · **Kode:**
`apps/web/src/modules/pos/components/ProductGrid.tsx`

Folder ini hanya berisi brief, tanpa `flow.mmd`/`sequence.mmd`: perubahannya tampilan satu komponen, tanpa
alur data baru.

## 1. Kasus

- **Subjek:** kartu barang di katalog Mode Kasir (POS): foto, nama, harga, kode, stok, jumlah di keranjang.
- **Audiens:** kasir toko bahan bangunan/retail; tablet landscape 1024–1340 px (utama) dan HP 375–430 px.
  Disentuh jari, sering sambil melayani pelanggan.
- **Tugas utama:** mengenali barang yang tepat dalam ±1 detik lalu mengetuknya, tanpa salah ambil barang yang
  namanya mirip (mis. dua ukuran pipa, dua warna cat).
- **Keluhan pengguna:** foto di kartu terlalu kecil. Saat ini foto 56×56 px (`size-14`) di pojok kiri atas kartu
  berlebar minimal 160 px, sekitar 12% luas kartu. Foto tidak membantu membedakan barang.

## 2. Riset (diakses 2026-10-26)

| Sumber | Temuan |
|---|---|
| [majoo — Mode tampilan kasir](https://majoo.id/panduan-pengguna/detail/53) | Mode **Grid** (bawaan) menampilkan "foto thumbnail produk dalam ukuran besar". Mode **SKU** adalah daftar tanpa gambar besar untuk ritel |
| [Square — Item grid](https://square.com/help/us/en/article/5328) | Kisi kasir bisa memakai **tile gambar** atau tile teks (Settings › Item Appearance). Foto diisi dari data item |
| [Lightspeed — POS layout](https://o-series-support.lightspeedhq.com/hc/articles/31329442916891) | Tata letak kisi/daftar dengan foto per produk diatur admin |
| [Odoo — modul ukuran kartu POS](https://apps.odoo.com/apps/modules/17.0/dev_pos_product_card_size), [pratinjau foto](https://apps.odoo.com/apps/modules/17.0/nf_pos_image_preview) | Kartu POS bawaan Odoo memakai pola foto-di-atas. Kebutuhan memperbesar kartu/foto cukup sering sampai ada modul pihak ketiga khusus |
| [Shopify — Thumbnail](https://shopify.dev/docs/api/app-home/web-components/media-and-visuals/thumbnail) | Satu ukuran thumbnail seragam di satu kisi; yang lebih kecil hanya untuk tata letak padat |
| [web.dev — Responsive images](https://web.dev/learn/design/responsive-images), [Shopify — Prevent image layout shift](https://shopify.dev/docs/storefronts/themes/best-practices/performance/prevent-image-layout-shift) | Sediakan ruang lebih dulu (`aspect-ratio` atau `width`/`height`) agar kisi tidak bergeser saat foto dimuat. Gambar di bawah layar memakai `loading="lazy"` |
| [Material — touch target 48 dp](https://support.google.com/accessibility/android/answer/7101858), Apple HIG 44 pt (dikutip [Deque](https://dequeuniversity.com/rules/attest-ios/1.0/touch-target-size)), WCAG 2.2 | Target sentuh ≥ 44–48 px. Kartu utuh sudah jauh di atas batas; tombol info di atas foto tetap 44 px |

Fakta proyek:
- Foto disimpan dengan sisi terpanjang maksimal 1600 px (`media/application/image.go`) dan katalog sudah
  mengunduhnya. Foto yang lebih besar tidak menambah unduhan dan tetap tajam di layar retina.
- Ribuan produk hasil migrasi kemungkinan belum berfoto. Cadangan tanpa foto harus terlihat disengaja.

## 3. Temuan → keputusan

| Temuan | Keputusan |
|---|---|
| POS modern (majoo Grid, Square, Odoo) menjadikan foto elemen utama kartu | **Foto di atas, selebar kartu (full-bleed)**, sudut ikut kartu. Teks di bawahnya |
| Satu ukuran seragam per kisi; sediakan ruang lebih dulu | Kotak foto **rasio 4:3 tetap** (`aspect-[4/3]`), `object-cover`. Kisi tidak bergeser saat foto dimuat atau gagal |
| 4:3 vs 1:1 | Dengan 4:3, satu layar tablet masih memuat 2 baris kartu penuh. Kalau 1:1, kasir harus lebih sering menggulir. Foto barang ritel yang umumnya lanskap juga tidak terlalu terpotong |
| Kartu lebih besar mengurangi kolom | Lebar minimal kolom 10rem di HP (2 kolom ±170 px) dan 11rem mulai `lg` (3–4 kolom ±200 px). Foto ±200×150 px di tablet, ±10× luas sebelumnya |
| Banyak produk tanpa foto | Cadangan **inisial besar** di kotak 4:3 yang sama (`bg-muted`), jadi baris tetap rata dan kasir tetap punya jangkar visual |
| Status di atas foto mudah terlewat kalau hanya berupa warna | Jumlah di keranjang tampil sebagai lencana di pojok kiri atas foto (angka). Stok habis: foto diredupkan *dan* chip "Stok habis" tetap ada di baris info (bukan warna saja) |
| Tombol info HP 44 px | Tombol tetap di pojok kanan atas, diberi latar kartu semi-pekat agar terbaca di atas foto |
| Kartu berfoto besar mengurangi jumlah barang per layar (verifikasi 1280×800: ±10 kartu, sebelumnya ±20); majoo punya mode Grid vs SKU, Square tile gambar vs teks, Lightspeed kisi vs daftar | **Pilihan tampilan** di header kisi: **Foto besar** (bawaan) dan **Ringkas** (kartu ringkas berfoto 56 px seperti sebelumnya). Preferensi per perangkat (`localStorage` `pos.catalogView`), tanpa izin baru dan tanpa data server |

**Elemen khas:** foto barang sebagai "jendela" kartu, dengan harga dalam huruf display proyek tepat di bawahnya.
Sisanya tenang: satu radius kartu, tanpa bayangan tambahan, tanpa animasi baru.

Pengecekan terhadap tampilan templat: kartu tetap pola produk-POS yang dikenal kasir (bukan kartu SaaS
dekoratif). Tidak ada label huruf kapital, gradien, atau titik tengah sebagai pemisah meta.

## 4. Token (design system v2, ADR-0017 — tidak ada token baru)

- Warna: `bg-card`, `border-border`, `bg-muted`/`text-muted-foreground` (cadangan), `bg-primary`/
  `text-primary-foreground` (lencana keranjang, cincin terpilih), `bg-warning-soft`/`bg-destructive-soft`
  (chip stok). Terang/gelap otomatis lewat token.
- Huruf: `font-display` (Plus Jakarta Sans) untuk harga dan inisial; sans bawaan untuk nama; `font-mono` untuk
  kode, sesuai kode yang ada.
- Radius: `rounded-xl` kartu; foto mengikuti lewat `overflow-hidden`.
- Gerak: tetap `active:scale-[0.98]` sebagai umpan balik ketuk, tanpa animasi baru.

## 5. Tata letak

```
Tablet (≥1024, 3–4 kolom ±200 px)        HP (375, 2 kolom ±170 px)
┌──────────────────────┐                 ┌────────────────┐
│ [×2]            (i)* │ ← foto 4:3      │ [×1]       (i) │
│                      │   full-bleed    │     FOTO 4:3   │
│        FOTO          │                 │                │
├──────────────────────┤                 ├────────────────┤
│ Semen Tiga Roda 50kg │ ← 2 baris tetap │ Semen Tiga …   │
│ (baris 2)            │                 │                │
│ Rp 68.000   3 varian │ ← harga display │ Rp 68.000      │
│ STR-50      Stok 120 │ ← kode + stok   │ STR-50  Stok 12│
└──────────────────────┘                 └────────────────┘
* tombol info hanya < lg; tablet: tekan-lama / klik kanan (tidak berubah)
```

Nama selalu menyediakan 2 baris (`min-h`), sehingga harga di satu baris kisi sejajar.

## 6. State

- **Memuat:** kerangka kartu dengan kotak foto 4:3 dan dua garis teks (bukan blok rata), jadi tinggi sama
  dengan kartu asli.
- **Foto gagal dimuat** (mis. URL bertanda tangan kedaluwarsa): kembali ke inisial di kotak yang sama.
- **Di keranjang:** cincin primer dan lencana ×qty.
- **Menambah:** redup (`opacity-70`).
- **Stok habis:** foto diredupkan, chip "Stok habis". Inisial cadangan tidak diredupkan agar kontrasnya tetap memenuhi AA (temuan uji V01).
- **Stok kurang** (tahanan gagal): chip "Stok kurang".

## 7. Aksesibilitas

- Kartu tetap satu tombol `Tambah <nama>`. Foto dekoratif (`alt=""`), karena nama sudah ada di label.
- Target sentuh: kartu ≥ 170×230 px; tombol info 44×44 px.
- Status tidak hanya warna: lencana angka, chip berteks + ikon.
- Kontras chip memakai token yang sama dengan sebelumnya. Lencana di atas foto memakai `bg-primary` pekat.

## 8. Verifikasi

Hasil (2026-10-26, server e2e lokal, 20 produk seed, 6 di antaranya diberi foto sintetis):
- **1280×800:** 4 kolom, foto ±200×150 px; sekitar 10 kartu terlihat. Lencana ×2 dan cincin terpilih jelas;
  stok habis meredup dan chip "Stok habis" tetap ada. Mode Ringkas menampilkan sekitar 20 kartu.
- **1024×768:** 3 kolom; tombol Bayar terlihat.
- **390×844 (HP):** 2 kolom, foto ±175×130 px; tombol info berlatar terbaca di atas foto.
- Konsol bebas galat.
- **Belum terverifikasi:** tema gelap (aplikasi belum punya tema gelap, STATUS "ditunda"), tablet fisik kasir,
  foto produk nyata dari produksi.

Rencana awal:

Screenshot POS di 1280×800, 1024×768, dan HP 390, tema terang dan gelap, dengan produk berfoto maupun tanpa
foto. Konsol bebas galat. V02 (tanpa gulir menyamping, tombol Bayar terlihat) dan E02/E23/E24 tetap lulus.
Perangkat tablet fisik kasir belum diuji (STATUS U5-5).
