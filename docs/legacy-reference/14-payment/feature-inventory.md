# Feature Inventory — Modul 14 Payment / Pembayaran

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode (`payment.controller.ts`, `payment.service.ts` 812 baris, 2 entity, migrasi
009/012/033/047, seksi bayar + hook + cetak bukti, E2E 07 + 22). Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = bayar per order + bayar gabungan per pihak (FIFO/manual) + saldo &
ledger pihak + bukti bayar + arsip bayar + cetak bukti. Form catat-bayar di detail order,
dialog tempo, dan tombol serah/terima milik modul 08/09/11 (di sini hanya kontrak endpoint
yang mereka panggil). Saldo di halaman finance milik 17 (konsumen `party-balances/ledger`).

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 7 — `payments/{list,party-balances,party-ledger,create,upload-proof,proof-url,archive}` |
| Halaman | **Tanpa halaman sendiri.** Tampil di: seksi Pembayaran detail order, kartu Piutang/Utang finance (konsumen saldo), cetak `/orders/:orderId/payments/:paymentId/print` (`order.view`) |
| Permission | `payment.view` (daftar/saldo/ledger/URL bukti), `payment.create` (buat + unggah bukti + COD-serah/terima), `payment.archive` (arsip) |
| Tabel yang dimiliki | `payments`, `payment_allocations` |
| Tabel yang dibaca | `orders` (+ status), `business_parties` (termasuk arsip!), `branch_document_sequences` (nomor) |
| Penomoran | `PAY-{KODECABANG}/{TAHUN}/{5 digit}` — tanpa reset. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | `payment.create/archive/proof_uploaded` |
| Aksi yang TIDAK ada | Edit bayar (kecuali ganti bukti); hapus permanen; bayar tanpa order/pihak; bayar desimal; bayar melebihi sisa; bayar order-batal; bayar pihak-tanpa-tunggakan; halaman daftar bayar global |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Bayar per order (`payments/create` + `id_order`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Syarat | Order ada-cabang-non-arsip (`Order tidak ditemukan`); bukan-batal (`Order yang sudah dibatalkan tidak dapat menerima pembayaran.`); sisa > 0 (`Order ini sudah lunas. Tidak dapat menambahkan pembayaran lagi.`); jumlah ≤ sisa (`Jumlah pembayaran melebihi sisa saldo order. Maksimal {n}.`) |
| F-01.2 | Uang | Wajib > 0 (`Jumlah pembayaran harus lebih dari 0`); **bulat rupiah** (`...harus bilangan bulat rupiah (tanpa koma/desimal).` — akar order recehan; FE membuang pecahan di input) |
| F-01.3 | Hasil | Baris `payments` (order + pihak-dari-order + sisi-dari-jenis + nomor + tanggal + jumlah + metode + referensi + catatan + pembuat) + audit `payment.create`; tanpa alokasi (langsung) |
| F-01.4 | Tanpa-target | Tanpa `id_order` + tanpa `id_business_party` → `Pilih order atau customer/supplier untuk mencatat pembayaran.` |

### F-02 — Bayar gabungan per pihak (FIFO/manual + alokasi)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Syarat | Pihak ada-perusahaan (**termasuk arsip** — sengaja agar tunggakan tak menggantung; E2E 22 + spec); sisi cocok (`Pembayaran hutang supplier harus memilih supplier.` / `...piutang customer harus memilih customer.`); sisi-bebas → tebak dari tipe (supplier→payable else receivable); sisi-salah → `Jenis saldo harus piutang customer atau hutang supplier.`; wajib ada tunggakan (`...tidak memiliki hutang/piutang terbuka.`) |
| F-02.2 | Alokasi | Default FIFO (tua dulu: jatuh-tempo → tanggal → id); manual: tiap baris milik-pihak + > 0 + ≤ sisa-baris (`Order {id} tidak memiliki saldo terbuka untuk pihak ini.` / `Jumlah alokasi harus lebih dari 0.` / `Alokasi untuk order {nomor} melebihi sisa saldo.`) + total = bayar (`Jumlah alokasi pembayaran harus sama dengan jumlah pembayaran.`); total > tunggakan → `Jumlah pembayaran melebihi total saldo terbuka. Maksimal {n}.`; hasil-nol → `Tidak ada tagihan yang bisa dialokasikan.` |
| F-02.3 | Hasil | Baris `payments` (order NULL + pihak + sisi) + N baris `payment_allocations` (fifo/manual + `allocated_at` = tanggal-bayar) + audit (+ hitungan + mode) |
| F-02.4 | Tampil di order | Daftar order memproyeksikan alokasi sebagai baris bayar (jumlah = alokasi, + `allocatedAmount`); total = langsung + alokasi (arsip dikecualikan dua-duanya) |

### F-03 — Saldo & ledger pihak (`party-balances`, `party-ledger`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Saldo | Sisi wajib (salah → jenis-saldo); cari kode/nama/telepon (kecil-semua); agregat per pihak (hitung order + sisa + tertua = tempo ?? tanggal); urut tertua + nama; `total_outstanding` selalu global; paginasi opt-in (tanpa limit = semua). Tanpa-pihak (walk-in) tak masuk (dilewati diam-diam!) |
| F-03.2 | Ledger | Pihak boleh arsip; sisi eksplisit else tebak-tipe; tagihan-terbuka (dengan umur-hari + bucket `0-30/31-60/61-90/>90`) + bayar (dengan alokasi per order) + `total_outstanding`; tagihan & bayar dipaginasi terpisah (tanpa limit = semua; total tetap global) |
| F-03.3 | Konsumen | Kartu finance Piutang/Utang + dialog `Rincian {nama}` (modul 17); helper E2E; kolektor menagih arsip (E2E 22) |

### F-04 — Bukti bayar (`upload-proof`, `proof-url`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Unggah (multipart `payment.create`) | Bayar ada-cabang-non-arsip (`Pembayaran tidak ditemukan`); file wajib (`File bukti pembayaran wajib diunggah`; 5MB; tipe via optimizer); optimasi profil-bayar → kunci `payment/pembayaran/...` + timpa path (ganti = timpa!) + audit `proof_uploaded` (+ kompresi) |
| F-04.2 | URL (`payment.view`) | Ada + punya-path (`Bukti pembayaran belum tersedia` bila tanpa) → `{url, expires_in}` |
| F-04.3 | UI | Input `(JPG/PNG, maks 5MB)` + `{nama} — akan dikompres sebelum disimpan`; ganti-bukti (`Edit`, butuh create); lihat (`Lihat` bila ada path → overlay + `✕`/`Tutup` + Escape; gagal diam); ganti menimpa (notice `...File baru akan menimpa bukti lama.`) |

### F-05 — Arsip bayar (`archive`)

Syarat: ada-cabang-non-arsip (`Pembayaran tidak ditemukan`); per order-terdampak (langsung +
alokasi): terminal + non-net + jadi-berutang-pasca-arsip → tolak (`Pembayaran tidak dapat
diarsipkan karena order {prabayar\|bayar saat serah} yang sudah selesai akan menjadi belum
lunas.`); net-terminal bebas. Hasil: bayar + alokasinya terarsip serentak + audit (+ hitungan).
UI: dialog `Hapus pembayaran?` / `...tidak bisa dibatalkan.` / `Hapus Pembayaran` → toast
`Pembayaran dihapus`; tombol disembunyikan bila terminal + non-net. E2E 07 mengunci saldo
kembali.

### F-06 — Cetak bukti (`/orders/:orderId/payments/:paymentId/print`, `order.view`)

Toolbar kertas (A4/Kontinu, tanpa preprinted/diskon) + Cetak/Tutup; kop `Bukti Pembayaran
{Customer|Supplier}` + status (`Lunas`/`Ada Sisa`) + tanggal; kotak pihak
(`Diterima Dari`/`Dibayarkan Kepada`: Nama/No HP/Alamat) + Rincian (No Order/Tanggal/Metode/
Referensi); tabel Total Tagihan + Jumlah Pembayaran Ini + Total Terbayar Setelah Ini + Sisa
Setelah Pembayaran (akumulasi urut-tanggal sampai bayar ini!); catatan (bila ada) + kalimat
verifikasi (`Bukti pembayaran ini dicatat dari sistem dan diverifikasi melalui nomor
pembayaran, metode, referensi, serta audit transaksi.`). Tanpa bayar → `Pembayaran tidak
ditemukan pada order ini.`; loading `Memuat bukti pembayaran...`.

---

## 3. Edge Case (24)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Jumlah 0/negatif/kosong | `Jumlah pembayaran harus lebih dari 0` |
| E-02 | Desimal (`10.5`, `12017,40`) | `...bilangan bulat rupiah (tanpa koma/desimal).` (FE membuang di input) |
| E-03 | Over-sisa order / over-total pihak | `...melebihi sisa saldo order. Maksimal {n}.` / `...melebihi total saldo terbuka. Maksimal {n}.` (E2E 07: 99jt ditolak `/alokasi\|saldo\|tagihan\|hutang/`) |
| E-04 | Order batal / lunas / hilang | 3 pesan (§F-01.1) |
| E-05 | Pihak hilang vs arsip | `Customer/supplier tidak ditemukan` hanya bila id tak ada; pihak terarsip tetap boleh (disengaja — komentar kode) |
| E-06 | Sisi vs tipe bentrok dua arah + sisi-sampah | 3 pesan (§F-02.1) |
| E-07 | Tanpa tunggakan | 2 pesan per sisi |
| E-08 | Manual: luar-pihak / 0 / over-baris / total-beda | 4 pesan (§F-02.2) |
| E-09 | Hasil-nol | `Tidak ada tagihan yang bisa dialokasikan.` |
| E-10 | Order-batal dikecualikan tunggakan (tak bisa dibayar!) tetapi bayar-langsung menolak batal — konsisten dua arah |
| E-11 | Batas 0.009 untuk baris-terbuka (vs 0 untuk bayar-order!) — inkonsistensi ambang ([PERLU KONFIRMASI] disengaja? → KI baru) |
| E-12 | Urut FIFO: tempo → tanggal → id; umur bucket hari-kalender |
| E-13 | `allocated_at` = tanggal-bayar (bukan kini!) — backdate konsisten |
| E-14 | Arsip: terminal-net bebas; terminal-non-net-berutang ditolak; tanpa-terminal bebas (walau jadi-berutang!) |
| E-15 | Arsip mengecualikan dua sisi (langsung kecuali-id + alokasi kecuali-id & arsip) |
| E-16 | Bukti: tanpa-file / hilang / tipe / tanpa-path-URL | 4 pesan (§F-04) |
| E-17 | Ganti bukti = timpa path (riwayat file hilang; objek lama yatim di storage!) |
| E-18 | Cetak: bayar-luar-order → pesan khusus; akumulasi sampai-bayar-ini (bukan total-akhir!) |
| E-19 | Status cetak = sisa-sesudah (bukan kini!) — cetak ulang historis stabil |
| E-20 | Walk-in: tanpa-pihak → tanpa-saldo/ledger (tak tertagih per pihak!) |
| E-21 | Retur completed menyesuaikan total + bayar-efektif (collect +, refund/kredit −) — satu-satunya penyesuaian non-bayar |
| E-22 | Tempo-lewat + berutang = overdue (net saja; prepaid/COD tanpa-tempo tak pernah overdue!) |
| E-23 | `payment_date` bebas (masa lalu/depan tanpa validasi! — backdate seperti terima/serah) |
| E-24 | Referensi bebas-ganda (KI-87); metode tanpa validasi runtime (tipe TS saja — string sampah tersimpan; → KI baru) |

---

## 4. Katalog Pesan (teks apa adanya)

**Buat:** `'Jumlah pembayaran harus lebih dari 0'` · `'Jumlah pembayaran harus bilangan bulat rupiah (tanpa koma/desimal).'` · `'Pilih order atau customer/supplier untuk mencatat pembayaran.'` · `'Order tidak ditemukan'` · `'Order yang sudah dibatalkan tidak dapat menerima pembayaran.'` · `'Order ini sudah lunas. Tidak dapat menambahkan pembayaran lagi.'` · `'Jumlah pembayaran melebihi sisa saldo order. Maksimal {n}.'` · `'Customer/supplier tidak ditemukan'` · `'Jenis saldo harus piutang customer atau hutang supplier.'` · `'Pembayaran hutang supplier harus memilih supplier.'` · `'Pembayaran piutang customer harus memilih customer.'` · `'Supplier ini tidak memiliki hutang terbuka.'` / `'Customer ini tidak memiliki piutang terbuka.'` · `'Tidak ada tagihan yang bisa dialokasikan.'` · `'Jumlah alokasi pembayaran harus sama dengan jumlah pembayaran.'` · `'Order {id} tidak memiliki saldo terbuka untuk pihak ini.'` · `'Jumlah alokasi harus lebih dari 0.'` · `'Alokasi untuk order {nomor} melebihi sisa saldo.'` · `'Jumlah pembayaran melebihi total saldo terbuka. Maksimal {n}.'`.

**Arsip/bukti:** `'Pembayaran tidak ditemukan'` · `'Pembayaran tidak dapat diarsipkan karena order {prabayar\|bayar saat serah} yang sudah selesai akan menjadi belum lunas.'` · `'File bukti pembayaran wajib diunggah'` · `'Bukti pembayaran belum tersedia'`.

**UI:** `Pembayaran berhasil dicatat` · `Pembayaran dihapus` · `Gagal memuat data pembayaran` · `Hapus pembayaran?` / `...tidak bisa dibatalkan.` / `Hapus Pembayaran` · `Termin diubah ke tempo` (dialog milik 08/09) · `Bukti Pembayaran {Customer|Supplier}` · `Diterima Dari` / `Dibayarkan Kepada` · `Total Tagihan` / `Jumlah Pembayaran Ini` / `Total Terbayar Setelah Ini` / `Sisa Setelah Pembayaran` · `Lunas` / `Ada Sisa` · `Memuat bukti pembayaran...` / `Pembayaran tidak ditemukan pada order ini.` · kalimat verifikasi (§F-06).

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Edit bayar (selain ganti bukti) | Tanpa endpoint (jumlah/tanggal/metode final) |
| NF-02 | Halaman daftar bayar global | Hanya per-order + per-pihak |
| NF-03 | Bayar desimal/negatif/nol/over | 4 penolakan |
| NF-04 | Bayar order-batal / tanpa-tunggakan | 2 penolakan |
| NF-05 | Hapus permanen | Arsip + alokasi |
| NF-06 | Alokasi-edit/pindah | Final saat buat |
| NF-07 | Menagih walk-in per pihak | Tanpa pihak = tanpa saldo |
| NF-08 | Overdue non-net | Hanya tempo + tanggal-lewat |
| NF-09 | Kembalian tersimpan | `amount_tendered` hanya tunai-serah (047) |
| NF-10 | Validasi tanggal-bayar | Bebas (E-23) |
