# Shared Business Rules — Aturan Global

Aturan yang berlaku **di semua modul**: middleware auth, envelope, format tanggal/uang/angka,
validasi umum, dan penegak otomatis.

Dokumen pendamping: [shared-services.md](shared-services.md) ·
[shared-data-model.md](shared-data-model.md)

Sumber: pembacaan kode langsung. Tanda **[PERLU KONFIRMASI]** menandai hal yang tidak bisa
dipastikan dari kode saja.

---

## 1. Autentikasi & Otorisasi (Backend)

### 1.1 Rantai auth: JwtStrategy → guards → `@ActiveSession()`

Alurnya satu arah dan tidak ada jalan pintas:

```
Request + Bearer token
  └─ JwtAuthGuard (passport 'jwt')
       └─ JwtStrategy.validate()      ← semua verifikasi nyata terjadi di sini
            └─ request.session_payload = ActiveSessionPayload
                 ├─ PermissionGuard   ← baca session_payload
                 ├─ BranchGuard       ← baca session_payload
                 └─ @ActiveSession()  ← baca session_payload
```

**`JwtStrategy.validate(req, payload)`** — `apps/api/src/modules/auth/strategies/jwt.strategy.ts`.
JWT payload hanya berisi `{ sub, sessionId, companyId }`; sisanya di-resolve dari DB **setiap
request**.

Empat kondisi yang membuat request ditolak `UnauthorizedException`:

| # | Kondisi |
|---|---|
| 1 | Session tidak ditemukan (`id: payload.sessionId` **dan** `idUser: payload.sub` — keduanya harus cocok) |
| 2 | `session.revokedAt` terisi |
| 3 | `session.expiresAt < new Date()` |
| 4 | `session.user.status !== 'active'` |

Konfigurasi strategy: `ignoreExpiration: false` (exp JWT ditegakkan Passport),
`passReqToCallback: true`, secret dari `JWT_SECRET` dengan **fallback `'fallback-secret'`**.

**✅ TERJAWAB — fallback secret.** `config.get<string>('JWT_SECRET', 'fallback-secret')` berarti
bila env tidak di-set, API tetap boot dan menandatangani token dengan secret yang tertulis di
source code. **Dikonfirmasi pemilik sistem: `JWT_SECRET` selalu di-set di production**, jadi
fallback ini hanya pernah aktif di dev.

**Aturan untuk sistem baru:** hilangkan fallback-nya — **gagal-boot** bila `JWT_SECRET` kosong.
Fallback yang diam-diam berhasil adalah pola berbahaya: satu kesalahan konfigurasi di masa depan
akan membuat API berjalan normal sambil menerbitkan token yang bisa ditempa siapa pun yang membaca
repo, tanpa satu pun tanda di log.

**Aturan penting: permission di-resolve ulang setiap request**, bukan diambil dari klaim JWT:

```ts
const [rps, role] = await Promise.all([
  this.rpRepo.find({ where: { idRole: session.idActiveRole }, relations: ['permission'] }),
  this.roleRepo.findOne({ where: { id: session.idActiveRole } }),
]);
const permissions = rps.map((rp) => rp.permission.permissionCode as PermissionCode);
```

Konsekuensi yang berlaku global: **perubahan permission langsung berlaku tanpa logout/refresh
token.** Ini keputusan desain yang harus dipertahankan atau diganti secara sadar (biaya:
2 query DB per request).

**Aturan kedua: akses cabang diverifikasi ulang setiap request.**

```ts
let idActiveBranch = session.idActiveBranch;
if (idActiveBranch != null) {
  const stillAllowed = await this.ubaRepo.findOne({
    where: { idUser: session.idUser, idBranch: idActiveBranch },
  });
  if (!stillAllowed) idActiveBranch = null;   // ← akses dicabut → cabang di-drop dari sesi
}
```

Jadi bila akses cabang seorang user dicabut saat ia sedang bekerja, sesinya **tidak** dimatikan —
cabang aktifnya jadi `null`, dan `BranchGuard` yang akan menolak operasi berikutnya dengan pesan
`"Branch aktif belum dipilih"`. Efek UX: user terdorong ke halaman pilih cabang, bukan
ter-logout.

Total biaya DB per request terautentikasi: **4 query** (session+user, role-permissions, role,
user-branch-access).

### 1.2 Tiga guard, urutan tetap

| Guard | File | Yang dilakukan |
|---|---|---|
| `JwtAuthGuard` | `guards/jwt-auth.guard.ts` | Extends `AuthGuard('jwt')`. `handleRequest`: `err` → throw; `!user` → `UnauthorizedException` |
| `PermissionGuard` | `guards/permission.guard.ts` | Baca metadata `required_permission` dari **handler dulu, lalu class**. Bila tidak ada → `return true` (endpoint terbuka). Bila ada: `!session` → `ForbiddenException`; `!session.permissions.includes(required)` → `ForbiddenException` |
| `BranchGuard` | `guards/branch.guard.ts` | `!session?.idActiveBranch` → `ForbiddenException('Branch aktif belum dipilih')` |

Urutan wajib untuk controller branch-scoped:

```ts
@UseGuards(JwtAuthGuard, PermissionGuard, BranchGuard)
```

Controller branch-scoped yang teramati: `order`, `delivery`, `payment`, `sales-return`,
`purchase-return`, `stock`, `dashboard`, `reporting`.

Dua hal yang perlu dicatat sebagai risiko global:

1. **`PermissionGuard` gagal-terbuka bila metadata tidak ada.** Endpoint tanpa
   `@RequirePermission` lolos otomatis. Tidak ada mekanisme yang memaksa setiap endpoint bisnis
   punya permission — lupa menambahkan berarti endpoint publik bagi siapa pun yang punya token.
   `check-architecture.mjs` juga **tidak** memeriksa ini (lihat §6).
2. **`BranchGuard` hanya memastikan cabang ada, tidak memvalidasi cabang mana.** Verifikasi
   "user boleh mengakses cabang ini" terjadi di `JwtStrategy` (via `UserBranchAccess`), bukan di
   guard. Jadi guard ini murni pemeriksaan presence.

### 1.3 Aturan mutlak: scope dari session, bukan dari body

```ts
export const ActiveSession = createParamDecorator(
  (_data, ctx): ActiveSessionPayload => ctx.switchToHttp().getRequest().session_payload,
);
```

`idCompany`, `idBranch`, `idUser`, role aktif, dan permission **selalu** dari `@ActiveSession()`,
**tidak pernah** dari request body. Pola controller:

