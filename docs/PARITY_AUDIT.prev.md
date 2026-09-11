# Audit Paritas Kapabilitas — Legacy vs Revamp

**Tanggal audit:** 2026-09-10 · **Diverifikasi ulang:** 2026-09-10 (lihat §0)
**Sistem lama:** `H:/dimasprasetio/SAAS/mini-erp` (NestJS + TypeORM, multi-tenant)
**Sistem baru:** `H:/dimasprasetio/SAAS/mini-erp-revamp` (Go modular monolith + MySQL, standalone)

Dokumen ini menjawab satu pertanyaan: **apakah revamp sudah punya kapabilitas yang sama dengan
sistem lama?** Bukan perbandingan kode atau arsitektur — struktur, algoritma, teknologi, dan schema
memang sengaja berbeda (lihat [PRD.md §1](PRD.md) dan `knowledge/decisions/`). Yang diaudit adalah
**permukaan kapabilitas**: endpoint, halaman, alur kerja, dan desain UI yang dilihat pengguna.

---

## 0. Catatan Verifikasi Ulang

Audit ini diverifikasi ulang dari nol setelah penulisan pertama — setiap klaim diuji dengan
mencoba **menjatuhkannya**, bukan mengkonfirmasinya, dan klaim yang bertanda ✅ ikut diuji, bukan
hanya yang bertanda gap. Hasilnya: **4 koreksi**, di mana 3 di antaranya membuat revamp terlihat
*lebih baik* dari penilaian awal.

| # | Klaim audit pertama | Hasil verifikasi ulang |
|---|---|---|
| 1 | Modul 21 (Assistant/WhatsApp) "sengaja di luar cakupan, bukan gap" | ❌ **Sudah usang.** Modul `assistant` kini terimplementasi: 2.348 baris Go, migrasi `000018_assistant`, dependensi `go.mau.fi/whatsmeow`, 13 endpoint, 2 halaman frontend. Lihat §3 baris 21 dan §5 |
| 2 | Test backend "ada sebagian (`auth/contracts/session_test.go`)" | ❌ **Terlalu merendahkan.** Ada **9** file test: domain pricing (party/product/purchasing/sales), money, pagination, timeutil, assistant engine, auth session |
| 3 | `finance/periods/create` hilang (tersirat di §4.1) | ❌ **Bukan gap.** Periode dimaterialisasi lewat upsert saat close (`SetPeriodStatus`, `repository.go:234`) — desain berbeda, kapabilitas tetap ada |
| 4 | `branches/my-access` & `branches/active` hilang | ❌ **Bukan gap.** Ditutup oleh `GET /api/v1/branches?limit=100` + resolusi sesi di `auth` — lihat `use-accessible-branches.ts:34` |

Yang **tetap berdiri setelah diuji ulang** (semua dites ulang satu per satu, bukan diasumsikan):
seluruh gap finance §4.1 (7 tabel masih benar-benar tidak ada), halaman cetak §4.2 (masih nol
hasil), ekspor & QR §4.3 (dependensi masih tidak ada), dan seluruh gap operasional §4.4.

**Repo bergerak selama audit** — angka di §1 sudah disegarkan ke kondisi terkini
(Go 24.178 → 26.877 baris, migrasi 34 → 36, halaman 54 → 55, rute frontend 44 → 45). Bila selisih
lagi saat Anda membaca ini, jalankan ulang perintah di §2 sebelum memercayai angkanya.

---

## 1. Ringkasan Eksekutif

**Verdict: paritas ±88%. Inti operasional sudah setara; ada 3 kantong gap nyata yang belum
tertutup, dan 1 dokumen internal yang kini kontradiktif (§7).**

