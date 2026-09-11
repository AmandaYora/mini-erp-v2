# Test Cases — Modul 09 Order Sales

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`order.service.spec.ts` (bagian sales), `order.service.financial.spec.ts` (prepaid/COD/tempo),
`order-pricing.service.spec.ts` (delegasi member), `order-form-page.test.tsx` (requote/klem),
E2E 02 (real-flow), 06 (delivery-konstruksi, sisi-SJ milik 11), 15-real-user (kasir-manual),
17-blackbox (urutan stok, milik 05). **GAP** = belum ada test.

---

## TC-S — SO & harga (TC-S-01…TC-S-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-S-01 | Sales + min 60rb, harga 50rb | `400` dua-nominal | Ya (spec) |
| TC-S-02 | Harga == min 50rb | Lolos | Ya (spec) |
| TC-S-03 | Diskon tersimpan + lawan-minimum efektif | Tersimpan + `400` bila tembus | Ya (spec ×2) |
| TC-S-04 | Member + min 50rb | Bypass lolos | Ya (spec) |
| TC-S-05 | Non-sales harga 30rb + min 60rb | Tanpa validasi | Ya (spec) |
| TC-S-06 | Net tanpa customer/HP; pihak supplier; arsip | `400` ×3–4 | Ya (spec, pola purchase) |
| TC-S-07 | Konstruksi net 10×1200 + 2×20000 + tempo | Tersimpan + snapshot + pihak live | Ya (E2E 02) |
| TC-S-08 | POS tunai 1×12000 → deliver `pay_now` | Stok −1 | Ya (E2E 02) |
| TC-S-09 | UI: saran-jual, helper min-max, klem 1000/3→333, requote hormat-manual, blokir-quote | Tepat | Ya (FE test) |
| TC-S-10 | Shortcut: spasi → inline; simpan → terpilih + requote; gagal → pesan | Tepat | Ya (E2E 20/21 pola) |
| TC-S-11 | Pesan tipe-salah menyebut "tempo" untuk tunai-bersupplier | Teks sama (E-02) | GAP → KI baru |
| TC-S-12 | Diskon tepat = harga (gratis) vs +1 | Lolos vs `400` | GAP |

## TC-D — Serah & tempo (TC-D-01…TC-D-10)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-D-01 | Prepaid aktif berutang / lunas | `400` / lolos | Ya (spec ×2) |
| TC-D-02 | Selesai prepaid/COD berutang; net bebas | `400` ×2 / lolos | Ya (spec ×3) |
| TC-D-03 | COD terima: syarat-keputusan / alih-tempo / tanpa-izin-403 | Tepat | Ya (spec ×3) |
| TC-D-04 | Tunai kurang vs pas vs pecahan (`100000.4`→`100000`) vs non-tunai/kosong | `400 /kurang dari total/` / pas / bulat / null | Ya (`delivery.service.spec.ts` `resolveTenderedAmount`) |
| TC-D-05 | Serah ganda / bukan-sales / tanpa-item / tanpa-status / tanpa-tahap | 6 pesan | GAP (sebagian; terima-purchase ada, serah belum) |
| TC-D-06 | Tanggal eksplisit vs bayar vs kini | Prioritas tepat | GAP |
| TC-D-07 | Reservasi terkonsumsi + id order | Tepat | GAP |
| TC-D-08 | Approve: ok / tanpa-tempo / purchase / sudah-net / terminal | Tepat / 4×`400` | Ya (spec, sebagian) |
| TC-D-09 | UI: dialog tempo +30-hari-default + disabled; toast tempo; tombol `Harga di bawah minimum` | Tepat | GAP |
| TC-D-10 | POS carve-out mati-efektif (pos tanpa serah tetap tertahan lapis-2) | Dua lapis | GAP → KI baru |

## TC-C — Bayar & cetak (TC-C-01…TC-C-08)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-C-01 | Form: pecahan dibuang, referensi auto, bukti-opsional, edit-bukti, hapus-dialog | Tepat | GAP (kode saja) |
| TC-C-02 | Hapus disembunyikan terminal-non-net | Tepat | GAP |
| TC-C-03 | Nota: 3 judul + `-` tempo + Staff-mentah + USER-cetak + ganjal-6 + preprinted + notes-sementara | Tepat | GAP (snapshot milik 14) |
| TC-C-04 | Sembunyi-diskon: penuh + bruto + sisa-riil + basis-member | Tepat | GAP |
| TC-C-05 | Disc hilang bila hanya level-order | Tepat | GAP |
| TC-C-06 | Non-sales → pesan khusus; gagal → `Order tidak ditemukan.` | Tepat | GAP |
| TC-C-07 | Bukti bayar: satu + sisa-sesudah, tanpa tangan | Tepat | GAP (milik 14?) |
| TC-C-08 | E2E cetak-dokumen-pajak + volume + blackbox-sisi-sales | Terkunci lintas | Ya (E2E 14/11/17) |

## GAP (G-01…G-08)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Serah langsung (pesan, tanggal, reservasi, tender) | Jalur warisan tanpa jaring langsung |
| G-02 | Cetak nota/diskon/bukti (aturan tampil) | Dokumen pelanggan tanpa jaring |
| G-03 | Form bayar (pecahan, referensi, hapus-sembunyi) | Uang tanpa jaring FE |
| G-04 | POS carve-out + pesan-tempo + diskon-pas | 3 KI butuh kunci perilaku |
| G-05 | Referensi `REF-` unik? | Keputusan + kemungkinan tabrakan |
| G-06 | Shortcut + requote + blokir-submit | Sudah E2E; belum unit |
| G-07 | `Tgl. Jtp` `-` + Staff-mentah + ganjal | Aturan kecil tanpa jaring |
| G-08 | Volume 100 sales (E2E 11) sisi jual | Performa + nomor-PJ bulanan |
