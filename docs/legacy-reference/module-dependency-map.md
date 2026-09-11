# Module Dependency Map — Mini ERP Legacy

Peta ketergantungan antar-modul, diturunkan dari **impor kode nyata** dan **tabel yang dibaca
lewat SQL mentah** — bukan dari perkiraan. Tujuannya: menentukan **urutan aman** membangun ulang,
dan mengetahui apa yang rusak bila satu modul diubah.

Berkas terkait: [00-overview.md](00-overview.md) · [00-module-map.md](00-module-map.md) ·
[open-questions.md](open-questions.md)

---

## 1. Peringatan Penting Sebelum Membaca

**Ketergantungan di sistem ini punya dua bentuk, dan yang kedua tidak terlihat dari `import`:**

| Bentuk | Cara mendeteksi | Contoh |
|---|---|---|
| **Ketergantungan kode** | `import` antar folder module | `order/` mengimpor `product/`, `stock/`, `payment/` |
| **Ketergantungan data** | Query SQL mentah ke tabel milik modul lain | `finance/` hanya mengimpor `audit-log/`, tapi **membaca 12 tabel** milik order, stok, payment, dan retur |

Modul **Finance** adalah contoh paling ekstrem: dari sisi kode ia tampak hampir mandiri, padahal
ia **konsumen terbesar** di seluruh sistem. Rebuild yang hanya mengikuti grafik `import` akan
salah menilai modul ini sepenuhnya.

---

## 2. Grafik Ketergantungan Kode (API)

Diambil dari impor lintas-folder di `apps/api/src/modules/`.

```
                          ┌──────────┐
                          │ storage  │  (tanpa ketergantungan)
                          └────┬─────┘
                               │
        ┌──────────┐      ┌────┴─────┐      ┌───────────┐
        │ audit-log│◄─────┤   auth   ├─────►│  branch   │
        └────▲─────┘      └────▲─────┘      └─────▲─────┘
             │                 │                  │
   ┌─────────┼─────────────────┼──────────────────┼──────────┐
   │         │                 │                  │          │
┌──┴───┐ ┌───┴────┐ ┌──────────┴───┐ ┌────────┐ ┌─┴──────┐ ┌─┴─────┐
│ user │ │company │ │   product    │ │knowledge│ │ stock │ │finance│
└──▲───┘ └───▲────┘ └──────▲───────┘ └────▲───┘ └───▲───┘ └───▲───┘
   │         │             │              │         │         │
   │         │      ┌──────┴────────┐     │         │         │
   │         │      │business-party │     │         │         │
   │         │      └──────▲────────┘     │         │         │
   │         │             │              │         │         │
   └─────────┴─────────────┼──────────────┼─────────┴─────────┘
                           │              │
                    ┌──────┴──────┐       │
                    │    order    │◄──────┤
                    └──────▲──────┘       │
                           │              │
                    ┌──────┴──────┐  ┌────┴────┐  ┌───────────┐
                    │   payment   │  │  tools  │  │ reporting │
                    └─────────────┘  └────▲────┘  └───────────┘
                                          │
                                   ┌──────┴──────┐
                                   │  assistant  │◄──── whatsapp
                                   └─────────────┘
                                                        dashboard
```

### Tabel ketergantungan langsung (arah: A → B berarti "A membutuhkan B")

| Modul | Bergantung pada |
|---|---|
| `storage` | — (**satu-satunya modul tanpa ketergantungan**) |
| `auth` | `branch`, `user` |
| `audit-log` | `auth` |
| `user` | `auth`, `audit-log` |
| `company` | `auth`, `audit-log` |
| `branch` | `auth`, `audit-log`, `user`, `stock` |
| `product` | `auth`, `audit-log`, `storage` |
| `business-party` | `auth`, `audit-log`, `product` |
| `knowledge` | `auth`, `audit-log`, `storage` |
| `stock` | `auth`, `audit-log`, `branch`, `company`, `product`, `user`, **`finance`** |
| `finance` | `auth`, `audit-log` *(kode)* — tapi lihat §3 |
| `order` | `auth`, `audit-log`, `branch`, `business-party`, `product`, `stock`, `payment`, `storage` |
| `payment` | `auth`, `audit-log`, `branch`, `business-party`, `order`, `storage` |
| `reporting` | `auth`, `branch`, `order`, `stock` |
| `tools` | `branch`, `knowledge`, `order`, `product`, `stock` |
| `assistant` | `auth`, `tools` |
| `whatsapp` | `auth`, `assistant`, `company` |
| `dashboard` | `auth`, `knowledge`, `order`, `product`, `whatsapp` |