```ts
create(@ActiveSession() s: ActiveSessionPayload,
       @Body('data') data: Parameters<OrderService['create']>[2]) {
  return this.svc.create(s.idActiveBranch!, s.idCompany, data, s);
}
```

Ini aturan keamanan paling kritis di sistem, dan penegakannya **sepenuhnya manual** — tidak ada
guard, pipe, atau linter yang mencegah service membaca `data.id_company`. Karena isolasi tenant
bergantung pada ini (lihat [shared-data-model.md](shared-data-model.md) §3), ini kandidat teratas
untuk diangkat ke lapisan yang ditegakkan otomatis di rebuild.

### 1.4 `@RequirePermission` wajib string literal

```ts
export const PERMISSION_KEY = 'required_permission';
export const RequirePermission = (permission: PermissionCode) => SetMetadata(PERMISSION_KEY, permission);
```

Argumen bertipe `PermissionCode` (union 60 literal), jadi typo tertangkap TypeScript — **asalkan
ditulis sebagai literal**, bukan variabel bertipe `string`.

### 1.5 Role `superadmin` — kebijakan reserved

`apps/api/src/modules/user/reserved-role-policy.ts`:

```ts
RESERVED_SUPERADMIN_ROLE_CODE = 'superadmin'
DEFAULT_SUPERADMIN_USER_ID = 1

isSuperadminSession(activeRoleCode)          // activeRoleCode === 'superadmin'
containsReservedSuperadminRole(roleCodes)    // ada 'superadmin' (trim + lowercase)
withoutReservedSuperadminRole(roleCodes)     // buang 'superadmin' dari daftar
```

Aturan globalnya: `superadmin` adalah role developer cadangan untuk akun default (`id_user = 1`)
saja — **tidak boleh** diekspos sebagai role yang bisa ditetapkan lewat create/update user atau
checkbox role UI, dan data user superadmin tidak boleh dikembalikan ke sesi non-superadmin.
Perbandingan memakai `trim().toLowerCase()` sehingga `"SuperAdmin "` juga tertangkap.

Migrasi terkait: `042_reserved_superadmin_role_policy.sql`.

Role sistem: `superadmin`, `owner`, `admin`, `staff`. **`kasir` adalah seeded custom role
per-company, bukan `SystemRoleCode`** — seed memasukkannya dengan `id_company` terisi
(`id_role = 5`).

---

## 2. Format & Zona Waktu — Aturan Tanggal Global

### 2.1 Akar masalah

```
DatabaseModule: timezone: 'Z'      →  semua datetime tersimpan UTC
Pengguna:       memilih hari WIB   →  "laporan tanggal 1 sampai 30 September"
```

Membandingkan `YYYY-MM-DD` polos ke kolom `datetime` UTC punya dua bug sekaligus:
`<= '2026-09-30'` berarti `2026-09-30 00:00:00` (memotong hampir seluruh hari terakhir), dan
seluruh rentang bergeser +7 jam.

### 2.2 `resolveZonedDateRange` — wajib untuk semua filter tanggal laporan

`apps/api/src/common/date-range.util.ts`

```ts
export const APP_TIME_ZONE = process.env.APP_TIME_ZONE || 'Asia/Jakarta';

resolveZonedDateRange(timeZone, dateFrom?, dateTo?) => {
  startAt:          Date | null,   // tengah malam WIB hari `from`, sebagai instant UTC
  endExclusiveAt:   Date | null,   // tengah malam WIB hari `to` + 1
  startSql:         string | null, // 'YYYY-MM-DD HH:mm:ss.000000'
  endInclusiveSql:  string | null, // 'YYYY-MM-DD HH:mm:ss.999999'
}
```

Cara kerjanya:

1. `parseYmd` — regex `/^(\d{4})-(\d{2})-(\d{2})/`; format lain → `null` (diabaikan, bukan error)
2. `zonedMidnightToUtc` — hitung offset zona lewat `Intl.DateTimeFormat` **dua lintasan** agar
   benar di sekitar transisi DST. `day + 1` boleh melewati akhir bulan (dinormalkan `Date.UTC`)
3. Batas akhir **eksklusif** = tengah malam hari berikutnya; batas **inklusif** = 1 µs sebelumnya

Detail yang mengikat ke schema: `Date` hanya presisi milidetik, jadi pecahan `.999999` dipaksa
manual di string SQL. Ini hanya benar karena semua kolom datetime memakai `precision: 6` (lihat
[shared-data-model.md](shared-data-model.md) §2.2).

`todayInZone(timeZone)` — `Intl.DateTimeFormat('en-CA')` karena locale itu memformat tanggal
sebagai `YYYY-MM-DD`. Dipakai sebagai default tutup buku harian.

**Aturan:** filter tanggal finance **wajib** lewat helper ini; **jangan pernah** bare
`YYYY-MM-DD` compare.

### 2.3 `parseDateInput` — parse tanggal tunggal dari payload

`apps/api/src/common/date.util.ts`

```ts
parseDateInput(value: string | undefined, label: string): Date | null
```

- `undefined` atau string kosong → `null` (field opsional)
- String tidak valid → `BadRequestException('<label> tidak valid')` — pesan ramah pengguna
  berbahasa Indonesia
- Selain itu → `new Date(value)`

Dipakai lintas service commerce (order, order-pricing, sales-return); dinaikkan ke `common/`
supaya tidak diduplikasi.

Catatan: helper ini memakai `new Date(value)` mentah — **tidak** zona-aware. Jadi ada dua rezim
tanggal di sistem: input tunggal (naif) dan rentang laporan (zona-aware). Untuk field seperti
`orderDate` yang hanya berisi tanggal, ini berarti interpretasinya bergantung format string yang
dikirim frontend. **[PERLU KONFIRMASI]** apakah frontend selalu mengirim ISO ber-timezone untuk
field tanggal, atau `YYYY-MM-DD` polos (yang akan di-parse sebagai UTC tengah malam oleh
`new Date()`).

### 2.4 Pelanggaran yang masih ada: nomor dokumen order

`order.service.ts:849` memakai `now.getMonth()` — **waktu lokal server**, bukan WIB:

```ts
const month = String(now.getMonth() + 1).padStart(2, '0');
const sequenceKey = `order_${orderKind}_${year}-${month}`;
```

Sementara sisi finance sudah benar — `formatDocumentSequenceNumber` mengambil `{year}`/`{month}`
dari hari kalender WIB pada tanggal **transaksi**, dan komentarnya mencatat ini menyatukan 4
salinan yang sebelumnya memakai `new Date()`.

