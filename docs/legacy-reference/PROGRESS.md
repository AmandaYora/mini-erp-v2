# PROGRESS — Analisis Legacy Mini ERP

Checklist analisis mendalam per modul. Peta modul + daftar file ada di
[00-module-map.md](00-module-map.md).

**Terakhir diperbarui:** 2026-09-09 — lapisan shared/common selesai (§5), 16 item
[PERLU KONFIRMASI] tertutup (§6), serta **modul 01 Auth & Session** (§5b),
**modul 02 Users, Roles & Permissions** (§5c), **modul 03 Company & Settings** (§5d),
**modul 04 Branch (Multi-Cabang)** (§5e), **modul 05 Product / Catalog** (§5f), dan
**modul 06 Business Party** (§5g), **modul 07 Member Type & Member Pricing** (§5h), dan
**modul 08 Order — Purchasing** (§5i), **modul 09 Order — Sales** (§5j), dan
**modul 10 Goods Receipt** (§5k), **modul 11 Delivery / Pengiriman** (§5l),
**modul 12 Sales Return** (§5m), **modul 13 Purchase Return** (§5n), dan
**modul 14 Payment / Pembayaran** (§5o) dan **modul 15 POS / Kasir** (§5p) dan **modul 16 Stock / Inventory & Gudang** (§5q)
dan **modul 17 Finance / Accounting & Pajak** (§5r)
dan **modul 19 Reporting** (§5s)
dan **modul 20 Audit Log** (§5t)
dan **modul 18 Dashboard** (§5u)
dan **modul 21 Assistant AI + WhatsApp + Knowledge/RAG** (§5v)
selesai dianalisis. Total **197 known-issue** (1 terverifikasi tertutup: KI-120; KI-198 tak terpakai).
Seluruh 21 modul (01–21) tuntas — tidak ada modul tersisa.

**Berkas lintas-modul:**

| Berkas | Isi |
|---|---|
| [00-overview.md](00-overview.md) | **Mulai dari sini** — ringkasan seluruh modul, status kelengkapan, kualitas & kedalaman per modul, kesiapan masuk fase desain |
| [module-dependency-map.md](module-dependency-map.md) | Ketergantungan antar-modul (kode **dan** data), 12 kontrak lintas modul, urutan rebuild 11 tahap |
| [open-questions.md](open-questions.md) | 130 tanda [PERLU KONFIRMASI] → 41 untuk pemilik sistem (12 memblokir), 51 gap analisis, 38 kosmetik |
| [known-issues.md](known-issues.md) | 143 perilaku aneh yang butuh keputusan perbaiki/replikasi, diisi per modul |

**Peninjauan cakupan (selesai):** seluruh 18 folder module API, 17 folder module web,
`android-pos-shell`, `packages/shared-*`, dan 27 spec E2E terverifikasi tercakup. `storage/`
masuk lapisan shared (S1), `tools/` masuk modul 21, `core/`+`shared/` web masuk lapisan shared.
**S10 (cetak dokumen) kini tertutup** lewat modul 09/11/14/15; **S11 tersisa sebagian**
(`pwa-registration.ts` + `chunk-load-recovery.ts` belum dibaca detail).

**Catatan kualitas:** kedalaman dokumen **tidak merata**. **Modul 17 Finance sudah diperdalam
(selesai)** — 973 → 2.674 baris, 19 gap analisis tertutup, 6 known-issue baru (KI-144…KI-149) dan
7 pertanyaan baru (OQ-A42…OQ-A48). Temuan terpentingnya: paket berkas pajak punya **dua versi**
(*"Data riil"* dan *"Versi dibatasi Rp4,8 M"*) — keputusan bisnis yang menunggu konfirmasi pemilik
(**OQ-A42**). Yang **masih tipis**: modul **16 Stock** (0,13×) dan **21 Assistant/WA/Knowledge**
(0,15×). Detail di [00-overview.md](00-overview.md) §3.

Legenda status:

| Simbol | Arti |
|---|---|
| ⬜ | Belum dianalisis |
| 🟡 | Sedang dianalisis |
| 🔵 | Analisis selesai, menunggu review |
| ✅ | Selesai & terverifikasi |
| ⛔ | Diblokir (tulis alasannya di kolom Catatan) |

Kolom **Prioritas** diusulkan dari ketergantungan: modul fondasi (auth, org, master data) harus
dipahami sebelum modul transaksional, dan finance paling akhir karena mengonsumsi hampir semua
modul lain.

---

## 1. Modul Fondasi & Master Data

| # | Modul | Status | Prioritas | Dokumen hasil | Catatan |
|---|---|---|---|---|---|
| 01 | Auth & Session | ✅ | P0 | [01-auth-session/](01-auth-session/) | 9 dokumen. 12 known-issue → [known-issues.md](known-issues.md) KI-01…KI-12 |
| 02 | Users, Roles & Permissions | ✅ | P0 | [02-users-roles-permissions/](02-users-roles-permissions/) | 9 dokumen. 15 known-issue → KI-13…KI-27 |
| 03 | Company & Settings | ✅ | P0 | [03-company-settings/](03-company-settings/) | 9 dokumen. 8 known-issue → KI-28…KI-35 |
| 04 | Branch (Multi-Cabang) | ✅ | P0 | [04-branch/](04-branch/) | 9 dokumen. 14 known-issue → KI-36…KI-49 |
| 05 | Product / Catalog | ✅ | P1 | [05-product-catalog/](05-product-catalog/) | 9 dokumen. 11 known-issue → KI-50…KI-60 |
| 06 | Business Party (Customer/Supplier) | ✅ | P1 | [06-business-party/](06-business-party/) | 9 dokumen. 10 known-issue → KI-61…KI-70 |
| 07 | Member Type & Member Pricing | ✅ | P2 | [07-member-pricing/](07-member-pricing/) | 9 dokumen. 6 known-issue → KI-71…KI-76 |

## 2. Modul Transaksional

| # | Modul | Status | Prioritas | Dokumen hasil | Catatan |
|---|---|---|---|---|---|
| 08 | Order — Purchasing | ✅ | P1 | [08-order-purchasing/](08-order-purchasing/) | 9 dokumen. 8 known-issue → KI-77…KI-84. Sales/delivery/retur milik 09/11–13 |
| 09 | Order — Sales | ✅ | P1 | [09-order-sales/](09-order-sales/) | 9 dokumen. 3 known-issue → KI-85…KI-87. SJ/POS/retur milik 11/15/12 |
| 10 | Goods Receipt (Penerimaan Barang) | ✅ | P1 | [10-goods-receipt/](10-goods-receipt/) | 9 dokumen. 5 known-issue → KI-88…KI-92. Mekanika PO milik 08 |
| 11 | Delivery / Pengiriman | ✅ | P1 | [11-delivery/](11-delivery/) | 9 dokumen. 5 known-issue → KI-93…KI-97. Serah-langsung milik 09, pengganti milik 12 |
| 12 | Sales Return (Retur Penjualan) | ✅ | P2 | [12-sales-return/](12-sales-return/) | 9 dokumen. 3 known-issue → KI-98…KI-100. SJ order milik 11 |
| 13 | Purchase Return (Retur Pembelian) | ✅ | P2 | [13-purchase-return/](13-purchase-return/) | 9 dokumen. 3 known-issue → KI-101…KI-103. Tanpa tukar/SJ |
| 14 | Payment / Pembayaran | ✅ | P1 | [14-payment/](14-payment/) | 9 dokumen. 3 known-issue → KI-104…KI-106. Form milik 08/09, kartu milik 17 |
| 15 | POS / Kasir (+ shell Android) | ✅ | P1 | [15-pos/](15-pos/) | 9 dokumen. 4 known-issue → KI-107…KI-110. Tanpa backend (order+bayar+stok) |
| 16 | Stock / Inventory & Gudang | ✅ | P1 | [16-stock-inventory/](16-stock-inventory/) | 9 dokumen (**970 baris — diperdalam dari kode**). 19 known-issue → KI-111…KI-114 + KI-150…KI-164. **25 endpoint**, 10 halaman |

## 3. Modul Keuangan

| # | Modul | Status | Prioritas | Dokumen hasil | Catatan |
|---|---|---|---|---|---|
| 17 | Finance / Accounting & Pajak | ✅ | P3 | [17-finance/](17-finance/) | 9 dokumen (**2.674 baris — diperdalam**). 14 known-issue → KI-115…KI-122 + KI-144…KI-149. **64 endpoint**, 16 entity, 16 rute, 164 test |

Sub-area finance yang layak jadi dokumen sendiri bila terlalu besar:

| Sub-area | Status | Catatan |
|---|---|---|
| 17a Chart of Account & Mapping | ✅ | `finance.controller.ts` |
| 17b Posting Sources & Journals | ✅ | `finance-posting.*` |
| 17c Inventory Cost / HPP | ✅ | `finance-inventory-cost.*` + cost ledger rebuild |
| 17d Reports (GL, TB, P&L, Neraca, Margin) | ✅ | `finance-reporting.*` |
| 17e Tax / SPT & Adjustment | ✅ | `finance-tax-adjustment.*`, `finance-export.*` |
| 17f Period Close & Daily Close | ✅ | `finance-close.*` |
| 17g Business Expense | ✅ | `business-expense.*` |
| 17h Opening Balance & Equity | ✅ | `finance/opening/*` |

## 4. Modul Observability & Intelligence

| # | Modul | Status | Prioritas | Dokumen hasil | Catatan |
|---|---|---|---|---|---|
| 18 | Dashboard | ✅ | P2 | [18-dashboard/](18-dashboard/) | 9 dokumen (**825 baris — diperdalam dari kode**). 11 known-issue → KI-132…KI-135 + KI-179…KI-185. 1 endpoint, 1 halaman, 21 field |
| 19 | Reporting | ✅ | P2 | [19-reporting/](19-reporting/) | 9 dokumen (**614 baris — diperdalam dari kode**). 10 known-issue → KI-123…KI-126 + KI-186…KI-191. 2 endpoint, 1 tabel tulis-tanpa-baca, tanpa UI |
| 20 | Audit Log | ✅ | P2 | [20-audit-log/](20-audit-log/) | 9 dokumen (**577 baris — diperdalam dari kode**). 11 known-issue → KI-127…KI-131 + KI-192…KI-197. 1 endpoint, 1 halaman, 102 situs / 103 actionKey |
| 21 | Assistant AI + WhatsApp + Knowledge/RAG | ✅ | P3 | [21-assistant-whatsapp-knowledge/](21-assistant-whatsapp-knowledge/) | 9 dokumen (**1.300 baris — diperdalam dari kode**). 22 known-issue → KI-136…KI-143 + KI-165…KI-178. 18 endpoint, 9 tabel, 2 halaman |

## 5. Lapisan Lintas Modul (Shared) — ✅ SELESAI

Hasil ditulis ke **`docs/legacy-analysis/shared/`** — tiga dokumen lintas-area, bukan satu
dokumen per area seperti rencana awal. Alasannya: aturan shared saling merujuk (mis. zona waktu
menyentuh util, entity, dan numbering sekaligus), sehingga memecahnya per-folder justru
memisahkan hal-hal yang harus dibaca bersama.

| Dokumen | Isi |
|---|---|
| [shared/shared-services.md](shared/shared-services.md) | Service & fungsi lintas modul + apa yang dilakukannya |
| [shared/shared-data-model.md](shared/shared-data-model.md) | Base model, konvensi kolom, tipe primitif, bentuk data bersama |
| [shared/shared-business-rules.md](shared/shared-business-rules.md) | Aturan global: auth/middleware, envelope, tanggal, uang, validasi, penegak otomatis |

Cakupan per area yang direncanakan:

| # | Area | Status | Dibahas di |
|---|---|---|---|
| S1 | Storage / Media (`storage`) | ✅ | services §2.2–2.3 · rules §8 |
| S2 | Shared packages (`packages/*`) | ✅ | data-model §4–§5 |
| S3 | Backend cross-cutting (`common/`) | ✅ | services §3 · rules §1, §2, §4 |
| S4 | Infrastruktur & migrasi DB | ✅ | services §7.1–7.5 · rules §9.5 |
| S5 | Frontend state (`store/`) | ✅ | services §4.4–4.5 · data-model §7.1, §7.4–7.5 |
| S6 | Frontend API layer (`lib/api.ts`) | ✅ | services §4.1–4.2 · rules §5.2 |
| S7 | Design system (`components/`) | ✅ | services §5 · data-model §7.2 |
| S8 | Routing & navigasi (`module-registry.tsx`) | ✅ | services §6.1–6.2 · rules §5.1 |
| S9 | Layout / App shell (`layout/`) | ✅ | services §6.3 · rules §5.6 |
| S10 | Cetak dokumen (print pages) | 🔵 | data-model §7.3 (tipe + kalibrasi kertas). **Komponen cetak per-dokumen belum** — ikut analisis modul Order/POS |
| S11 | PWA & perangkat (printer, QR scan) | 🔵 | services §4.7 (daftar + peran). **Isi `native-printer`/`pwa-registration` belum dibaca detail** |
| S12 | Tooling & arch guard (`scripts/`) | ✅ | rules §6 (5 aturan HARD, 3 WARN, + yang TIDAK diperiksa) |
| S13 | Test infrastruktur (unit + E2E) | ✅ | services §7.7 · rules §9.7 |

