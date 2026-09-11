# Shared Services — Fungsi Lintas Modul

Analisis detail lapisan shared/common yang teridentifikasi di
[00-module-map.md](../00-module-map.md) §3. Dokumen ini menjawab: **fungsi/service apa yang
dipakai lintas modul, dan apa yang dilakukannya.**

Dokumen pendamping: [shared-data-model.md](shared-data-model.md) ·
[shared-business-rules.md](shared-business-rules.md)

Sumber: pembacaan kode langsung. Tanda **[PERLU KONFIRMASI]** menandai hal yang tidak bisa
dipastikan dari kode saja.

---

## 1. Ringkasan: Siapa Dipakai Siapa

| Service / Fungsi | Lokasi | Jumlah konsumen | Sifat |
|---|---|---|---|
| `AuditLogService.log()` | `modules/audit-log/` | **28 service** di 12 module | Wajib di setiap write |
| `StorageService` | `modules/storage/` | 4 service (product, payment, delivery-proof, knowledge) | Gerbang tunggal file |
| `ImageOptimizerService` | `modules/storage/` | via StorageService flow | Validator/optimizer akhir |
| `BranchDocumentSequence` (numbering) | `modules/branch/entities/` | 6 module | **Tanpa** formatter bersama |
| `resolveZonedDateRange()` | `common/date-range.util.ts` | finance + reporting | Wajib untuk filter tanggal |
| `applyTokenizedLike()` | `common/search/tokenized-search.ts` | list/search-options | Pencarian seragam |
| `normalizePageLimit()` / `paginateRows()` | `common/pagination.util.ts` | list endpoints | Pagination seragam |
| `roundRupiah()` | `packages/shared-types/money.ts` | API + Web | Pembulatan uang transaksional |
| `money()` / `toMoney()` | `common/number.util.ts` | finance | Pembulatan 2 desimal — **berbeda** |
| `apiPost()` / `apiUpload()` | `apps/web/src/lib/api.ts` | seluruh frontend | Satu-satunya `fetch` |
| `loadingBus` | `apps/web/src/lib/loading-bus.ts` | api.ts + GlobalLoader | Indikator loading global |
| `useToasts().notify` | `apps/web/src/store/use-toasts.ts` | semua slice + UI | Notifikasi global |
| `AppProvider` / `useAppStore()` | `apps/web/src/store/app-store.tsx` | seluruh frontend | Root state |

---

## 2. Backend — Service Lintas Modul

### 2.1 `AuditLogService` — jejak audit wajib

`apps/api/src/modules/audit-log/audit-log.service.ts`

Dua method saja:

| Method | Fungsi |
|---|---|
| `log(input)` | Insert satu baris `audit_logs`. `happenedAt` diisi `new Date()` **saat log dipanggil**, bukan dari input. Semua field opsional (`idActor`, `idEntity`, `before`, `after`, `metadata`) di-coerce ke `null`. Mengembalikan `void`. |
| `list(idCompany, filters)` | Query berpaginasi, urut `happenedAt DESC`, filter opsional `id_branch`, `entity_type`, `action_key` (LIKE partial). |

**Dipakai oleh 28 service di 12 module** — daftar lengkap:

```
branch(1) business-party(3) company(1) finance(6) knowledge(1)
order(6) payment(1) product(2) stock(5) user(2)
```

Module yang **tidak** memanggilnya: `assistant`, `whatsapp`, `tools`, `auth`, `reporting`,
`dashboard`, `audit-log` sendiri. Ini konsisten dengan daftar EXEMPT di
`.claude/rules/backend-nestjs.md` (intelligence memakai `AssistantRun` sebagai jejaknya, auth
memakai `UserSession`).

Bentuk pemanggilan seragam di seluruh codebase:

```ts
await this.auditLog.log({
  idCompany, idBranch, actorType: 'user', idActor: session.idUser,
  actionKey: 'resource.create', entityType: 'resource', idEntity: saved.id,
  after: { ... },
});
```

**Catatan implementasi yang relevan untuk rebuild:**

- `log()` **tidak** menerima `EntityManager`, jadi ia menulis lewat koneksi/repository sendiri —
  bukan bagian dari transaction pemanggil. Konsekuensinya: bila transaction bisnis di-rollback
  setelah `log()` dipanggil, baris audit **tetap tertulis** — tercatat aksi yang sebenarnya tidak
  pernah tersimpan. Tidak ada mekanisme kompensasi/pembatalan baris audit.
  **✅ Keputusan: di sistem baru `log()` ikut transaction pemanggil** (terima `EntityManager`),
  sehingga jejak audit menjadi cermin akurat dari apa yang benar-benar tersimpan. Perubahan
  perilaku yang perlu disadari: percobaan yang gagal tidak lagi meninggalkan jejak — kalau itu
  dibutuhkan, gunakan jalur log terpisah, bukan tabel audit.
- `list()` **tidak** memakai `normalizePageLimit()` dari `common/pagination.util.ts`. Ia punya
  clamp inline sendiri: `page ?? 1` dan `Math.min(limit ?? 20, 100)`. Berbeda dari helper bersama
  yang juga menangani NaN/negatif/Infinity. Jadi `audit-logs/list` dengan `limit: -5` atau
  `limit: "abc"` berperilaku lain dari list endpoint lain.

### 2.2 `StorageService` — gerbang tunggal file

`apps/api/src/modules/storage/storage.service.ts` — module tanpa controller (service-only).

Dua driver di balik satu antarmuka `StorageDriver` (`putObject`, `getSignedUrl`):

| Driver | Kapan | Perilaku |
|---|---|---|
| `LocalStorageDriver` | `STORAGE_DRIVER=local` (default) | Tulis ke `path.resolve(process.cwd(), 'Upload')`. `getSignedUrl` **tidak menandatangani apa pun** — ia mengembalikan URL publik yang sama dengan `putObject`, plus `expiresIn` yang tidak ditegakkan. |
| `S3StorageDriver` | `STORAGE_DRIVER=s3` | Butuh `STORAGE_BUCKET`, `STORAGE_ACCESS_KEY_ID`, `STORAGE_SECRET_ACCESS_KEY` — kalau kosong, konstruktor **melempar saat boot**. |

**Builder object-key per domain** (semua melewati `assertSafeKey`):

| Method | Prefix default |
|---|---|
| `buildProductImageKey(idCompany, idProduct, contentType)` | `<uploadRoot>/product` |
| `buildProductDefaultImageKey()` | `<uploadRoot>/product/default.png` |
| `buildKnowledgeSourceKey(...)` | `<uploadRoot>/assistant/knowledge` |
| (payment transfer) | `<uploadRoot>/order/transfer` |
| (delivery proof) | `<uploadRoot>/order/delivery-proof` |

`uploadRoot` default = `<STORAGE_ROOT_PREFIX>/<environment>/upload`, dengan
`STORAGE_ROOT_PREFIX` default **`'vioni'`**.

**✅ TERJAWAB — dan ini sinyal arsitektur, bukan sekadar nama.** `vioni` adalah **nama tenant
pertama** dari rancangan awal ketika aplikasi ini dimaksudkan **multi-tenant**. Konsepnya sudah
berubah menjadi **standalone satu perusahaan**.

Artinya prefix ini adalah fosil dari arsitektur yang ditinggalkan — dan ia menjelaskan beberapa
hal lain di lapisan shared yang tadinya tampak janggal:

