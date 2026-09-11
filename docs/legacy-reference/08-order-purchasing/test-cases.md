# Test Cases — Modul 08 Order Purchasing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`order.service.spec.ts` (1971 baris), `order.service.financial.spec.ts` (428),
`order-status.service.spec.ts` (115), `order-export.service.spec.ts` (196), E2E 05
(+ 11 volume, 16 export, 08 transfer). Hanya kasus purchase di bawah (sales milik 09).
**GAP** = belum ada test.

---

## TC-O — Order pembelian (TC-O-01…TC-O-18)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-O-01 | Create purchase lengkap | Subtotal benar; nomor `.../PB/...` independen dari sales; prefix fallback `ORD-{id}` | Ya (spec ×3) |
| TC-O-02 | Qty pecahan × harga | Rupiah utuh tanpa sisa sen | Ya (spec) |
| TC-O-03 | Snapshot UOM beli + qty-base | purchaseUom/faktor/base tersimpan | Ya (spec) |
| TC-O-04 | Non-net → due null; net wajib tempo | Dibersihkan / `400` | Ya (spec ×2) |
| TC-O-05 | Tanpa status awal | `400 Status awal...` | Ya (spec) |
| TC-O-06 | Kind terhapus (`job`) | `400` (create + update) | Ya (spec ×2) |
| TC-O-07 | Produk tak dikenal | `404 Produk ID...` | Ya (spec) |
| TC-O-08 | Tanpa supplier / pihak customer/partner / arsip | `400` 3 varian pesan | Ya (spec ×4) |
| TC-O-09 | Net: invoice + hari → tempo (FE+API); prepaid abaikan hari | Terhitung / null | Ya (spec ×2) |
| TC-O-10 | Purchase bebas minimum | Tanpa validasi min | Ya (spec) |
| TC-O-11 | Pindah status: sukses / tak ada / terminal / selesai-pre-terima | Ok / `404` / `400` ×2 | Ya (spec ×4) |
| TC-O-12 | Batal pasca-movement/bayar/finance | `400` 3 pesan | Ya (spec ×3) |
| TC-O-13 | Update: catatan ok; prepaid→net bersihkan tempo; ganti pihak/jenis ditolak; baris pasca-finance ditolak | Perilaku tepat | Ya (spec ×5) |
| TC-O-14 | Arsip: ok / tak ada / pasca-gerak/bayar/finance | `archivedAt` / `404` / `400` ×3 | Ya (spec ×5) |
| TC-O-15 | Cancelled-status: kind-persis dulu lalu `all`; null tanpa konfigurasi | Resolusi tepat | Ya (spec ×3) |
| TC-O-16 | List: summary ringan; cari nomor+identitas; `createdByName` tanpa bocor; badge retur (3 varian + scoping + batch Tunggal + lewati-sales + mati-saat-summary + detail) | Tepat | Ya (spec ×10) |
| TC-O-17 | approve-credit purchase | `400 tidak memerlukan...` | Ya (kode; test milik 09) |
| TC-O-18 | Update `order_kind` sales↔purchase; pajak inklusif/eksklusif per baris; termin hari 3651/negatif/pecahan | Validasi ulang / rumus / `400` | GAP (sebagian) |