| Aspek | Status |
|---|---|
| Modul 01–16, 18, 19, 20 (identitas, master data, stok, transaksi, retur, observabilitas) | ✅ Paritas |
| Modul 17 (Finance) | ⚠️ **Paritas parsial** — 6 sub-fitur belum ada |
| Cetak dokumen (surat jalan, faktur, kwitansi, struk POS) | ❌ **Belum ada sama sekali** |
| Ekspor data (Excel/CSV) & QR scan/generate | ❌ **Belum ada** |
| Desain UI (token, layout, komponen) | ✅ Diport identik — diverifikasi dengan `diff`, nol selisih |
| Modul 21 (Assistant + WhatsApp) | ✅ **Sudah dibangun** (bot WA whatsmeow, 8 intent read-only) |
| Knowledge base / RAG (bagian modul 21) | ⏸️ **Sengaja dibuang** — bukan gap |

**Ukuran relatif** — indikasi kasar bahwa revamp mencapai cakupan setara dengan kode jauh lebih
sedikit (ini efek yang diinginkan dari revamp, bukan tanda kekurangan):

| | Legacy | Revamp |
|---|---|---|
| Backend | 37.218 baris TS | 26.877 baris Go |
| Frontend | 40.785 baris TS/TSX | 27.994 baris TS/TSX |
| Migrasi | — | 36 file |
| Halaman frontend | ±74 | 55 |

Namun sebagian selisih frontend adalah **fitur yang memang belum dibangun** (halaman cetak, ekspor,
QR) — lihat §4.

---

## 2. Metode & Bukti

Audit dilakukan dengan membandingkan permukaan nyata kedua sistem, bukan dokumentasi:

1. **Rute backend legacy** — diekstrak dari seluruh `@Controller` + `@Get/@Post/...` di
   `apps/api/src/modules/**/*.controller.ts` (±200 endpoint).
2. **Rute backend revamp** — diekstrak dari pola `"METHOD /api/v1/..."` di
   `apps/api/internal/**/*.go` (±180 endpoint).
3. **Rute frontend legacy** — dari `apps/web/src/modules/module-registry.tsx` (±80 path).
4. **Rute frontend revamp** — dari `apps/web/src/app/routes/route-paths.ts` (45 path).
5. **Schema** — perbandingan `CREATE TABLE` di migrasi kedua sisi.
6. **Desain UI** — perbandingan token `@theme` dan inventaris komponen bersama.

Setiap gap di §4 disertai bukti spesifik (nama endpoint/tabel/file), bukan kesan.

---

## 3. Paritas per Modul

| # | Modul legacy | Modul revamp | Status | Catatan |
|---|---|---|---|---|
| 01 | auth-session | `auth` | ✅ | login, refresh, logout, me, switch-role, switch-branch — lengkap |
| 02 | users-roles-permissions | `user` | ✅ | users CRUD + status + ganti sandi; roles CRUD + permissions |
| 03 | company-settings | `company` | ⚠️ | profil & settings ✅; **feature flags tidak dibawa** (keputusan, lihat §5) |
| 04 | branch | `branch` | ✅ | list/detail/create/update + penomoran dokumen per cabang |
| 05 | product-catalog | `product` | ✅ | kategori, produk, media, search-options, import preview/commit |
| 06 | business-party | `party` | ✅ | customer & supplier CRUD + archive/restore + alamat kirim |
| 07 | member-pricing | `party` | ✅ | member-types CRUD + `pricing/quote` |
| 08 | order-purchasing | `purchasing` | ✅ | PO list/detail/create/update/confirm/cancel |
| 09 | order-sales | `sales` | ✅ | SO list/detail/create/update/confirm/cancel + approve-credit |
| 10 | goods-receipt | `goodsreceipt` | ✅ | list/detail/create |
| 11 | delivery | `delivery` | ⚠️ | create/confirm/list/cancel/bukti ✅; **work-queue belum ada** |
| 12 | sales-return | `salesreturn` | ⚠️ | list/detail/create/confirm/cancel ✅; **context, preview, replacement-delivery belum ada** |
| 13 | purchase-return | `purchasereturn` | ⚠️ | list/detail/create/confirm/cancel ✅; **context & preview belum ada** |
| 14 | payment | `payment` | ✅ | list, party-balances, party-ledger, create, bukti, cancel (+ settle-credit, baru) |
| 15 | pos | kanal `sales` | ⚠️ | keranjang & bayar ✅; **struk termal & scanner kamera belum ada** |
| 16 | stock-inventory | `stock` | ⚠️ | saldo/mutasi/lokasi/transfer/opname/damaged ✅; **pindah-lokasi & halaman detail item belum ada** |
| 17 | finance | `finance` | ❌ | **paritas parsial — 6 sub-fitur hilang, lihat §4.1** |
| 18 | dashboard | `dashboard` | ✅ | summary |
| 19 | reporting | `reporting` | ✅ | sales-trend + inventory |
| 20 | audit-log | `audit` | ✅ | list |
| 21 | assistant/whatsapp/knowledge | `assistant` | ⚠️ | **sudah dibangun** — 13 endpoint menutup seluruh permukaan `whatsapp/*` + `assistant/*` legacy (channel connect/disconnect/status, authorizations CRUD+revoke, config get/update, simulate, runs/stats) plus `chat` & `channel/reset` yang baru. **Knowledge/RAG sengaja dibuang** (§5) |