| Jejak multi-tenant yang tertinggal | Di mana |
|---|---|
| `idCompany` di 45 entity padahal seed hanya membuat `id_company = 1` | [shared-data-model.md](shared-data-model.md) §3 |
| `ApiRequest` punya `guid`/`code`/`info` yang tidak pernah dikirim | data-model §5.1 |
| `companyLocale`/`companyTimezone` tersimpan tapi tidak pernah dibaca formatter | rules §2.5 |
| Role sistem ber-`id_company = NULL` vs role kustom per-company | services §7.5 |

Untuk rebuild: perlakukan single-company sebagai **keputusan sadar**, bukan keterbatasan
sementara. Kolom/field yang hanya ada demi multi-tenant boleh disederhanakan — tapi lihat
peringatan di data-model §3 kalau multi-tenant sewaktu-waktu jadi target lagi.

Prefix `'vioni'` sendiri sebaiknya diganti nama netral (mis. `mini-erp`) di sistem baru; kalau
bucket S3 sudah terisi, itu menuntut rencana pemindahan object key.

**Pengaman path traversal** — `assertSafeKey()` menolak key yang:
- mengandung segmen `.` atau `..`
- diawali drive letter Windows (`/^[a-zA-Z]:/`)
- kosong setelah normalisasi

dan menormalkan `\` → `/`, runtuhkan `//`, buang slash awal/akhir. `sanitizeFileName()`
memisahkan basename, mengganti semua karakter non-`[a-zA-Z0-9._-]` jadi `-`, membuang titik di
awal, dan memotong ke 180 karakter.

**Konsumen:** `product.service.ts`, `payment.service.ts`, `delivery-proof.service.ts`,
`knowledge.service.ts` — persis 4, sesuai desain "semua upload lewat backend".

**✅ TERJAWAB — dua lokasi penyimpanan lokal.** Driver lokal menulis ke `process.cwd()/Upload`
(disajikan `main.ts` di `/uploads`), sementara repo berisi
**`apps/api/storage/knowledge/company-1/`** dengan puluhan file `.txt` ter-commit yang tidak
dirujuk kode mana pun.

Dikonfirmasi pemilik sistem: **`apps/api/storage/` adalah lokasi lama yang sudah mati** — sisa
sebelum refactor `StorageService`. Boleh diabaikan saat migrasi data, dan sebaiknya dihapus dari
repo (file `.txt` knowledge ter-commit tidak punya alasan berada di version control).

### 2.3 `ImageOptimizerService`

`apps/api/src/modules/storage/image-optimizer.service.ts` — berbasis `sharp`.

- Format diterima: **hanya** `jpeg`, `jpg`, `png`, `webp` → selain itu
  `BadRequestException('Hanya file gambar JPG, PNG, atau WebP yang diizinkan')`.
- Ada beberapa profil kualitas (82 / 88 / 88) untuk peran gambar berbeda.
- Output utama WebP; menolak dimensi tidak valid dan gambar gagal di-decode.

Detail mana profil untuk peran mana belum ditelusuri — bukan bagian shared-layer inti.

### 2.4 Nomor dokumen — **dua mekanisme, hanya satu punya helper**

Ini temuan paling penting di bagian numbering, karena menyangkut 6 module.

**A. `finance_document_sequences` — punya satu formatter bersama.**
`modules/finance/finance-document-sequence.util.ts` → `formatDocumentSequenceNumber(seq, referenceDate)`:
- template mendukung `{prefix}`, `{year}`, `{month}`, `{seq}`, `{seq:<width>}`
- `{year}`/`{month}` diambil dari **hari kalender WIB pada `referenceDate` transaksi**
  (`journalDate`/`expenseDate`/`cutoverDate`), bukan jam server saat generate
- komentar di kode menyebut ini menyatukan **4 salinan** logic yang sebelumnya semuanya memakai
  `new Date()`, sehingga posting mundur/reversal menghasilkan nomor bulan saat diposting

**B. `branch_document_sequences` — tidak ada formatter bersama.** Entity dipakai 6 module
(`order`, `delivery`, `purchase-return`, `sales-return`, `payment`, `stock-transfer`), tapi tiap
service memformat nomornya sendiri dengan `padStart(5, '0')` yang diulang-ulang:

| Lokasi | `sequenceKey` |
|---|---|
| `order.service.ts:844,851` | `order`, lalu `order_<kind>_<year>-<month>` |
| `order.service.ts:905` | `payment` |
| `payment.service.ts:707` | `payment` |
| `stock-transfer.service.ts:472` | `stock_transfer` |

Dan `order.service.ts:849` menurunkan `year`/`month` dari `now.getMonth()` — **waktu lokal
server**, bukan WIB seperti sisi finance. Bila server berjalan di UTC, nomor order di sekitar
tengah malam WIB bisa jatuh ke bulan yang salah. Ini asimetri nyata terhadap aturan
`resolveZonedDateRange` di §shared-business-rules §2.

**✅ TERVERIFIKASI — `format_template` pada `branch_document_sequences` ditulis tapi NEVER READ.**
Penelusuran seluruh penulis dan pembaca kolom itu:

| Penulis | Nilai yang ditulis |
|---|---|
| `seed-runner.ts:164` | `'{prefix}/{year}/{seq:05}'` untuk key `order`, `payment`, `sj`, `sales_return`, `purchase_return` (semua `reset_policy: 'yearly'`) |
| `branch.service.ts:63,76` | `'{prefix}/{year}/{seq:05}'` |
| `order.service.ts` (upsert atomik) | `'ORD-{code}/{kind}/{year}/{month}/{seq:05}'`, `reset_policy: 'monthly'` |
| `stock-transfer.service.ts:482` | `null` |

**Pembaca: tidak ada.** Setiap generator membangun stringnya sendiri dengan template literal +
`padStart(5, '0')`. Hanya `finance_document_sequences` yang punya pembaca
(`formatDocumentSequenceNumber`).

Lebih jauh: **template yang tersimpan bahkan tidak cocok dengan output nyatanya.**

| Sequence | `format_template` tersimpan | Output sebenarnya |
|---|---|---|
| `order` | `{prefix}/{year}/{seq:05}` | `${prefix}/${kindTag}/${year}/${month}/${num}` — 5 segmen, ada `kind` + `month` |
| `payment` | `{prefix}/{year}/{seq:05}` | `${prefix}/${year}/${num}` — cocok |

`reset_policy` juga dekoratif untuk skema bulanan: seed menulis `'yearly'` untuk `order`, service
menulis `'monthly'`, tapi **reset tidak pernah dijalankan oleh keduanya**. Reset terjadi sebagai
efek samping desain kunci — `sequenceKey` menyertakan `order_<kind>_<YYYY>-<MM>`, sehingga bulan
baru otomatis memakai baris baru. Komentar di `order.service.ts` menyatakannya:
*"reset terjadi tanpa perlu job/cron terpisah"*.

Satu detail lagi yang mudah menyesatkan: baris `sequenceKey: 'order'` **hanya dibaca untuk
mengambil `prefix`**, tidak pernah di-increment — `current_value`-nya tetap 0 selamanya. Counter
sebenarnya hidup di baris `order_<kind>_<YYYY>-<MM>`.