Jadi aturan zona waktu **sudah ditegakkan di finance tapi belum di numbering order**. Bila server
berjalan UTC, order yang dibuat antara 00:00–07:00 WIB di awal bulan bisa masuk ember sequence
bulan sebelumnya. Detail di [shared-services.md](shared-services.md) §2.4.

### 2.5 Format tampilan (Frontend)

`apps/web/src/utils.ts` — semua berbasis `Intl` dengan locale **`"id-ID"` hardcoded**:

| Fungsi | Format | Contoh |
|---|---|---|
| `formatDateTime(v?)` | `dd MMM yyyy, HH.mm` | `"24 Jun 2026, 14.30"` |
| `formatDateOnly(v?)` | `dd MMM yyyy` | `"24 Jun 2026"` |
| `formatLongDate(v?)` | `dd MMMM yyyy` | `"24 Juni 2026"` — dokumen cetak |
| `formatCurrency(v, code="IDR")` | currency, `maximumFractionDigits: 0` | `"Rp 95.000"` |
| `compactNumber(v)` | compact, 1 desimal | `"1,2 jt"` |

`undefined` → `"-"` di semua formatter tanggal (bukan string kosong, bukan crash).

**✅ TERJAWAB — locale hardcoded vs `companySettings.locale`.** `StoredSession` menyimpan
`companyLocale` (default `"id-ID"`) dan `CompanyProfileInput` punya field `locale`, tapi semua
formatter memakai literal `"id-ID"`. Hal sama berlaku untuk `companyTimezone` (default
`"Asia/Jakarta"`) vs `APP_TIME_ZONE` yang dibaca dari env. Jadi mengubah setting company **tidak**
mengubah tampilan apa pun.

**Keputusan: field `companyLocale`/`companyTimezone` DIBUANG di sistem baru.** Aplikasi standalone
satu perusahaan Indonesia — locale `id-ID` dan zona `Asia/Jakarta` ditetapkan sebagai konstanta
aplikasi, bukan data yang bisa diubah pengguna. Ini menghapus satu jalur konfigurasi yang tidak
pernah berfungsi, sekaligus menghilangkan ilusi bahwa mengubahnya berdampak.

Konsekuensi yang perlu diikuti: `APP_TIME_ZONE` boleh tetap ada sebagai env (berguna untuk test),
tapi tidak lagi berpasangan dengan field company mana pun.

`formatQuantity` (dari `@mini-erp/shared-types`) juga memakai `toLocaleString('id-ID')` —
konsisten hardcoded.

---

## 3. Aturan Uang, Kuantitas, dan Angka

### 3.1 Uang — dua standar yang hidup bersamaan

| Aturan | Helper | Cakupan |
|---|---|---|
| **Rupiah utuh** | `roundRupiah()` (`shared-types/money.ts`) | Nominal yang benar-benar ditagih/dibayar: total order, baris order, pembayaran, selisih retur |
| **2 desimal** | `money()`, `toMoney()` (`common/number.util.ts`) | "Perilaku lama seluruh service finance" |
| **Bebas desimal** | — | Tarif/rate, mis. persentase pajak — sengaja **tidak** dibulatkan |

Alasan aturan rupiah utuh (dari komentar `money.ts`): rupiah tidak punya satuan di bawah Rp 1;
menyisakan pecahan `0,x` membuat saldo nyangkut recehan yang mustahil dilunasi.

`Number.EPSILON` ditambahkan sebelum `Math.round` untuk mengoreksi galat float dari
"harga × kuantitas desimal" (contoh di kode: `0.2748 * 273000`).

Migrasi `043_round_money_to_whole_rupiah.sql` menandakan pergeseran ke rupiah utuh pernah
dilakukan di level data.

**Aturan operasional saat ini: jangan campur keduanya.** Peringatan itu tertulis eksplisit di
`number.util.ts`. Untuk rebuild ini butuh keputusan sadar — dibahas di
[shared-services.md](shared-services.md) §3.3.

Di frontend, `formatCurrency` memakai `maximumFractionDigits: 0`, jadi **tampilan selalu rupiah
utuh** apa pun nilai yang datang dari server. Artinya nilai 2-desimal dari finance akan tampak
dibulatkan di UI meski datanya tidak — sumber "angka di layar tidak cocok dengan angka di
export" yang potensial.

### 3.2 Kuantitas — snap ke nol dan ke bilangan bulat

Toleransi global `QUANTITY_ZERO_TOLERANCE = 1e-6`:

| Fungsi | Aturan |
|---|---|
| `roundQuantity(v, scale=12)` | Bulatkan ke 12 desimal |
| `snapQuantityToZero(v, tol)` | `\|v\| < tol` → **tepat 0** |
| `snapQuantity(v, tol)` | Snap ke 0, lalu snap ke integer terdekat bila selisih < tol |
| `normalizeUomFactor(v, fallback=1)` | Faktor **wajib > 0**; `null`/`""`/`0`/negatif/NaN → fallback |
| `quantityInputStep(factor)` | `1` bila factor integer ≥ 1, selain itu `0.0001` |

`snapQuantity` adalah pertahanan terhadap stok yang "tidak pernah nol" akibat sisa pembagian UOM
(`0.0000000001` tersisa). Tanpa ini, saldo mustahil ditutup.

### 3.3 Guard harga modal — dua aturan wajib sebelum movement

**A. Presence — cost basis wajib ada.** Sebelum movement `stock/adjust`:
`average_cost > 0` untuk pengurangan; `average_cost` **atau** `purchase_price > 0` untuk
penambahan. Adjustment tanpa cost basis membuat HPP finance macet di status
`"Menunggu data"`.

**B. Magnitude — `assertReasonableUnitCost(entered, reference, context)`.** Rasio
`entered / reference` wajib di `[0.1, 10]`. Di luar rentang → `BadRequestException` dengan pesan
owner-friendly yang menyebut kemungkinan salah digit/desimal dan menyarankan perbarui harga master
lebih dulu.

Lolos tanpa cek bila `reference <= 0` atau `entered <= 0` — itu wilayah guard A.

Latar belakang tertulis di kode: insiden Agustus 2026, saldo awal 7 produk dientri 100–1000× harga
wajar, mencemari HPP setiap penjualan selama 6+ minggu, berdampak ~Rp 3,2 miliar ke jurnal yang
sudah terposting. Sebelum guard ini, satu-satunya validasi di seluruh codebase adalah
"harus > 0".

**C. Konversi UOM — `resolveBaseUomCost(purchasePrice, purchaseToBaseFactor)`.** Satu-satunya cara
menurunkan harga modal per satuan dasar dari harga beli. Aturan tegas di komentar: **jangan
duplikasi rumus `purchase_price / factor` di tempat lain.**