S10 dan S11 sengaja ditinggal 🔵: keduanya lebih tepat diselesaikan bersama modul yang
memakainya (Order/POS), bukan sebagai lapisan shared murni.

### Temuan Menonjol dari Lapisan Shared

Diangkat ke sini karena memengaruhi keputusan arsitektur rebuild, bukan hanya satu modul:

| # | Temuan | Detail |
|---|---|---|
| 1 | **Tidak ada base entity** — konvensi kolom diulang manual di 70 entity | data-model §1 |
| 2 | **Validasi payload praktis absen** — `class-validator` dipakai di 1 file saja (`auth.controller.ts`), 0 file `*.dto.ts`; ~214 endpoint tanpa validasi runtime | rules §4.1a |
| 3 | **Migration runner tanpa tabel pelacak** — 50 migrasi dijalankan ulang setiap kali; aman hanya karena 6 errno diabaikan + SQL idempoten | services §7.4 |
| 4 | **Dua standar pembulatan uang hidup bersamaan** — rupiah utuh vs 2 desimal | services §3.3 |
| 5 | **6 aturan paling kritis ditegakkan manual** — scope dari session, permission per request, akses cabang, audit wajib; `arch:check` tidak memeriksa satu pun | rules §6, §10 |
| 6 | **Audit log di luar transaction pemanggil** — baris audit tetap tertulis meski transaction rollback | services §2.1 |
| 7 | **Aturan zona waktu belum konsisten** — finance sudah WIB-aware, numbering order masih waktu lokal server | rules §2.4 |
| 8 | **`PermissionGuard` gagal-terbuka** — endpoint tanpa `@RequirePermission` lolos otomatis | rules §1.2 |
| 9 | **Isolasi tenant bergantung disiplin per-service** — tanpa row-level security atau query filter global | data-model §3 |
| 10 | **Kelas CSS legacy = hook selector E2E** — `button-*`, `data-table`, `sidebar-panel` tidak boleh dihapus | services §5 |

Insiden nyata yang terekam di komentar kode dan **wajib** tetap dijaga guard-nya:

| Insiden | Guard yang lahir darinya |
|---|---|
| Saldo awal 7 produk dientri 100–1000× harga wajar → HPP tercemar 6+ minggu, ~Rp 3,2 M jurnal terposting | `assertReasonableUnitCost` (rasio wajib di `[0.1, 10]`) |
| Produk dibeli per DUS isi 300 pcs, harga modal tercatat 300× | `resolveBaseUomCost` |
| Filter `YYYY-MM-DD` polos ke kolom UTC memotong hari terakhir & geser 7 jam | `resolveZonedDateRange` |
| Posting mundur/reversal bernomor bulan saat diposting, bukan bulan transaksi | `formatDocumentSequenceNumber` |
| Margin kanan cetak dot-matrix di luar jangkauan print head | `CONTINUOUS_PAPER_DEFAULTS.marginRightMm = 37.91` |

---

## 5b. Hasil Analisis Modul 01 — Auth & Session ✅

Dokumen di **[`01-auth-session/`](01-auth-session/)**. Struktur mengikuti pembagian Kelompok A
(kontrak presisi) dan Kelompok B (konsep saja).

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](01-auth-session/feature-inventory.md) | 9 kelompok fitur, **36 edge case**, 4 elemen UI yang tidak berfungsi, tabel akun bawaan |
| A | [ui-ux-spec.md](01-auth-session/ui-ux-spec.md) | Spesifikasi 2 screen + 4 komponen sesi + halaman 403 + splash, lengkap dengan token warna, urutan field, dan teks apa adanya |
| A | [user-flows.md](01-auth-session/user-flows.md) | **15 alur end-to-end** (UF-01…UF-15) + matriks redirect + tabel sanitasi tujuan |
| A | [business-rules.md](01-auth-session/business-rules.md) | **30 aturan** (BR-01…BR-30) + 12 kondisi khusus. Validasi, token, guard, envelope, kebijakan `superadmin` |
| A | [reports-list.md](01-auth-session/reports-list.md) | **Tidak ada laporan** — diverifikasi, bukan diasumsikan. Plus inventaris bahan mentah bila laporan sesi ingin dibuat |
| A | [numbering-sequence.md](01-auth-session/numbering-sequence.md) | **Tidak ada penomoran dokumen** — diverifikasi. Plus catatan identitas teknis (id sesi, struktur token, kunci penyimpanan) |
| A | [test-cases.md](01-auth-session/test-cases.md) | **60+ kasus** input→output. Menandai mana yang sudah ada di test (39) vs turunan pembacaan kode, plus **10 celah test** (GAP-01…GAP-10) |
| B | [data-model-legacy.md](01-auth-session/data-model-legacy.md) | 9 tabel + relasi, kebutuhan data per entitas, 6 kebutuhan yang hilang |
| B | [algorithms-legacy.md](01-auth-session/algorithms-legacy.md) | 9 logika kunci (A-01…A-09) dijelaskan sebagai **hasil yang harus dicapai**, plus tabel "wajib dipertahankan vs bebas diubah" |

### Temuan Menonjol Modul 01

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Modul auth adalah satu-satunya pemakai `class-validator`** di seluruh API | Menegaskan temuan lapisan shared: 214 endpoint lain tanpa validasi runtime |
| 2 | **Permission di-resolve ulang dari DB setiap request** (4 query/request) | Perubahan hak berlaku seketika tanpa logout — karakter inti modul, wajib dipertahankan |
| 3 | **Refresh token sekali pakai** (hash dirotasi tiap refresh) | Properti keamanan kuat: replay terdeteksi. Belum ada test-nya (GAP-03) |
| 4 | **Sesi tidak punya batas umur absolut** (sliding expiry penuh) | Sesi aktif bisa hidup selamanya → KI-12, butuh keputusan |
| 5 | **Percobaan login gagal tidak dicatat sama sekali** | Tidak ada rate limit, tidak ada lockout, tidak ada jejak → KI-11 |
| 6 | **User tanpa cabang aktif masuk jalan buntu** | Login berhasil tapi tanpa cabang; halaman pilih cabang kosong **dan tanpa tombol logout** → KI-01 |
| 7 | **Pesan error login menampilkan kode mesin** (`unauthorized`) | Pesan Indonesia yang sudah disiapkan server tidak pernah tampil → KI-04 |
| 8 | **Modul auth EXEMPT dari audit log** — jejaknya `user_sessions` | Riwayat ganti role/cabang hilang (kolom ditimpa); percobaan gagal tak terekam |
| 9 | **Tabel auth memakai `DATETIME(3)`**, entity mendeklarasikan presisi 6 | Koreksi terhadap ringkasan shared §2.2 — konvensi presisi 6 berlaku untuk entity, bukan DB baseline |
| 10 | **Kebijakan `superadmin` ditegakkan di lapisan data, bukan di modul auth** | Login & ganti role memperlakukannya seperti role biasa; yang mencegah adalah seed + migrasi 042 |
| 11 | **Password akun `kasir` mengandung spasi** (`kasir 123`) | Penting saat migrasi & penulisan test |
| 12 | **Fitur "Ganti Role" praktis tidak terpakai** | Kelima akun bawaan hanya punya 1 role → submenu tidak pernah tampil |

### Pertanyaan Terbuka Modul 01

Ditandai [PERLU KONFIRMASI] di dokumen, di luar 12 known-issue:

| # | Pertanyaan | Lokasi |
|---|---|---|
| M1-Q1 | Campur bahasa Indonesia–Inggris di halaman auth: dipertahankan atau diseragamkan? | ui-ux-spec §pembuka |
| M1-Q2 | Pesan "User tidak memiliki role" membocorkan bahwa akun ada & password benar — disengaja? | business-rules BR-05 |
| M1-Q3 | Apakah modul Users menegakkan "satu cabang default per user"? | business-rules BR-07 |
| M1-Q4 | Status `inactive` vs `locked` berperilaku identik — apakah `locked` dimaksudkan berbeda? | data-model §3.2 |
| M1-Q5 | Apakah approval stok memanggil ulang verifikasi password auth atau punya salinan sendiri? | business-rules §9 |
| M1-Q6 | Teks halaman 403 memakai istilah developer — ditulis ulang? | ui-ux-spec §7 |
| M1-Q7 | Apakah `trust proxy` di-set? Tanpa itu IP yang tercatat adalah IP proxy | reports-list §3 |
| M1-Q8 | Apakah `users.last_login_at` ditampilkan di modul Users? | reports-list §2 |
| M1-Q9 | Boleh menambahkan pesan "Sesi Anda telah berakhir" saat sesi tidak sah? | algorithms A-07 |
| M1-Q10 | Perlu rate limiting / lockout di sistem baru? (fitur baru, bukan replikasi) | algorithms A-01 |
| M1-Q11 | Identitas sesi sebaiknya UUID alih-alih auto-increment? | numbering-sequence §2.1 |

---

## 5c. Hasil Analisis Modul 02 — Users, Roles & Permissions ✅

Dokumen di **[`02-users-roles-permissions/`](02-users-roles-permissions/)**.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](02-users-roles-permissions/feature-inventory.md) | 10 kelompok fitur, **50 edge case**, 5 elemen yang tak berfungsi seperti dugaan, data bawaan seed |
| A | [ui-ux-spec.md](02-users-roles-permissions/ui-ux-spec.md) | 4 screen, urutan field form, **daftar lengkap 22 kelompok × 59 label permission**, katalog 9 toast |
| A | [user-flows.md](02-users-roles-permissions/user-flows.md) | **17 alur** (UF-01…UF-17) + matriks permission per aksi + kemampuan per role bawaan |
| A | [business-rules.md](02-users-roles-permissions/business-rules.md) | **29 aturan** (BR-01…BR-29) + 17 kondisi khusus |
| A | [reports-list.md](02-users-roles-permissions/reports-list.md) | **Tidak ada laporan** — tapi modul ini memasok 8 `actionKey` ke Riwayat Aktivitas, dengan 4 batasan yang membuatnya tak bisa menjawab "siapa memberi hak apa" |
| A | [numbering-sequence.md](02-users-roles-permissions/numbering-sequence.md) | Tidak ada penomoran dokumen, tapi ada **generator kode role** — aturan + 15 contoh transformasi |
| A | [test-cases.md](02-users-roles-permissions/test-cases.md) | **90+ kasus**, menandai 49 yang sudah ada di test vs turunan kode, plus **14 celah test** |
| B | [data-model-legacy.md](02-users-roles-permissions/data-model-legacy.md) | 8 tabel + rantai otorisasi, **sumber izin tersebar di 11 tempat**, 9 kebutuhan hilang |
| B | [algorithms-legacy.md](02-users-roles-permissions/algorithms-legacy.md) | 9 logika kunci (A-01…A-09) sebagai **hasil yang harus dicapai** + tabel wajib-dipertahankan vs bebas-diubah |

### Temuan Menonjol Modul 02

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Pengguna bisa kehilangan seluruh role dan terkunci dari sistem** — dua jalur, keduanya tak dijaga | Login menolak pengguna tanpa role; pemulihan butuh admin lain → KI-13 |
| 2 | **Izin role sistem dapat diubah pemegang `role.manage`** — termasuk role `admin` yang memegangnya | Admin dapat menaikkan hak dirinya sendiri; pembagian tugas hanya konvensi → KI-15 |
| 3 | **Pengguna di luar 100 data pertama tidak bisa diedit** — form baca dari store, bukan fetch detail | Form terbuka dalam mode tambah tanpa peringatan → KI-14 |
| 4 | **Sumber kebenaran izin tersebar di 11 tempat** — seed (56) + 10 migrasi + 2 tempat di kode | Menambah izin menuntut edit di 4 lokasi tanpa penjaga sinkronisasi |
| 5 | **Seed memunculkan kembali izin yang sudah dihapus** dari role sistem | Konfigurasi hak yang disesuaikan bisa kembali ke bawaan tanpa jejak audit → KI-18 |
| 6 | **Perubahan role & akses cabang tidak dicatat audit sama sekali** | Riwayat Aktivitas tidak bisa menjawab "siapa memberi hak apa kepada siapa" |
| 7 | **Pesan galat spesifik dibuang di lapisan antarmuka** — dua pola berbeda | Tiga penyebab berbeda tampil identik; akar sama dengan KI-04 → KI-17 |
| 8 | **Cabang default ditentukan urutan array yang tak terlihat** | Admin tak punya kendali; hasil benar tapi caranya tak terkendali → KI-20 |
| 9 | **Operasi role tanpa transaksi** — beda dari operasi pengguna yang semuanya dilindungi | Bisa meninggalkan role tanpa izin, tak terdeteksi → KI-27 |
| 10 | **`whatsapp.simulate` disembunyikan dari UI tapi tetap bisa diberikan lewat API** | Penjagaan kosmetik; satu-satunya izin dari 60 yang tak ada di matriks |
| 11 | **Pemutusan sesi dirancang baik** — ganti password sendiri mempertahankan sesi sendiri | Properti UX yang mudah terlewat, ada test khusus. **Wajib dipertahankan** |
| 12 | **Validasi izin mendahului penghapusan** | Salah ketik kode izin tidak merusak konfigurasi. Pola yang benar — sayangnya tidak dipakai untuk role |

### Pertanyaan Modul 01 yang Terjawab di Sini

