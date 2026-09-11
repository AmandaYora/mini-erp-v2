# Test Cases — Modul 15 POS / Kasir

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`use-pos-cart.test.ts` (diskon/sinkron/catatan/UOM-reserve), `pos-product-card.test.tsx`
(foto/dedup/bunyi), `pos-customer-panel.test.tsx` (walk-in/tambah), E2E 07 (scan/mobile/
tunai/hutang), 15-real-user (kasir-manual + UI-produk), 11-volume (POS massal), 17 (kartu-
stok). **GAP** = belum ada test.

---

## TC-P — Cart & katalog (TC-P-01…TC-P-08)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-P-01 | Diskon 6000/2-qty | 3000/unit + 6000-total | Ya (unit) |
| TC-P-02 | Sinkron-sama | Array-sama (tanpa-render!) | Ya (unit) |
| TC-P-03 | Catatan + clear | Simpan + reset | Ya (unit) |
| TC-P-04 | Batas-UOM-jual + reserve-base | Tepat | Ya (unit) |
| TC-P-05 | Foto-square + fallback-2-huruf + dedup-halaman + bunyi-sukses/gagal-diam | Tepat | Ya (unit ×4) |
| TC-P-06 | Walk-in ada/tanpa + tambah-langsung | Tepat | Ya (unit ×3) |
| TC-P-07 | Scan-sukses (+ bunyi + kosong) vs kosong (dialog + bunyi!) | Tepat | Ya (E2E 07) |
| TC-P-08 | Mobile-bar (`1 item` + `Bayar`, 390px) | Tepat | Ya (E2E 07) |

## TC-C — Checkout & struk (TC-C-01…TC-C-10)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-C-01 | Tunai walk-in 1×12rb | Stok −1 (tanpa-pelanggan!) | Ya (E2E 07) |
| TC-C-02 | Hutang 5×10rb → saldo 50rb → 27rb FIFO-3 → 23rb → 0 | Tepat (E2E 07!) | Ya (E2E 07) |
| TC-C-03 | Kasir-manual + bayar + stok-3 (E2E 15) | Tepat | Ya (E2E 15) |
| TC-C-04 | 50-PO + 100-sales + POS (E2E 11) | Volume | Ya (E2E 11) |
| TC-C-05 | Kartu-stok-tersedia-dulu (E2E 17) | Urutan | Ya (E2E 17) |
| TC-C-06 | Struk: 8-baris + kembalian + rekening + hide-simetris | Tepat | Ya (FE test? — `payment-receipt-print-page.test` + snapshot-14; thermal-spesifik GAP!) |
| TC-C-07 | Tanpa-`payment.create` + tunai | Order-yatim + 403 | GAP → KI baru |
| TC-C-08 | Kode-mati-3 (hidden/Wide/`if-false`) | Mati terverifikasi | GAP → KI baru |
| TC-C-09 | Reservasi-hantu + meterai-beda + cart-berubah | Guard tepat | GAP |
| TC-C-10 | Shell: jembatan + render-48 + tes + laci + pengaturan | Kontrak Kotlin | GAP (tanpa-test-native!) |

## GAP (G-01…G-06)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Order-yatim tanpa-bayar-izin | Lubang izin + data yatim |
| G-02 | Kode-mati-3 (hidden/Wide/false) | Buang atau hidupkan |
| G-03 | Reservasi-hantu/meterai/ubah-cart | Lomba halus tanpa jaring |
| G-04 | Thermal-spesifik + native-bridge | Cetak-fisik tanpa jaring |
| G-05 | Member-POS-ujung (quote-gagal-blokir-tunai!) | Tunai-terblokir-quote tanpa jaring |
| G-06 | `items`-Inggris + fallback-kasir + saran-cepat | Teks kecil tanpa jaring |