### 3.4 Coercion angka

| Helper | Aturan |
|---|---|
| `toNumber(v, fallback=0)` (API) | `Number(v)`; non-finite → fallback |
| `toFiniteNumber(v, fallback=0)` (shared) | idem — **duplikasi** dengan `toNumber` |
| `toOptionalNumber(v)` (Web) | `null`/`undefined`/`""` → `undefined`; non-finite → `undefined` |

Tiga fungsi untuk hal yang hampir sama, di tiga tempat. Perbedaan nyatanya hanya nilai balik untuk
input kosong (`0` vs `undefined`).

---

## 4. Validasi & Envelope (Global)

### 4.1 ValidationPipe global

`main.ts`:

```ts
new ValidationPipe({
  whitelist: true,               // buang properti yang tidak dideklarasikan
  forbidNonWhitelisted: false,   // TIDAK error, hanya dibuang diam-diam
  transform: true,
  transformOptions: { enableImplicitConversion: true },   // "5" → 5, "true" → true
})
```

Dua implikasi global:

1. **`forbidNonWhitelisted: false`** → field asing di payload dibuang tanpa peringatan. Typo nama
   field (`id_bracnh`) tidak menghasilkan error, hanya field yang hilang → perilaku "diam-diam
   salah". Ini pilihan toleran yang memudahkan evolusi klien tapi menyembunyikan bug.
2. **`enableImplicitConversion: true`** → string dikonversi ke tipe target otomatis. Nyaman, tapi
   berarti `"abc"` → `NaN` untuk field number, yang lolos validasi kalau tidak ada
   `@IsNumber()`. Inilah sebabnya `normalizePageLimit` perlu menangani NaN secara eksplisit
   (komentarnya menyebut mencegah `LIMIT NaN` → `QueryFailedError`).

### 4.1a Temuan: ValidationPipe praktis hanya aktif di `auth`

Terverifikasi lewat pencarian, bukan dugaan:

| Ukuran | Hasil |
|---|---|
| File `*.dto.ts` di `apps/api/src` | **0** |
| File yang mengimpor `class-validator` | **1** — `modules/auth/auth.controller.ts` |
| `class-validator` di `package.json` | ada (`^0.14.0`) |

`auth.controller.ts` adalah satu-satunya yang memakai dekorator
(`IsNumber`, `IsOptional`, `IsPositive`, `IsString`, `MinLength`). Semua controller lain mengikat
tipe payload lewat `Parameters<Service['method']>[n]` — **tipe TypeScript murni yang hilang saat
runtime**.

Konsekuensinya untuk **~214 dari 220 endpoint**:

- `whitelist: true` tidak punya daftar properti untuk dipatuhi → **tidak ada** properti yang
  dibuang
- Tidak ada pemeriksaan tipe, required, panjang, atau rentang di lapisan pipe
- Seluruh validasi terjadi **manual di dalam service** (mis. `parseDateInput`,
  `assertReasonableUnitCost`, guard cost basis, `normalizePageLimit`) atau **tidak terjadi sama
  sekali**

Ini menjelaskan mengapa util seperti `normalizePageLimit` perlu menangani `NaN` secara defensif
dan mengapa `toNumber`/`toFiniteNumber`/`toOptionalNumber` bertebaran: mereka **menggantikan**
lapisan validasi yang tidak jalan, satu pemanggilan sekaligus.

Untuk rebuild: ini bukan detail konfigurasi, melainkan lapisan validasi yang absen. Pilihannya
sadar — DTO/skema per endpoint (Zod, class-validator, atau schema-first) atau tetap manual di
service dengan konsekuensi yang dipahami.

### 4.2 Envelope respons — jangan pernah dibungkus manual

**Sukses** — `ResponseInterceptor` (global):

```ts
{ code: ApiCode.SUCCESS /* 0 */, info: 'success', data: data ?? null }
```

`data ?? null` — service yang mengembalikan `undefined` tetap menghasilkan `data: null`, bukan
field hilang.

**Error** — `HttpExceptionFilter` (`@Catch()` — menangkap **semua**, bukan hanya `HttpException`):

```ts
{ code, info, data: null, errors: message }
```

Pemetaan status → kode envelope:

| HTTP status | `code` | `info` |
|---|---|---|
| 401 | `AUTH_ERROR` (100) | `unauthorized` |
| 403 | `FORBIDDEN` (400) | `forbidden` |
| 404 | `NOT_FOUND` (300) | `not_found` |
| 400, 422 | `VALIDATION_ERROR` (200) | `validation_failed` |
| lainnya | `ERROR` (1) | `error` |

`errors` diambil dari `exceptionResponse.message` bila ada, kalau tidak dari `exception.message` —
inilah yang membuat pesan Indonesia dari `BadRequestException` (mis. pesan
`assertReasonableUnitCost`) sampai ke pengguna.

**Non-`HttpException`** (bug tak tertangkap): `console.error('Unhandled exception:', exception)`
lalu 500 dengan `{ code: ERROR, info: 'internal_error', data: null }` — **tanpa** field `errors`,
sehingga detail internal tidak bocor ke klien. Frontend menangani ketiadaan `errors` lewat
`extractErrorMessage` yang jatuh ke `info`.

**Aturan:** controller dan service mengembalikan data domain **mentah**. Jangan `return { code,
info, data }` dari mana pun.

### 4.3 Bentuk endpoint

| Aturan | Detail |
|---|---|
| Method | Semua endpoint bisnis `@Post` + `@HttpCode(200)`. Satu-satunya `@Get` legacy: `assistant/runs/stats` |
| Prefix | `api/v1` (global prefix di `main.ts`) |
| Body | `@Body('data') data` — payload selalu di bawah kunci `data` |
| Penamaan | `resource/verb` kebab-case: `products/list`, `finance/expenses/create`, `orders/approve-credit` |
| Permission | String literal `module.action` |
| Audit `actionKey` | snake_case `resource.action`: `order.create`, `payment.upload_proof` |
| FK di request | `id_<entity>`; field entity tetap camelCase |

Terhitung **220 endpoint**; 219 `@Post` + 1 `@Get`.

### 4.4 Pagination — aturan seragam

| Konteks | Helper | Default |
|---|---|---|
| SQL `.skip()/.take()` | `normalizePageLimit` | fallback 50, max 100 |
| Slice in-memory | `paginateRows` | max 200 |
| SQL LIMIT manual | `clampPageLimit` | fallback 50, max 100 |