**Untuk rebuild:** pilih satu — kolom template benar-benar dipakai satu formatter bersama (seperti
finance), atau kolomnya dibuang dan format ditetapkan di kode. Kondisi sekarang adalah yang
terburuk dari keduanya: kolom yang tampak sebagai sumber kebenaran, berisi nilai yang salah, dan
tidak berpengaruh apa pun.

Dua strategi increment berdampingan di `order.service.ts`:
`incrementMonthlySequenceAtomic` (SQL upsert) dan `incrementMonthlySequenceViaOrm`
(`lock: { mode: 'pessimistic_write' }`). Pemilihan di antara keduanya belum ditelusuri —
bukan lapisan shared.

---

## 3. Backend — Utility Lintas Modul (`apps/api/src/common/`)

### 3.1 Pagination — `pagination.util.ts`

Tiga fungsi berlapis, satu sumber kebenaran clamp:

| Fungsi | Untuk | Perilaku kunci |
|---|---|---|
| `clampPageLimit(page, limit, fallbackLimit=50, maxLimit=100)` | primitif | limit di-clamp ke `[1, maxLimit]`; nilai 0/negatif/NaN/Infinity → `fallbackLimit` lalu clamp; page minimal 1 |
| `normalizePageLimit(...)` | QueryBuilder `.skip()/.take()` | sama + `skip = (page-1) * limit`. Alasan eksplisit di komentar: mencegah `LIMIT NaN` → `QueryFailedError` |
| `paginateRows(rows, page, limit, maxLimit=200)` | slice in-memory | **`limit == null` → kembalikan SEMUA baris** (`meta.limit = total`) |

Perilaku `limit == null` itu bukan kebetulan: komentar menyebut `FinanceExportService` bergantung
padanya untuk menerima seluruh data berkas pajak. Aturannya: **frontend selalu kirim `limit`,
konsumen internal/export tidak**. Ini kontrak implisit yang mudah dilanggar saat rebuild.

Catatan: `paginateRows` memanggil `clampPageLimit(page, limit, maxLimit, maxLimit)` — argumen
`fallbackLimit` diisi `maxLimit`, jadi fallback-nya 200, bukan 50 seperti default.

### 3.2 Pencarian — `search/tokenized-search.ts`

| Fungsi | Fungsi |
|---|---|
| `tokenizeSearch(raw)` | Normalisasi whitespace → split per kata → **maksimal 8 token** (`MAX_TOKENS`) |
| `applyTokenizedLike(qb, columns, raw, paramPrefix='tk')` | Tambah filter ke QueryBuilder: **AND antar token, OR antar kolom**. Tiap token jadi grup berkurung sendiri agar tidak bocor ke filter lain (company/archive). Mengembalikan jumlah token |
| `rawTokenizedLike(columns, raw)` | Versi SQL mentah (placeholder `?`) → `{ clause, params }` |

Efek yang dijamin: pencarian tahan spasi ganda (di input **maupun** di data) dan tidak peduli
urutan kata. Contoh di komentar berasal dari data nyata hasil import:
`"ACLOSE SUPER SOLID WARNA SPECIAL  1 KG"`.

### 3.3 Uang & angka — **dua standar pembulatan yang berbeda**

Ini titik paling mudah salah saat rebuild.

| Helper | Lokasi | Pembulatan | Untuk |
|---|---|---|---|
| `roundRupiah(value)` | `packages/shared-types/src/money.ts` | **rupiah utuh** (`Math.round(v + EPSILON)`) | nominal yang ditagih/dibayar: total order, baris order, pembayaran, selisih retur |
| `money(value)` | `apps/api/src/common/number.util.ts` | **2 desimal** | "perilaku lama seluruh service finance" |
| `toNumber(value, fallback=0)` | idem | — | `Number()` aman, fallback bila bukan finite |
| `toMoney(value)` | idem | 2 desimal | gabungan `toNumber` + `money` |

`number.util.ts` menyimpan peringatannya sendiri di komentar:

> CATATAN: `money` di sini membulatkan ke 2 desimal (perilaku lama seluruh service finance).
> Ini BERBEDA dari `PaymentService.money` (pembulatan rupiah utuh) — jangan dicampur.

Sementara `roundRupiah` mengklaim dirinya "SATU-SATUNYA sumber kebenaran" aturan rupiah utuh,
dan mengecualikan tarif/persentase pajak (boleh berdesimal). Migrasi
`043_round_money_to_whole_rupiah.sql` menandakan pergeseran ke rupiah utuh pernah dilakukan di
level data.

**✅ KEPUTUSAN — rupiah utuh di SEMUA lapisan.** Koeksistensi dua standar dihentikan di sistem
baru: `roundRupiah()` menjadi satu-satunya aturan, dan `money()` 2-desimal **tidak dibawa**.

Konsekuensi yang wajib direncanakan, bukan ditemukan belakangan:

| Dampak | Tindakan |
|---|---|
| Angka laporan finance akan **bergeser sedikit** dari sistem lama (pembulatan berbeda di lapisan internal) | Rekonsiliasi saat migrasi: bandingkan trial balance / P&L sistem lama vs baru per periode, dokumentasikan selisihnya |
| Saldo piutang/utang hasil migrasi bisa menyisakan pecahan dari data lama | Bulatkan saat migrasi + catat penyesuaiannya sebagai jurnal koreksi yang bisa dilacak, jangan dibulatkan diam-diam |
| Tarif/persentase pajak **tetap** boleh berdesimal | Aturan `money.ts` soal ini tidak berubah — jangan ikut dibulatkan |

Migrasi `043_round_money_to_whole_rupiah.sql` menunjukkan pergeseran ini sebagian sudah pernah
dilakukan di level data; keputusan ini menuntaskannya sampai ke lapisan perhitungan.

### 3.4 Kuantitas & UOM — `packages/shared-types/src/quantity.ts`

Dipakai API **dan** Web. Konstanta: `QUANTITY_SCALE = 12`, `DISPLAY_QUANTITY_SCALE = 4`,
`QUANTITY_ZERO_TOLERANCE = 0.000001`.

| Fungsi | Fungsi |
|---|---|
| `toFiniteNumber(v, fallback=0)` | coerce aman |
| `roundQuantity(v, scale=12)` | bulatkan ke 12 desimal (koreksi EPSILON) |
| `snapQuantityToZero(v, tol)` | nilai di bawah toleransi → **tepat 0** |
| `snapQuantity(v, tol)` | snap ke 0, lalu snap ke bilangan bulat terdekat bila selisihnya di bawah toleransi |
| `normalizeUomFactor(v, fallback=1)` | faktor harus `> 0`, kalau tidak pakai fallback |
| `toBaseQuantity(qty, factor)` | qty × factor, di-snap |
| `fromBaseQuantity(qtyBase, factor)` | qtyBase ÷ factor, di-snap |
| `formatQuantity(v, maxFrac=4)` | format `id-ID` |
| `formatConversionLabel(txUom, baseUom, factor)` | `"1 DUS = 300 PCS"` |
| `formatBaseToTransactionLabel(...)` | arah sebaliknya |
| `formatDualUomStockLabel({...})` | `"2 DUS (600 PCS)"`; jika UOM sama & factor 1 → hanya label dasar |
| `quantityInputStep(factor)` | `1` bila factor integer ≥ 1, selain itu `0.0001` |

