# Shared Data Model — Struktur Data Lintas Modul

Base model, konvensi kolom, tipe primitif, dan bentuk data yang dipakai lintas modul.

Dokumen pendamping: [shared-services.md](shared-services.md) ·
[shared-business-rules.md](shared-business-rules.md)

Sumber: pembacaan kode langsung (70 entity, 2 shared package, tipe frontend). Tanda
**[PERLU KONFIRMASI]** menandai hal yang tidak bisa dipastikan dari kode saja.

---

## 1. Temuan Utama: Tidak Ada Base Entity

**Tidak ada satu pun base class entity di seluruh backend.** Hasil pencarian
`extends *Entity` / `BaseEntity` pada 70 file `*.entity.ts`: **nol hasil**.

Artinya konvensi kolom bersama (id, scope perusahaan, timestamp, soft-delete) **diulang manual di
setiap entity**. Ini konvensi yang ditegakkan oleh disiplin penulis, bukan oleh tipe.

Sebarannya pada 70 entity:

| Pola | Jumlah entity | % |
|---|---|---|
| `PrimaryGeneratedColumn` | 66 | 94% |
| `createdAt` | 53 | 76% |
| `CreateDateColumn` | 52 | 74% |
| `idCompany` | 45 | 64% |
| `updatedAt` | 42 | 60% |
| `UpdateDateColumn` | 41 | 59% |
| `idBranch` | 28 | 40% |
| `archivedAt` | 23 | 33% |

Angka-angka ini adalah peta risiko rebuild: setiap kolom yang seharusnya universal punya
**pengecualian**, dan pengecualiannya tidak terdokumentasi di kode.

**✅ TERVERIFIKASI — 4 entity tanpa `PrimaryGeneratedColumn`.** Semuanya punya alasan sah, jadi
bukan penyimpangan yang perlu diperbaiki:

| Entity | Tabel | Bentuk PK | Peran |
|---|---|---|---|
| `RolePermission` | `role_permissions` | `@PrimaryColumn` × 2 (`id_role`, `id_permission`) | Pivot many-to-many |
| `UserRole` | `user_roles` | `@PrimaryColumn` × 2 (`id_user`, `id_role`) | Pivot many-to-many |
| `UserBranchAccess` | `user_branch_accesses` | `@PrimaryColumn` × 2 (`id_user`, `id_branch`) + `is_default_branch` | Pivot **beratribut** |
| `CompanySettings` | `company_settings` | `@PrimaryColumn` (`id_company`) | Satelit 1:1 ke `companies` |

Tiga pertama adalah tabel pivot ber-PK komposit — pola yang benar, dan PK komposit sekaligus
mencegah baris duplikat tanpa perlu unique index terpisah.

`CompanySettings` berbeda sifatnya: satelit 1:1 yang seluruh isinya **kolom JSON**
(`business_labels_json`, `operational_rules_json`, `ui_preferences_json`,
`assistant_preferences_json`, `reporting_preferences_json`) dan **tanpa satu pun timestamp** — tidak
ada `createdAt`, `updatedAt`, maupun `archivedAt`. Jadi perubahan setting perusahaan tidak punya
jejak waktu di tabelnya sendiri; yang mencatat hanya `audit_logs`.

`UserBranchAccess` layak dicatat khusus: ia pivot **beratribut** (`is_default_branch`), dan tabel
inilah yang dibaca `JwtStrategy` setiap request untuk memverifikasi ulang akses cabang (lihat
[shared-business-rules.md](shared-business-rules.md) §1.1).

Untuk rebuild: base entity/mixin adalah salah satu perbaikan paling murah dengan dampak paling
besar di lapisan ini.

---

## 2. Konvensi Kolom Bersama

Pola kanonik, terbaca dari `product-category.entity.ts` (contoh paling lengkap):

```ts
@Entity('product_categories')
export class ProductCategory {
  @PrimaryGeneratedColumn({ name: 'id_product_category' })   // ← nama kolom = id_<entity>
  id: number;                                                //   properti selalu `id`

  @Column({ name: 'id_company' })
  idCompany: number;

  @Column({ name: 'id_parent_category', type: 'int', nullable: true })
  idParentCategory: number | null;

  @CreateDateColumn({ name: 'created_at', type: 'datetime', precision: 6 })
  createdAt: Date;

  @UpdateDateColumn({ name: 'updated_at', type: 'datetime', precision: 6 })
  updatedAt: Date;

  @Column({ name: 'archived_at', type: 'datetime', precision: 6, nullable: true })
  archivedAt: Date | null;                                   // ← soft delete

  @ManyToOne(() => Company)
  @JoinColumn({ name: 'id_company' })
  company: Company;
}
```

### 2.1 Aturan penamaan

| Lapisan | Konvensi | Contoh |
|---|---|---|
| Nama tabel | `snake_case` plural | `product_categories`, `inventory_movements` |
| Primary key (DB) | `id_<entity singular>` | `id_product_category` |
| Primary key (properti TS) | **selalu `id`** | `id: number` |
| Foreign key (DB) | `id_<entity>` | `id_company`, `id_parent_category` |
| Foreign key (properti TS) | `camelCase` | `idCompany`, `idParentCategory` |
| Kolom lain (DB) | `snake_case` | `sort_order`, `action_key` |
| Kolom lain (properti TS) | `camelCase` | `sortOrder`, `actionKey` |
| Kolom JSON (DB) | ber-sufiks `_json` | `before_json`, `after_json`, `metadata_json` |
| Kolom JSON (properti TS) | **tanpa** sufiks | `before`, `after`, `metadata` |