Aturan clamp: `limit` → `[1, maxLimit]`, nilai 0/negatif/NaN/Infinity → fallback lalu clamp;
`page` minimal 1.

**Kontrak implisit `paginateRows`:** `limit == null` → kembalikan **semua** baris
(`meta.limit = total`). Frontend selalu kirim `limit`; konsumen internal/export tidak.
`FinanceExportService` bergantung pada ini untuk berkas pajak. Agregat/total laporan dihitung di
luar helper dari **seluruh** data, sehingga SummaryCard tetap akurat meski tabel dipaginasi.

Bentuk meta seragam: `{ page, limit, total }` (`PageMeta` / `ApiListData.meta`).

Pengecualian yang tercatat: `audit-log.service.list()` memakai clamp inline sendiri
(`Math.min(limit ?? 20, 100)`, tanpa penanganan NaN) — lihat
[shared-services.md](shared-services.md) §2.1.

### 4.5 Pencarian — aturan seragam

`applyTokenizedLike(qb, columns, raw, paramPrefix)`:

- Input dinormalisasi whitespace lebih dulu (`normalizeWhitespace`)
- Maksimal **8 token** (`MAX_TOKENS`)
- **AND antar token, OR antar kolom** — setiap kata wajib cocok di salah satu kolom
- Tiap token jadi grup berkurung sendiri agar tidak bocor ke filter lain (company/archive)
- Urutan kata tidak berpengaruh; tahan spasi ganda di input **maupun** di data

Nama produk dinormalisasi whitespace saat simpan/import, jadi kedua sisi konsisten.

### 4.6 Konflik unik → 4xx, bukan 500

`isUniqueViolation(err)` mendeteksi `ER_DUP_ENTRY` / errno `1062` (termasuk di
`err.driverError`). Aturannya: race "pre-check lalu insert" yang lolos harus jadi
`ConflictException` (4xx), bukan `internal_error` (500) — situasi nyata ketika dua request
membuat kode/periode yang sama bersamaan.

---

## 5. Aturan Frontend Global

### 5.1 Penegakan akses rute — tiga tingkat

`app.tsx` memilih wrapper dari `route.access`:

| `access` | Wrapper | Aturan |
|---|---|---|
| `"public"` | `PublicOnly` | Sudah login + ada cabang → `/dashboard`. Login tanpa cabang → `/select-branch` |
| `"authenticated"` | `AuthenticatedOnly` | Belum login → `/login` (dengan `state.from`) |
| `"protected"` | `RouteGuard` | Belum login → `/login`; **tidak ada cabang aktif → `/select-branch`**; `permission && !can(permission)` → `/403` |

Urutan pemeriksaan di `RouteGuard` mengikat: autentikasi → **cabang** → permission. Jadi tanpa
cabang aktif, tidak ada halaman terproteksi yang bisa dibuka, apa pun permission-nya. Ini cermin
`BranchGuard` di backend.

Semua redirect memakai `replace` (tidak menumpuk history) dan menyertakan
`state={{ from: location }}` untuk kembali setelah login.

`can(permission?)` — `!permission` → `true`; selain itu `permissions.includes(permission)`. Sama
seperti `PermissionGuard` di backend: **tanpa permission = terbuka**.

**✅ TERJAWAB — dua catch-all berdampingan.** `<Routes>` punya `<Route path="*">` → `/login`,
sementara `APP_ROUTES` juga punya entri `path: "*"` (NotFoundPage, `shell: true`, `protected`)
yang ter-nest di dalam `<Route element={<AppLayout />}>`.

Yang menang: **NotFoundPage**. Alasannya (react-router-dom `^7.9.4`):

1. Kedua rute berskor sama — parent `AppLayout` pathless tidak menambah skor, jadi keduanya
   dinilai sebagai splat `*` biasa.
2. Tie-break react-router hanya membandingkan indeks untuk rute **bersaudara**; kedua splat ini
   berbeda kedalaman, jadi perbandingan menghasilkan 0 dan **urutan deklarasi dipertahankan**
   (sort stabil).
3. Blok `<Route element={<AppLayout />}>` dideklarasikan **sebelum** `<Route path="*">` terakhir,
   sehingga splat di dalam shell lebih dulu di daftar cabang.

Jadi `<Route path="*">` → `/login` di level atas **efektif tidak terjangkau** (dead code).

Yang penting: **keduanya tidak berkonflik secara perilaku.** NotFoundPage berstatus `protected`,
jadi pengunjung belum-login yang membuka URL asing tetap dilempar `RouteGuard` ke `/login` —
persis seperti maksud catch-all level atas. Bedanya hanya terasa bagi pengguna yang sudah login
dengan cabang aktif: mereka melihat halaman 404 di dalam shell (perilaku yang benar), bukan
terlempar ke login.

Untuk sistem baru: cukup satu catch-all di dalam shell. Yang di level atas redundan, bukan
berbahaya.

### 5.2 Aturan pemanggilan API

**Jangan pernah `fetch()` langsung.** Selalu `apiPost`/`apiUpload` dari `lib/api.ts` — ditegakkan
otomatis (§6, aturan W2).

Aturan turunan:

| Aturan | Alasan |
|---|---|
| Service tidak membungkus envelope manual | `apiPost` sudah membuka `json.data` dan melempar `ApiError` |
| Payload dikirim mentah | `apiPost` yang membungkus `{ data }` |
| Loading global otomatis | `loadingBus` terintegrasi di `apiPost` — tak perlu state loading per-halaman |
| `silent: true` untuk refresh background | Agar tidak memicu GlobalLoader |
| Refresh token otomatis, sekali | 401 → refresh → retry `_retried: true`; gagal → `clearTokens()` + `ApiError(401, 'session_expired')` |
| Satu antrean refresh | `refreshPromise` singleton mencegah badai refresh saat banyak 401 bersamaan |

**Pesan error wajib ramah pengguna.** `parseApiJson` memetakan respons non-JSON ke pesan
Indonesia per status (413 file terlalu besar, 408/504 timeout + saran pecah file, 502/503 server
sibuk, ≥500 kesalahan server) — bukan `Unexpected token '<'`.

`extractErrorMessage(err, fallback)` — prioritas `errors` → `info` → `"Terjadi kesalahan. Silakan
coba lagi."`

### 5.3 Aturan state

**React Context `AppProvider` + slice `useState`. Jangan tambah Redux/Zustand** — ditegakkan
otomatis (§6, aturan W4), dan daftar terlarangnya termasuk `@tanstack/react-query`, MobX,
Recoil, Jotai.