`snapQuantity` adalah pertahanan utama terhadap sisa floating-point pada stok — tanpa itu,
pembagian UOM meninggalkan `0.0000000001` yang membuat saldo "tidak pernah nol".

### 3.5 Harga modal — `uom-cost.util.ts` dan `cost-sanity.util.ts`

**`resolveBaseUomCost(purchasePrice, purchaseToBaseFactor)`** — satu-satunya cara menurunkan
harga modal per satuan dasar dari `products.purchase_price`. Alasannya ada di komentar:
`purchase_price` tersimpan per **satuan beli**, sedangkan `average_cost`/ledger biaya selalu per
**satuan dasar**. Insiden nyata yang disebut: produk dibeli per DUS isi 300 pcs, tanpa pembagian
ini harga modal tercatat 300× lebih tinggi. Aturan tegas di komentar: **jangan duplikasi rumus
`purchase_price / factor` di tempat lain.**

Perilaku tepi: `price <= 0` → `0`; `factor <= 0` → kembalikan `price` apa adanya (tanpa bagi).

**`assertReasonableUnitCost(enteredCost, referenceCost, context)`** — guard **magnitude**, bukan
presence. Rasio `entered / reference` harus di `[0.1, 10]`, kalau di luar → `BadRequestException`
berpesan panjang dan owner-friendly (menyebut kemungkinan salah digit/desimal, dan menyarankan
perbarui harga master dulu).

Lolos tanpa cek bila `referenceCost <= 0` **atau** `enteredCost <= 0` — guard lain yang menangani
kasus itu.

Latar belakangnya ditulis eksplisit di kode: insiden Agustus 2026, saldo awal 7 produk dientri
100–1000× harga wajar, mencemari HPP selama 6+ minggu, berdampak ~Rp 3,2 miliar ke jurnal yang
sudah terposting. Rentang sengaja longgar agar kenaikan harga supplier wajar tidak terblokir.

### 3.6 Tanggal — `date.util.ts` dan `date-range.util.ts`

Dibahas sebagai aturan global di [shared-business-rules.md](shared-business-rules.md) §2 karena
keduanya menegakkan kebijakan, bukan hanya menyediakan fungsi.

Ringkas: `parseDateInput(value, label)` untuk parse input tunggal;
`resolveZonedDateRange(tz, from, to)` + `todayInZone(tz)` + konstanta `APP_TIME_ZONE` untuk
rentang laporan.

### 3.7 Error DB — `db-error.util.ts`

`isUniqueViolation(err)` — deteksi `ER_DUP_ENTRY` / `errno 1062`, termasuk saat terbungkus
`driverError`. Tujuannya disebut di komentar: mengubah race "pre-check lalu insert" yang lolos
menjadi `ConflictException` (4xx), bukan `internal_error` (500), ketika dua request membuat
kode/periode yang sama bersamaan.

---

## 4. Frontend — Service Lintas Modul

### 4.1 `lib/api.ts` — satu-satunya `fetch` di seluruh web

Ini titik tersempit di frontend, dan ditegakkan otomatis (lihat
[shared-business-rules.md](shared-business-rules.md) §6).

**Token storage** — `localStorage`, kunci `mini-erp-access` dan `mini-erp-refresh`:
`getStoredTokens()`, `storeTokens(access, refresh)`, `clearTokens()`.

**`apiPost<T>(path, data = {}, options)`**

| Aspek | Perilaku |
|---|---|
| Method | Selalu `POST` ke `${BASE_URL}/api/v1/${path}` |
| Body | Selalu `JSON.stringify({ data })` — pemanggil kirim payload mentah |
| Auth | `Authorization: Bearer <access>` kecuali `noAuth: true` |
| Envelope | Buka `json.data`; bila `json.code !== 0` lempar `ApiError(code, info, errors)` |
| 401 | Refresh token **sekali** lalu retry dengan `_retried: true`. Gagal refresh → `clearTokens()` + `ApiError(401, 'session_expired')` |
| Loading | `loadingBus.start()` di awal, `end()` di `finally` — retry **tidak** menambah counter (`isFreshCall`) |
| `silent: true` | Lewati loading bus (untuk refresh data background) |

**`apiUpload<T>(path, formData, options)`** — sama, tapi `Content-Type` **tidak** di-set (browser
menambah boundary sendiri) dan tidak ada opsi `noAuth`.

**Antrean refresh** — `refreshPromise` singleton memastikan puluhan request 401 bersamaan hanya
memicu **satu** panggilan `auth/refresh`. Refresh dilakukan lewat `fetch` langsung, bukan
`apiPost`, agar tidak memicu loading bus — disebut di komentar sebagai "operasi infrastruktur,
bukan aksi user".

**`parseApiJson`** — bila respons bukan JSON (halaman error HTML dari proxy/gateway), tidak
melempar `Unexpected token '<'` tapi `ApiError` berpesan Indonesia sesuai status:

| Status | Pesan |
|---|---|
| 413 | File terlalu besar untuk diproses server. |
| 408, 504 | Server terlalu lama merespons (timeout). Untuk data besar, pecah file lalu ulangi. |
| 502, 503 | Server sedang sibuk/tidak tersedia. Coba beberapa saat lagi. |
| ≥ 500 | Terjadi kesalahan di server. Coba lagi. |

**`extractErrorMessage(err, fallback)`** — prioritas `errors` (user-friendly dari server) → `info`
(kode mesin) → fallback `"Terjadi kesalahan. Silakan coba lagi."`

`ApiError` sendiri memakai `errors ?? info` sebagai `message`.

### 4.2 `loadingBus` + `useGlobalLoading()`

`lib/loading-bus.ts` — singleton counter **di luar React** (agar `apiPost`, yang plain TS, bisa
mengaksesnya). `start()` increment, `end()` decrement dengan `Math.max(0, …)`, `isLoading()` =
`pendingCount > 0`, `subscribe(listener)` mengembalikan unsubscribe.

`hooks/use-global-loading.ts` — membacanya via `useSyncExternalStore` (server snapshot selalu
`false`; tidak ada SSR). Dikonsumsi `components/feedback/global-loader.tsx`.

Efeknya: setiap `apiPost`/`apiUpload` otomatis terpantau GlobalLoader **tanpa perubahan di sisi
pemanggil** — tidak ada module yang perlu mengelola state loading global sendiri.

### 4.3 `useToasts()` — notifikasi global

`store/use-toasts.ts`. `notify(title, description?, tone = "info")` menambah toast ber-ID
`safeRandomId("toast")` dan menghapusnya otomatis setelah **3400 ms**. Timer disimpan di `useRef`
dan dibersihkan saat unmount.

`notify` diteruskan ke **setiap slice** sebagai argumen konstruktor
(`useProductsSlice(setCompanyData, notify)`, dst) — jadi tipe `NotifyFn` ini adalah kontrak
lintas-slice, didefinisikan **dua kali**: di `store/use-toasts.ts` dan di `store/store-types.ts`
(identik). Duplikasi kecil, tapi nyata.

### 4.4 `AppProvider` / `useAppStore()` — root state

`store/app-store.tsx` (834 baris). Context tunggal; `useAppStore()` melempar
`"useAppStore must be inside AppProvider"` bila dipakai di luar provider.