Pemetaan nama dilakukan **eksplisit** per kolom lewat `{ name: '...' }` — tidak ada naming
strategy global. Jadi tiap kolom baru wajib menuliskan nama DB-nya sendiri.

### 2.2 Tipe datetime

Semua kolom waktu: `type: 'datetime', precision: 6` (mikrodetik). Presisi 6 ini bukan hiasan —
`date-range.util.ts` bergantung padanya untuk batas inklusif `.999999` (lihat
[shared-business-rules.md](shared-business-rules.md) §2).

Nullable ditulis eksplisit dua kali: `nullable: true` di dekorator **dan** `| null` di tipe TS.

### 2.3 Soft delete

`archivedAt: Date | null` ada di 23 entity. Pola pemakaiannya (dari
`.claude/rules/backend-nestjs.md`, terkonfirmasi oleh keberadaan kolom):
`row.archivedAt = new Date(); repo.save(row)` — bukan `repo.delete()`.

Kolom ini **bukan** `DeleteDateColumn` TypeORM, melainkan `@Column` biasa. Konsekuensinya:
TypeORM tidak menyaring baris terarsip secara otomatis — **setiap query wajib menambahkan filter
`archived_at IS NULL` sendiri**. Ini beban yang tersebar ke semua service dan kandidat kuat untuk
diperbaiki di rebuild (soft-delete native atau scope query terpusat).

47 entity tanpa `archivedAt` — sebagian jelas append-only (`audit_logs`, `inventory_movements`),
sebagian perlu diperiksa saat merancang schema baru.

---

## 3. Scope Data: Company & Branch

Dua tingkat scope, dan keduanya adalah **kolom biasa**, bukan mekanisme framework.

| Kolom | Entity | Arti |
|---|---|---|
| `idCompany` | 45 | Tenant. Seed hanya membuat `id_company = 1` → sistem berjalan **single-company** |
| `idBranch` | 28 | Cabang operasional. Sering `nullable` (data level-perusahaan) |

Contoh `idBranch` nullable: `audit_logs.id_branch` bertipe `int, nullable: true` — aksi
level-perusahaan (mis. ubah profil company) dicatat tanpa cabang.

**Tidak ada row-level security, query filter global, atau multi-tenant middleware.** Isolasi
tenant sepenuhnya bergantung pada setiap service menambahkan `where: { idCompany }` sendiri,
dengan nilai yang **wajib** berasal dari `@ActiveSession()` (lihat
[shared-business-rules.md](shared-business-rules.md) §1.3). Satu service yang lupa = kebocoran
lintas-perusahaan tanpa peringatan apa pun dari tipe atau runtime.

Untuk rebuild single-company ini mungkin tidak terasa. Tapi kalau multi-tenant jadi target, ini
titik yang wajib diangkat ke lapisan infrastruktur, bukan dibiarkan per-service.

---

## 4. Tipe Primitif Bersama (`packages/shared-types`)

Satu-satunya paket yang dikonsumsi **API dan Web sekaligus** untuk tipe/nilai domain.
Barrel `index.ts` sengaja **selektif** — hanya nama tertentu yang diekspor ulang, bukan
`export *` dari `money`/`text`/`quantity`.

### 4.1 Enum domain — `enums.ts`

25 type alias union string. Ini kosakata status/jenis di seluruh sistem:

| Kategori | Type | Nilai |
|---|---|---|
| Produk | `ItemType` | `physical` `service` `bundle` `non_stock` |
| Produk | `ProductStatus` | `active` `inactive` |
| Order | `OrderKind` | `sales` `purchase` |
| Order | `StatusGroup` | `pending` `active` `completed` `cancelled` |
| Bayar | `PaymentMethod` | `cash` `bank_transfer` `check` |
| Bayar | `PaymentStatus` | `unpaid` `partial` `paid` |
| Bayar | `PaymentTerms` | `prepaid` `cod` `net` |
| Finance | `FinancialSide` | `receivable` `payable` |
| Finance | `FinancialStatus` | `open` `partial` `settled` `overdue` |
| Stok | `MovementType` | `in` `out` `adjustment` |
| Mitra | `PartyType` | `customer` `supplier` `partner` |
| User | `UserStatus` | `active` `inactive` `locked` |
| Cabang | `BranchStatus` | `active` `inactive` |
| Knowledge | `DocumentType` | `sop` `policy` `glossary` `guide` |
| Knowledge | `KnowledgeStatus` | `processing` `ready` `failed` `archived` |
| Knowledge | `ChunkingStatus` / `EmbeddingStatus` | `pending` `processed` `failed` |
| Assistant | `AssistantMode` | `rule_based` `ai_assisted` |
| Assistant | `ResolutionMode` | `bot_rule` `bot_ai` `fallback` |
| Assistant | `RunStatus` | `running` `completed` `failed` |
| Assistant | `ExecutionStatus` | `success` `failed` `skipped` |
| Audit | `ActorType` | `user` `system` `assistant` `whatsapp` |
| WhatsApp | `AccessLevel` | `owner` `authorized_party` |
| WhatsApp | `AuthorizationStatus` | `active` `revoked` |
| Umum | `Severity` | `info` `warning` `error` |