| Aturan | Detail |
|---|---|
| Slice baru untuk state global baru | Bukan library baru |
| Hook per-module = adapter tipis | `use-*-module.ts` **tidak boleh** memanggil `apiPost` (ditegakkan W3) |
| Pengecualian W3 | Alur yang genuinely scoped per-entity (`use-payments`, `use-delivery-notes`, `use-customer-addresses`) memanggil `apiPost` langsung karena scoped per order/pelanggan |
| Data dimuat lazy per module | `moduleStatus: 'idle' \| 'loading' \| 'ready'`; `loadBootstrap()` hanya muat status definitions + settings |
| `notify` diteruskan ke setiap slice | Notifikasi jadi tanggung jawab slice, bukan halaman |

Ganti cabang → `loadBootstrap()` + pesan eksplisit ke pengguna: *"Data tiap modul akan dimuat
ulang saat Anda mengunjunginya."*

### 5.4 Aturan struktur module

```
apps/web/src/modules/<domain>/
  pages/        ← route page + barrel SAJA
  components/   ← semua yang bukan page
  hooks/        ← adapter tipis store→UI
```

- Komposit shared lintas-module → `apps/web/src/components/`
- State → `apps/web/src/store/slices/`
- Mapping respons API → tipe FE → `apps/web/src/store/adapters/`
- **Jangan** mengimpor entity backend ke frontend — pakai tipe web atau shared package
- Module **tidak boleh** import file `pages/` module lain (ditegakkan W1)
- Import `hooks/use-<domain>-module` module lain **diperbolehkan** (analog import service di
  backend); import `components/`/`hooks/` internal module lain → warning kandidat shared (W5)

### 5.5 Aturan tampilan & wording

| Aturan | Detail |
|---|---|
| Tampilkan snapshot, bukan join live | Dokumen order historis menampilkan `productNameSnapshot` dst, bukan data produk live |
| Wording owner-friendly | UI finance dan alur rutin tidak boleh menuntut istilah akuntansi/developer untuk pekerjaan expense/payment harian |
| Kelas CSS legacy dipertahankan | `button-*`, `data-table`, `table-clickable-row`, `sidebar-panel` adalah **hook selector spec E2E** — menghapusnya merusak test, bukan tampilan |
| Satu sumber styling | `tailwind.css`; `styles.css` lama sudah dihapus; preflight Tailwind sengaja OFF |
| `:root` legacy tetap ada | recharts butuh string warna nyata, bukan class utility |
| Jangan tetapkan `superadmin` lewat UI | Lihat `role-access-config.ts` dan §1.5 |

Semua teks UI berbahasa Indonesia, termasuk pesan error dari backend.

### 5.6 Ketahanan `localStorage`

Tiga kunci, semuanya dibungkus penanganan kegagalan:

| Kunci | Isi | Fallback bila gagal |
|---|---|---|
| `mini-erp-access` / `mini-erp-refresh` | Token | — (tidak dibungkus try/catch) |
| `mini-erp-session` | `StoredSession` | `loadSession()` try/catch → `null` |
| `mini-erp.sidebar.collapsed` | Preferensi UI | try/catch; komentar: *"sidebar remains usable if browser privacy settings block localStorage"* |

Konvensi penamaan tidak seragam (`-` vs `.`).

**✅ TERJAWAB — risiko nyata, terlokalisasi di satu baris.** `getStoredTokens()`/`storeTokens()`
memang memanggil `localStorage` tanpa try/catch. Penelusuran seluruh call site:

| Call site | Konteks | Dampak bila `localStorage` melempar |
|---|---|---|
| `lib/api.ts:91` | dalam `refreshAccessToken()` (async) | Promise reject → tertangkap sebagai error request biasa |
| `lib/api.ts:130` | dalam `try` di `apiPost()` | idem — muncul sebagai kegagalan request |
| `lib/api.ts:178` | dalam `try` di `apiUpload()` | idem |
| **`store/app-store.tsx:353`** | **`useEffect` mount, DI LUAR try/catch** | **Effect melempar saat commit → aplikasi mati** |

Baris 353 adalah satu-satunya titik sinkron yang tidak terlindungi:

```ts
useEffect(() => {
  const { access } = getStoredTokens();   // ← di luar try/catch
  if (!access || !storedSession) { setIsBooting(false); return; }
  (async () => { try { … } catch { … } })();   // try/catch hanya di dalam IIFE
}, []);
```

Diperparah oleh: **tidak ada `ErrorBoundary` sama sekali di `apps/web/src`** (pencarian
`ErrorBoundary`/`componentDidCatch`/`errorElement`: nol hasil), dan `main.tsx` me-render
`<AppProvider><App /></AppProvider>` tanpa pembungkus apa pun. Jadi lemparan di effect itu
berujung **layar putih**, bukan pesan error.

Bandingkan dengan dua pembaca `localStorage` lain yang **sudah** ditangani benar:
`loadSession()` (try/catch → `null`) dan `readInitialSidebarCollapsed()` di `app-layout.tsx`
(try/catch, dengan komentar eksplisit soal pengaturan privasi browser). Jadi polanya sudah
diketahui penulisnya — hanya jalur token yang terlewat.

Untuk sistem baru: bungkus semua akses `localStorage` di satu wrapper aman, dan sediakan
`ErrorBoundary` di root.

---

## 6. Penegak Otomatis — `scripts/check-architecture.mjs`

`npm run arch:check` (alias `npm run lint`). Node murni tanpa dependency; alasannya tertulis di
kode: environment tidak bisa `npm install` ESLint (npm-cache ENOSPC).

**Setelah refactor apa pun, ini WAJIB tetap hijau.**

### Aturan HARD (exit ≠ 0)

| ID | Aturan | Detail teknis |
|---|---|---|
| **E1** | Backend: tidak ada reach-in lintas-module | Import antar-module hanya via permukaan publik. Diizinkan: file berakhiran `.service` / `.module`, apa pun di bawah `entities/`, atau module infra (`auth`, `storage`, `tools`) |
| **W1** | Frontend: tidak boleh import `pages/` module lain | Reach-in page |
| **W2** | Frontend: `fetch(` hanya di `lib/` | Regex `/(^\|[^.\w])fetch\s*\(/` di **seluruh** `apps/web/src` kecuali `lib/` dan file test |
| **W3** | `use-<domain>-module.ts` tidak boleh `apiPost`/`apiUpload` | Hook module wajib adapter tipis |
| **W4** | Dilarang import state-lib | `redux`, `react-redux`, `@reduxjs/toolkit`, `zustand`, `mobx`, `mobx-react(-lite)`, `@tanstack/react-query`, `recoil`, `jotai` |