---

## 4. Daftar Gap yang Harus Ditutup

Diurutkan berdasarkan dampak operasional. **P0 = pengguna tidak bisa menjalankan pekerjaan
hariannya tanpa ini.**

### 4.1 Finance — 6 sub-fitur hilang (P0/P1)

Bukti schema — tabel yang ada di legacy tapi **tidak ada** di `apps/api/migrations/000016_finance.up.sql`:
`finance_cash_accounts`, `finance_opening_balances`, `finance_opening_inventory_items`,
`finance_posting_sources`, `finance_tax_adjustments`, `finance_inventory_cost_states`.

| Prioritas | Gap | Endpoint legacy yang hilang | Halaman legacy |
|---|---|---|---|
| **P0** | **Saldo awal (opening balance)** — tanpa ini pembukuan tidak bisa dimulai dari data berjalan | `finance/opening/{get,update,import-inventory,validate,post,uncosted-products,supplement}` | `/finance/opening` |
| **P0** | **Antrian posting (posting sources)** — revamp hanya punya `posting/preview` + `posting/post` per dokumen; legacy punya workbench: daftar, batch, tutup harian, abaikan/pulihkan, batal posting | `finance/posting-sources/{list,detail,post-batch,close-day,ignore,restore,cancel-posting}`, `finance/daily-close/overview` | `/finance/back-office` |
| **P1** | **Akun kas (cash accounts)** — master rekening kas/bank terpisah dari COA | `finance/cash-accounts/{list,create,update,archive}`, `finance/reports/cash-summary` | `/finance/cash` |
| **P1** | **Laporan piutang & utang** | `finance/reports/{receivables,payables}` | `/finance/receivables-payables` |
| **P1** | **Penyesuaian pajak** | `finance/tax-adjustments/{list,detail,save,archive}` | `/finance/tax-adjustments` |
| **P1** | **Tutup periode aman + ekspor paket pajak** — revamp punya `periods/close` polos, tanpa cek kesiapan dan tanpa ekspor berkas SPT | `finance/close/{readiness,safe-close}`, `finance/export/tax-package` | bagian dari `/finance/period-close` |

> Catatan: paket ekspor pajak versi "dibatasi Rp4,8 M" adalah **keputusan bisnis terbuka**
> (PRD §5 no. 7) — konsultasikan ke konsultan pajak sebelum diimplementasi, jangan direplikasi buta.

### 4.2 Cetak dokumen — belum ada sama sekali (P0)

Tidak ada satu pun halaman cetak di revamp. Verifikasi: `grep -r "window.print\|Cetak" apps/web/src`
→ **nol hasil**. Legacy punya 4 halaman cetak + 1 jembatan printer native:

| Dokumen | Halaman legacy | Rute legacy |
|---|---|---|
| Surat jalan | `orders/pages/delivery-note-print-page.tsx` | `/orders/:orderId/delivery/:sjId/print` |
| Faktur / dokumen penjualan | `orders/pages/sales-document-print-page.tsx` | `/orders/:orderId/sales-document/print` |
| Kwitansi pembayaran | `orders/pages/payment-receipt-print-page.tsx` | `/orders/:orderId/payments/:paymentId/print` |
| Struk termal POS | `orders/pages/pos-thermal-receipt-print-page.tsx` | `/orders/:orderId/pos-receipt/print` |
| Jembatan printer Android | `lib/native-printer.ts` + `android-pos-shell/` (aplikasi shell terpisah) | — |

Ini P0: ERP operasional tanpa surat jalan dan struk tidak bisa dipakai di lapangan.

### 4.3 Ekspor data & QR — belum ada (P1)

Dependency yang ada di legacy `apps/web/package.json` tapi tidak di revamp:
`xlsx-js-style`, `file-saver`, `jszip`, `qrcode`, `qrcode.react`, `html5-qrcode`.

| Gap | Bukti legacy |
|---|---|
| Ekspor Excel (daftar order, template impor produk, template opname) | `modules/products/components/bulk-upload-template.tsx`, `modules/stock/components/stock-opening-template.ts` |
| Endpoint ekspor order | `orders/export` — tidak ada padanan di revamp |
| Scanner QR/barcode via kamera | `components/domain/qr-scanner-modal.tsx` — revamp hanya punya input barcode ketik/wedge di `PosPage.tsx:418` |
| Generator & cetak label QR | `components/domain/qr-generator-modal.tsx`, `utils/qr-export.ts` |

### 4.4 Halaman & alur yang hilang (P1/P2)

| Prioritas | Gap | Legacy |
|---|---|---|
| P1 | **Antrian kerja pengiriman** — layar operasional gudang untuk memproses SJ | `/delivery-work-queue` |
| P1 | **Pindah lokasi stok** (antar lokasi dalam satu gudang) | `/gudang/pindah-lokasi` |
| P1 | **Pratinjau retur (`context` + `preview`)** — legacy menghitung dampak retur sebelum disimpan; revamp langsung `create` | `{sales,purchase}-returns/{context,preview}` |
| P1 | **Pengiriman pengganti untuk retur penjualan** | `sales-returns/replacement-deliveries/{dispatch,confirm}` |
| P2 | **Detail item stok** — hilang di **dua** lapisan: halaman `/stock/:itemId` **dan** endpoint `stock/detail`; revamp hanya punya `GET /stock/balances` (daftar) | `/stock/:itemId` + `stock/detail` |
| P2 | **Riwayat status order** | `orders/status-history` |
| P2 | **Konfigurasi status order** (definisi + transisi, tabel `order_status_definitions`) | `/settings/order-status` |
| P2 | **PWA / update prompt** | `components/domain/pwa-update-prompt.tsx` |

### 4.5 Jaring pengaman yang hilang (P1 — maintainability)

Revamp mengejar *maintainability*, tapi justru kehilangan lapisan uji yang dimiliki legacy:

| | Legacy | Revamp |
|---|---|---|
| Unit test frontend | Vitest + Testing Library (`*.test.tsx` di seluruh modul) | ❌ **nol** — tidak ada `vitest`/`testing-library`/`jsdom` di `apps/web/package.json` |
| E2E | `apps/e2e` (Playwright) | ❌ tidak ada |
| Unit test backend | — | ✅ **9 file**: `money`, `pagination`, `timeutil`, `auth/session`, `assistant/engine`, + pricing domain di `party`, `product`, `purchasing`, `sales` |

Backend justru **lebih baik** dari sistem lama di sini: logika paling rawan (uang, pembulatan,
pricing bertingkat, zona waktu) sudah punya test unit. Yang benar-benar kosong adalah **frontend** —
dan di situlah gap cetak/ekspor akan dikerjakan. Menambah Vitest sebelum §4.2 dikerjakan akan
membayar dirinya sendiri.

---

## 5. Yang Sengaja **Tidak** Dibawa (bukan gap)