### Tiga siklus yang nyata di kode

Ini penting untuk rebuild — ketiganya **tidak bisa dipecah tanpa keputusan sadar**:

| # | Siklus | Sifatnya |
|---|---|---|
| 1 | `auth` ⇄ `branch` ⇄ `user` | Auth butuh cabang & pengguna untuk membentuk sesi; cabang butuh akses-pengguna; pengguna butuh auth. **Melingkar erat** — di sistem baru ketiganya sebaiknya satu modul "Identitas & Organisasi" |
| 2 | `order` ⇄ `payment` | Order membuat pembayaran (COD, uang muka); pembayaran mengubah status order. **Melingkar** |
| 3 | `stock` ⇄ `finance` | Stok memanggil finance saat saldo awal persediaan; finance membaca mutasi stok untuk HPP. **Melingkar** |

---

## 3. Ketergantungan Data yang Tidak Terlihat dari Impor

Modul yang membaca tabel milik modul lain lewat SQL mentah:

### 3.1 Finance — konsumen terbesar sistem

Hanya mengimpor `audit-log`, tetapi membaca **12 tabel milik modul lain**:

| Tabel yang dibaca | Pemiliknya | Untuk apa |
|---|---|---|
| `orders`, `order_items` | Order (08/09) | Pendapatan, DPP, margin |
| `inventory_movements` | Stock (16) | Basis HPP |
| `products` | Product (05) | Harga beli sebagai basis biaya |
| `payments`, `payment_allocations` | Payment (14) | Kas masuk & alokasi |
| `goods_receipts`, `goods_receipt_items` | Goods Receipt (10) | Nilai persediaan masuk |
| `sales_returns`, `sales_return_items`, `sales_return_settlements` | Sales Return (12) | Koreksi pendapatan |
| `purchase_returns`, `purchase_return_settlements` | Purchase Return (13) | Koreksi pembelian |
| `branches` | Branch (04) | Dimensi cabang di jurnal |

**Konsekuensi untuk rebuild:** setiap perubahan bentuk tabel di modul 04, 05, 08–14, atau 16
**berpotensi memecahkan Finance secara diam-diam** — tidak akan ketahuan dari kompilasi
TypeScript, hanya dari angka laporan yang salah. Ini alasan Finance harus dibangun **paling
akhir** dan diberi test integrasi paling ketat.

### 3.2 Dashboard — pembaca lintas-modul kedua

Membaca 9 tabel: `orders`, `order_items`, `payments`, `payment_allocations`,
`inventory_balances`, `stock_issue_allocations`, `sales_returns`, `sales_return_items`,
`sales_return_settlements`.

### 3.3 Reporting — job agregasi harian

Mengiterasi cabang aktif dan menulis `daily_operational_metrics`. Membaca order dan stok.

---

## 4. Lapisan Ketergantungan (urutan aman membangun ulang)

Setiap lapisan hanya boleh bergantung pada lapisan **di atasnya**.