| # | Pertanyaan | Jawaban |
|---|---|---|
| M1-Q3 | Apakah modul Users menegakkan "satu cabang default per user"? | **Ya, tapi implisit** — penetapan ganti-total membuat tepat satu cabang selalu default (cabang pertama pada array). Aturan data tidak menjaminnya; urutan array yang menjaminnya |
| M1-Q4 | Status `inactive` vs `locked` — apakah `locked` dimaksudkan berbeda? | **Tidak ada bedanya di kode.** Seluruh sistem hanya membandingkan `status === 'active'`. Modul ini bahkan **tidak bisa** menetapkan `locked` — tidak ada jalur UI maupun API |
| M1-Q8 | Apakah `users.last_login_at` ditampilkan di modul Users? | **Tidak.** Kolom ikut dikembalikan `users/list` (entity di-spread apa adanya) tetapi tidak ditampilkan di UI mana pun — praktis data mati |

### Pertanyaan Terbuka Modul 02

| # | Pertanyaan | Lokasi |
|---|---|---|
| M2-Q1 | Apakah kebutuhan "satu pengguna dengan pengecualian izin" pernah muncul? Model saat ini role-only, tanpa izin per-pengguna | data-model §2 |
| M2-Q2 | Apakah `locked` perlu diberi arti berbeda, atau hapus salah satu status non-aktif? | data-model §3.1 |
| M2-Q3 | Perlu dukungan nama role non-Latin/beraksen? Sekarang karakternya dibuang, bukan ditransliterasi | numbering §2.2 |
| M2-Q4 | Apakah nomor WhatsApp pengguna dipakai untuk pengiriman nyata? Kolomnya bernama E.164 tapi tanpa validasi | numbering §4 |
| M2-Q5 | Apakah 4 ketidaklengkapan audit (role, cabang, nilai baru, izin lama) dapat diterima? | business-rules BR-22 |
| M2-Q6 | Perlu batas minimum izin per role? Sekarang role tanpa izin diterima | business-rules BR-09 |
| M2-Q7 | Perlu toast sukses untuk aksi pengguna? Sekarang hanya aksi role yang bertoast sukses | ui-ux-spec §5 |
| M2-Q8 | Teks status `active`/`inactive` tampil mentah dalam UI berbahasa Indonesia — diterjemahkan? | ui-ux-spec §2.6 |

---

## 5d. Hasil Analisis Modul 03 — Company & Settings ✅

Dokumen di **[`03-company-settings/`](03-company-settings/)**.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](03-company-settings/feature-inventory.md) | 8 kelompok fitur, **50 edge case**, **8 elemen yang tidak berfungsi**, data bawaan seed |
| A | [ui-ux-spec.md](03-company-settings/ui-ux-spec.md) | 2 screen, urutan field, seluruh placeholder & teks tombol, katalog 6 toast |
| A | [user-flows.md](03-company-settings/user-flows.md) | **11 alur** (UF-01…UF-11) + matriks permission + kemampuan per role |
| A | [business-rules.md](03-company-settings/business-rules.md) | **28 aturan** (BR-01…BR-28) + 20 kondisi khusus |
| A | [reports-list.md](03-company-settings/reports-list.md) | Tidak ada laporan; **4 dari 6 endpoint tidak pernah dipanggil UI**; modul ini memasok konfigurasi cetak ke modul Order |
| A | [numbering-sequence.md](03-company-settings/numbering-sequence.md) | Tidak ada penomoran dokumen; **kode perusahaan & kode status order diisi manual** — kontras dengan kode role yang dihasilkan otomatis |
| A | [test-cases.md](03-company-settings/test-cases.md) | **80+ kasus**, menandai 26 yang sudah ada di test vs turunan kode, plus **12 celah test** |
| B | [data-model-legacy.md](03-company-settings/data-model-legacy.md) | 3 tabel, **5 blok JSON dengan ~18 kunci tetapi hanya 5 yang punya pembaca** |
| B | [algorithms-legacy.md](03-company-settings/algorithms-legacy.md) | 8 logika kunci (A-01…A-08) sebagai hasil yang harus dicapai |

### Temuan Menonjol Modul 03

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Hanya 5 dari ~18 setelan yang benar-benar dibaca** — dua blok penuh (label bisnis, preferensi pelaporan) mati total | Sebagian besar "pengaturan" tidak mengatur apa pun |
| 2 | **Zona waktu, mata uang, dan bahasa sistem tersimpan tapi tidak berpengaruh** — dan tidak divalidasi | Pengguna wajar mengira setelan ini bekerja → KI-28 |
| 3 | **Seluruh sistem feature flag mati di 3 lapisan** — tanpa UI, tanpa pembaca, dan jalur datanya diblokir adapter | Fosil rancangan awal, seperti prefix `vioni` → KI-33 |
| 4 | **4 dari 6 endpoint tidak pernah dipanggil UI** — halaman mengisi form dari konteks sesi, bukan dari server | Nilai di form bisa basi bila diubah dari sesi lain |
| 5 | **Simpan menjalankan 2 permintaan; yang kedua tidak ditunggu** | Profil bisa tersimpan sementara pengaturan gagal — dengan toast sukses tetap muncul → KI-29 |
| 6 | **Penyimpanan ganti-total per kelompok** — penggabungan hanya terjadi di klien | Integrasi yang mengirim satu setelan akan menghapus sisanya → KI-30 |
| 7 | **Audit modul ini paling lengkap di seluruh sistem** — mencatat seluruh field/blok di `before` DAN `after` | Jadikan acuan; bandingkan `user.update` modul 02 yang hanya 1 field di `after` |
| 8 | **Mode persetujuan koreksi stok satu-satunya setelan operasional dengan efek terverifikasi** | Dibaca ulang per operasi oleh modul Stock; ada test E2E-nya |
| 9 | **Salah ketik mode persetujuan melemahkan penjagaan** — nilai apa pun selain `strict` menjadi `simple` | Arah kegagalan tidak aman |
| 10 | **Kebijakan approval bisa dilonggarkan dalam satu klik tanpa penjagaan** | Teraudit lengkap, tetapi tidak dicegah |
| 11 | **Kalibrasi kertas membedakan "wajib > 0" (ukuran) vs "boleh 0" (margin)** | Pembedaan disengaja & terdokumentasi — wajib dipertahankan |
| 12 | **Mode baca-saja tiga lapis** (kendali nonaktif + teks penjelas + submit diabaikan) | Pola terbaik di sistem; layak ditiru modul lain |
| 13 | **Halaman Status Order hanya bisa menambah** — tidak bisa ubah, hapus, atau urutkan | Status baru selalu "Menunggu" berwarna di luar palet → KI-35 |

### Pertanyaan Terbuka Modul 03

| # | Pertanyaan | Lokasi |
|---|---|---|
| M3-Q1 | **Kode perusahaan wajib diisi tetapi tidak dipakai untuk apa pun yang terlihat** — nomor dokumen memakai kode cabang. Dipakai di luar sistem? | numbering §2.4 |
| M3-Q2 | Nama legal perusahaan tersimpan tetapi tidak ditemukan pembacanya — dipakai di dokumen resmi? | data-model §3.1 |
| M3-Q3 | Kolom status perusahaan tidak pernah dibaca dan tidak bisa diubah — dibuang? | feature-inventory NF-05 |
| M3-Q4 | Jumlah cabang di kartu "Cabang" menghitung cabang **milik pengguna**, bukan total perusahaan — sesuai maksud? | reports-list §2 |
| M3-Q5 | Pengaturan tersebar di dua pola URL (`/settings/*` dan `/finance/settings`) — disengaja? | reports-list §6 |
| M3-Q6 | Halaman Status Order menuntut izin **kelola** untuk dibuka, sementara Profil Perusahaan cukup izin **baca** — diseragamkan? | ui-ux-spec §1 |
| M3-Q7 | Apakah modul WhatsApp menormalkan ulang kuota AI? Normalisasi rentang 1–500 hanya ada di klien | business-rules BR-08 |
| M3-Q8 | Cabang "tanpa sesi" pada penyimpanan pengaturan/flag tidak punya pemanggil — masih dibutuhkan? | business-rules BR-16 |
| M3-Q9 | Perubahan kebijakan approval hanya teraudit, tidak dicegah — perlu penjagaan lebih ketat? | business-rules §9 |

---

## 5e. Hasil Analisis Modul 04 — Branch (Multi-Cabang) ✅

Dokumen di **[`04-branch/`](04-branch/)**.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](04-branch/feature-inventory.md) | 6 kelompok fitur (F-01…F-06), **9 hal yang tidak ada di modul ini**, 10 titik sentuh cabang di modul lain, matriks izin 5 peran |
| A | [ui-ux-spec.md](04-branch/ui-ux-spec.md) | 3 rute / 2 komponen halaman, urutan 5 field dalam 3 kartu, seluruh placeholder & teks bantu, katalog keadaan galat |
| A | [user-flows.md](04-branch/user-flows.md) | **11 alur** (UF-01…UF-11), termasuk UF-11 yang berakhir buntu (menutup cabang) |
| A | [business-rules.md](04-branch/business-rules.md) | **26 aturan** (BR-01…BR-26), 5 dampak status cabang lintas modul, 8 aturan yang **tidak** ada, 8 formula turunan |
| A | [reports-list.md](04-branch/reports-list.md) | Tidak ada laporan milik sendiri; cabang adalah **sumbu pembatas** hampir semua laporan; job metrik harian satu-satunya agregasi per-cabang |
| A | [numbering-sequence.md](04-branch/numbering-sequence.md) | **Modul ini pemilik seluruh penomoran dokumen cabang** — 5 penghitung wajib + 1 menyimpang, 2 pola nomor, jalur nomor darurat, kebijakan reset yang tidak pernah dijalankan |
| A | [test-cases.md](04-branch/test-cases.md) | **40+ kasus** (TC-L/C/U/M/A/E), menandai 12 yang sudah ada di test vs turunan kode, plus rantai E2E 11 langkah dan **8 celah test** |
| B | [data-model-legacy.md](04-branch/data-model-legacy.md) | 2 tabel milik sendiri, 1 tabel yang ikut ditulis, 26 tabel yang menaut ke cabang, salinan cabang di penyimpanan lokal browser |
| B | [algorithms-legacy.md](04-branch/algorithms-legacy.md) | 8 logika kunci sebagai hasil yang harus dicapai, bukan cara implementasinya |

### Temuan Menonjol Modul 04

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Modul terkecil dengan konsekuensi terluas** — 5 endpoint dan 2 permission, tapi `id_branch` dirujuk 26 tabel dan jadi sumbu pembatas hampir semua layar berangka | Mengubah cara cabang disimpan itu murah; mengubah cara cabang diidentifikasi itu mahal |
| 2 | **Cabang lahir siap pakai dalam satu transaksi** — gudang default + 5 penghitung nomor sekaligus, gagal-bersama | Jaminan terkuat modul ini; dibuktikan rantai E2E 11 langkah. Wajib dipertahankan |
| 3 | **Pola "pastikan ada" bersifat idempoten** — menyimpan cabang melengkapi penghitung yang hilang tanpa mereset nomor | Cara cabang lama otomatis mendapat jenis dokumen baru (retur). Pola yang layak ditiru |
| 4 | **Kode cabang punya konsekuensi yang jauh melampaui tampilan** — ia sumber 5 prefix nomor dokumen + kode gudang default | Mengubahnya memutus keseragaman dokumen resmi tanpa peringatan → KI-40 |
| 5 | **Cabang tidak bisa ditutup maupun dihapus** — padahal status non-aktif punya arti nyata di 5 tempat | Mekanisme lengkap di server, tombolnya tidak pernah dibuat → KI-36 |
| 6 | **Kebijakan reset tahunan tersimpan tapi tidak pernah dibaca** — nomor pembayaran/SJ/retur tidak pernah kembali ke 00001 | Menyentuh rekap tahunan & dokumen resmi → KI-45 |
| 7 | **Nomor mutasi stok menyimpang** — memakai nomor internal cabang (`TRF-2/...`) alih-alih kode | Satu-satunya dokumen yang cabang asalnya tidak terbaca dari nomornya → KI-46 |
| 8 | **Nama gudang default punya dua pemilik** — label di cabang selalu menimpa nama di lokasi stok | Perubahan dari menu Stok hilang tanpa peringatan → KI-42 |
| 9 | **Peran tanpa `branch.view` kehilangan alamat cabang di kop nota** — kegagalan izin ditelan diam-diam jadi daftar kosong | Nota kasir berbeda dari nota owner untuk order yang sama → KI-47 |
| 10 | **Kolom "Status / Akses" tidak pernah menampilkan status** — cabang non-aktif tampak identik dengan yang aktif | Judul kolom tidak ditepati → KI-39 |
| 11 | **3 field ada di API tapi tidak ada di form** — telepon, status, penanda pusat | Tiga fitur setengah jadi sekaligus → KI-36, KI-37, KI-38 |
| 12 | **Pemeriksaan kode ganda memakai teks mentah saat tambah, teks ternormalkan saat ubah** | Kode berspasi lolos lalu jadi galat teknis; pesan penolakan tidak seragam → KI-48 |
| 13 | **Tidak ada alur persetujuan sama sekali** — kontras dengan modul Stok dan Finance | Perubahan cabang berlaku seketika bagi pemegang `branch.manage` |
| 14 | **Menyimpan berhasil tanpa toast, gagal dengan toast kosong** | Pesan server yang sudah jelas dibuang di antarmuka → KI-43, akar sama dengan KI-04/KI-17 |