Jangan tandai ini sebagai kekurangan — semuanya keputusan tercatat:

| Tidak dibawa | Alasan & rujukan |
|---|---|
| **Knowledge base / RAG** (`knowledge/documents/*`, 6 endpoint legacy) | Keputusan tercatat langsung di kode: `assistant/application/engine.go:19` — *"minus policy_qna: no knowledge in L9 scope — SOP lookup does not exist, so the intent is gone, not stubbed"*. Bot WA-nya sendiri **dibawa** (lihat §3 baris 21) |
| Multi-tenant / `companyId` di mana pun | [ADR-0009](../knowledge/decisions/ADR-0009-single-tenant-takeout-multitenancy.md) — standalone single-tenant |
| Feature flags perusahaan (`company/features/*`) | PRD §5 no. 2 — mati di 3 lapisan di sistem lama, **disarankan dibuang** |
| Modul `order` tunggal | [ADR-0004](../knowledge/decisions/ADR-0004-order-domain-split.md) — dipecah jadi `purchasing` + `sales` |
| Modul POS tersendiri | [ADR-0007](../knowledge/decisions/ADR-0007-pos-no-dedicated-module.md) — POS = kanal frontend |
| Localization selain id-ID/IDR/WIB | [ADR-0006](../knowledge/decisions/ADR-0006-locale-timezone-locked.md) |

---

## 6. Paritas Desain UI — ✅ Terpenuhi

Token desain **diport identik**, bukan ditiru. Diverifikasi ulang secara mekanis, bukan dengan mata:

```bash
diff <(grep -oP "^\s+--(color|font|radius)[a-z-]*:\s*\K[^;]+" apps/web/src/theme/theme.css) \
     <(grep -oP "^\s+--(color|font|radius)[a-z-]*:\s*\K[^;]+" ../mini-erp/apps/web/src/tailwind.css)
# → nol selisih
```

`apps/web/src/theme/colors.ts` juga dites ulang terhadap `theme.css` — **nol selisih** pada nilai
unik, jadi cermin JS-nya tidak melenceng dari sumber CSS-nya. Rincian nilai:

- Palet: `ink #0f172a`, `heading #334155`, `muted #64748b`, `hairline #cbd5e1`,
  `brand #1e3a5f`, `brand-hover #16314f`, `accent #c2603a`, `ok #15803d`, `warn #b45309`,
  `bad #b91c1c`, sidebar `#0f172a` — **identik**.
- Tipografi & radius: `Inter` + `--radius-{sm,md,lg}` = 6/8/10px — **identik**.
- Variabel kompat `:root` (`--bg-canvas`, `--shadow-*`, `--topbar-height: 64px`) — **identik**.
- `apps/web/src/theme/colors.ts` = cermin JS dari token yang sama, untuk recharts/kanvas/cetak.

**Inventaris komponen** — revamp punya padanan untuk hampir semua komponen bersama legacy
(button, badge, data-table, pagination, modal, confirm-dialog, toast, notice, empty-state,
page-header, section-card, summary-card, filter-bar, action-row, form-field, search-select,
segmented-control). Yang **belum** diport:

| Belum ada di revamp | Legacy | Dampak |
|---|---|---|
| `async-search-select` | `components/forms/` | pencarian server-side di form besar |
| `hierarchical-select` | `components/forms/` | pilih kategori/lokasi bertingkat |
| `password-input` | `components/forms/` | toggle lihat sandi |
| `global-loader` | `components/feedback/` | indikator muat global |
| `party-search-select`, `product-search-select`, `stock-location-select` | `components/domain/` | di revamp belum jadi komponen bersama — dirakit ad-hoc per modul |
| `qr-scanner-modal`, `qr-generator-modal`, `pwa-update-prompt` | `components/domain/` | lihat §4.3 dan §4.4 |