**`MovementType` hanya punya `in`/`out`/`adjustment` — tidak ada `transfer`.** Ini menegakkan di
level tipe aturan bahwa transfer stok = **dua** movement (`out` sumber + `in` tujuan), sesuai
`.claude/rules/database.md`.

Nilai-nilai ini disimpan sebagai `varchar` di DB (mis. `audit_logs.actor_type` =
`varchar(30)` bertipe TS `ActorType`), **bukan** MySQL `ENUM`. Jadi integritasnya dijaga
TypeScript, bukan database. Menambah nilai tidak butuh migrasi — tapi data tidak valid juga tidak
akan ditolak DB.

### 4.2 Uang — `money.ts`

Satu fungsi: `roundRupiah(value)` → `Math.round(Number(value) + Number.EPSILON)`.

Dokumentasinya menetapkan cakupan dengan tegas:
- **Wajib bilangan bulat**: total order, baris order, pembayaran, selisih retur — semua yang
  benar-benar ditagih atau dibayar
- **Boleh berdesimal**: tarif/rate (mis. persentase pajak) — **tidak** memakai helper ini
- `Number.EPSILON` mengoreksi galat float dari "harga × kuantitas desimal"
  (contoh di kode: `0.2748 * 273000`)

Alasan bisnisnya: rupiah tidak punya satuan di bawah Rp 1, pecahan `0,x` membuat saldo nyangkut
recehan yang mustahil dilunasi.

Bandingkan dengan `common/number.util.ts` yang membulatkan ke 2 desimal — konflik yang dibahas di
[shared-services.md](shared-services.md) §3.3.

### 4.3 Kuantitas & UOM ganda — `quantity.ts`

Model UOM ganda adalah salah satu keputusan data paling berpengaruh di sistem ini.

| Konstanta | Nilai | Arti |
|---|---|---|
| `QUANTITY_SCALE` | 12 | Presisi internal kuantitas |
| `DISPLAY_QUANTITY_SCALE` | 4 | Presisi tampilan |
| `QUANTITY_ZERO_TOLERANCE` | 0.000001 | Di bawah ini dianggap tepat nol |

**Tiga satuan hidup berdampingan per produk:**

| Satuan | Peran | Contoh |
|---|---|---|
| `base_uom` | Satuan pelacakan stok — semua saldo & ledger biaya | PCS |
| `transaction_uom` | Satuan jual/beli di dokumen | DUS |
| `purchase_uom` | Satuan `products.purchase_price` | DUS |

Konversi: `uomToBaseFactor` (mis. 1 DUS = 300 PCS → factor 300).
`toBaseQuantity(qty, factor)` = qty × factor · `fromBaseQuantity(qtyBase, factor)` = qtyBase ÷ factor.

Aturan penting yang terbaca dari `resolveBaseUomCost`: **`average_cost` dan ledger biaya SELALU
per `base_uom`, sedangkan `purchase_price` per `purchase_uom`.** Mencampur keduanya = harga modal
salah sebesar faktor konversi.

`normalizeUomFactor(v, fallback=1)` menjaga faktor selalu `> 0` — `null`, `""`, `0`, negatif, dan
NaN semuanya jatuh ke fallback.

Migrasi terkait: `008_product_dual_uom.sql`, `035_keep_legacy_product_uom_writable.sql`,
`039_expand_quantity_precision.sql`.

### 4.4 Teks — `text.ts`

`normalizeWhitespace(value)` → semua rentetan whitespace (spasi ganda, tab, non-breaking space
U+00A0) jadi satu spasi, lalu trim. `null`/`undefined` → `""`.

Dipakai untuk dua hal, dan keduanya harus konsisten agar pencarian bekerja:
1. Normalisasi nama produk saat simpan/import
2. Tokenisasi pencarian (`tokenizeSearch` memanggilnya lebih dulu)

Contoh nyata dari komentar: `"ACLOSE SUPER SOLID WARNA SPECIAL  1 KG"` — spasi dobel hasil
import.

---

## 5. Kontrak API Bersama (`packages/shared-contracts`)

### 5.1 Envelope — `api-envelope.ts`

```ts
type ApiRequest<T>  = { guid?: string; code?: string; info?: string; data: T };
type ApiResponse<T> = { code: number; info: string; data: T };
type ApiListData<T> = { items: T[]; meta: { page: number; limit: number; total: number } };
```

`ApiCode`:

| Konstanta | Nilai | Dipetakan dari HTTP status |
|---|---|---|
| `SUCCESS` | 0 | — (selalu, oleh `ResponseInterceptor`) |
| `ERROR` | 1 | default / 500 |
| `AUTH_ERROR` | 100 | 401 |
| `VALIDATION_ERROR` | 200 | 400, 422 |
| `NOT_FOUND` | 300 | 404 |
| `FORBIDDEN` | 400 | 403 |

**Perhatikan:** `FORBIDDEN = 400` (kode envelope) bukan HTTP 400 — tabrakan angka yang mudah
membingungkan. HTTP 400 memetakan ke `VALIDATION_ERROR = 200`. Frontend membedakan sukses/gagal
**hanya** dari `code !== 0`, bukan dari HTTP status.

`ApiRequest` punya field `guid`, `code`, `info` opsional di sisi request, tapi
`apps/web/src/lib/api.ts` **hanya** pernah mengirim `{ data }`.

**✅ TERJAWAB: tidak ada klien lain yang memakainya — dibuang di sistem baru.** Envelope request
disederhanakan menjadi `{ data }` saja. (Shell Android bukan klien API terpisah: ia WebView yang
memuat web app yang sama, jadi ia memakai `lib/api.ts` juga.)

