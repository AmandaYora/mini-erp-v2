# Test Cases — Modul 07 Member Type & Member Pricing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`member-type.service.spec.ts` (142 baris), `member-pricing.service.spec.ts` (151 baris),
pemakaian terkunci di `order.service.spec.ts` (bypass minimum), `order-pricing.service.spec.ts`
(delegasi), `order-form-page.test.tsx` (requote + klem), E2E 20 (pintu member) + E2E 22-B (guard
arsip). **GAP** = belum ada test.

---

## TC-M — Master rule (TC-M-01…TC-M-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-M-01 | `create {code:'a', ...purchase+1000}` | `code:'A'` + audit `member_type.create` | Ya (spec) |
| TC-M-02 | `create` kode aktif dipakai | `409 ConflictException` | Ya (spec) |
| TC-M-03 | `create {price_adjustment_value:-1}` | `400 BadRequestException` | Ya (spec) |
| TC-M-04 | `update {name}` tanpa id / `archive(undefined)` | `400`, tanpa query | Ya (spec ×2) |
| TC-M-05 | `archive(1)` dipakai 2 aktif | `400 '...masih digunakan oleh 2 pelanggan aktif'`, tanpa save/audit | Ya (spec) |
| TC-M-06 | `update {status:'inactive'}` dipakai 1 aktif | `400 '...1 pelanggan aktif...'`, tanpa save/audit | Ya (spec) |
| TC-M-07 | `create {code:'A', name:'Member Grosir', subtract 10% selling, active}` (E2E) | Tersimpan; badge + picker label `(kode)` | Ya (E2E 20) |
| TC-M-08 | Arsip dipakai → error `/pelanggan\|digunakan\|masih/`; lepas → arsip lolos | Guard relasi dua arah | Ya (E2E 22-B) |
| TC-M-09 | Kode milik arsip dipakai ulang | **500** (tanpa pesan) | GAP → KI-71 |
| TC-M-10 | Update kode/ganti basis/naikkan persen pada rule dipakai | Lolos; order lama tetap | GAP (kunci future-only) |
| TC-M-11 | `rounding_increment: 0` vs negatif vs tanpa mode | null / `400` / diam-tanpa-efek | GAP → KI-73 |
| TC-M-12 | List `limit: 0`/negatif; edit id >1000 | Diteruskan / layar tak-ditemukan | GAP → KI-75 |

## TC-H — Hitung harga (TC-H-01…TC-H-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-H-01 | Beli 120rb (box12→pack3) +1000 | basis 30.000, unit 31.000, `member_rule` | Ya (spec) |
| TC-H-02 | Jual 20rb +1,5% | basis 20.000, unit 20.300 | Ya (spec) |
| TC-H-03 | Min 12.500 −500 | basis 12.500, unit 12.000 | Ya (spec) |
| TC-H-04 | Jual 1000 −2000 | `400 BadRequest` (negatif) | Ya (spec) |
| TC-H-05 | Basis beli + faktor 0 | `400 BadRequest` (konversi) | Ya (spec) |
| TC-H-06 | Tanpa member, jual 15.000 | `standard`, 15.000, member null | Ya (spec) |
| TC-H-07 | `quote {id_member_type:-1}` | `400` sebelum query repo | Ya (spec) |
| TC-H-08 | Sales + member → bypass minimum (jual 75rb, min 50rb, beli 10rb) | Harga rule lolos guard | Ya (order spec) |
| TC-H-09 | Sales tanpa member → `resolvePrice(product, null, manual)` | Delegasi null | Ya (order-pricing spec) |
| TC-H-10 | Requote ganti pelanggan: baris manual lestari, diskon di-klem ulang | Merge + klem (order-form test) | Ya (FE test) |
| TC-H-11 | Rounding up/nearest/down + basis beli-jual beda + persen di atas minimum | Nilai batas tepat | GAP (hanya none teruji) |
| TC-H-12 | Quote degradasi diam (pihak tanpa member) vs error eksplisit | Keduanya terkunci sebagai perilaku | GAP → KI-72 |

## GAP (G-01…G-06)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Kode arsip dipakai ulang (500) | Mengunci KI-71 |
| G-02 | Future-only perubahan rule (order lama + draft) | Kontrak terbesar tanpa jaring |
| G-03 | Rounding non-none + kelipatan 0/tanpa-mode | Mengunci KI-73 |
| G-04 | Batas atas besaran/persen (1000% sah) | Mengunci KI-76 |
| G-05 | Degradasi diam vs error quote | Mengunci KI-72 |
| G-06 | Baris quote-gagal di POS (lestari vs reset) + tanpa-`order.create` | Perilaku kasir tak teruji |