**Komposisi 11 slice** — semuanya di-instansiasi di dalam `AppProvider`, tiap slice menerima
setter dan `notify`:

```
useProductsSlice(setCompanyData, notify)
useStockSlice(setBranchData, notify)
useOrdersSlice(setBranchData, notify, sliceStock.reloadStock)   ← satu-satunya slice
usePartiesSlice(setCompanyData, notify)                            yang bergantung slice lain
useMemberTypesSlice(setCompanyData, notify)
useUsersSlice(setCompanyData, setCompanyUsers, notify)
useRolesSlice(setCompanyData, companyData, notify)
useStatusesSlice(setCompanyData, notify)
useAssistantSlice(setCompanyData, storedSession, notify)
useCompanySlice(setCompanyData, setCompanyBranches, setStoredSession, notify)
useAuditLogSlice(setCompanyData, companyBranches)                ← tanpa notify
```

`useOrdersSlice` menerima `sliceStock.reloadStock` — order yang mengubah stok memicu reload stok.
Ini satu-satunya kopling antar-slice yang eksplisit.

**Pemuatan data: lazy per module.** State `moduleStatus: Record<string, 'idle'|'loading'|'ready'>`
dibungkus di sekitar tiap `reload*()`:

```ts
const reloadProducts = useCallback(async () => {
  setModuleStatus((prev) => ({ ...prev, products: 'loading' }));
  await sliceProducts.reloadProducts();
  setModuleStatus((prev) => ({ ...prev, products: 'ready' }));
}, [sliceProducts]);
```

Ada 10 `reload*` yang diekspos (products, orders, stock, locations, parties, users, roles,
statuses, memberTypes, auditLogs). `loadBootstrap()` hanya memuat status definitions + settings;
sisanya menunggu halamannya dikunjungi. Saat ganti cabang, pesan yang muncul menegaskan model
ini: *"Data tiap modul akan dimuat ulang saat Anda mengunjunginya."*

**Perilaku sesi** dibahas di [shared-business-rules.md](shared-business-rules.md) §4.

`can(permission?)` — `!permission` → `true`; selain itu `permissions.includes(permission)`.

**Helper lintas modul** yang diekspos context: `findUserById`, `findUserName`, `findUsername`,
`findItemName`, `notify`.

**✅ KEPUTUSAN — bentuk DAN nama dirancang ulang.** Tipe `MockDatabase` dan `DemoUser` masih
dipakai sebagai tipe context utama (`database: MockDatabase`, `activeUser?: DemoUser`), dan header
file menyebut *"Real application context replacing the mock provider … Provides an interface
compatible with MockAppContextValue so existing pages don't need to change"*. Jadi bentuk state
frontend sekarang mewarisi bentuk mock lama, bukan diturunkan dari kebutuhan domain.

Di sistem baru, bentuk state didesain dari kebutuhan domain. **Batasan yang mengikat: UX flow dan
tampilan wajib tetap sama persis** — yang berubah hanya struktur internal. Karena itu urutannya
penting: petakan dulu data apa yang setiap halaman benar-benar butuhkan (dari analisis modul),
baru rancang state-nya. Merancang state lebih dulu lalu memaksa halaman menyesuaikan adalah cara
tercepat kehilangan kesamaan tampilan.

Pembagian `CompanyData` / `BranchData` yang mengikuti scope DB (lihat
[shared-data-model.md](shared-data-model.md) §7.4) layak dipertahankan sebagai konsep — ia
memetakan langsung ke perilaku "ganti cabang me-reset data cabang".

### 4.5 Adapter — `store/adapters/`

Barrel `adapters/index.ts` me-re-export 5 file per domain (`common`, `catalog`, `commerce`,
`inventory`, `org`) sehingga path import publik `store/adapters` tetap satu pintu.

`common.adapter.ts` memegang yang benar-benar lintas domain:

| Fungsi/Konstanta | Peran |
|---|---|
| `saveSession` / `loadSession` / `clearSession` | Persist `StoredSession` ke `localStorage` kunci `mini-erp-session`; `loadSession` dibungkus try/catch → `null` bila JSON rusak |
| `StoredSession` (tipe) | Bentuk sesi yang dipersist — lihat [shared-data-model.md](shared-data-model.md) §5 |
| `normalizeOrderKind` | **Peta nilai legacy:** `"transaction"` → `"sales"`, `"request"` → `"purchase"`; selain itu fallback `"sales"` |
| `normalizeApplicableOrderKind` | idem + `"all"` |
| `normalizePaymentTerms(v, orderKind)` | fallback per jenis: purchase → `"net"`, sales → `"cod"` |
| `normalizeFinancialSide(v, orderKind)` | fallback: purchase → `"payable"`, sales → `"receivable"` |
| `normalizeFinancialStatus(v)` | fallback `"open"` |
| `DEFAULT_SETTINGS` | Default `CompanySettings` lengkap (label bisnis, aturan operasional, preferensi AI/laporan) |
| `EMPTY_COMPANY_DATA` / `EMPTY_BRANCH_DATA` | Bentuk state awal |
| `toOptionalNumber(v)` | `null`/`undefined`/`""` → `undefined`; non-finite → `undefined` |

Peta legacy `transaction`/`request` selaras dengan migrasi `007_rename_order_kind_values.sql` —
adapter masih menoleransi data lama. Untuk rebuild dengan schema baru, ini kandidat kuat untuk
**tidak** dibawa (tapi datanya perlu dikonversi saat migrasi).

`DEFAULT_SETTINGS` berisi nilai yang punya arti bisnis dan patut disalin apa adanya:

```
businessLabels: itemLabel "Produk", orderLabel "Order", stockLabel "Stok", customerLabel "Pelanggan"
operationalRules: defaultOrderKind "sales", defaultDueDays 7, stockAdjustmentApprovalMode "simple"
aiPreferences: tone "professional", responseLanguage "id", answerStyle "concise",
               whatsapp_mode "rule_based", ai_daily_limit 50
reportingPreferences: defaultRange "month"
```

### 4.6 Formatter tampilan — `apps/web/src/utils.ts`

Semua berbasis `Intl` dengan locale **`id-ID`** keras (bukan dari company locale —
lihat [shared-business-rules.md](shared-business-rules.md) §3).

| Fungsi | Output |
|---|---|
| `formatCurrency(amount, currencyCode="IDR")` | `style: "currency"`, `maximumFractionDigits: 0` → `"Rp 95.000"` |
| `formatDateTime(value?)` | `"24 Jun 2026, 14.30"`; `undefined` → `"-"` |
| `formatDateOnly(value?)` | `"24 Jun 2026"`; `undefined` → `"-"` |
| `formatLongDate(value?)` | `"24 Juni 2026"` — untuk dokumen cetak |
| `compactNumber(value)` | notasi compact, 1 desimal |
| `formatRoleLabel(role?)` | Peta eksplisit `superadmin`/`owner`/`admin`/`staff`; role lain → Title Case dari `snake_case` |
| `statusTone(group)` | `pending`→warning, `active`→info, `completed`→success, `cancelled`→danger |
| `channelTone(state)` | `connected`→success, `reconnecting`→warning, `disconnected`→danger |
| `safeRandomId(prefix="id")` | `crypto.randomUUID()` bila ada; fallback counter + timestamp base36 + random |
| `safeParseJson<T>(value, fallback)` | try/catch → fallback |
| `terbilangRupiah(amount?)` | Eja nominal ke kata Indonesia |