Ini kemungkinan besar fosil dari rancangan multi-tenant yang sama dengan prefix `'vioni'` — lihat
[shared-services.md](shared-services.md) §2.2.

`ApiListData<T>` mendefinisikan bentuk list resmi, dan `meta`-nya sama dengan `PageMeta` di
`common/pagination.util.ts` (`{ page, limit, total }`) — dua deklarasi terpisah untuk bentuk yang
sama.

### 5.2 Permission — `permission-code.ts`

`PermissionCode` = union **60 string literal** `module.action`. Sebaran per prefix:

| Prefix | Jumlah | Prefix | Jumlah |
|---|---|---|---|
| `finance.*` | 14 | `user.*` | 4 |
| `order.*` | 5 | `sales_return.*` | 4 |
| `product.*` | 4 | `purchase_return.*` | 4 |
| `stock.*` | 5 | `knowledge.*` | 4 |
| `payment.*` | 3 | `whatsapp.*` | 3 |
| `company_config.*` | 2 | `branch.*` | 2 |
| `member_type.*` | 2 | `dashboard.*` `reporting.*` `role.*` `audit_log.*` | 1 masing-masing |

Dua permission memakai **tiga** segmen, memecah pola `module.action`:
`stock.adjust.approve` dan `finance.close.force`, plus keluarga `finance.posting.*`,
`finance.journal.*`, `finance.report.*`, `finance.tax_report.*`, `finance.expense.*`,
`finance.tax_adjustment.*`. Jadi pola sebenarnya adalah `<area>.<subarea?>.<action>`.

**Role:**

```ts
type SystemRoleCode = 'superadmin' | 'owner' | 'admin' | 'staff';
type RoleCode = SystemRoleCode | (string & {});
```

Trik `(string & {})` mempertahankan autocomplete untuk 4 role sistem sambil tetap menerima role
kustom (mis. `kasir`, yang di-seed sebagai role per-company — **bukan** `SystemRoleCode`).

`DEFAULT_ROLE_PERMISSIONS: Record<SystemRoleCode, PermissionCode[]>` — matriks default:

| Role | Cakupan |
|---|---|
| `superadmin` | Seluruh 60 permission |
| `owner` | 59 — sama seperti superadmin **kecuali `whatsapp.simulate`** |
| `admin` | Operasional penuh + view finance; **tanpa** `finance.manage`, `finance.posting.post`, `finance.journal.reverse`, `finance.close.force`, `user.create/update/archive`, `company_config.manage`, `branch.manage`, `finance.expense.cancel`, `finance.tax_adjustment.manage`, `order.export`✱ |
| `staff` | 12 permission: dashboard, product.view, order view/create/update, sales_return view/create, purchase_return.view, stock view/adjust, payment view/create |

✱ `admin` **punya** `order.export` (terbaca di daftar). Detail perbedaan admin vs owner sebaiknya
dibaca langsung dari file saat menyusun matriks role baru — daftarnya panjang dan mudah salah
kutip.

**✅ TERJAWAB — `admin` punya `role.manage` tapi tidak `user.create`: disengaja.** Pembagian
tugasnya memang: **admin mengatur struktur akses, owner mengelola akun orang.** Matriks ini
disalin apa adanya ke sistem baru.

Satu properti keamanan yang perlu diketahui saat mengimplementasikannya (bukan keberatan terhadap
keputusan di atas, tapi konsekuensi teknis yang perlu dijaga): `role.manage` memberi kemampuan
mengubah permission **role mana pun, termasuk role yang sedang dipakai sendiri**. Tanpa pembatas,
seorang admin dapat menambahkan `finance.manage` atau `user.create` ke role `admin` lalu memakainya
— sehingga batas "admin tidak boleh mengelola akun" bisa dilewati lewat satu langkah tambahan,
bukan diblokir.

Karena pembagian tugasnya disengaja, cara menjaganya di sistem baru: **larang `role.manage`
memberikan permission yang tidak dimiliki aktor**, atau kunci daftar permission tertentu
(`user.*`, `finance.manage`, `role.manage`) hanya bisa diubah oleh `owner`. Dengan begitu maksud
pembagian tugas tetap utuh secara teknis, bukan hanya secara konvensi.

Sisi frontend punya cerminnya sendiri: `modules/company/components/role-access-config.ts`
mendefinisikan `ROLE_ACCESS_MODULES` — pengelompokan permission ke modul berlabel Indonesia
(`"Produk & Item"`, `"Jenis Member"`) dengan label aksi (`"Lihat"`, `"Tambah"`, `"Ubah"`,
`"Arsipkan"`) untuk UI checkbox. Ada juga `FINANCE_ROLE_ACCESS_PERMISSION_CODES` (14 kode finance
sebagai satu daftar terpisah). **Ini duplikasi struktural**: menambah permission baru menuntut
edit di dua tempat — `permission-code.ts` dan `role-access-config.ts` — tanpa ada yang memaksa
keduanya sinkron.

### 5.3 Session payload — `common/types/session.types.ts`

Bentuk yang di-inject ke request setelah JWT terverifikasi, dan satu-satunya sumber scope yang
boleh dipercaya:

```ts
type ActiveSessionPayload = {
  idUser: number;
  idCompany: number;
  idSession: number;
  idActiveBranch: number | null;   // ← nullable: user bisa login tanpa cabang aktif
  idActiveRole: number;
  activeRoleCode: string;
  permissions: PermissionCode[];
};
```