### Aturan WARN (hijau kecuali `--strict-health`)

| ID | Aturan | Budget |
|---|---|---|
| **E2** | Service / controller terlalu besar | service > **400 LOC**; controller > **12 `@Post`** |
| **W5** | Import `components/`/`hooks/` internal module lain | Kandidat pindah ke shared |
| **W6** | File di atas budget | page > 300, komponen > 400, slice > 250, adapter/types > 400, hook-module > **80** LOC |

File `*.spec.ts` / `*.test.ts` dikecualikan dari pemeriksaan budget (tapi pelanggaran boundary di
spec tetap dilaporkan, ditandai `(spec)`).

**Yang TIDAK diperiksa** — celah yang perlu diketahui:

- Tidak ada pemeriksaan bahwa endpoint bisnis punya `@RequirePermission`
- Tidak ada pemeriksaan bahwa service write memanggil `auditLog.log()`
- Tidak ada pemeriksaan bahwa `idCompany`/`idBranch` tidak diambil dari body
- Tidak ada pemeriksaan bahwa filter tanggal finance memakai `resolveZonedDateRange`
- Tidak ada pemeriksaan `@Get`/`@Put`/`@Patch`/`@Delete` baru

Kelima aturan itu ada di `.claude/rules/` tapi penegakannya **manual**. Padahal empat pertama
adalah aturan dengan konsekuensi paling serius (kebocoran tenant, audit hilang, akses tanpa
permission, laporan salah hari). Ini kesempatan jelas di rebuild: pindahkan dari konvensi ke
penegakan.

---

## 7. Aturan Transaksi, Audit, dan Soft Delete

### 7.1 Audit wajib di setiap write

Setiap operasi write di service **wajib** `await this.auditLog.log({...})` dengan `actionKey`
snake_case.

**Module EXEMPT** (jangan tambah audit log): `assistant/`, `whatsapp/`, `tools/`, `auth/`,
`reporting/`, `audit-log/`. Auth memakai `UserSession` sebagai jejaknya; intelligence memakai
`AssistantRun` + `AssistantToolExecution`.

Terkonfirmasi dari kode: 28 service di 12 module memanggilnya; module exempt tidak.

**Catatan penting:** `AuditLogService.log()` tidak menerima `EntityManager`, jadi ia menulis di
luar transaction pemanggil — baris audit tetap tertulis meski transaction bisnis di-rollback.
**✅ Keputusan: di sistem baru `log()` ikut transaction pemanggil**, sehingga jejak audit menjadi
cermin akurat dari apa yang benar-benar tersimpan. Lihat
[shared-services.md](shared-services.md) §2.1.

### 7.2 Multi-write wajib dalam transaction

`dataSource.transaction(async (manager) => {...})` wajib untuk alur berikut:

```
order create · receive-goods · deliver-goods · delivery confirm · stock transfer
payment + audit · purchase return create · sales return create · finance posting/reversal
```

### 7.3 Tidak ada hard-delete

Pakai `row.archivedAt = new Date(); repo.save(row)`.

**Satu-satunya pengecualian:** `roles/delete` — hard-delete dengan guard "role tidak boleh
dipakai" (in-use check).

Jangan pakai `delete()`/`remove()` repository untuk data bisnis lain.

Karena `archivedAt` adalah `@Column` biasa (bukan `DeleteDateColumn`), **setiap query wajib
menambahkan filter `archived_at IS NULL` sendiri** — TypeORM tidak menyaringnya otomatis. Lihat
[shared-data-model.md](shared-data-model.md) §2.3.

### 7.4 Aturan reason text

**Wajib `reason_text`** saat membuka kembali periode finance/tax yang terkunci; alasan tetap
owner-readable di konteks audit. Migrasi terkait: `045_finance_pending_reason_code.sql`,
`048_finance_close_force_permission.sql`.

`StockAdjustmentInput` juga mewajibkan `reasonText` (non-opsional di tipe).

---

## 8. Aturan Media & Storage

| Aturan | Detail |
|---|---|
| Semua upload lewat backend `StorageService` | Tidak ada direct-upload dari frontend |
| `ImageOptimizerService` validator/optimizer akhir | Hanya `jpeg`/`jpg`/`png`/`webp` diterima |
| Foto produk pakai tabel `media_files` | Multi-gambar + primary — **bukan** kolom `products.image_url` |
| Jangan taruh secret object storage di env frontend | — |
| Jangan simpan signed URL ke DB | Simpan object key; URL dihasilkan saat diminta |
| Object key wajib lewat `assertSafeKey` | Tolak `.`/`..`/drive letter Windows; normalisasi separator |
| Nama file wajib lewat `sanitizeFileName` | Non-`[a-zA-Z0-9._-]` → `-`; buang titik di awal; potong 180 char |

Kompresi sisi klien (`lib/image-compression.ts`) adalah optimasi bandwidth, **bukan** pengganti
validasi server.

Catatan driver lokal: `getSignedUrl` **tidak** benar-benar menandatangani — ia mengembalikan URL
publik yang sama. Jadi "signed URL" hanya bermakna pada driver S3. File di
`process.cwd()/Upload` dapat diakses siapa pun yang tahu object key-nya (disajikan static di
`/uploads`).

---

## 9. Aturan Operasional Lain

### 9.1 Aktor manual (driver / petugas gudang)

Driver dan petugas gudang diperlakukan sebagai **aktor dokumen kertas manual**, bukan app user
wajib. Admin/owner mencatat nama/tanda tangan dari surat fisik yang kembali.

Terkonfirmasi di tipe: `StockTransferInput.driverName` adalah `string`, bukan `userId`. Jangan
mewajibkan akun aplikasi untuk alur kertas delivery/receipt/transfer.

### 9.2 Titik pengurangan stok delivery

**Stok berkurang saat `deliveries/create`**, bukan saat `deliveries/confirm` pertama kali. Jangan
memindahkan titik ini tanpa merancang ulang spesifikasi delivery.

### 9.3 Finance wajib reviewable & reversible

Setiap laporan/posting finance wajib mendukung: preview, post, restore, cancel posting, reverse
journal, safe close. Tercermin di endpoint: `finance/posting-sources/{preview,post,restore,
cancel-posting,ignore}`, `finance/journals/reverse`, `finance/close/{readiness,safe-close}`.

### 9.4 Identitas WhatsApp

Nomor channel WhatsApp yang tersambung adalah akun **BOT**; baris `whatsapp_authorizations` adalah
entri whitelist **pengirim**. Jangan pernah menurunkan nomor tampilan BOT dari pengirim yang
diotorisasi — nomor tampilan channel wajib dari socket Baileys.