**`terbilangRupiah`** layak dicatat khusus — dipakai di nota cetak, jadi outputnya bagian dari
"tampilan sama persis". Aturan yang terbaca dari `terbilangInteger`:
- `0` → `"nol rupiah"`
- kasus khusus Indonesia ditangani: `"sebelas"`, `"… belas"`, `"seratus"`, `"seribu"`
- skala sampai **triliun** (`triliun`, `miliar`, `juta`, `ribu`)
- nilai negatif diawali `"minus "`; pecahan dibulatkan (`Math.round(Math.abs(...))`)

### 4.7 Utility frontend lain

| File | Peran |
|---|---|
| `lib/image-compression.ts` | Kompresi gambar di browser sebelum upload |
| `lib/native-printer.ts` | Bridge ke shell Android (`android-pos-shell`) untuk printer ESC/POS |
| `lib/pwa-registration.ts` | Registrasi service worker |
| `lib/chunk-load-recovery.ts` | Recovery saat chunk lazy gagal dimuat (setelah deploy baru) |
| `utils/qr-export.ts` | Export QR produk |
| `modules/shared/attribute-field.ts` | Field atribut dinamis lintas modul |

`lib/` adalah satu-satunya direktori yang dikecualikan dari larangan `fetch` — dan
`chunk-load-recovery` + `pwa-registration` sama-sama menyentuh lifecycle deploy, sehingga
keduanya perlu dipikirkan bersama saat rebuild (route lazy + service worker + versi bundle).

---

## 5. Design System — `apps/web/src/components/`

Lapisan yang paling menentukan target "tampilan SAMA PERSIS". Barrel `components/index.ts`
mengekspor primitif + struktur + feedback + overlay + form + data; komposit domain
(`*-search-select`, `qr-*`, `pagination`, `password-input`) sengaja **tidak** di-barrel dan
diimpor lewat path langsung.

| Kelompok | Komponen |
|---|---|
| `primitives/` | `Button` (+ `buttonClass()`), `Badge` |
| `structure/` | `PageHeader`, `SectionCard`, `SummaryCard`, `FilterBar`, `ActionRow` |
| `feedback/` | `Notice`, `EmptyState`, `LoadingState`, `GlobalLoader` |
| `overlays/` | `Modal`, `ConfirmDialog`, `ToastViewport` |
| `forms/` | `FormField`, `SelectField`, `SegmentedControl`, `SearchSelect`, `HierarchicalSelect`, `AsyncSearchSelect`, `FieldHint`, `PasswordInput`, `select-shared` |
| `data/` | `DataTable`, `Pagination` |
| `domain/` | `ProductSearchSelect`, `PartySearchSelect`, `StockLocationSelect`, `QrScannerModal`, `QrGeneratorModal`, `PwaUpdatePrompt` |

**Kontrak props terpusat di `components/types.ts`** — semua props komponen DS didefinisikan di
satu file (`ButtonProps`, `ModalProps`, `DataTableProps<T>`, `SelectOption`, dst), bukan di file
komponennya. `Tone` adalah union bersama:
`neutral | info | success | warning | danger | accent`.

**`DataTable<T>`** adalah kontrak tabel seluruh aplikasi:

```ts
type DataColumn<T> = { header: string; width?: string; align?: "left"|"right"; render: (row: T) => ReactNode };
type DataTableProps<T> = { columns; rows; rowKey; rowHref?; onRowClick? };
```

Perilaku bawaan: wrapper `overflow-x-auto`, `min-w-[720px]`, header uppercase + tracking,
baris terakhir tanpa border bawah, baris jadi `cursor-pointer` + hover bila ada `rowHref` atau
`onRowClick`.

**✅ TERVERIFIKASI — `rowHref` adalah permukaan API mati.** Pencarian `rowHref` di seluruh
`apps/web/src` hanya menghasilkan **3 kemunculan, semuanya di dalam design system itu sendiri**:

```
components/types.ts:170          deklarasi  rowHref?: (row: T) => string | undefined
components/data/data-table.tsx:7  destructuring
components/data/data-table.tsx:31 const href = rowHref?.(row)   ← hanya untuk kelas hover
```

**Nol konsumen.** Tidak satu pun halaman list di seluruh aplikasi meneruskan `rowHref`; semua
baris klikabel memakai `onRowClick`. Jadi tidak ada navigasi via `href` yang hilang — fitur itu
memang tidak pernah terpakai.

Untuk rebuild: buang `rowHref` dari kontrak `DataTable`. Kalau nanti butuh baris sebagai tautan
nyata (agar bisa "buka di tab baru" / ramah SEO), rancang ulang dengan `<Link>` di dalam sel,
bukan menghidupkan prop ini.

**Token desain** — `apps/web/src/tailwind.css` (128 baris) adalah satu-satunya sumber styling;
`styles.css` lama sudah dihapus dan **preflight Tailwind sengaja dimatikan** (hanya layer `theme`
+ `utilities` yang diimpor, reset dasar ditulis manual di `@layer base`).

```
Netral   ink #0f172a · heading #334155 · muted #64748b · hairline #cbd5e1
         surface #ffffff · surface-subtle #f8fafc · surface-sunken #f1f5f9
Brand    brand #1e3a5f · brand-hover #16314f          (steel-blue, "BUKAN indigo generik")
Aksen    accent #c2603a · accent-soft #fceae0          (terracotta)
Status   ok #15803d/#dcfce7 · warn #b45309/#fef3c7 · bad #b91c1c/#fee2e2
Sidebar  sidebar #0f172a · sidebar-fg #94a3b8 · sidebar-strong #f8fafc
Bentuk   font Inter · radius 6/8/10px
```

Blok `:root` legacy (`--color-primary`, `--text-muted`, `--shadow-*`, `--topbar-height`)
dipertahankan **karena recharts butuh string warna nyata, bukan class utility** — bukan sisa yang
bisa dihapus begitu saja.

`field-classes.ts` memegang satu konstanta `INPUT`: kelas kanonik semua kontrol input, dipakai
bersama field dan trigger select agar konsisten lintas module.

**Kelas legacy dipertahankan sebagai hook E2E.** `Button` masih menempelkan
`button button-<variant> button-<size>` di samping utility Tailwind, dengan komentar: *"Kelas
legacy `button/button-*` dipertahankan sbg hook E2E; styling murni utility."* Hal serupa di
`DataTable` (`table-wrap`, `data-table`, `table-clickable-row`) dan `AppLayout`
(`sidebar-panel`). **Menghapus kelas-kelas ini akan merusak spec E2E**, bukan tampilan.

---

## 6. App Shell & Routing

### 6.1 `modules/module-registry.tsx` — sumber tunggal navigasi

Satu file (944 baris) memegang route + menu + permission + judul halaman. Semua page di-`lazy()`
sehingga bundle per module diunduh saat pertama dikunjungi.

`AppRouteDefinition`:

```ts
{ path; title; access: "public"|"authenticated"|"protected"; element;
  permission?; shell?; menu?: { group; section?; label; icon?; note?; end? } }
```

Turunan otomatis dari `APP_ROUTES` — inilah sebabnya file ini jadi sumber tunggal:

| Turunan | Cara |
|---|---|
| `APP_MENU_GROUPS` | Filter per `menu.group` mengikuti `MENU_GROUP_ORDER`, lalu per section mengikuti `MENU_SECTION_ORDER`; section tak terdaftar diletakkan setelahnya; group/section kosong dibuang |
| `ROUTE_ACCESS_RULES` (di `route-access.ts`) | `APP_ROUTES.filter(r => r.shell && r.access === "protected")` → `{ pattern, permission }` |
| `getPageTitle(pathname)` | `matchPath` `end: true`; fallback `"Dashboard"` |
| `getActiveMenuPath(pathname, items)` | `matchPath` `end: item.end ?? false`; bila banyak yang cocok, **pilih `to` terpanjang** |

`MENU_GROUP_ORDER` = `Utama`, `Operasional`, `Stok & Gudang`, `Produk & Mitra`, `Keuangan`,
`Pantauan`, `Asisten WA`, `Pengaturan`. Untuk grup `Keuangan` ada urutan section:
`Harian`, `Cek Bisnis`, `Pajak & Akhir Bulan`, `Detail`.

`AppMenuIconKey` adalah union **41 nama ikon** — mengunci ikon per menu, bagian dari
"tampilan sama persis".

### 6.2 `app.tsx` — penegakan akses

Tiga wrapper dipilih dari `route.access` (lihat [shared-business-rules.md](shared-business-rules.md) §5):
`PublicOnly`, `AuthenticatedOnly`, `RouteGuard`.

Struktur render: route `!shell` di-render standalone (login, print pages), route `shell: true`
di-nest dalam `<AppLayout />`. Semua dibungkus `<Suspense fallback="Memuat halaman...">`.
Di luar `<Routes>`: `GlobalLoader`, `PwaUpdatePrompt`, `ToastViewport`.

**✅ TERVERIFIKASI — `route-access.ts` punya peran nyata dan spesifik, bukan sisa desain.**
`getRequiredPermission()` memang hanya diimpor `layout/avatar-menu.tsx:116`, dan penegakan akses
rute yang sebenarnya di `app.tsx` memakai `route.permission` langsung dari registry. Tapi
pemakaian tunggalnya menjawab pertanyaan yang berbeda: **"setelah ganti role, apakah pengguna
masih boleh berada di halaman ini?"**

```ts
const changed = await switchRole(role);
if (!changed) return;

const requiredPermission = getRequiredPermission(location.pathname);
const canStay =
  !requiredPermission ||
  (activeCompanyData?.rolePermissions[role] ?? []).includes(requiredPermission);

if (!canStay) navigate("/dashboard");
```

Perhatikan ia membaca `activeCompanyData.rolePermissions[role]` — set permission **role baru** dari
data company, bukan dari sesi (yang belum tentu ter-update saat itu).

Ini perilaku UX yang harus direplikasi: ganti role sambil berada di halaman yang role barunya tidak
punya akses **tidak** meninggalkan pengguna di halaman kosong/403 — ia dipindahkan ke
`/dashboard`. Namanya (`route-access`) memang terlalu luas untuk perannya; di sistem baru lebih
tepat dinamai sesuai fungsinya (mis. `canStayOnRouteAfterRoleSwitch`).

### 6.3 `layout/app-layout.tsx`

Shell aplikasi: sidebar (tema gelap) + topbar + `<Outlet />`. Status collapse sidebar dipersist ke
`localStorage` kunci **`mini-erp.sidebar.collapsed`**, dibungkus try/catch dengan komentar
eksplisit: *"The sidebar remains usable if browser privacy settings block localStorage."*
Breakpoint mobile `max-[900px]:`. `layout/avatar-menu.tsx` memegang menu profil/role/cabang.

Perhatikan tiga kunci `localStorage` terpisah dengan konvensi penamaan **tidak seragam**:
`mini-erp-access` / `mini-erp-refresh` / `mini-erp-session` (tanda hubung) vs
`mini-erp.sidebar.collapsed` (titik).

---

## 7. Infrastruktur & Tooling Bersama

### 7.1 Bootstrap API — `main.ts`

| Konfigurasi | Nilai |
|---|---|
| Global prefix | `api/v1` |
| ValidationPipe | `whitelist: true`, `forbidNonWhitelisted: false`, `transform: true`, `enableImplicitConversion: true` |
| Interceptor global | `ResponseInterceptor` |
| Filter global | `HttpExceptionFilter` |
| CORS | `app.enableCors()` — **tanpa argumen** (semua origin) |
| Static | `process.cwd()/Upload` disajikan di `/uploads` |
| Shutdown | `enableShutdownHooks()` |
| Port | `process.env.PORT ?? 3001` |

`enableCors()` tanpa opsi berarti seluruh origin diizinkan. **[PERLU KONFIRMASI — masih
terbuka]** apakah dibatasi di level reverse proxy VPS; pemilik sistem belum memastikan. Ini satu
dari dua item yang masih menggantung di lapisan shared.

Kalau ternyata tidak dibatasi di mana pun, situs mana pun bisa memanggil API ini dari browser
korban. Yang membatasi dampaknya: token disimpan di `localStorage` (bukan cookie), jadi request
lintas-origin tidak otomatis membawa kredensial — penyerang tetap butuh token. Jadi ini
memperlebar permukaan serang, bukan lubang langsung. Di sistem baru: batasi origin secara
eksplisit di aplikasi, apa pun yang dilakukan proxy.

### 7.2 `app.module.ts`

`ConfigModule.forRoot({ isGlobal: true, envFilePath: '.env' })` + `ScheduleModule.forRoot()` +
`DatabaseModule` + 16 module domain. `StorageModule` dan `ToolsModule` **tidak** terdaftar di
sini — keduanya diimpor oleh module yang memakainya, bukan global.

### 7.3 `DatabaseModule`

```
type mysql · synchronize: FALSE · charset utf8mb4 · timezone 'Z'
entities: __dirname + '/../../**/*.entity{.ts,.js}'  (auto-glob, 70 entity)
logging: ['error','warn','schema','migration'] · logger: CleanDatabaseLogger
maxQueryExecutionTime: 1000
```

`timezone: 'Z'` adalah akar dari seluruh urusan zona waktu di §shared-business-rules §2:
datetime tersimpan UTC, pengguna berpikir dalam WIB.

### 7.4 `migration-runner.ts`

**Temuan penting: tidak ada tabel pelacak migrasi.** Runner membaca seluruh
`migrations/*.sql`, mengurutkan by nama, dan menjalankan **semuanya, setiap kali dijalankan**.
Yang membuat ini aman adalah daftar error yang diabaikan:

| errno | Arti |
|---|---|
| 1050 | Table already exists |
| 1054 | Unknown column (backfill kolom yang sudah dihapus) |
| 1060 | Duplicate column name |
| 1061 | Duplicate key name |
| 1091 | Key/column sudah tidak ada |
| 1826 | Duplicate foreign key constraint name |

Statement dipecah dengan `splitStatements()`: strip komentar `--`, lalu split pada `;`.
Koneksi memakai `multipleStatements: false` agar tiap statement bisa di-catch sendiri. Error di
luar daftar → tutup koneksi dan lempar.

Konsekuensi yang harus dipahami: **idempotensi adalah tanggung jawab penulis SQL**, bukan runner.
Split naif pada `;` juga berarti SQL yang mengandung `;` di dalam string literal atau body
trigger/procedure akan pecah salah — belum saya periksa apakah ada migrasi seperti itu di antara
50 berkas tersebut.