## TC-R — Penerimaan (TC-R-01…TC-R-14)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-R-01 | Terima penuh | Movement + selesai + tanggal + history otomatis | Ya (spec) |
| TC-R-02 | Multi-lokasi satu penerimaan | Saldo per lokasi tepat | Ya (spec) |
| TC-R-03 | Tak ada / bukan-purchase / sudah-penuh / terminal | `404` / `400` ×3 | Ya (spec ×4) |
| TC-R-04 | Parsial 50+10 pilah lokasi → null tanggal; over 60/50 → `400 sisa`; sisa → tanggal + selesai; daftar ≥2; utang > 0 | Alur E2E penuh | Ya (E2E 05-1) |
| TC-R-05 | Prabayar belum lunas → `prabayar\|lunas`; lunasi 300rb → terima → penuh + stok 10 | Blokir-cair | Ya (E2E 05-2 + spec) |
| TC-R-06 | COD 2+2: tanpa keputusan → akhir wajib pilih; `pay_now` → id bayar + saldo 0 | Keputusan akhir | Ya (E2E 05-3 + spec ×3) |
| TC-R-07 | `pay_now` tanpa `payment.create` | `403` | Ya (spec) |
| TC-R-08 | COD → net: termin + tempo + approve + audit sumber-penerimaan | Switch tepat | Ya (spec) |
| TC-R-09 | Lokasi induk/nonaktif/default-hilang; tanggal tak valid; item non-stok; bukti gagal | Pesan / tanpa-movement / tetap-sah | GAP (sebagian) |
| TC-R-10 | Alias create (`order_id`, `stock_location_id`, `quantity_received`, `notes`) + wajib (`Order penerimaan wajib diisi`, `Item order...`) | Normalisasi tepat | GAP (kode saja) |
| TC-R-11 | Daftar/detail penerimaan + saring order + urut | Tepat / `404` | GAP (kode saja) |
| TC-R-12 | Header seragam vs campur (id lokasi vs null) | Aturan tepat | GAP |
| TC-R-13 | Backdate tanggal terima (movement + tanggal PO ikut masa lalu) | Tercatat masa lalu | GAP → KI baru |
| TC-R-14 | Dialog kosong (tanpa sisa) bisa dibuka → API menolak | UX buntu (E-17-area) | GAP → KI baru |

## TC-S/E — Status & export (TC-S-01…TC-S-08)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-S-01 | Status: list urut; kode ganda `409`; kind invalid `400`; update label; transisi buat/ubah; tak dikenal `404` ×2 | Tepat | Ya (spec, sebagian) |
| TC-S-02 | Seed 5 status + 6 transisi (`all`) | Fondasi purchase | Ya (seed; dibaca E2E implisit) |
| TC-S-03 | Export: validasi (jenis/format/tanggal/balik/366/5000) | `400` spesifik | Ya (spec 196 baris, umum) |
| TC-S-04 | Export purchase: file + sheet + invoice supplier + snapshot | Tepat | Ya (E2E 16 umum + kode) |
| TC-S-05 | UI: tab/filter/tabel/termin/badge; form saran-beli/derivasi-tempo; dialog terima 5 label; toast 5 deskripsi | Tepat | GAP (E2E UI volume menyentuh sebagian) |
| TC-S-06 | Badge Diretur mati di daftar | Selalu false (E-30) | GAP → KI baru |
| TC-S-07 | `date_to` mentah kehilangan order hari-akhir | Selisih FE vs API (E-29) | GAP → KI baru |
| TC-S-08 | Nomor darurat `ORD-{id}` / `PAY-{id}-{ts}` + bulan = server (bukan tanggal order) | Fallback + zona | GAP → KI baru |

## GAP (G-01…G-08)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Backdate penerimaan (masa lalu) | Movement + tanggal PO masa lalu; efek HPP/finance |
| G-02 | Dialog terima tanpa sisa + `requires_payment_completion` mati | UX buntu + field mati |
| G-03 | Badge Diretur daftar + `date_to` mentah + nomor darurat | 3 KI tampilan/API |
| G-04 | Alias `goods-receipts/create` + daftar/detail penerimaan | Kontrak kompatibilitas tanpa jaring |
| G-05 | Ganti `order_kind`, pajak per-baris, batas hari | Jalur update jarang-teruji |
| G-06 | Invoice supplier ganda + termin hari 0 vs null | Keputusan produk |
| G-07 | UI purchase (saran, derivasi, label dinamis, 5 toast) | Tanpa jaring FE khusus purchase |
| G-08 | Volume 50 PO (E2E 11) untuk sisi beli | Performa daftar + nomor bulanan |