`idActiveBranch` nullable inilah yang membuat `BranchGuard` perlu ada sebagai guard terpisah
(lihat [shared-business-rules.md](shared-business-rules.md) §1.2).

---

## 6. Pola Data Bersama di Backend

### 6.1 Document sequence — dua tabel, dua bentuk

**`branch_document_sequences`** — `BranchDocumentSequence`:

```
PK id_branch_document_sequence
@Unique(['idBranch', 'sequenceKey'])          ← unique constraint di level entity
id_branch                int
sequence_key             varchar(50)
prefix                   varchar(30) nullable
current_value            bigint default 0
reset_policy             varchar(30) default 'none'
format_template          varchar(100) nullable
updated_at               datetime(6)          ← hanya UpdateDateColumn, TANPA created_at
```

`sequenceKey` yang teramati: `order`, `order_<kind>_<YYYY>-<MM>` (monthly), `payment`,
`stock_transfer`. `resetPolicy` bernilai `'none'` atau `'monthly'`.

Entity ini **tidak punya `idCompany`** — scope-nya lewat `idBranch`. Konsisten dengan
single-company, tapi berarti nomor dokumen tidak unik lintas-perusahaan bila multi-tenant
diaktifkan.

**`finance_document_sequences`** — `FinanceDocumentSequence`, ber-scope `id_company` (terbaca dari
seed: `INSERT INTO finance_document_sequences (id_company, sequence_key, prefix, current_value,
reset_policy, format_template)`).

Jadi dua tabel sequence dengan **granularitas scope berbeda** (branch vs company) dan hanya yang
finance punya formatter bersama. Lihat [shared-services.md](shared-services.md) §2.4.

### 6.2 Ledger append-only

Dua tabel bersifat append-only dan menjadi sumber kebenaran historis:

| Tabel | Entity | Pasangannya |
|---|---|---|
| `inventory_movements` | `InventoryMovement` | Setiap perubahan `InventoryBalance` **wajib** diiringi satu insert movement di transaction yang sama |
| `audit_logs` | `AuditLog` | Tidak ada pasangan; tanpa `updatedAt`/`archivedAt` |

`AuditLog` adalah contoh entity paling minimal: PK + scope + aktor + aksi + entity + 3 kolom JSON
+ `happened_at`. Tidak ada `createdAt`, `updatedAt`, maupun `archivedAt` — memang append-only.

Kolom JSON-nya (`before_json`, `after_json`, `metadata_json`) bertipe TypeORM `json` dengan tipe
TS `Record<string, unknown> | null` — **tidak ada skema** untuk isinya. Bentuk payload audit
sepenuhnya bergantung pemanggil. Untuk rebuild, ini kandidat untuk diberi tipe per `actionKey`.

### 6.3 Hierarki self-referencing

Pola parent-child dengan FK ke tabelnya sendiri:

| Entity | Kolom parent |
|---|---|
| `ProductCategory` | `id_parent_category` |
| `StockLocation` | (parent location — `020_stock_location_allocation_flags.sql`) |
| `FinanceAccount` | (chart of account, terbaca dari `sort_order` + `system_key` di seed) |

Untuk `StockLocation` ada aturan yang mengikat: mutasi stok **wajib** resolve ke **leaf**
(tidak ada child aktif, status `active`) — node parent adalah pengelompokan/mapping, bukan bin
penyimpanan.

Di frontend, hierarki ini dilayani `components/forms/hierarchical-select.tsx` dan
`SelectOption.depth` / `SelectOption.path` (lihat §7.2).

### 6.4 Snapshot vs join live

`OrderItem` menyimpan salinan data produk pada saat transaksi:
`productNameSnapshot`, `productCodeSnapshot`, `baseUomSnapshot`, `transactionUomSnapshot`,
`uomToBaseFactor`, `stockTrackedSnapshot`, plus cost snapshot
(`049_order_item_cost_snapshot.sql`).

Alasannya struktural, bukan optimasi: dokumen historis (nota, riwayat) tidak boleh bergeser saat
master data berubah. UI **wajib** menampilkan snapshot, bukan join `Product` live.

Ini pola data yang harus dipertahankan di schema baru apa pun bentuknya — kalau hilang, nota lama
akan berubah isinya sendiri saat produk di-rename.

---

## 7. Bentuk Data Bersama di Frontend

### 7.1 `StoredSession` — bentuk sesi yang dipersist

`store/adapters/common.adapter.ts`, disimpan di `localStorage` kunci `mini-erp-session`:

```ts
type StoredSession = {
  userId, userFullName, userEmail: string;
  companyId, companyName: string;
  companyCode?, companyLegalName?, companyTimezone?, companyCurrencyCode?, companyLocale?;
  activeBranchId: string | null;
  activeBranchName: string | null;
  activeBranchCode: string;
  activeRoleCode: string;
  availableRoles: { code: string; name: string }[];
  permissions: string[];                       // ← string[], BUKAN PermissionCode[]
  accessibleBranches: {
    id, name, code, city, defaultStockLocationLabel: string;
    isDefault: boolean;
  }[];
  displayTitle: string;
};
```

Dua hal yang menonjol:

1. **Semua ID adalah `string` di frontend, `number` di backend.** Konversi terjadi di adapter
   (`String(me.id)`, `parseInt(branchId, 10)`). `selectBranch` bahkan memvalidasi ulang:
   `Number.isFinite(numericId) && numericId > 0`, kalau gagal → toast
   `"ID cabang tidak valid"`. Ini sumber bug klasik dan patut diseragamkan di rebuild.
2. **`permissions: string[]`, bukan `PermissionCode[]`.** Jadi permission yang tersimpan di sesi
   kehilangan pengetikan ketat justru di tempat ia dipakai untuk gating UI.

Field default saat restore sesi (dari `app-store.tsx`): `companyTimezone` → `"Asia/Jakarta"`,
`companyCurrencyCode` → `"IDR"`, `companyLocale` → `"id-ID"`,
`defaultStockLocationLabel` → `"Default"`.

### 7.2 Kontrak komponen — `components/types.ts`

Satu file memegang props **semua** komponen design system (172 baris). Ini kontrak UI lintas
modul.

`Tone` — union bersama, dipakai Badge, SummaryCard, Notice, ConfirmDialog, Toast:

```ts
type Tone = "neutral" | "info" | "success" | "warning" | "danger" | "accent";
```

Catat: `types/shared.ts` mendefinisikan `ToastTone` terpisah dengan **5** nilai
(`info | success | warning | danger` + …) sementara `Tone` punya 6 (tambahan `neutral`,
`accent`). `utils.ts` `statusTone()` mengembalikan `ToastTone`. Jadi ada dua skala tone yang
tumpang tindih — **[PERLU KONFIRMASI]** apakah pemisahan ini disengaja (toast tidak boleh
`neutral`/`accent`) atau kebetulan sejarah.

`SelectOption` — bentuk opsi terpadu untuk seluruh keluarga select (biasa, searchable, async,
hierarchical):

```ts
type SelectOption = {
  label: string; value: string;
  description?: ReactNode; code?: ReactNode;
  depth?: number;              // ← untuk HierarchicalSelect (indentasi)
  disabled?: boolean; disabledReason?: ReactNode;
  isGroup?: boolean;           // ← node pengelompokan, tidak bisa dipilih
  path?: string[];             // ← jejak leluhur (breadcrumb opsi)
  searchText?: string;         // ← teks pencarian terpisah dari label
};
```

`depth` + `isGroup` + `path` adalah cara hierarki `StockLocation` / `ProductCategory` diratakan
menjadi daftar opsi. `disabledReason` memungkinkan UI menjelaskan **kenapa** sebuah lokasi tidak
bisa dipilih (mis. bukan leaf) — pesan itu bagian dari UX yang harus dipertahankan.

`DataColumn<T>` / `DataTableProps<T>` — kontrak tabel, dibahas di
[shared-services.md](shared-services.md) §5.

### 7.3 Tipe dokumen cetak — `types/shared.ts`

Ini bagian data model yang paling terikat ke dunia fisik, dan komentarnya menyimpan hasil tes
cetak nyata. Untuk target "tampilan sama persis", angka-angka di sini harus disalin apa adanya.

```ts
type DocumentBankAccount = { bank; holder; number };        // "BCA" / "HANDOKO" / "196 195 0541"
type DocumentPhone        = { label; number };              // "TOKO" / "0813-3052-1729"
```

`DocumentProfile` — profil kop dokumen, semua opsional dengan fallback ke data company/branch:

| Field | Fallback / contoh |
|---|---|
| `headerName` | nama perusahaan |
| `addressLine` | alamat cabang |
| `tagline` | contoh: `"PEREMPATAN KAMOLAN"` (baris lokasi pada struk) |
| `cityLine` | kota cabang (baris tanda tangan surat jalan) |
| `phones` | `DocumentPhone[]` |
| `bankAccounts` | `DocumentBankAccount[]` |
| `sellingPoints` | blok `"TB SUMBER ABADI MENJUAL"` pada nota |
| `returnNote` | contoh: `"barang yang sudah dibeli tidak dapat ditukar kembali"` |
| `thanksNote` | contoh: `"TERIMA KASIH"` |
| `continuousNota` | `ContinuousPaperProfile` |

`ContinuousPaperProfile` + `CONTINUOUS_PAPER_DEFAULTS` — kalibrasi kertas kontinu dot-matrix
(Epson LQ-310, form 3-ply 9.5" × 11"/2):

```ts
const CONTINUOUS_PAPER_DEFAULTS = {
  isDefault: false,
  widthMm: 241.3,        // 9.5"  — JANGAN disempitkan
  heightMm: 139.7,       // 11"/2 = 5.5"
  marginTopMm: 4,
  marginRightMm: 37.91,  // BUKAN margin visual
  marginBottomMm: 7,
  marginLeftMm: 6,
  showLetterhead: true,
};
```

Dua angka wajib dipahami sebelum disentuh, karena komentarnya menjelaskan keduanya berbeda
sifat:

- **`marginRightMm: 37.91`** bukan margin visual. Itu ruang cadangan untuk keterbatasan fisik
  print head printer narrow-carriage, yang jangkauannya hanya ±7.85" (~199.4mm) dari tepi kiri —
  jauh lebih sempit dari kertas 9.5" (yang lebar totalnya termasuk strip sprocket tractor-feed
  yang tidak bisa dicetak). `widthMm` **tetap** 241.3mm supaya `@page` CSS identik dengan ukuran
  kertas terdaftar di driver; menyusutkan `widthMm` membuat sebagian browser/driver gagal
  mencocokkan ukuran custom dan jatuh ke fallback yang salah — **pernah terjadi dan sudah
  diverifikasi lewat tes cetak fisik**. Hasilnya: `contentWidthMm = 241.3 − 6 − 37.91 = 199.39mm`.