Implementasi memakai Baileys (`@whiskeysockets/baileys`), **bukan** `whatsapp-web.js` — dokumen
`whatsapp-web.js` mana pun tidak berlaku.

### 9.5 Migrasi database

| Aturan | Detail |
|---|---|
| `synchronize: false` **permanen** | TypeORM tidak pernah membuat/mengubah tabel |
| Schema hanya lewat migrasi SQL bernomor | `apps/api/src/database/migrations/<NNN>_<name>.sql` |
| **Jangan pernah** menyunting migrasi yang sudah mungkin dijalankan | Selalu buat nomor berikutnya |
| Tulis SQL idempoten | Runner **menjalankan semua migrasi setiap kali** — tidak ada tabel pelacak |
| Tiap `ALTER` sebaiknya statement terpisah | Runner mengabaikan errno 1050/1054/1060/1061/1091/1826 per statement |

Poin keempat adalah yang paling mudah terlewat: idempotensi adalah tanggung jawab **penulis SQL**,
bukan runner. Detail di [shared-services.md](shared-services.md) §7.4.

### 9.6 Shared packages wajib di-build

Setelah menyunting `packages/shared-*`:

```bash
npm run build:packages
```

Karena keduanya memengaruhi API **dan** Web sekaligus, melewatkan ini membuat salah satu sisi
memakai tipe/nilai lama tanpa error yang jelas.

### 9.7 Test mengikuti kode

| Perubahan | Test yang wajib disesuaikan |
|---|---|
| Logika service | `*.service.spec.ts` |
| UI / flow | `apps/e2e/tests/` |
| Perbaikan bug | **Mulai dari reproducer test yang gagal dulu** |

Guard E2E yang perlu diketahui: `global-setup.ts` menolak berjalan bila `DB_NAME` bukan
`mini_erp_test` (`"E2E reset blocked because DB_NAME is not mini_erp_test"`) — pengaman agar suite
tidak pernah me-reset database dev/production. Semua test mulai pre-authenticated sebagai
`superadmin` lewat `storageState` tersimpan.

---

## 10. Ringkasan: Aturan yang Wajib Ada di Sistem Baru

Diurutkan dari konsekuensi terberat bila hilang.

| # | Aturan | Konsekuensi bila hilang |
|---|---|---|
| 1 | Scope selalu dari session, tidak pernah dari body | Kebocoran data lintas-perusahaan/cabang |
| 2 | Permission di-resolve per request dari DB | Pencabutan akses tidak berlaku sampai logout |
| 3 | Akses cabang diverifikasi per request | User tetap bekerja di cabang yang aksesnya sudah dicabut |
| 4 | Audit wajib di setiap write | Tidak ada jejak siapa mengubah apa |
| 5 | Multi-write dalam transaction | Stok/jurnal setengah jadi |
| 6 | Cost basis + magnitude guard sebelum movement | HPP tercemar diam-diam (insiden ~Rp 3,2 M) |
| 7 | Filter tanggal zona-aware (`resolveZonedDateRange`) | Laporan kehilangan hari terakhir, geser 7 jam |
| 8 | Nomor dokumen dari tanggal transaksi, bukan jam generate | Posting mundur bernomor bulan salah |
| 9 | Rupiah utuh untuk nominal tagihan | Saldo nyangkut recehan yang mustahil dilunasi |
| 10 | Snap kuantitas ke nol (toleransi 1e-6) | Stok "tidak pernah nol" |
| 11 | Tidak ada hard-delete | Data bisnis hilang permanen |
| 12 | Upload hanya lewat backend + validasi format | Path traversal, file tak tervalidasi |
| 13 | Envelope tunggal (interceptor + filter) | Bentuk respons tidak konsisten per endpoint |
| 14 | Satu pintu HTTP di frontend | Refresh token tersebar, loading state tidak konsisten |
| 15 | Snapshot, bukan join live | Nota historis berubah saat master data berubah |
| 16 | Validasi payload di batas API (**saat ini absen** kecuali `auth`) | Data tidak valid masuk sampai ke service/DB tanpa tertahan |

**Enam aturan teratas saat ini ditegakkan secara manual** — tidak ada guard, tipe, atau linter
yang mencegah pelanggarannya (lihat §6, "Yang TIDAK diperiksa"). Memindahkan aturan 1, 2, 3, dan
4 ke lapisan yang ditegahkan otomatis adalah perbaikan bernilai tertinggi yang bisa dilakukan
rebuild pada lapisan shared ini.

---

## 11. Status Item [PERLU KONFIRMASI] di Dokumen Ini

Lima dari enam item yang semula terbuka di dokumen ini sudah **tertutup** — lihat tabel di bawah.
Rekapitulasi lengkap seluruh keputusan lintas-dokumen ada di
[PROGRESS.md](../PROGRESS.md) §5 "Keputusan & Resolusi".

| # | Hal | Lokasi | Status |
|---|---|---|---|
| 1 | `JWT_SECRET` fallback `'fallback-secret'` | §1.1 | ✅ Selalu di-set di production → sistem baru gagal-boot bila kosong |
| 2 | Cakupan `class-validator` | §4.1a | ✅ Terverifikasi: 1 file, 0 DTO → validasi praktis absen |
| 3 | Locale/timezone per-company | §2.5 | ✅ Keputusan: field dibuang, `id-ID`/WIB jadi konstanta |
| 4 | Dua catch-all route | §5.1 | ✅ NotFoundPage menang; catch-all level atas redundan |
| 5 | `localStorage` token tanpa try/catch | §5.6 | ✅ Risiko nyata di `app-store.tsx:353`; tidak ada ErrorBoundary |

### Masih terbuka

| # | Hal | Lokasi | Cara memverifikasi |
|---|---|---|---|
| 1 | Interpretasi tanggal input tunggal (`parseDateInput` naif) | §2.3 | Cek format yang dikirim frontend untuk `orderDate` dkk — ditelusuri saat analisis modul Order |
| 2 | `enableCors()` tanpa batas origin — dibatasi di reverse proxy? | §9 / services §7.1 | Pemilik sistem perlu cek konfigurasi nginx/proxy di VPS |

Rekapitulasi seluruh keputusan & resolusi lintas-dokumen (termasuk 6 item yang diverifikasi dari
kode) ada di [PROGRESS.md](../PROGRESS.md) §6. Tabel status per dokumen:
[shared-services.md](shared-services.md) §8 dan
[shared-data-model.md](shared-data-model.md) §8.