### Pertanyaan Terbuka Modul 04

| # | Pertanyaan | Lokasi |
|---|---|---|
| M4-Q1 | Apakah kode cabang pernah diubah di production? Bila ya, apakah dokumen berprefix campur pernah membingungkan saat rekap? | user-flows UF-06, KI-40 |
| M4-Q2 | Apakah pernah ada cabang yang perlu ditutup, dan bagaimana selama ini ditangani? | user-flows UF-11, KI-36 |
| M4-Q3 | Apakah nomor pembayaran/SJ/retur **seharusnya** kembali ke `00001` tiap 1 Januari? | numbering §4, KI-45 |
| M4-Q4 | Apakah nomor berbentuk `PAY-<angka>-<angka panjang>` pernah terlihat di data? Itu penanda cabang yang lahir di luar alur normal | numbering §5 |
| M4-Q5 | Apakah penanda "Pusat" punya arti operasional, atau sekadar label tampilan? | KI-38 |
| M4-Q6 | Apakah nomor telepon per-cabang memang dibutuhkan, atau cukup satu nomor perusahaan di profil dokumen? | KI-37 |
| M4-Q7 | Pernahkah dilaporkan nota kasir tampak berbeda dari nota owner (baris alamat hilang)? | KI-47 |
| M4-Q8 | Mana pemilik nama gudang default — form cabang atau menu Stok? | KI-42, algorithms §5 |
| M4-Q9 | Apakah pernah dibutuhkan angka gabungan seluruh cabang (mis. total penjualan perusahaan)? Sekarang tidak ada layarnya | reports-list §2.1 |
| M4-Q10 | Teks bantu kode cabang menyebut 3 jenis dokumen padahal kode jadi prefix 5 jenis — perlu diperbarui? | ui-ux-spec §3.2 |
| M4-Q11 | Perlukah pemangkasan spasi diseragamkan antara alur tambah (memangkas) dan ubah (tidak)? | business-rules BR-03 |

---

## 5f. Hasil Analisis Modul 05 — Product / Catalog ✅

Dokumen di **[`05-product-catalog/`](05-product-catalog/)**.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](05-product-catalog/feature-inventory.md) | 9 kelompok fitur (F-01…F-09), **52 edge case**, katalog pesan/toast, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](05-product-catalog/ui-ux-spec.md) | 5 rute + modal bulk + 6 komponen dipakai-ulang, urutan field, seluruh teks tombol/placeholder, inkonsistensi label Bundle |
| A | [user-flows.md](05-product-catalog/user-flows.md) | **11 alur** (UF-01…UF-11) + matriks izin 4 permission |
| A | [business-rules.md](05-product-catalog/business-rules.md) | **50 aturan** (BR-01…BR-50) + sinonim import + aturan lintas modul + aturan yang tidak ada |
| A | [reports-list.md](05-product-catalog/reports-list.md) | **Tidak ada laporan** — diverifikasi. Plus tabel bahan laporan modul lain + bahan mentah |
| A | [numbering-sequence.md](05-product-catalog/numbering-sequence.md) | **Tidak ada penomoran** — diverifikasi. Plus identitas teknis + generator slug varian |
| A | [test-cases.md](05-product-catalog/test-cases.md) | **70+ kasus** (TC-C/L/P/U/M/I/Q), menandai yang sudah ada di test vs turunan kode, plus **10 celah test** (G-01…G-10) |
| B | [data-model-legacy.md](05-product-catalog/data-model-legacy.md) | 3 tabel milik + `media_files`, relasi, jejak baca-tulis, 8 anomali (presisi desimal, `standard_price` mati, unik varian non-unique) |
| B | [algorithms-legacy.md](05-product-catalog/algorithms-legacy.md) | 9 logika kunci (A-01…A-09) sebagai **hasil yang harus dicapai** + tabel wajib-dipertahankan vs bebas-diubah |

### Temuan Menonjol Modul 05

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Guard modul ini adalah penjaga lintas modul** — kunci UOM, stok-nol-sebelum-arsip, harga-beli-wajib melindungi Order, Stock, dan Finance sekaligus | Melonggarkan satu guard merusak tiga modul; satu jalur validasi untuk form + import wajib |
| 2 | **Arsip satu arah tanpa restore** — satu-satunya master data tanpa pemulihan; dialog menyebut "nonaktif" padahal arsip permanen; pesan import menyuruh "Pulihkan" yang tidak ada | → KI-56, keputusan produk sebelum rebuild |
| 3 | **Import lebih longgar dari form** — bypass kunci UOM/tipe/varian, tapi justru lebih ketat soal harga beli | → KI-53 (kebalikan KI-52 yang hanya di update) |
| 4 | **Kode kategori di form edit diabaikan server** — field bisa diketik, perubahan hilang diam-diam | → KI-51 |
| 5 | **Kode create tidak di-trim, update di-trim** — duplikat berspasi lolos (pola sama dengan KI-48) | → KI-50 |
| 6 | **Varian default tersembunyi** — setiap produk punya baris `Standar` tak terlihat yang menjadi jangkar saldo, QR, dan migrasi 036 | Konsep wajib dipahami sebelum menyentuh stok/QR |
| 7 | **QR operasional = per varian, bukan per produk** — produk bervarian tidak punya QR induk | Kontrak cetak-tempel + scan POS |
| 8 | **Dua presisi desimal + kolom mati** (`DECIMAL(24,12)` vs `(18,4)`; `standard_price`; `DATETIME(3)` vs 6) | → verifikasi production sebelum menetapkan skema baru |
| 9 | **Edit produk di luar 100 pertama tetap benar** — fetch detail dari server (kontras dengan KI-14) | Pola yang benar; jadikan acuan modul lain |
| 10 | **Tidak ada laporan, tidak ada penomoran** — keduanya diverifikasi, bukan diasumsikan | Mempersempit lingkup rebuild modul ini ke CRUD + import + media + QR |

### Pertanyaan Terbuka Modul 05

| # | Pertanyaan | Lokasi |
|---|---|---|
| M5-Q1 | Label form "Bundle ringan" vs daftar "Bundle" — diseragamkan? | ui-ux-spec §7 |
| M5-Q2 | `alert()` mentah saat unduh QR gagal — diganti toast? | ui-ux-spec §1 |
| M5-Q3 | `"Tidak ada perubahan data"` vs `"Tidak ada perubahan data."` (beda titik) — diseragamkan? | feature-inventory §4 |
| M5-Q4 | `min_selling_price ≤ selling_price` dan harga negatif — ditolak, diperingatkan, atau bebas? | business-rules §11, KI-58 |
| M5-Q5 | Definisi "masih punya stok" untuk arsip: `on_hand` atau `available`? | business-rules BR-31, KI-57 |
| M5-Q6 | Default-true untuk ketikan sampah di kolom Pantau Stok — disengaja? | business-rules BR-44 |
| M5-Q7 | Perilaku cache nama kategori arsip di daftar (nama lama sampai reload) — sesuai maksud? | user-flows UF-05 |
| M5-Q8 | Presisi live faktor konversi + datetime — `(18,4)/(3)` atau `(24,12)/(6)`? | data-model §4 |

---

## 5g. Hasil Analisis Modul 06 — Business Party (Customer / Supplier) ✅

Dokumen di **[`06-business-party/`](06-business-party/)**. Member type & pricing (modul 07)
sengaja tidak disentuh — di sini hanya kolom `id_member_type` dan badge nama member.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](06-business-party/feature-inventory.md) | 7 kelompok fitur (F-01…F-07), **34 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](06-business-party/ui-ux-spec.md) | 6 rute + 3 komponen (buku alamat, picker alamat, picker pihak), urutan field, teks apa adanya |
| A | [user-flows.md](06-business-party/user-flows.md) | **8 alur** (UF-01…UF-08) + matriks izin (`order.*` pinjaman + 1 pengecualian) |
| A | [business-rules.md](06-business-party/business-rules.md) | **34 aturan** (BR-01…BR-34) + 8 aturan yang tidak ada |
| A | [reports-list.md](06-business-party/reports-list.md) | **Tidak ada laporan** — diverifikasi. Plus tabel bahan laporan modul lain |
| A | [numbering-sequence.md](06-business-party/numbering-sequence.md) | Kode otomatis `CUS-###`/`SUP-###` — tabel aturan + contoh terkunci test |
| A | [test-cases.md](06-business-party/test-cases.md) | **40+ kasus** (TC-L/P/A/E), menandai yang sudah ada di test vs turunan kode, plus **8 celah test** (G-01…G-08) |
| B | [data-model-legacy.md](06-business-party/data-model-legacy.md) | 2 tabel milik + snapshot order, relasi, jejak baca-tulis, 6 anomali |
| B | [algorithms-legacy.md](06-business-party/algorithms-legacy.md) | 5 logika kunci (A-01…A-05) sebagai **hasil yang harus dicapai** + tabel wajib-dipertahankan vs bebas-diubah |

### Temuan Menonjol Modul 06

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Tanpa kode izin sendiri** — seluruh endpoint memakai `order.*`; arsip alamat memakai `order.update` (bukan `archive`) | Siapa bisa order bisa kelola pihak; melepas/mengetatkan butuh kode izin baru |
| 2 | **Simpan gagal diam total** — slice tanpa notifikasi, form tanpa notice; satu-satunya jalur tulis tanpa umpan balik gagal | → KI-61 (tinggi), E2E selama ini hanya menutup jalur sukses |
| 3 | **Arsip relasional, bukan putus** — piutang/ledger/pelunasan/order-lama tetap jalan; hanya order baru ditolak (oleh modul Order) | Model mental "Sampah = sembunyi, bukan hapus" wajib dipertahankan |
| 4 | **Kode unik lintas arsip** — keputusan sadar (pesan Sampah ramah + auto-code anti-pakai-ulang) | Jangan "diperbaiki" menjadi unik-aktif-saja tanpa mengganti pesan + generator |
| 5 | **Kode tak bisa diubah tetapi bisa diketik** (form) + **saran restore tak bisa dijalankan** | → KI-62, perlu keputusan arah |
| 6 | **Anomali 100-baris sudah diperbaiki** (by-id + server-side + seed, dikunci E2E 21) | Regresi wajib dibawa; pola acuan modul lain |
| 7 | **Tiga pintu satu entitas** (menu/Order/POS) dengan kode unik lintas pintu (dikunci E2E 20) | Kontrak konsistensi saat rebuild pintu mana pun |
| 8 | **Nama live by-id di order** (bukan snapshot) + rename tanpa duplikat (dikunci E2E 22) | Berbeda dari snapshot item/ship-to — jangan diseragamkan tanpa sadar |
| 9 | **Alamat: hapus tanpa dialog, gagal tanpa kabar, update/archive lolos untuk pemilik arsip** | → KI-63 |
| 10 | **Tanpa validasi nama/kontak di server** (`phone_e164` tanpa cek E.164 — M2-Q4 tetap terbuka) | → KI-66 |

### Pertanyaan Terbuka Modul 06

| # | Pertanyaan | Lokasi |
|---|---|---|
| M6-Q1 | Format tanggal kolom `Dihapus` (`toLocaleString`) — perlu format baku? | ui-ux-spec §7 |
| M6-Q2 | Tombol alamat tanpa gate izin di UI — tambah gate atau andalkan server? | user-flows matriks |
| M6-Q3 | Submit ganda form pihak — perlu anti-double-submit? | ui-ux-spec §2, KI-61 |
| M6-Q4 | Presisi datetime `(3)` vs `(6)` — mana nilai live? | data-model §4 |

---

## 5h. Hasil Analisis Modul 07 — Member Type & Member Pricing ✅

Dokumen di **[`07-member-pricing/`](07-member-pricing/)**.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](07-member-pricing/feature-inventory.md) | 4 kelompok fitur (F-01…F-04), **26 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](07-member-pricing/ui-ux-spec.md) | 3 rute 1 komponen + 6 permukaan di layar modul lain, teks dan format Rule |
| A | [user-flows.md](07-member-pricing/user-flows.md) | **7 alur** (UF-01…UF-07) + matriks izin (`member_type.*` + `order.create` untuk quote) |
| A | [business-rules.md](07-member-pricing/business-rules.md) | **30 aturan** (BR-01…BR-30) + contoh hitung terkunci + 6 aturan yang tidak ada |
| A | [reports-list.md](07-member-pricing/reports-list.md) | **Tidak ada laporan** — diverifikasi |
| A | [numbering-sequence.md](07-member-pricing/numbering-sequence.md) | **Tidak ada penomoran** — kode manual uppercase |
| A | [test-cases.md](07-member-pricing/test-cases.md) | **24 kasus** (TC-M/H) + E2E terkunci, plus **6 celah test** (G-01…G-06) |
| B | [data-model-legacy.md](07-member-pricing/data-model-legacy.md) | 1 tabel milik + snapshot order, relasi, 4 anomali |
| B | [algorithms-legacy.md](07-member-pricing/algorithms-legacy.md) | 5 logika kunci (A-01…A-05) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 07

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Satu rule global** — tanpa pengecualian produk/kategori/cabang/periode; dihitung dalam UOM jual | Sederhana tapi kaku; perluasan adalah fitur baru, bukan replikasi |
| 2 | **Snapshot 7 kolom + future-only** — order lama abadi; ubah rule tanpa peringatan/re-price | Kontrak terbesar modul; belum ada testnya (G-02) |
| 3 | **Bypass minimum disengaja** untuk `member_rule`-sales; harga member adalah floor diskonnya sendiri | Jangan "perbaiki" menjadi dijaga minimum |
| 4 | **Kode arsip dipakai ulang → 500** — tiga modul tiga sifat untuk masalah yang sama (KI-50/51 vs BR-02 modul 06 vs KI-71) | → KI-71, perlu satu keputusan seragam |
| 5 | **Quote galak vs diam** — id eksplisit error spesifik, jalur pelanggan degradasi diam ke standard | → KI-72 |
| 6 | **Mode tanpa kelipatan = tanpa efek diam-diam** + `0` berarti null | → KI-73 |
| 7 | **Tanpa batas atas besaran/persen** — 1000% tersimpan; hanya hasil negatif yang ditolak saat hitung | → KI-76 |
| 8 | **Store 1000 + tanpa by-id + tanpa endpoint detail** — plafon tertinggi, jaring terlemah dibanding modul 05/06 | → KI-75 |
| 9 | **Anti-double-submit ADA** di form ini (beda dengan form pihak) + toast gagal berpesan | Pola yang benar; jadikan acuan |
| 10 | **"Tidak aktif" vs "Nonaktif"** — label status beda dari produk/pihak | Inkonsistensi kecil, perlu keputusan seragam |