- **`marginLeftMm: 6` dan `marginBottomMm: 7`** justru murni jarak visual — dikonfirmasi terlalu
  mepet ke tepi kertas dan area perforasi lewat tes cetak fisik. Naik dari 4mm bawaan lama.

`showLetterhead: false` dipakai bila kop/rekening/daftar jual sudah preprinted di kertas.

Konstanta ini sengaja diletakkan di `types/` (bukan di dalam `modules/orders/`) supaya module lain
— mis. `company/settings` — bisa memakainya sebagai placeholder tanpa reach-in lintas-module.
Resolusi runtime-nya (baca dari company settings) ada di
`modules/orders/components/print-shared.tsx` → `useContinuousPaperProfile`.

`ToastMessage` — `{ id, title, description?, tone? }`.

### 7.4 Bentuk state global

Dari `store/adapters/common.adapter.ts` dan `store/app-store.tsx`:

```
CompanyData  = userRecords, roleDefinitions, rolePermissions, itemCategories, items,
               memberTypes, businessParties, statusDefinitions, statusTransitions,
               knowledgeDocuments, whatsappAuthorizations, auditLogs, settings,
               whatsappChannelStatus
BranchData   = orders, stockBalances, stockMovements, stockLocations
WorkspaceData = CompanyData & BranchData
```

Pembagian ini mencerminkan scope DB: data ber-`idCompany` masuk `CompanyData`, data ber-`idBranch`
masuk `BranchData`. Ganti cabang → `BranchData` di-reset, `CompanyData` sebagian besar tetap.

`EMPTY_COMPANY_DATA` / `EMPTY_BRANCH_DATA` adalah bentuk awal (semua array kosong,
`settings: DEFAULT_SETTINGS`, `whatsappChannelStatus.state: "disconnected"`).

**✅ KEPUTUSAN — bentuk dan namanya dirancang ulang.** Tipe pembungkusnya masih bernama
`MockDatabase` dan `DemoUser`, dan `app-store.tsx` mengekspos `database: MockDatabase` sebagai
bagian context — warisan provider mock lama (header file menyatakan context baru dibuat
"compatible with MockAppContextValue so existing pages don't need to change").

Di sistem baru keduanya diganti: bentuk state diturunkan dari kebutuhan domain, bukan dari bentuk
mock. **Batasan yang mengikat: UX flow dan tampilan wajib tetap sama persis.** Urutan kerjanya
karena itu: petakan dulu data yang setiap halaman benar-benar butuhkan (hasil analisis modul), baru
rancang state-nya.

Yang layak **dipertahankan sebagai konsep**: pembagian `CompanyData` / `BranchData` di atas. Ia
memetakan langsung ke scope DB dan ke perilaku "ganti cabang me-reset data cabang, data perusahaan
tetap" — itu bukan artefak mock, melainkan cerminan model datanya.

### 7.5 Tipe input mutasi — `store/store-types.ts`

238 baris berisi tipe payload untuk setiap mutasi. Pola yang konsisten: `Omit<Domain, "id"> &
{ id?: string }` — satu tipe untuk create **dan** update, dibedakan ada/tidaknya `id`.

```ts
type ItemInput       = Omit<Item, "id"> & { id?: string };
type StatusInput     = Omit<StatusDefinition, "id"> & { id?: string };
type MemberTypeInput = Omit<MemberType, "id"> & { id?: string };
type PartyInput      = Omit<BusinessParty, "id"|"memberType"|"memberTypeId"> & { id?; memberTypeId? };
```

Beberapa input punya bentuk khusus yang mengungkap alur bisnisnya:

| Tipe | Yang menonjol |
|---|---|
| `OrderInput` | Alamat kirim punya **dua mode**: `shipToAddressId` (pilih dari buku alamat) **atau** `shipToAddress` + label/recipient/phone (sekali-pakai). Komentar: kirim `shipToAddress = ""` untuk mengosongkan saat edit |
| `ReceiveGoodsInput` | `settlementMode?: "pay_now" \| "switch_to_net"` — penerimaan barang bisa langsung menuntaskan pembayaran atau beralih ke termin |
| `DeliverGoodsInput` | `= ReceiveGoodsInput & { deliveredAt? }` — terima & kirim berbagi bentuk |
| `StockAdjustmentInput` | `approval?: { username; password }` — **kredensial approver dikirim di payload** (mode approval `simple`) |
| `StockLocationInput` | `forceTransfer?: boolean` — override saat lokasi masih berisi stok |
| `StockTransferInput` | `= StockLocationMoveInput & { toBranchId; driverName; vehicleNumber? }` — driver sebagai teks, bukan user |
| `ProductBulkImportPreview` | Struktur preview/commit import 4 sheet, dengan `failures?` untuk commit resumable |
| `UserFormInput` | `roleCodes: RoleCode[]` + `branchIds: string[]` |

`StockAdjustmentInput.approval` layak ditandai: username+password approver melewati request body
sebagai data biasa. Dalam alur "atasan mengetik password di layar operator" ini memang bentuknya —
tapi di rebuild perlu ditinjau (token approval sekali-pakai, misalnya).