### 7.5 `seed-runner.ts`

Mengisi data awal: company, branches, roles, permissions, status definitions + transitions, dan
baseline finance.

| Aspek | Nilai terbaca |
|---|---|
| Company | `id_company = 1` — **single-company** |
| Branch | `id_branch = 1`, `is_default = 1`, plus default stock location |
| Role ID tetap | `superadmin: 1, owner: 2, admin: 3, staff: 4, kasir: 5` |
| Role sistem | Di-insert dengan `id_company = NULL`, `is_system_role = 1` |
| `kasir` | **Custom role per-company** (`id_company` terisi), bukan role sistem |
| Permission | Diturunkan dari `DEFAULT_ROLE_PERMISSIONS` di `@mini-erp/shared-contracts` |
| Finance baseline | Chart of account (dengan `system_key`), `finance_account_mappings` (termasuk `expense_default` → `expense_other`), `finance_document_sequences`, cash account `CASH_MAIN` + `BANK_MAIN` |

Seluruh insert memakai `INSERT IGNORE` / `WHERE NOT EXISTS` sehingga seed bisa dijalankan
berulang. Ada juga pembersihan `branch_document_sequences` yang branch-nya sudah hilang
(`LEFT JOIN … WHERE b.id_branch IS NULL`).

### 7.6 `check-architecture.mjs` — penegak batas

Node murni tanpa dependency; komentar menyebut alasannya: environment tidak bisa `npm install`
ESLint (npm-cache ENOSPC). Aturannya dirinci di
[shared-business-rules.md](shared-business-rules.md) §6.

### 7.7 Test infrastruktur bersama

**E2E** — `apps/e2e/global-setup.ts`:
1. Reset DB `mini_erp_test` (migrasi + seed) — **ada guard**: kalau `DB_NAME` bukan
   `mini_erp_test`, throw `"E2E reset blocked because DB_NAME is not mini_erp_test"`
2. Login `superadmin` / `superadmin123` via API test di port 3002
3. Simpan `storageState` ke `playwright/.auth/superadmin.json` → semua test mulai
   pre-authenticated

`apps/e2e/helpers/business.ts` (1024 baris) adalah lapisan fixture bisnis bersama — ~40 helper
(`createPhysicalProduct`, `createParty`, `createBranch`, `addStock`, `getPartyBalance`,
`moveDamagedStock`, `switchActiveBranch`, dst) plus `apiPost`/`apiUpload` versi Playwright dan
`e2eRunId = 'BIZ' + Date.now().toString(36)` untuk isolasi data antar-run.

**Unit web** — `apps/web/src/test/setup.ts` + `test/mock-store.ts` (Vitest + Testing Library).
`mock-store.ts` menyediakan store tiruan agar komponen bisa dites tanpa `AppProvider` nyata.

---

## 8. Ringkasan Temuan untuk Rebuild

Hal-hal di lapisan shared yang **tidak boleh hilang** karena masing-masing membawa cerita insiden
atau kontrak implisit:

| # | Hal | Kenapa |
|---|---|---|
| 1 | `roundRupiah` (rupiah utuh) | Mencegah saldo nyangkut recehan yang mustahil dilunasi |
| 2 | `snapQuantity` + toleransi 1e-6 | Mencegah stok "tidak pernah nol" akibat sisa float |
| 3 | `resolveBaseUomCost` | Insiden nyata: harga modal 300× akibat beli per DUS, stok per PCS |
| 4 | `assertReasonableUnitCost` | Insiden nyata: HPP tercemar 6+ minggu, ~Rp 3,2 M jurnal terposting |
| 5 | `resolveZonedDateRange` | Filter tanggal UTC vs hari WIB memotong hari terakhir laporan |
| 6 | `formatDocumentSequenceNumber` pakai tanggal transaksi | Posting mundur pernah menghasilkan nomor bulan salah |
| 7 | `paginateRows` dengan `limit == null` → semua baris | `FinanceExportService` bergantung padanya |
| 8 | Antrean `refreshPromise` tunggal | Mencegah badai refresh token saat banyak 401 bersamaan |
| 9 | `parseApiJson` non-JSON handling | Menghindari `Unexpected token '<'` bocor ke pengguna |
| 10 | Kelas CSS legacy (`button-*`, `data-table`, `sidebar-panel`) | Hook selector spec E2E |
| 11 | `CONTINUOUS_PAPER_DEFAULTS.marginRightMm = 37.91` | Batas fisik print head dot-matrix, terverifikasi lewat tes cetak |
| 12 | `terbilangRupiah` | Muncul di nota cetak — bagian dari "tampilan sama persis" |

Inkonsistensi yang sebaiknya **diputuskan sadar** saat rebuild (bukan diwarisi diam-diam):

| # | Inkonsistensi | Di mana | Status |
|---|---|---|---|
| A | Dua standar pembulatan uang (utuh vs 2 desimal) | `money.ts` vs `number.util.ts` | ✅ **Keputusan: rupiah utuh di semua lapisan** (§3.3) |
| B | Numbering cabang tanpa formatter bersama; `order` pakai waktu lokal server; `format_template` ditulis tapi tak dibaca & isinya tidak cocok | 6 module vs `finance-document-sequence.util.ts` | ✅ Terverifikasi (§2.4) — perlu satu formatter bersama atau buang kolomnya |
| C | `audit-log/list` clamp pagination sendiri | `audit-log.service.ts` vs `pagination.util.ts` | Terbuka — seragamkan ke helper bersama |
| D | Audit log di luar transaction pemanggil | `AuditLogService.log()` | ✅ **Keputusan: masuk transaction** (§2.1) |
| E | `route-access.ts` hanya dipakai 1 file | vs penegakan asli di `app.tsx` | ✅ Terverifikasi (§6.2) — peran nyata, namanya saja terlalu luas |
| F | Konvensi kunci `localStorage` campur (`-` vs `.`); akses token tanpa try/catch & tanpa ErrorBoundary | `api.ts`, `common.adapter.ts`, `app-layout.tsx`, `app-store.tsx:353` | ✅ Terverifikasi — risiko layar putih (rules §5.6) |
| G | Bentuk state FE masih mewarisi `MockDatabase`/`DemoUser` | `app-store.tsx` | ✅ **Keputusan: rancang ulang bentuk + nama** (§4.4) |
| H | Dua lokasi file lokal (`Upload/` vs `apps/api/storage/`) | `local-storage.driver.ts` vs isi repo | ✅ `apps/api/storage/` mati — abaikan & hapus (§2.2) |
| I | `enableCors()` tanpa batas origin | `main.ts` | ⬜ **Masih terbuka** — perlu cek proxy VPS (§7.1) |
| J | `NotifyFn` didefinisikan dua kali | `use-toasts.ts` + `store-types.ts` | Terbuka — satukan |
| K | `rowHref` di `DataTable` tanpa satu pun konsumen | `types.ts` + `data-table.tsx` | ✅ Terverifikasi (§5) — buang dari kontrak |
| L | Prefix storage `'vioni'` = tenant pertama dari konsep multi-tenant yang ditinggalkan | `storage.service.ts` | ✅ Terjawab (§2.2) — single-company adalah keputusan sadar |