### Pertanyaan Terbuka Modul 07

| # | Pertanyaan | Lokasi |
|---|---|---|
| M7-Q1 | "Tidak aktif" vs "Nonaktif" — diseragamkan? | ui-ux-spec §4 |
| M7-Q2 | Baris quote-gagal di POS: lestari atau reset? Tanpa `order.create`: perlu pemberitahuan? | user-flows UF-05/07 |

---

## 5i. Hasil Analisis Modul 08 — Order Purchasing ✅

Dokumen di **[`08-order-purchasing/`](08-order-purchasing/)**. Sales/deliver/retur dalam folder
`modules/order/` yang sama milik modul 09/11–13; di sini hanya sebagai batas
(`deliver-goods`/`approve-credit` ditolak/dokumentasi-batas).

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](08-order-purchasing/feature-inventory.md) | 6 kelompok fitur (F-01…F-06), **30 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](08-order-purchasing/ui-ux-spec.md) | 4 rute bersama + dialog terima + modal export, label dinamis, format mengikat |
| A | [user-flows.md](08-order-purchasing/user-flows.md) | **8 alur** (UF-01…UF-08) + matriks izin (BranchGuard + 403 khusus `payment.create`) |
| A | [business-rules.md](08-order-purchasing/business-rules.md) | **41 aturan** (BR-01…BR-41) + 9 aturan yang tidak ada |
| A | [reports-list.md](08-order-purchasing/reports-list.md) | Export Order (aturan + sheet + batas) + angka operasional + bahan modul lain |
| A | [numbering-sequence.md](08-order-purchasing/numbering-sequence.md) | `ORD-{cabang}/PB/YYYY/MM/00001` + nomor terkait + bukan-nomor |
| A | [test-cases.md](08-order-purchasing/test-cases.md) | **40+ kasus** (TC-O/R/S/E) + E2E 05 terkunci, plus **8 celah test** (G-01…G-08) |
| B | [data-model-legacy.md](08-order-purchasing/data-model-legacy.md) | 7 tabel milik, relasi, jejak baca-tulis, 6 anomali |
| B | [algorithms-legacy.md](08-order-purchasing/algorithms-legacy.md) | 8 logika kunci (A-01…A-08) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 08

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Terima = transaksi raksasa** (penerimaan + stok + bayar/tempo + status + 1–3 audit) dengan keputusan COD di titik final | Inti modul; guard tahap + sisa + izin lapis wajib direplikasi utuh |
| 2 | **Guard tiga lapis** (movement/bayar/finance) + pesan berarah ke retur/adjustment/refund/reversal | Pola penjagaan terbaik di commerce; calon standar modul 09 |
| 3 | **Badge Diretur mati di daftar** — UI ada, data dimatikan `summary_only` | → KI-77 |
| 4 | **`requires_payment_completion` mati** (selalu false) + dialog-tanpa-sisa buntu | → KI-81, KI-79 |
| 5 | **Backdate penerimaan bebas** (movement + tanggal PO masa lalu) | → KI-80, sentuh HPP/finance |
| 6 | **Ganti jenis tanpa baris** meninggalkan UOM sisi lama | → KI-84 |
| 7 | **Nomor darurat + bulan-server + `date_to` mentah** | → KI-82, KI-78 |
| 8 | **Tanpa edit/batal penerimaan** — salah catat hanya via adjustment stok | Keputusan sadar; kunci sebagai kontrak |
| 9 | **PO tanpa diskon/minimum/member** — harga = kesepakatan supplier, sisi beli murni | Batas tegas vs sales |
| 10 | **E2E 05 mengunci segitiga** parsial/multi-lokasi/over-tolak + prabayar-blokir-cair + COD-bayar-di-akhir | Jaring regresi terkuat modul ini |

### Pertanyaan Terbuka Modul 08

| # | Pertanyaan | Lokasi |
|---|---|---|
| M8-Q1 | Fallback lokasi dialog (nama `*penyimpanan sementara*` vs default) — prioritas mana? | ui-ux-spec §4 |
| M8-Q2 | Invoice supplier ganda + termin-30-implisit — peringatan atau larang atau biarkan? | KI-83 |
| M8-Q3 | Margin di export milik modul ini atau Finance? | reports-list §1 |

---

## 5j. Hasil Analisis Modul 09 — Order Sales ✅

Dokumen di **[`09-order-sales/`](09-order-sales/)**. Mekanika bersama milik 08; SJ/POS/retur
milik 11/15/12; CRUD bayar & saldo milik 14 (di sini hanya seksi tampil + dialog tempo).

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](09-order-sales/feature-inventory.md) | 6 kelompok fitur (F-01…F-06), **24 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](09-order-sales/ui-ux-spec.md) | 4 rute bersama + 3 cetak, shortcut, seksi bayar, format mengikat |
| A | [user-flows.md](09-order-sales/user-flows.md) | **8 alur** (UF-01…UF-08) + matriks izin (403 khusus bayar-serah) |
| A | [business-rules.md](09-order-sales/business-rules.md) | **33 aturan** (BR-01…BR-33) + 7 aturan yang tidak ada |
| A | [reports-list.md](09-order-sales/reports-list.md) | Export filter sales + 3 dokumen cetak + angka + bahan modul lain |
| A | [numbering-sequence.md](09-order-sales/numbering-sequence.md) | `ORD-{cabang}/PJ/...` + `REF-` + bukan-nomor |
| A | [test-cases.md](09-order-sales/test-cases.md) | **30 kasus** (TC-S/D/C) + E2E terkunci, plus **8 celah test** (G-01…G-08) |
| B | [data-model-legacy.md](09-order-sales/data-model-legacy.md) | Delta sales (kolom, relasi, jejak), 6 anomali |
| B | [algorithms-legacy.md](09-order-sales/algorithms-legacy.md) | 5 logika kunci (A-01…A-05) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 09

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Harga berlapis** (saran → member-bypass → diskon-terklem → tolak-di-bawah-minimum) + requote hormat-manual | Inti pricing sales; contoh 1000/3→333 terkunci |
| 2 | **Serah sekali-tembak** + tunai-kurang-ditolak + kembalian-tak-tersimpan + tanggal prioritas-3 | Beda watak dengan terima-parsial purchase |
| 3 | **Nota jujur dua-mode** (penuh + bruto + sisa-riil; judul = status bayar; Staff vs USER) | Aturan cetak mengikat rekap |
| 4 | **Pengecualian POS tampak mati** (lapis-2 menuntut serah untuk semua) | → KI-86 |
| 5 | **Pesan tempo untuk semua sales** + **referensi tanpa unik** | → KI-85, KI-87 |
| 6 | **Uang bulat di pintu kasir** (pecahan dibuang, bukan digabung — komentar kode eksplisit) | Detail mudah-salah-tiru |
| 7 | **Sek shortcut + requote + blokir-submit** sudah E2E; tender murni sudah unit | Jaring terbaik di form sales |
| 8 | **Tanpa serah-parsial/cicilan/tanda-tangan** — keputusan sadar berjejak | Batas vs SJ (11) dan POS (15) |
| 9 | **E2E 02 mengunci segitiga sales** (konstruksi-net + POS-tunai-serah + purchase-parsial) | Regresi lintas purchase-sales |
| 10 | **Celah terbesar**: cetak + form bayar + serah-tanggal/reservasi tanpa spec langsung | → G-01…G-03 |

### Pertanyaan Terbuka Modul 09

| # | Pertanyaan | Lokasi |
|---|---|---|
| M9-Q1 | Referensi `REF-` perlu unik? (→ KI-87) | numbering §2 |

---

## 5k. Hasil Analisis Modul 10 — Goods Receipt ✅

Dokumen di **[`10-goods-receipt/`](10-goods-receipt/)**. Mekanika PO (validasi tahap, finansial,
guard, dialog penuh) milik 08; di sini sisi dokumen penerimaan.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](10-goods-receipt/feature-inventory.md) | 7 kelompok fitur (F-01…F-07), **18 edge case**, katalog pesan, 8 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](10-goods-receipt/ui-ux-spec.md) | Tanpa screen sendiri: dialog ringkas + kartu riwayat + toast + format |
| A | [user-flows.md](10-goods-receipt/user-flows.md) | **6 alur** (UF-01…UF-06) + matriks izin |
| A | [business-rules.md](10-goods-receipt/business-rules.md) | **22 aturan** (BR-01…BR-22) + 6 aturan yang tidak ada |
| A | [reports-list.md](10-goods-receipt/reports-list.md) | **Tidak ada laporan** — diverifikasi |
| A | [numbering-sequence.md](10-goods-receipt/numbering-sequence.md) | **Tidak ada penomoran** — identitas = tanggal + SJ + lokasi |
| A | [test-cases.md](10-goods-receipt/test-cases.md) | **12 kasus** (TC-G) + E2E 05, plus **5 celah test** (G-01…G-05) |
| B | [data-model-legacy.md](10-goods-receipt/data-model-legacy.md) | 2 tabel (018 + 036), relasi, 5 anomali |
| B | [algorithms-legacy.md](10-goods-receipt/algorithms-legacy.md) | 4 logika kunci (A-01…A-04) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 10

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Alias `create` tanpa pemanggil + `list`/`detail` tanpa pemakai FE** — ketiganya tanpa spec | → KI-88 (buang atau uji; sekelas KI-49) |
| 2 | **Tanpa nomor penerimaan** — rujukan lisan lemah (tanggal + SJ + lokasi) | → KI-89 |
| 3 | **Tanpa lock sisa** — satu-satunya tulis-kumulatif commerce tanpa `pessimistic_write` | → KI-92 |
| 4 | **Arsip setengah** (kolom header tanpa endpoint; item tanpa kolom) + **presisi drift** (18,4 vs 24,12) | → KI-91, KI-90 |
| 5 | **Tanpa halaman/cetak/edit/batal** — kartu riwayat satu-satunya wajah dokumen | Kontrak kesederhanaan; koreksi via adjustment |
| 6 | **`stock_issue_allocations` bukan milik terima** — hanya sisi serah (koreksi peta modul 00) | Catatan akurasi peta |
| 7 | **E2E 05 + 14 sebutan spec** menutup inti; alias/konkuren/nomor tanpa jaring | → G-01…G-05 |

### Pertanyaan Terbuka Modul 10

Tidak ada di luar KI-88…KI-92 (semuanya sudah ber-ID).

---

## 5l. Hasil Analisis Modul 11 — Delivery / Pengiriman ✅

Dokumen di **[`11-delivery/`](11-delivery/)**. Serah-langsung milik 09; SJ pengganti milik 12.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](11-delivery/feature-inventory.md) | 8 kelompok fitur (F-01…F-08), **26 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](11-delivery/ui-ux-spec.md) | Antrean + seksi + 3 dialog + cetak SJ, format mengikat |
| A | [user-flows.md](11-delivery/user-flows.md) | **7 alur** (UF-01…UF-07) + matriks izin (cetak login-saja) |
| A | [business-rules.md](11-delivery/business-rules.md) | **30 aturan** (BR-01…BR-30) + 7 aturan yang tidak ada |
| A | [reports-list.md](11-delivery/reports-list.md) | **Tidak ada laporan** — dokumen operasional + angka + bahan |
| A | [numbering-sequence.md](11-delivery/numbering-sequence.md) | `SJ-{cabang}/{tahun}/{5 digit}` tanpa-reset |
| A | [test-cases.md](11-delivery/test-cases.md) | **25 kasus** (TC-D/B/W) + E2E 06/18, plus **6 celah test** (G-01…G-06) |
| B | [data-model-legacy.md](11-delivery/data-model-legacy.md) | 3 tabel (014+037+046), relasi, 6 anomali |
| B | [algorithms-legacy.md](11-delivery/algorithms-legacy.md) | 4 logika kunci (A-01…A-04) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 11

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Kurang-saat-terbit** (bukan konfirmasi; tanpa double-deduct, dikunci E2E) + batal mengembalikan tepat-sasaran | Invarian inti; beda watak dengan terima |
| 2 | **Kembali-menutup**: arsip-fisik-dulu + TTD/alasan + auto-close semua-terpenuhi + finansial di penutup | Rantai kertas-ke-sistem |
| 3 | **Alokasi petik-otomatis** (area→primer→default→terbanyak) + manual + reservasi | Mesin paling rumit; tanpa jaring langsung (G-05) |
| 4 | **API terbit tanpa cek tahap** (hanya UI + antrean menyaring) | → KI-93 |
| 5 | **Cetak SJ login-saja** (tanpa `order.view`) | → KI-95 |
| 6 | **Tanpa `id_company`/`updated_at`** + baris-tanpa-snapshot-sendiri + drift presisi | Anomali skema 1/2/4 |
| 7 | **Pengganti tak terlihat** di list/cetak-order (NULL) | Batas modul 12, bukan bug |
| 8 | **Karbon terkunci** (`PUTIH : SOPIR MERAH : PELANGGAN`) + status cetak dari order | Keputusan tercetak |
| 9 | **Sopir/gudang teks-bebas** (pekerja kertas, bukan user) + string-kosong lolos API | → KI-94 (+ pola knowledge) |
| 10 | **E2E 06/18** menutup tempo/COD/prabayar/parsial-batal/pengganti | Jaring ujung-ke-ujung terkuat commerce |