| Lapisan | Modul | Kenapa di sini |
|---|---|---|
| **L0 — Infrastruktur** | `shared/` (envelope, guard, decorator, util uang/kuantitas/tanggal), Storage/Media | Tidak bergantung apa pun; **wajib pertama** |
| **L1 — Identitas & Organisasi** | 01 Auth, 02 Users/Roles, 03 Company, 04 Branch | Saling melingkar (siklus #1) — **bangun sebagai satu paket** |
| **L2 — Master Data** | 05 Product, 06 Business Party, 07 Member Pricing | Bergantung L1 saja. 06 butuh 05; 07 butuh 06 |
| **L3 — Stok** | 16 Stock & Gudang | Butuh L1 + 05. Punya siklus dengan Finance (saldo awal) |
| **L4 — Transaksi Inti** | 08 Order-Purchasing, 09 Order-Sales | Butuh L1, L2, L3 |
| **L5 — Turunan Transaksi** | 10 Goods Receipt, 11 Delivery, 14 Payment | Butuh L4. 14 melingkar dengan L4 |
| **L6 — Retur** | 12 Sales Return, 13 Purchase Return | Butuh L4 + L5 (retur menunjuk order & pengiriman) |
| **L7 — Kasir** | 15 POS | Tanpa backend sendiri — **merakit** 05, 06, 07, 09, 14, 16 |
| **L8 — Keuangan** | 17 Finance & Pajak | **Membaca hampir semua di atasnya** (§3.1). Paling akhir |
| **L9 — Pantauan** | 18 Dashboard, 19 Reporting, 20 Audit Log | Baca-saja lintas modul. 20 sebenarnya L0 (dipakai 27 produsen) tapi UI-nya paling akhir |
| **L10 — Kecerdasan** | 21 Assistant + WhatsApp + Knowledge/RAG | Membaca L2–L8 lewat 8 tool baca-saja |

**Catatan L9:** modul 20 Audit Log punya kedudukan ganda. **Service-nya** milik L0 — dipanggil 27
produsen di hampir semua modul, jadi harus ada sejak awal. **Halaman & endpoint pembacanya**
boleh dibangun paling akhir.

---

## 5. Modul Paling Berisiko Diubah

Diukur dari berapa banyak modul yang rusak bila modul ini berubah bentuk.

| Modul | Dipakai oleh | Risiko |
|---|---|---|
| **04 Branch** | ~26 tabel menaut `id_branch`; seluruh transaksi, jurnal, metrik | 🔴 Tertinggi. Mengubah cara cabang diidentifikasi menyentuh seluruh sistem |
| **05 Product** | Order, Stock, POS, Finance (HPP), Business Party, Assistant | 🔴 Tinggi. Satuan & faktor konversi adalah kontrak lintas modul |
| **16 Stock** | Order, Delivery, Goods Receipt, Retur, Finance, Dashboard, Assistant | 🔴 Tinggi. Saldo & mutasi jadi basis HPP |
| **08/09 Order** | Payment, Delivery, Goods Receipt, Retur, POS, Finance, Dashboard, Reporting | 🔴 Tinggi. Modul terbesar kedua (9.057 baris) |
| **01 Auth** | Semua modul (guard + `@ActiveSession`) | 🟠 Sedang-tinggi. Bentuk sesi adalah kontrak universal |
| **20 Audit Log** | 27 produsen | 🟠 Sedang. Kontraknya sempit dan stabil |
| **17 Finance** | Hampir tidak ada yang bergantung padanya (hanya Stock, untuk saldo awal) | 🟢 Rendah **sebagai dependensi** — tapi 🔴 paling rapuh **sebagai konsumen** |
| **15 POS** | Tidak ada | 🟢 Terendah. Murni perakit; boleh dibangun ulang kapan saja |

---

## 6. Kontrak Lintas Modul yang Tidak Boleh Berubah

Hal-hal yang **beberapa modul sepakati bersama**. Mengubah salah satunya memecahkan lebih dari
satu modul sekaligus.

| # | Kontrak | Pemilik | Konsumen |
|---|---|---|---|
| 1 | **Cakupan selalu dari sesi** (`idCompany`/`idBranch`/`idUser`/role/permission) | 01 Auth | Semua |
| 2 | **Envelope respons** `{code, info, data}` + galat `{code, info, data:null, errors}` | shared | Semua |
| 3 | **Satuan stok + faktor konversi** — semua saldo disimpan dalam satuan stok | 05 Product | 08–16, 17 |
| 4 | **Snapshot `OrderItem`** — dokumen historis menampilkan salinan, bukan join produk hidup | 08/09 Order | 11–15, 17 |
| 5 | **Setiap `InventoryBalance` berpasangan `InventoryMovement`**, resolve ke lokasi daun | 16 Stock | 08–13, 17 |
| 6 | **Transfer stok = dua mutasi** (`out` + `in`), bukan tipe `transfer` | 16 Stock | 17 |
| 7 | **Prefix nomor dokumen diturunkan dari kode cabang** | 04 Branch | 08–14 |
| 8 | **Stok berkurang saat `deliveries/create`**, bukan saat confirm | 11 Delivery | 16, 17 |
| 9 | **Harga beli produk = basis HPP** | 05 Product | 17 Finance |
| 10 | **Rentang tanggal finance memakai zona Asia/Jakarta**, bukan `YYYY-MM-DD` mentah | 17 Finance | 19 Reporting |
| 11 | **Audit `actionKey` snake_case** `resource.action` | 20 Audit Log | 27 produsen |
| 12 | **Varian default tersembunyi** — setiap produk selalu punya satu baris varian | 05 Product | 16 Stock, 15 POS |

Detail tiap kontrak ada di `business-rules.md` modul pemiliknya.

---

## 7. Modul yang Bisa Dibangun Paralel

Setelah L0–L2 selesai, pekerjaan berikut **tidak saling menunggu**:

| Kelompok | Modul | Syarat |
|---|---|---|
| A | 16 Stock | L1 + 05 |
| B | 06 Business Party + 07 Member Pricing | L1 + 05 |
| C | 20 Audit Log (UI) | L1 |
| D | 03 Company Settings (UI) | L1 |

Setelah L4 (Order) selesai:

| Kelompok | Modul | Syarat |
|---|---|---|
| E | 10 Goods Receipt | 08 |
| F | 11 Delivery | 09 + 16 |
| G | 14 Payment | 08 + 09 |
| H | 19 Reporting | 08/09 + 16 |

**Yang tidak boleh diparalelkan:** 12/13 Retur (butuh 10/11 selesai), 17 Finance (butuh semuanya),
15 POS (butuh 05, 06, 07, 09, 14, 16).

---

## 8. Ketergantungan Sisi Web

Struktur frontend **tidak sejajar** dengan backend — 17 folder modul web memetakan ke 18 modul API
dengan beberapa penggabungan:

| Folder web | Modul API yang dilayani | Catatan |
|---|---|---|
| `auth/` | auth | + halaman pilih cabang |
| `users/` | user | |
| `company/` | company, **branch** | Halaman cabang berada di folder company |
| `products/` | product | |
| `business-party/` | business-party | |
| `member-types/` | business-party (sub) | Member type & pricing hidup di modul API business-party |
| `orders/` | order (purchasing + sales + delivery + goods receipt) | Satu folder untuk 4 modul dokumen |
| `sales-returns/`, `purchase-returns/` | order (sub) | |
| `stock/` | stock | |
| `finance/` | finance | 18 halaman |
| `pos/` | — | **Tanpa backend sendiri** |
| `dashboard/`, `audit-log/`, `assistant/` | dashboard, audit-log, assistant+whatsapp+knowledge | |
| `core/`, `shared/` | — | Lapisan bersama |

Seluruh frontend juga bergantung pada **satu titik tunggal** yang harus ada lebih dulu:

| Berkas | Peran |
|---|---|
| `lib/api.ts` | **Satu-satunya** tempat yang boleh memanggil `fetch` |
| `store/app-store.tsx` + `store/slices/*` | State global; tiap modul punya slice |
| `modules/module-registry.tsx` | Sumber tunggal rute + menu + permission (79 rute) |
| `components/` | Design system — yang menjaga "tampilan sama persis" |

---

## 9. Urutan Rebuild yang Disarankan

Diturunkan dari §4 dan §7, dengan modul berisiko tinggi didahulukan agar kesalahan arsitektur
ketahuan awal.

| Tahap | Isi | Alasan |
|---|---|---|
| 1 | Infrastruktur L0 + Audit Log (service) | Tidak ada yang bisa jalan tanpa ini |
| 2 | L1 Identitas & Organisasi (01–04) sebagai **satu paket** | Siklus erat; kontrak sesi dipakai semua |
| 3 | 05 Product | Kontrak satuan menentukan bentuk Stock, Order, dan Finance |
| 4 | 16 Stock | Kontrak saldo/mutasi menentukan bentuk Order & Finance |
| 5 | 06 + 07 (paralel dengan 4) | Independen dari stok |
| 6 | 08 + 09 Order | Modul terbesar kedua |
| 7 | 10, 11, 14 (paralel) | Turunan order |
| 8 | 12, 13 Retur | Butuh 10/11 |
| 9 | 15 POS | Perakit; ujian nyata bahwa L2–L5 sudah benar |
| 10 | 17 Finance | Konsumen terbesar; kesalahan di lapisan bawah muncul di sini |
| 11 | 18, 19, 21 | Baca-saja; boleh menyusul |

**Titik uji paling berharga:** akhir tahap 9. Bila POS bisa menjual produk bervarian dengan harga
member, mencetak struk, mengurangi stok, dan mencatat pembayaran — berarti sepuluh kontrak di §6
sudah benar sebelum Finance dibangun.