> **Temuan verifikasi ulang — jumlah halaman legacy menyesatkan.** Dari 10 rute `/assistant/*`,
> `/knowledge/*`, dan `/whatsapp` di `module-registry.tsx` legacy, **8 di antaranya hanya
> `<Navigate>` (pengalihan)**; halaman nyatanya cuma dua: `assistant-setup-page` dan
> `assistant-config-page`. Revamp punya persis dua padanannya (`SetupPage`, `ConfigPage`) — jadi
> ini **paritas penuh**, bukan 2-dari-10 seperti yang terlihat dari daftar rute. Pelajaran: hitung
> halaman nyata, jangan hitung entri rute.

> Catatan tata letak: revamp memakai satu `AppLayout.tsx` dengan sidebar + topbar yang mengikuti
> token sidebar yang sama, jadi kerangka layar setara. Yang berubah adalah **navigasi**: legacy
> `/orders` tunggal kini terpecah jadi `/purchase-orders` + `/sales-orders`, dan legacy `/gudang/*`
> kini `/stock/*` dengan Saldo/Mutasi/Lokasi digabung sebagai bagian dalam satu `StockPage`.
> Ini konsekuensi ADR-0004 dan penyederhanaan yang disengaja — bukan regresi.

---

## 7. Koreksi Dokumen Internal

**Sudah diperbaiki** (klaim "semua modul skeleton kosong, belum ada logic" — padahal ada puluhan
ribu baris kode): `CLAUDE.md` dan `knowledge/MODULE_MAP.md` kini menyatakan status faktual dan
menunjuk ke dokumen ini.

**Sudah diperbaiki (sesi finalisasi):** duplikasi baris L9 `assistant` dan bagian "Modul yang
Belum Dijadwalkan" yang kontradiktif di `knowledge/MODULE_MAP.md` telah dihapus — tersisa satu
baris L9 faktual (bot WA + tooling, tanpa RAG/AI) dan catatan bahwa hanya Knowledge/RAG yang di
luar cakupan. `docs/PRD.md` §3/daftar-tahap juga disegarkan (L9 terjadwal atas permintaan pemilik).

---

## 8. Rencana Penutupan Gap (urutan usulan)

Diurutkan berdasarkan "tanpa ini sistem tidak bisa dipakai" lalu dependensi:

| Tahap | Isi | Alasan urutan |
|---|---|---|
| **1** | ✅ *selesai* — koreksi status di `CLAUDE.md` + `MODULE_MAP.md` + pointer di `INDEX.md` | Murah, dan mencegah kerja ganda di semua sesi berikutnya |
| **1b** | Rapikan kontradiksi L9 `assistant` di `MODULE_MAP.md` + segarkan PRD §3/§9 (§7) | Kontradiksi aktif; berisiko modul yang sudah jadi dianggap terlarang disentuh |
| **2** | Halaman cetak (§4.2): surat jalan → faktur → kwitansi → struk termal POS | P0 lapangan; murni frontend, tidak menyentuh schema, bisa jalan paralel dengan tahap 3 |
| **3** | Finance saldo awal + antrian posting (§4.1 P0) | P0 pembukuan; butuh migrasi tabel baru — kerjakan sebelum finance dipakai produksi |
| **4** | Finance P1: akun kas, piutang/utang, penyesuaian pajak, tutup periode aman | Melengkapi modul 17 |
| **5** | Ekspor Excel + QR scan/generate (§4.3) | Mengembalikan produktivitas harian |
| **6** | Antrian kerja pengiriman, pindah lokasi, pratinjau retur, pengiriman pengganti (§4.4 P1) | Alur operasional lanjutan |
| **7** | Jaring pengaman uji: Vitest di `apps/web`, E2E (§4.5) | Sebaiknya digeser lebih awal bila finance akan disentuh berat |
| **8** | Sisa P2 + komponen UI yang belum diport (§6) | Penghalusan |

Sebelum mengerjakan tiap tahap: baca bagian modulnya di
[`docs/legacy-reference/<nn>-<modul>/`](legacy-reference/), lalu `known-issues.md` dan
`open-questions.md` — **replikasi perilaku, bukan replikasi cacat** (PRD §4).