### Pertanyaan Terbuka Modul 11

Tidak ada di luar KI-93…KI-97 (semuanya sudah ber-ID).

---

## 5m. Hasil Analisis Modul 12 — Sales Return ✅

Dokumen di **[`12-sales-return/`](12-sales-return/)**. SJ order milik 11; retur-beli milik 13.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](12-sales-return/feature-inventory.md) | 5 kelompok fitur (F-01…F-05), **28 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](12-sales-return/ui-ux-spec.md) | 3 rute + bar lengket + modal preview + seksi kirim, format mengikat |
| A | [user-flows.md](12-sales-return/user-flows.md) | **8 alur** (UF-01…UF-08) + matriks izin (`sales_return.refund` khusus) |
| A | [business-rules.md](12-sales-return/business-rules.md) | **29 aturan** (BR-01…BR-29) + 7 aturan yang tidak ada |
| A | [reports-list.md](12-sales-return/reports-list.md) | **Tidak ada laporan** — angka + bahan |
| A | [numbering-sequence.md](12-sales-return/numbering-sequence.md) | `RTR-{cabang}/{tahun}/{5 digit}` tanpa-reset |
| A | [test-cases.md](12-sales-return/test-cases.md) | **18 kasus** (TC-R/E) + E2E 18, plus **7 celah test** (G-01…G-07) |
| B | [data-model-legacy.md](12-sales-return/data-model-legacy.md) | 3 tabel (038+046), relasi, 6 anomali |
| B | [algorithms-legacy.md](12-sales-return/algorithms-legacy.md) | 5 logika kunci (A-01…A-05) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 12

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Kas tanpa baris bayar** — collect/refund hanya settlement, piutang langsung terkoreksi | → KI-98 (tinggi): rekap kas/finance/SPT |
| 2 | **Tunda-Order vs langsung-POS** satu fungsi teruji + guard modal di keluar | Watak inti tukar; E2E 18 mengunci stok-tetap-5 |
| 3 | **Potong-piutang-dulu + kredit/refund** + `reduce` tanpa baris (implisit via selisih) | Rumus paling rumit commerce; unit-test 6 menutupnya |
| 4 | **`cancelled_*` + status tanpa penulis** (+ arsip tanpa penulis) | → KI-99 |
| 5 | **Confirm pengganti terbalik vs SJ order** (nama wajib, arsip bebas) | → KI-100 |
| 6 | **Preview-mengikat + invalidasi-total** (1 skenario FE teruji) | Kontrak UI anti-basi |
| 7 | **RUSAK-otomatis + proporsi-rasio + modal-guard** | Jalur stok khusus berjejak |
| 8 | **Pengganti di bawah minimum tanpa pengecualian** (beda order) | Batas tegas |
| 9 | **Drift presisi terverifikasi** `(18,4)` vs `(24,12)` | Sequel KI-90 |
| 10 | **Bar lengket live + verdict** + label Indonesia penuh | UX kasir terbaik di commerce |

### Pertanyaan Terbuka Modul 12

Tidak ada di luar KI-98…KI-100 (semuanya sudah ber-ID).

---

## 5n. Hasil Analisis Modul 13 — Purchase Return ✅

Dokumen di **[`13-purchase-return/`](13-purchase-return/)**. Tanpa tukar/pengganti/SJ
(cermin sales yang disederhanakan — komentar kode).

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](13-purchase-return/feature-inventory.md) | 5 kelompok fitur (F-01…F-05), **24 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](13-purchase-return/ui-ux-spec.md) | 3 rute + bar lengket + modal preview, format mengikat |
| A | [user-flows.md](13-purchase-return/user-flows.md) | **6 alur** (UF-01…UF-06) + matriks izin (`purchase_return.refund` khusus) |
| A | [business-rules.md](13-purchase-return/business-rules.md) | **21 aturan** (BR-01…BR-21) + 6 aturan yang tidak ada |
| A | [reports-list.md](13-purchase-return/reports-list.md) | **Tidak ada laporan** — angka + bahan (+ akun 5300 terverifikasi) |
| A | [numbering-sequence.md](13-purchase-return/numbering-sequence.md) | `RTB-{cabang}/{tahun}/{5 digit}` tanpa-reset (+ izin mati) |
| A | [test-cases.md](13-purchase-return/test-cases.md) | **19 kasus** (TC-P + TC-E E2E 17!) plus **6 celah test** (G-01…G-06) |
| B | [data-model-legacy.md](13-purchase-return/data-model-legacy.md) | 3 tabel (050, migrasi terakhir!) + sampingan izin/sequence/akun, 6 anomali |
| B | [algorithms-legacy.md](13-purchase-return/algorithms-legacy.md) | 5 logika kunci (A-01…A-05) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 13

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Izin `cancel` tanpa penegak** (seed + matriks + union, nol endpoint) — kembaran `sales_return.cancel` | → KI-101 |
| 2 | **E2E vs kode bertentangan** (badge daftar dituntut E2E 17, dimatikan `summary_only`) | → KI-102 (tinggi): jalankan E2E dulu |
| 3 | **Kas refund tanpa baris bayar** (seperti sales) | → KI-103 (ikut KI-98) |
| 4 | **Lock order-dulu** (komentar: tak mewarisi celah sales) — pola terbaik, usulkan standar | A-03; rujukan KI-92/97 |
| 5 | **Metadata tanpa `idOrderItem`** (lindungi margin sales) — komentar + modul 17 | A-04 verbatim; tanpa jaring (G-04 modul 13) |
| 6 | **Penuh-membatalkan kumulatif** (multi-dokumen!) + non-gagal-tanpa-status | A-02; E2E 17 mengunci |
| 7 | **Tanpa drift presisi** (terverifikasi = migrasi) — pengecualian pola KI-90 | Acuan benar |
| 8 | **Akun 5300 + mapping** terverifikasi di 050 (nota vs avg-cost) | Kontrak finance |
| 9 | **Peta 00 keliru** (cakupan di 17, bukan 08); FE unit test nol | Koreksi + G-02 |
| 10 | **Tanpa tukar/SJ/kondisi/diskon/`collect`** — batas tegas vs sales | Kesederhanaan disengaja |

### Pertanyaan Terbuka Modul 13

Tidak ada di luar KI-101…KI-103 (semuanya sudah ber-ID).

---

## 5o. Hasil Analisis Modul 14 — Payment / Pembayaran ✅

Dokumen di **[`14-payment/`](14-payment/)**. Tanpa halaman sendiri (seksi + kartu + cetak);
form milik 08/09, kartu finance milik 17.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](14-payment/feature-inventory.md) | 6 kelompok fitur (F-01…F-06), **24 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](14-payment/ui-ux-spec.md) | Seksi + kartu + cetak + format mengikat |
| A | [user-flows.md](14-payment/user-flows.md) | **6 alur** (UF-01…UF-06) + matriks izin (403 khusus) |
| A | [business-rules.md](14-payment/business-rules.md) | **22 aturan** (BR-01…BR-22) + 6 aturan yang tidak ada |
| A | [reports-list.md](14-payment/reports-list.md) | Dua agregat operasional (saldo + ledger) + dokumen + bahan |
| A | [numbering-sequence.md](14-payment/numbering-sequence.md) | `PAY-{cabang}/{tahun}/{5 digit}` tanpa-reset (+ generator-kembar) |
| A | [test-cases.md](14-payment/test-cases.md) | **20 kasus** (TC-P/A) + E2E 07/22, plus **6 celah test** (G-01…G-06) |
| B | [data-model-legacy.md](14-payment/data-model-legacy.md) | 2 tabel (009+033+047), relasi, 6 anomali |
| B | [algorithms-legacy.md](14-payment/algorithms-legacy.md) | 5 logika kunci (A-01…A-05) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 14

| # | Temuan | Dampak |
|---|---|---|
| 1 | **FIFO + bulat-anti-recehan** + arsip-selektif + arsip-boleh-ditagih | Inti kasir-kolektor; E2E 07 mengunci dua sisi |
| 2 | **Alokasi manual tanpa UI** (API + 4 pesan, nol jalan UI) | → KI-104 |
| 3 | **Ambang 0,009 vs 0 + arsip longgar** | → KI-105 |
| 4 | **Metode/tanggal/bukti-timpa longgar** | → KI-106 |
| 5 | **Cetak stabil-historis** (akumulasi + sisa-sesudah) | Aturan cetak-ulang |
| 6 | **Generator-kembar-3 + prefix-fosil + darurat** | Duplikasi + fosil |
| 7 | **Tanpa `id_company`/`updated_at`** + tanpa-cek-metode (acuan cocok) | Anomali 1–2 |
| 8 | **Backfill-033 rekonstruksi** (bukan asli) | Catatan migrasi data |
| 9 | **Tanpa halaman/FE-test-bayar** + E2E manual-nol | → G-01, G-05 |
| 10 | **Kas-retur tanpa-baris** (KI-98/103) tetap terbuka | Rujukan silang |

### Pertanyaan Terbuka Modul 14

Tidak ada di luar KI-104…KI-106 (semuanya sudah ber-ID).

---

## 5p. Hasil Analisis Modul 15 — POS / Kasir ✅

Dokumen di **[`15-pos/`](15-pos/)**. Tanpa backend sendiri (order + bayar + stok + quote);
mekanika milik 07/09/14/16.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](15-pos/feature-inventory.md) | 7 kelompok fitur (F-01…F-07), **26 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](15-pos/ui-ux-spec.md) | Grid 2-kolom + cart + dialog + scan + struk + shell |
| A | [user-flows.md](15-pos/user-flows.md) | **7 alur** (UF-01…UF-07) + matriks izin (order yatim!) |
| A | [business-rules.md](15-pos/business-rules.md) | **24 aturan** (BR-01…BR-24) + 6 aturan yang tidak ada |
| A | [reports-list.md](15-pos/reports-list.md) | **Tidak ada laporan** — struk + angka + bahan |
| A | [numbering-sequence.md](15-pos/numbering-sequence.md) | **Tidak ada penomoran** (`source='pos'` + kunci-acbak) |
| A | [test-cases.md](15-pos/test-cases.md) | **18 kasus** (TC-P/C) + E2E 07/11/15/17, plus **6 celah test** (G-01…G-06) |
| B | [data-model-legacy.md](15-pos/data-model-legacy.md) | Tanpa tabel (1 kolom!) + kunci-memori, 6 anomali |
| B | [algorithms-legacy.md](15-pos/algorithms-legacy.md) | 5 logika kunci (A-01…A-05) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 15

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Tanpa `payment.create` = order yatim** (guard halaman hanya `order.create`) | → KI-107 (tinggi) |
| 2 | **Tiga kode mati** (Termin-tersembunyi, Wide, `if (false)`) | → KI-108 |
| 3 | **Quote menimpa-manual** (tanpa `priceEdited`, beda form order) | → KI-109 |
| 4 | **Quote-gagal blokir-tunai + cart-hilang-refresh** | → KI-110 |
| 5 | **Meterai + abaikan-ganda + hantu-guard** (reservasi TTL-600) | Pola idempoten terbaik FE |
| 6 | **Angka-final-sekali** (web-hitung, Kotlin-cetak; kembalian-dari-bruto!) | Kontrak cetak-fisik |
| 7 | **Jepit-6-pintu UOM-jual + langkah-pecahan** | Anti-minus kasir |
| 8 | **Satu kolom empat efek** (`source`!) + Web-tanpa-bundle (offline-mati) | Anomali arsitektur |
| 9 | **`items`-Inggris + fallback-kasir-sendiri** | Teks kecil |
| 10 | **E2E 07/15/11/17** menutup scan/mobile/tunai/hutang/volume/urutan | Jaring kasir terluas |

### Pertanyaan Terbuka Modul 15

Tidak ada di luar KI-107…KI-110 (semuanya sudah ber-ID).

---

## 5q. Hasil Analisis Modul 16 — Stock / Inventory & Gudang ✅