`ProductBulkImportIssue.sheet` mengunci nama 4 sheet Excel sebagai union literal:
`"Kategori Produk" | "Daftar Produk" | "Varian Produk" | "Info Tambahan Produk"` — nama sheet
adalah bagian kontrak, bukan sekadar label.

### 7.6 Barrel tipe frontend — `types/`

| File | Isi |
|---|---|
| `types/index.ts` | Barrel (10 baris) |
| `types/catalog.ts` | Item, ItemCategory, MemberType, varian |
| `types/commerce.ts` | Order, OrderItem, BusinessParty, Payment, Delivery (252 baris — terbesar) |
| `types/inventory.ts` | StockBalance, StockMovement, StockLocation |
| `types/org.ts` | Company, Branch, User, Role, CompanySettings (179 baris) |
| `types/enums.ts` | 19 baris — kemungkinan re-export dari `@mini-erp/shared-types` |
| `types/shared.ts` | Tipe UI & dokumen cetak (§7.3) |

Pembagian ini sejajar dengan pembagian adapter (`catalog`, `commerce`, `inventory`, `org`,
`common`) — satu-ke-satu, memudahkan menelusuri "tipe FE ini dipetakan dari respons mana".

Aturan yang ditegakkan `check-architecture.mjs`: file di `types/` maksimal 400 LOC (warning), dan
frontend **tidak boleh** mengimpor entity backend — hanya tipe web atau shared package.

---

## 8. Ringkasan untuk Perancangan Schema Baru

Yang **wajib dipertahankan** (masing-masing menjaga kebenaran data, bukan sekadar gaya):

| # | Pola | Kenapa |
|---|---|---|
| 1 | Snapshot `OrderItem` | Nota historis tidak boleh bergeser saat master data berubah |
| 2 | `InventoryBalance` ↔ `InventoryMovement` berpasangan | Saldo stok harus selalu bisa direkonstruksi dari ledger |
| 3 | Transfer = 2 movement (`out` + `in`), bukan `transfer` | Ditegakkan di level tipe `MovementType` |
| 4 | Mutasi stok resolve ke **leaf** `StockLocation` | Node parent bukan bin penyimpanan |
| 5 | UOM ganda: `base_uom` untuk stok/cost, `transaction_uom` untuk dokumen | Mencampurnya = harga modal salah sebesar faktor |
| 6 | `average_cost` per `base_uom`, `purchase_price` per `purchase_uom` | Insiden nyata 300× |
| 7 | `datetime(6)` presisi mikrodetik | Dibutuhkan batas rentang `.999999` |
| 8 | Soft delete lewat `archivedAt` | Tidak ada hard-delete untuk data bisnis |
| 9 | `CONTINUOUS_PAPER_DEFAULTS` apa adanya | Terverifikasi lewat tes cetak fisik |

Yang sebaiknya **diperbaiki secara sadar**:

| # | Masalah | Usulan arah | Status |
|---|---|---|---|
| A | Tidak ada base entity — konvensi diulang 70× | Base entity/mixin untuk id + scope + timestamp + soft delete | ⬜ Usulan |
| B | `archivedAt` bukan `DeleteDateColumn` → filter manual di setiap query | Soft-delete native atau query scope terpusat | ⬜ Usulan |
| C | Isolasi tenant bergantung disiplin per-service | Single-company adalah keputusan sadar (bukan sementara) — sederhanakan, tapi jaga `idCompany` tetap konsisten | ✅ Terjawab: konsep multi-tenant ditinggalkan |
| D | ID `string` di FE vs `number` di BE | Seragamkan satu tipe | ⬜ Usulan |
| E | `permissions: string[]` di `StoredSession` | Pakai `PermissionCode[]` | ⬜ Usulan |
| F | Permission didefinisikan di 2 tempat (`permission-code.ts` + `role-access-config.ts`) | Turunkan UI dari satu sumber | ⬜ Usulan |
| G | Enum domain sebagai `varchar` tanpa constraint DB | Pilih sadar: ENUM/lookup table, atau terima dan tegakkan di aplikasi | ⬜ Usulan |
| H | Kolom JSON audit tanpa skema | Tipe payload per `actionKey` | ⬜ Usulan |
| I | Dua tabel sequence dengan scope berbeda (branch vs company); `format_template` cabang ditulis tapi tak dibaca | Satu formatter bersama, atau buang kolom template | ✅ Terverifikasi (services §2.4) |
| J | `Tone` (6 nilai) vs `ToastTone` (4 nilai) tumpang tindih | Satu skala tone | ⬜ Terbuka |
| K | Nama `MockDatabase`/`DemoUser` di state produksi | Rancang ulang bentuk **dan** nama, UX tetap identik | ✅ Keputusan diambil (§7.4) |
| L | Field `companyLocale`/`companyTimezone` tidak pernah dibaca formatter | Dibuang; `id-ID` + WIB jadi konstanta aplikasi | ✅ Keputusan diambil (rules §2.5) |
| M | `roundRupiah` vs `money()` 2-desimal | Rupiah utuh di semua lapisan; siapkan rekonsiliasi laporan | ✅ Keputusan diambil (services §3.3) |
| N | `rowHref` di `DataTableProps` tanpa konsumen | Buang dari kontrak `DataTable` | ✅ Terverifikasi (services §5) |
| O | `ApiRequest.guid`/`code`/`info` tidak pernah dikirim | Buang; envelope request jadi `{ data }` | ✅ Keputusan diambil (§5.1) |