Dokumen di **[`16-stock-inventory/`](16-stock-inventory/)**. Modul terbesar kedua (25 endpoint,
10 halaman, 5 service). **Diperdalam dari kode**: 970 baris, 25/25 endpoint terkontrak (E-01…E-25),
UF-09…UF-10, TC ±40 kasus, KI-150…KI-164 baru.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](16-stock-inventory/feature-inventory.md) | 10 kelompok fitur (F-01…F-10), **50+ edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](16-stock-inventory/ui-ux-spec.md) | 10 rute + drill + modal + wizard + surat-fisik |
| A | [user-flows.md](16-stock-inventory/user-flows.md) | **8 alur** (UF-01…UF-08) + matriks izin (otorisasi-bukan-guard!) |
| A | [business-rules.md](16-stock-inventory/business-rules.md) | **35 aturan** (BR-01…BR-35) + 9 aturan yang tidak ada |
| A | [reports-list.md](16-stock-inventory/reports-list.md) | 3 agregat-kerja (tanpa berkas!) + dokumen + bahan |
| A | [numbering-sequence.md](16-stock-inventory/numbering-sequence.md) | `TRF-{idcabang}/{tahun}/...` (KI-46!) + bukan-nomor |
| A | [test-cases.md](16-stock-inventory/test-cases.md) | **30 kasus** (TC-S/T/D/W) + E2E 04/08/12, plus **8 celah test** (G-01…G-08) |
| B | [data-model-legacy.md](16-stock-inventory/data-model-legacy.md) | 6 tabel, relasi, 6 anomali (unik-arsip + tanpa-timestamp!) |
| B | [algorithms-legacy.md](16-stock-inventory/algorithms-legacy.md) | 7 logika kunci (A-01…A-07) sebagai **hasil yang harus dicapai** |

### Temuan Menonjol Modul 16

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Berpasangan-ke-daun** (invarian arsitektur hidup di sini!) + dual-UOM-dua-kasus | Inti terdalam sistem |
| 2 | **Otorisasi-manusia** (bcrypt + izin + simple/strict!) + modal-gagal-spesifik | Guard terbaik commerce |
| 3 | **Modal-guard + lock-klik-ganda** (pola A-03 modul 13!) — dua praktik terbaik lahir di sini | Standarkan ke terima/dispatch |
| 4 | **Buka-harga-master** (insiden-2026-08: 100–1000×, 3,2M!) + tolak-posted + lewati-peringatan | Pelajaran termahal repo |
| 5 | **RUSAK-otomatis + rugi-global + yatim-E2E** | Siklus rusak utuh |
| 6 | **Pindah-otomatis-induk-berisi** (dialog + tak-batal!) + susun-drag + arsip-ganda-gate | Master gudang hidup |
| 7 | **Kode-arsip-500 + tanpa-pindah-induk + arsip-tanpa-pakai + tanpa-tanggal-buka** | → KI-111…KI-114 |
| 8 | **Tanpa-timestamp movement/saldo + ref-polimorfik + agregat-MIN** | Anomali skema |
| 9 | **Reservasi-hold/rilis/konsumsi/habis + scan-6-status + saran-prioritas** | Mesin kasir |
| 10 | **E2E 04/08/12 + spec-1662-baris** menutup inti; UI + batas-tanpa-jaring | → G-01…G-08 |

### Pertanyaan Terbuka Modul 16

KI-111…KI-114 + KI-150…KI-164 (temuan baru dari pendalaman kode, sudah ber-ID di `known-issues.md`).

---

## 5r. Hasil Analisis Modul 17 — Finance / Accounting & Pajak 🔵

Dokumen di **[`17-finance/`](17-finance/)**. Modul terbesar (63 endpoint, 16 entity,
18 halaman web, 8 service — sub-area 17a…17h tercakup dalam 9 dokumen yang sama).

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](17-finance/feature-inventory.md) | 8 fitur (F-01…F-08), **30 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](17-finance/ui-ux-spec.md) | 16 rute + Tutup-Buku 2-mode + wizard saldo-awal + pola console/detail |
| A | [user-flows.md](17-finance/user-flows.md) | **9 alur** (F-01…F-09: tutup-harian, koreksi-pembalik, go-live, biaya, tutup-bulan, SPT, tutup-kas, baca-laporan, setup) |
| A | [business-rules.md](17-finance/business-rules.md) | Aturan BR-01…BR-08 + per-sub-area + katalog pesan lengkap |
| A | [reports-list.md](17-finance/reports-list.md) | 12 laporan + tutup-hari + paket-SPT-8-sheet + matriks paginasi/layer/cutoff |
| A | [numbering-sequence.md](17-finance/numbering-sequence.md) | `formatDocumentSequenceNumber` WIB-transaksi (perbaikan bug PLAN.md #8) |
| A | [test-cases.md](17-finance/test-cases.md) | **40 kasus** (T-01…T-40) + pesan persis |
| B | [data-model-legacy.md](17-finance/data-model-legacy.md) | 16 tabel + hubungan + 3 hal yang tidak dimodelkan |
| B | [algorithms-legacy.md](17-finance/algorithms-legacy.md) | 11 algoritma + 6 keputusan desain untuk rebuild |

### Temuan Menonjol Modul 17

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Hash-tanpa-jumlah** (ubah-nilai = warning, identitas = tahan) + watermark + batch-5000 | Desain idempotensi terbaik repo |
| 2 | **`Menunggu` vs `Perlu-dicek`** (diam vs aksi-manusia) — jangan disatukan | Beda UX, beda mesin |
| 3 | **Tanpa-modal-manual di mana pun** (dua insiden 100–1000×) + unifikasi-masuk + bagi-faktor-UOM | Pelajaran termahal kedua setelah modul 16 |
| 4 | **Pembalik-hari-ini + biaya-simetris + snapshot-lestari** (tanpa-edit-tanpa-hapus) | Aturan emas koreksi |
| 5 | **Layer-2 jurnal-utuh** (CRC32-per-order, margin-lestari) + paket-8-sheet konsisten | SPT-simulasi tetap seimbang |
| 6 | **Plug-3999** (jembatan PL↔Neraca terverifikasi) + nomor-WIB-transaksi | Dua invariansi angka |
| 7 | **Final-salah jalan-buntu + tutup-paksa-diam + kode-izin-mentah** | → KI-122, KI-116, KI-117 |
| 8 | **Direktori E2E finance tak-ada + tarif-PPN-di-luar-finance** | → KI-118, KI-119 |

### Pertanyaan Terbuka Modul 17

Tidak ada di luar KI-115…KI-122 (semuanya sudah ber-ID).

---

## 5s. Hasil Analisis Modul 19 — Reporting 🔵

Dokumen di **[`19-reporting/`](19-reporting/)**. Modul terkecil (2 endpoint,
1 entity, 2 service, 0 halaman — dikerjakan sebelum modul 18/20 karena mandiri).

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](19-reporting/feature-inventory.md) | 3 fitur (F-01…F-03), **12 edge case**, katalog pesan, 8 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](19-reporting/ui-ux-spec.md) | Tanpa UI — hanya label izin "Lihat Laporan" (system-only!) |
| A | [user-flows.md](19-reporting/user-flows.md) | **5 alur** (laporan-order, stok-kritis, cron, catch-up, ditolak) |
| A | [business-rules.md](19-reporting/business-rules.md) | **23 aturan** (BR-01…BR-23) + katalog pesan |
| A | [reports-list.md](19-reporting/reports-list.md) | 2 laporan + 1 tabel agregat tulis-tanpa-baca |
| A | [numbering-sequence.md](19-reporting/numbering-sequence.md) | Tanpa penomoran (terverifikasi) |
| A | [test-cases.md](19-reporting/test-cases.md) | **18 kasus** (T-01…T-18) + 5 celah test (G-01…G-05) |
| B | [data-model-legacy.md](19-reporting/data-model-legacy.md) | 1 tabel + bacaan lintas-modul + anomali presisi |
| B | [algorithms-legacy.md](19-reporting/algorithms-legacy.md) | 3 logika kecil + 4 keputusan desain |

### Temuan Menonjol Modul 19

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Tanpa UI apa pun** (klaim "konsumen belum teridentifikasi" terjawab: hanya E2E!) | Kontrak = API mentah |
| 2 | **`reporting.view` system-only** (sejajar `whatsapp.simulate`, dijaga test!) | Izin developer, bukan user |
| 3 | **Tabel metrik tulis-tanpa-baca** ("tidak dipakai di MVP" sejak migrasi 001!) | → KI-126 |
| 4 | **`total_sales` mencakup pembelian/batal + kritis-per-baris + cron-UTC** | → KI-123…KI-125, KI-186…KI-191 |
| 5 | **`metricRepo` di-inject tapi tak dipakai + presisi-3-vs-6** | → KI-188, KI-189 (dari pendalaman kode) |

### Pertanyaan Terbuka Modul 19

KI-123…KI-126 + KI-186…KI-191 (temuan baru dari pendalaman kode, sudah ber-ID di `known-issues.md`).

---

## 5t. Hasil Analisis Modul 20 — Audit Log ✅

Dokumen di **[`20-audit-log/`](20-audit-log/)**. Modul kecil tapi lintas-modul:
1 endpoint tulis-tanpa-tulis (baca saja!), 1 halaman, 102 situs / 103 actionKey runtime.
**Diperdalam dari kode**: 577 baris, enumerasi kunci pasti per modul, matriks diaudit/tidak,
KI-192…KI-197 baru.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](20-audit-log/feature-inventory.md) | 4 fitur (F-01…F-04), **14 edge case**, katalog pesan, 8 hal yang sengaja tidak ada + **inventarisasi ~70 kunci nyata** |
| A | [ui-ux-spec.md](20-audit-log/ui-ux-spec.md) | 1 halaman (5 kolom + paginasi 25 + empty) + pemuatan slice/hook |
| A | [user-flows.md](20-audit-log/user-flows.md) | **4 alur** (tinjau, telusur-via-API, tulis-otomatis, ditolak) |
| A | [business-rules.md](20-audit-log/business-rules.md) | **17 aturan** (BR-01…BR-17) + katalog pesan |
| A | [reports-list.md](20-audit-log/reports-list.md) | 1 daftar (jejak itu sendiri — tanpa agregat/ekspor!) |
| A | [numbering-sequence.md](20-audit-log/numbering-sequence.md) | Tanpa penomoran (terverifikasi) |
| A | [test-cases.md](20-audit-log/test-cases.md) | **15 kasus** (T-01…T-15) + 4 celah test (G-01…G-04) |
| B | [data-model-legacy.md](20-audit-log/data-model-legacy.md) | 1 tabel + tanpa-FK-sengaja + anomali presisi |
| B | [algorithms-legacy.md](20-audit-log/algorithms-legacy.md) | 3 logika + 4 keputusan desain |

### Temuan Menonjol Modul 20

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Tanpa BranchGuard sengaja** (review lintas-cabang!) + tanpa-FK sengaja (jejak outlives master!) | Dua kesengajaan arsitektur |
| 2 | **Peta label basi dua arah** (~10 label mati + ~15 kunci tanpa label) | → KI-127 |
| 3 | **Keterangan selalu default + JSON tak tampil** (ditulis semua, dibaca nol!) | → KI-128 |
| 4 | **Halaman tanpa filter + indeks timpang + prefetch-100-gagal-diam** | → KI-129, KI-131 |
| 5 | **KI-120 tertutup terverifikasi** (`business_expense.cancel` ada!) + path E2E basi | KI-120 ✅, → KI-130 |

### Pertanyaan Terbuka Modul 20

KI-127…KI-131 + KI-192…KI-197 (temuan baru dari pendalaman kode, sudah ber-ID di `known-issues.md`).

---

## 5u. Hasil Analisis Modul 18 — Dashboard 🔵

Dokumen di **[`18-dashboard/`](18-dashboard/)**. Satu endpoint + satu halaman;
analisis terberat di observabilitas (service 1075 baris, 8 agregat SQL + hierarki
HPP-5-sumber di TypeScript).

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](18-dashboard/feature-inventory.md) | 8 fitur (F-01…F-08), **16 edge case**, katalog pesan, 8 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](18-dashboard/ui-ux-spec.md) | 1 halaman (4 KPI + strip + tren + 2 ranking + 2 kartu) + fallback + `summary-card` tak dipakai |
| A | [user-flows.md](18-dashboard/user-flows.md) | **4 alur** (pantau-pagi, kejar-tempo, cek-margin, verifikasi-API) |
| A | [business-rules.md](18-dashboard/business-rules.md) | **17 aturan** (BR-01…BR-17) + katalog pesan |
| A | [reports-list.md](18-dashboard/reports-list.md) | 8 seksi layar + 1 baris diambil-tak-tampil |
| A | [numbering-sequence.md](18-dashboard/numbering-sequence.md) | Tanpa penomoran (terverifikasi) |
| A | [test-cases.md](18-dashboard/test-cases.md) | **16 kasus** (T-01…T-16) + 4 celah test (G-01…G-04) |
| B | [data-model-legacy.md](18-dashboard/data-model-legacy.md) | Tanpa tabel milik sendiri (100% baca!) |
| B | [algorithms-legacy.md](18-dashboard/algorithms-legacy.md) | 8 algoritma + 4 keputusan desain |

### Temuan Menonjol Modul 18

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Fallback berlogika beda tanpa penanda** (kritis/tren/prioritas dihitung ulang lebih sederhana!) | → KI-132 |
| 2 | **Tiga batas-hari** (server vs zona-perusahaan-naive vs UTC vs CURRENT_DATE!) | → KI-133 |
| 3 | **Field diambil-tak-tampil** (completed/cancelled + 2 total transaksi = 2 query mati!) | → KI-134 |
| 4 | **Persen-0-untuk-rugi + serah-menang + estimasi-mengaku** (2 jujur, 1 bohong kecil!) | → KI-135 |
| 5 | **Hook tanpa-preload (dijaga test!) + kritis-samakan-daftar-stok (dijaga spec!)** | Dua invariansi tested |

### Pertanyaan Terbuka Modul 18

KI-132…KI-135 + KI-179…KI-185 (temuan baru dari pendalaman kode, sudah ber-ID di `known-issues.md`).

---

## 5v. Hasil Analisis Modul 21 — Assistant AI + WhatsApp + Knowledge/RAG ✅

Dokumen di **[`21-assistant-whatsapp-knowledge/`](21-assistant-whatsapp-knowledge/)**.
Modul terakhir: 4 API module + 1 domain UX, dibaca via 3 deep-read paralel +
verifikasi controller/izin/E2E/migrasi langsung. **Diperdalam dari kode**: 1.300 baris,
18/18 endpoint terkontrak, 10 tool, pipeline RAG, KI-165…KI-178 baru.

| Kelompok | Dokumen | Isi |
|---|---|---|
| A | [feature-inventory.md](21-assistant-whatsapp-knowledge/feature-inventory.md) | 8 fitur (F-01…F-08), **24 edge case**, katalog pesan, 10 hal yang sengaja tidak ada |
| A | [ui-ux-spec.md](21-assistant-whatsapp-knowledge/ui-ux-spec.md) | 2 halaman + 4 komponen + toast/hook/slice + 7 alias-mati |
| A | [user-flows.md](21-assistant-whatsapp-knowledge/user-flows.md) | **4 alur** (hidupkan-BOT, atur-SOP, bertanya, uji-pantau) |
| A | [business-rules.md](21-assistant-whatsapp-knowledge/business-rules.md) | Aturan BR-01…BR-16 + katalog pesan lengkap |
| A | [reports-list.md](21-assistant-whatsapp-knowledge/reports-list.md) | 7 keluaran (jawaban + stats + daftar + status + config) |
| A | [numbering-sequence.md](21-assistant-whatsapp-knowledge/numbering-sequence.md) | Tanpa penomoran (terverifikasi) |
| A | [test-cases.md](21-assistant-whatsapp-knowledge/test-cases.md) | **20 kasus** (T-01…T-20) + 4 celah test (G-01…G-04) |
| B | [data-model-legacy.md](21-assistant-whatsapp-knowledge/data-model-legacy.md) | 9 tabel + anomali status + thread-per-pengirim |
| B | [algorithms-legacy.md](21-assistant-whatsapp-knowledge/algorithms-legacy.md) | 7 algoritma + 6 keputusan desain |

### Temuan Menonjol Modul 21

| # | Temuan | Dampak |
|---|---|---|
| 1 | **Cek kanal terbalik?** (simulate wajib, nyata bebas!) | → KI-136 |
| 2 | **Asing = senyap** (keamanan-vs-UX!) | → KI-137 |
| 3 | **Status `Siap` menipu + timpa-kosong + skip-diam** | → KI-138 |
| 4 | **Hitung-cap-5 + produk-kode-saja** (angka & kapabilitas bohong kecil!) | → KI-139, KI-140 |
| 5 | **Mode/kuota-tak-tampil + simulate-tanpa-UI + sesi-plain** | → KI-141, KI-142 |
| 6 | **Tanpa-AI-tetap-hidup + otorisasi-sebagai-batas + jejak-bukan-audit** | 3 keputusan arsitektur sehat |

### Pertanyaan Terbuka Modul 21

KI-136…KI-143 + KI-165…KI-178 (temuan baru dari pendalaman kode, sudah ber-ID di `known-issues.md`).

---

## 6. Keputusan & Resolusi Lapisan Shared

16 item yang semula ditandai [PERLU KONFIRMASI] sudah tertutup: **10 dijawab pemilik sistem**,
**6 diverifikasi langsung dari kode**. Tiga item tetap terbuka (§6.3) dan empat temuan baru
muncul dari verifikasi (§6.4).

### 6.1 Dijawab pemilik sistem

| # | Pertanyaan | Jawaban | Konsekuensi untuk rebuild |
|---|---|---|---|
| S-Q1 | `JWT_SECRET` fallback `'fallback-secret'` | **Selalu di-set di production** | Hilangkan fallback — **gagal-boot** bila kosong |
| S-Q2 | Dua standar pembulatan uang | **Rupiah utuh di semua lapisan** | `money()` 2-desimal tidak dibawa. **Wajib: rekonsiliasi laporan finance lama vs baru per periode** |
| S-Q4 | `STORAGE_ROOT_PREFIX` = `'vioni'` | **Nama tenant pertama** dari konsep multi-tenant yang kini jadi **standalone** | Single-company = keputusan sadar. Ganti prefix ke nama netral |
| S-Q5 | `apps/api/storage/` vs `Upload/` | **Lokasi lama, sudah mati** | Abaikan saat migrasi data; hapus dari repo |
| S-Q8 | Locale/timezone per-company | **Buang field-nya** | `id-ID` + `Asia/Jakarta` jadi konstanta aplikasi |
| S-Q11 | `enableCors()` tanpa batas origin | **Belum dipastikan** — perlu cek server | ⬜ **Masih terbuka** (lihat §6.3) |
| S-Q12 | Nama & bentuk `MockDatabase`/`DemoUser` | **Rancang ulang bentuk + nama** | UX & tampilan tetap identik; struktur internal baru. Petakan kebutuhan data per halaman dulu |
| S-Q13 | Audit log di luar transaction | **Masukkan ke transaction** | `log()` terima `EntityManager`. Percobaan gagal tidak lagi berjejak di tabel audit |
| S-Q14 | `admin` punya `role.manage` tanpa `user.create` | **Disengaja** — admin atur struktur akses, owner kelola akun | Salin apa adanya. Perlu pembatas teknis agar `role.manage` tidak bisa menaikkan hak sendiri |
| S-Q16 | `ApiRequest.guid`/`code`/`info` | **Tidak dipakai klien lain — buang** | Envelope request jadi `{ data }` saja |

### 6.2 Diverifikasi dari kode

| # | Pertanyaan | Hasil verifikasi | Dokumen |
|---|---|---|---|
| S-Q3 | `format_template` di `branch_document_sequences` | **Ditulis 4 tempat, dibaca NOL tempat.** Isinya bahkan tidak cocok dengan output nyata (`order` tersimpan 3 segmen, output 5 segmen). `reset_policy` juga dekoratif — reset terjadi karena `sequenceKey` menyertakan `YYYY-MM`. Baris `sequenceKey: 'order'` hanya dibaca untuk `prefix`, `current_value`-nya 0 selamanya | services §2.4 |
| S-Q6 | `DataTableProps.rowHref` | **Nol konsumen.** Hanya 3 kemunculan, semuanya di dalam design system sendiri. Semua baris klikabel memakai `onRowClick` → prop mati, buang dari kontrak | services §5 |
| S-Q7 | `route-access.ts` hanya dipakai 1 file | **Peran nyata, bukan sisa desain:** menjawab "setelah ganti role, apakah pengguna masih boleh di halaman ini?" → kalau tidak, `navigate("/dashboard")`. Membaca `rolePermissions[role]` (role baru), bukan dari sesi. Perilaku UX ini wajib direplikasi; namanya saja terlalu luas | services §6.2 |
| S-Q9 | Dua catch-all route | **NotFoundPage menang** (splat di dalam shell, dideklarasikan lebih dulu, tie-break react-router v7 mempertahankan urutan). Catch-all level atas → `/login` **tidak terjangkau**, tapi **tidak berkonflik**: NotFoundPage berstatus `protected`, jadi pengunjung belum-login tetap dilempar ke `/login` | rules §5.1 |
| S-Q10 | `localStorage` token tanpa try/catch | **Risiko nyata, satu titik:** `app-store.tsx:353` memanggil `getStoredTokens()` di `useEffect` **di luar try/catch**. Tiga call site lain aman (di dalam `try` async). Diperparah: **tidak ada `ErrorBoundary` sama sekali** di `apps/web/src` → layar putih, bukan pesan error, bila browser blokir site data | rules §5.6 |
| S-Q15 | 4 entity tanpa `PrimaryGeneratedColumn` | **Semuanya sah:** `RolePermission`, `UserRole`, `UserBranchAccess` = pivot ber-PK komposit (`UserBranchAccess` beratribut `is_default_branch`); `CompanySettings` = satelit 1:1 ber-PK `id_company`, seluruh isi kolom JSON, **tanpa satu pun timestamp** | data-model §1 |

### 6.3 Masih terbuka

| # | Hal | Kenapa belum tertutup | Tindakan |
|---|---|---|---|
| S-Q11 | `enableCors()` tanpa batas origin | Perlu cek konfigurasi nginx/reverse proxy di VPS | Pemilik sistem cek server. Yang membatasi dampak: token di `localStorage`, bukan cookie — jadi request lintas-origin tidak otomatis bawa kredensial |
| S-Q17 | Interpretasi `parseDateInput` (naif, bukan zona-aware) untuk field tanggal tunggal | Bergantung format string yang dikirim frontend (`orderDate` dkk) | Ditelusuri saat analisis modul Order (§7 Q7) |
| S-Q18 | `Tone` (6 nilai) vs `ToastTone` (4 nilai) tumpang tindih — pemisahan disengaja? | Keputusan desain UI, tidak menghalangi pekerjaan lain | Diputuskan saat merancang design system baru |

### 6.4 Temuan baru yang lahir dari verifikasi

| # | Temuan | Dampak |
|---|---|---|
| N1 | **Tidak ada `ErrorBoundary` di seluruh frontend** — `main.tsx` me-render `<AppProvider><App /></AppProvider>` tanpa pembungkus | Error apa pun saat render/effect = layar putih tanpa pesan. Wajib ada di sistem baru |
| N2 | **`format_template` menyimpan nilai yang salah** — bukan hanya tidak dipakai | Kolom yang tampak sumber kebenaran, isinya keliru, dan tak berpengaruh. Kondisi terburuk dari dua pilihan |
| N3 | **`CompanySettings` tanpa timestamp** | Perubahan setting perusahaan tidak punya jejak waktu di tabelnya; hanya `audit_logs` yang mencatat |
| N4 | **`role.manage` bisa dipakai menaikkan hak sendiri** | Pembagian tugas admin/owner disengaja, tapi ditegakkan hanya oleh konvensi. Perlu pembatas: `role.manage` tidak boleh memberi permission yang tidak dimiliki aktor |

---

## 7. Pertanyaan Terbuka Modul Bisnis (belum diinvestigasi)

Sengaja belum dikejar — semuanya butuh analisis modul yang bersangkutan, bukan lapisan shared.

| # | Pertanyaan | Ditelusuri saat | Jawaban |
|---|---|---|---|
| Q1 | Siapa konsumen frontend dari `reporting/orders` & `reporting/stock`? | Modul 19 Reporting | **Tidak ada.** Hanya E2E 10 + pemanggil API langsung; izin `reporting.view` system-only |
| Q2 | Kenapa `modules/order/` menampung 6 domain — batas mana yang aman dipecah saat rebuild? | Modul 08–13 | (lihat dokumen modul 08–13) |
| Q3 | Apakah 7 rute redirect-only (`/knowledge*`, `/whatsapp`, `/assistant/{channel,knowledge,tools}`) masih perlu dipertahankan? | Modul 21 Assistant | **Terjawab struktur:** semua `<Navigate>` ke `/assistant/setup`/`/assistant/config`; tanpa halaman nyata — keputusan hapus/pertahankan untuk kompatibilitas bookmark |
| Q4 | Bagaimana POS dibedakan dari order biasa di level data (`015_pos_order_source`)? | Modul 15 POS | (lihat dokumen modul 15) |
| Q5 | Seberapa dalam ketergantungan finance ke stok/order — apa yang wajib ada sebelum finance bisa jalan? | Modul 17 Finance | (lihat dokumen modul 17) |
| Q6 | Apakah `tools` (assistant) mengeksekusi aksi tulis ke modul bisnis lain? | Modul 21 Assistant | **Tidak.** 8 tools baca-saja (`Get*`/`Retrieve*`); kapabilitas UI menegaskan "tidak bisa ubah data" |
| Q7 | Interpretasi `parseDateInput` (naif, bukan zona-aware) untuk field tanggal tunggal — format apa yang dikirim frontend? | Modul 08–09 Order | (lihat dokumen modul 08–09) |

---

## Cara Memakai Checklist Ini

1. Ambil satu baris berstatus ⬜ dengan prioritas tertinggi.
2. Ubah statusnya jadi 🟡 sebelum mulai.
3. Tulis hasil analisis ke file `docs/legacy-analysis/<dokumen hasil>` sesuai kolomnya.
4. Ubah status jadi 🔵 saat dokumen selesai, ✅ setelah diverifikasi terhadap kode/E2E.
5. Temuan yang memengaruhi modul lain: catat di kolom Catatan modul terkait, jangan hanya di
   dokumen sendiri.
