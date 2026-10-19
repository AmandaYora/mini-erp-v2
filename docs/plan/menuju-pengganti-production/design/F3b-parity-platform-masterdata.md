# Rancangan F3b: Paritas PLATFORM & MASTER DATA

Status dokumen: VERSI 2 (lengkap: F3b-1 s/d F3b-9, Lampiran A-C, Risiko, Pertanyaan). Urutan implementasi yang disarankan ada di bagian "Urutan & ukuran" paling bawah.
Cakupan: auth, user/role/permission, company, branch, product, party + member pricing, audit, media, assistant.
Bukan cakupan (agen lain): seeding katalog izin/role awal + hardening sesi (F1), penomoran bulanan UTC-vs-Jakarta (F2a), ETL buku/riwayat, indeks/perf schema.
Semua path relatif ke `H:/dimasprasetio/SAAS/mini-erp-revamp/apps/api` (revamp, "R") dan `H:/dimasprasetio/SAAS/mini-erp/apps/api/src` (legacy, "L") kecuali tertulis lain.
Migrasi berikutnya bernomor mulai `000032` (terakhir `000031_list_indexes`; 000026-000029 tidak ada, jangan diisi ulang). Semua perubahan schema = migrasi tambahan (nanti di-squash).

Legenda status temuan: TERBUKTI (saya baca kodenya, benar), TERBANTAH (klaim audit keliru), SEBAGIAN.

---

## 0. Ringkasan temuan (koreksi atas audit)

| # | Klaim audit | Status | Catatan singkat |
|---|---|---|---|
| 1a | Login hanya username | TERBUKTI | R `user/infrastructure/repository.go:132-140` `GetByUsername`; `auth/application/service.go:198`; `login.schema.ts`. Legacy `auth.service.ts:40-43`. |
| 1b | Izin sesi = gabungan semua role | TERBUKTI | R `auth/application/service.go:407` (Me) dan `:574` (resolveSession) memakai `roleIDs` semua. Legacy `jwt.strategy.ts:44-49` hanya role aktif. Login memilih `roleIDs[0]` (`:267`). |
| 1c | `/auth/switch-role` tak dipanggil UI | TERBUKTI | R `web/src/modules/auth/services/auth.service.ts` tak punya `switchRole`; `shared/layouts/AppLayout.tsx` `AvatarMenu` hanya "Ganti Cabang". Legacy `avatar-menu.tsx:85-130` punya "Ganti Role". |
| 1d | Data users/user_sessions hilang | TERBUKTI | Legacy `users.phone_e164, display_title, last_login_at`; `user_sessions.user_agent, ip_address`. R migrasi 000001/000002 tidak punya. |
| 2a | Katalog 61 vs 61 | SEBAGIAN | Revamp memang 61 kode (dihitung). Legacy `permission-code.ts` = 60 kode (+`stock.create` yang dihapus migrasi legacy 029; `purchase_return.*` ditambah migrasi 050, tidak ada di `seed-runner.ts`). ETL wajib membaca tabel `permissions` produksi, bukan konstanta. |
| 2b | Granularitas dilebur | TERBUKTI | lihat Lampiran A dan F3b-2. |
| 2c | `POST /deliveries/{id}/proof` tanpa RequireBranch | TERBUKTI | R `delivery/presentation/handler.go:305-306` satu-satunya `mux.Handle` yang tidak lewat `guarded(...)` (diperiksa untuk semua modul ber-branchGuard). Stock tidak menerima `branchGuard` tetapi memakai `guarded` internal (aman). |
| 2d | `stock.approve` hanya dipakai di service | TERBANTAH sebagai gap | Itu desain legacy juga: izin milik PENYETUJU (kredensial username+password) bukan pemanggil: legacy `stock.service.ts:176-218`, R `stock/application/transactions.go:358-380`. Union semua role penyetuju benar (legacy `userHasPermission` juga union). |
| 2e | Refund retur tanpa izin refund | TERBUKTI (temuan baru) | R `salesreturn/presentation` `POST /sales-returns/{id}/settlements` hanya `salesreturn.create`; legacy `sales-return.service.ts:904` dan `purchase-return.service.ts:569` mewajibkan `*_return.refund` untuk settlement `refund`. |
| 2f | Export order tanpa izin khusus | TERBUKTI (temuan baru) | R `purchasing`/`sales` `GET .../export` dijaga `*.view`; legacy `order.export`. |
| 3a | StockCheck tak pernah di-assign | TERBUKTI | R `product/application/service.go:36` dipakai di `:475,528`; tak ada assignment di `cmd/server/main.go`. `stock/infrastructure/repository.go:480 HasMovementsAnywhere` ada tapi tak dipakai siapa pun. |
| 3b | `replaceVariants` mengarsipkan varian ber-stok diam-diam | TERBUKTI | R `product/infrastructure/repository.go:291-298`. Lebih berbahaya: `UpdateProduct` mengganti kode produk dan `buildVariants` (`service.go:404-427`) membuat varian default baru berkode baru bila `variants` kosong, sehingga varian lama (ber-stok) diarsipkan. |
| 3c | Kode produk tak dicek tabrakan dengan kode/barcode varian | TERBUKTI | R `validateProduct` (`service.go:301-392`) tak mengecek; legacy `product.service.ts:445,692,713`. |
| 3d | Scan hanya barcode | TERBUKTI + dampak lebih besar | R `product/infrastructure/repository.go:428-435`, dipakai `stock/application/service.go:647`. Varian default dibuat TANPA barcode (`buildVariants`), sedangkan payload label QR = `barcode` atau `code` (`web/.../QrModal.tsx:27`, `print/lib/labels.ts:24`). Hasil: QR/label berisi kode varian TIDAK bisa di-scan di POS (`PosPage.tsx:337` -> 404). Legacy: varian default tersembunyi berbarcode = kode produk, dan scan cocok `variant_code OR barcode` lalu fallback `product_code` (`stock.service.ts:973-1060`). |
| 3e | Status legacy `inactive` terpisah dari archived | TERBUKTI | Legacy `products.status` (`active`/`inactive`) + `archived_at`. R hanya `active`/`archived`. |
| 3f | Tipe legacy 4 vs revamp 2 | TERBUKTI | Legacy `ItemType = physical|service|bundle|non_stock`; R `barang|jasa`. Perilaku bundle/non_stock di legacy identik dengan service (tidak dilacak stok, UOM tunggal): `product.service.ts:487-505`. Modul lain R bercabang pada `Tracked`, bukan `Type` (kecuali QR). |
| 3g | Impor produk | TERBUKTI, lebih rinci | lihat F3b-4. Catatan koreksi: "semua-atau-tidak" legacy berlaku pada GERBANG VALIDASI (`product-import.service.ts:98-101` menolak seluruh commit bila ada error); penulisan DB-nya chunked + upsert idempoten. Revamp commit per-baris parsial. |
| 3h | Media: resize/EXIF/WebP tak ada | TERBUKTI | R `media/application/service.go:49-100` simpan mentah. Legacy `image-optimizer.service.ts` (max 1600 produk / 2400 bukti, WebP q82/q88, limit 48 MP, `.rotate()`). |
| 3i | PDF tertolak | TERBANTAH sebagai gap | Legacy juga hanya gambar (optimizer menolak selain jpeg/png/webp; `ACCEPTED_*_TYPES` di web hanya image). `.pdf` di whitelist ekstensi R `media/application/service.go:71` adalah kode mati yang menyesatkan; hapus. |
| 3j | Setelah hapus foto utama tak ada pengganti utama | TERBUKTI (baru) | R `media/infrastructure/repository.go:108-117 Delete` tak mempromosikan foto lain; legacy `product.service.ts archiveMedia` mempromosikan. Hapus fisik vs arsip legacy. |
| 3k | Daftar produk tanpa thumbnail | TERBUKTI (baru) | R `productView` tak memuat gambar utama; legacy `attachPrimaryImages`. |
| 4a | Informasi Tambahan party hilang | TERBUKTI | Legacy `business_parties.metadata_json` + UI `customers-pages.tsx:459-480`; R tabel `parties` (000007) tak punya kolom, DTO `party/presentation/handler.go` tak punya field. |
| 4b | Basis purchase_price tanpa konversi UOM | TERBUKTI | R `party/application/service.go:801` `base = p.PurchasePrice`; legacy `member-pricing.service.ts:127-137` `(pp/purchaseToBase)*salesToBase`. |
| 4c | Harga kosong tak lagi error | TERBUKTI | Legacy `requirePrice` (null -> 400). R harga NOT NULL DEFAULT 0, tak dicek. |
| 4d | Rounding/arah/persen beda | TERBUKTI | peta enum di Lampiran B. Batas persen 0-100 dan ambang konfirmasi nominal 1.000.000 adalah tambahan revamp, tidak ada di legacy (`member-type.service.ts normalizeInput`). |
| 4e | Quote: gagal satu baris = gagal semua + N+1 | TERBUKTI (baru) | R `service.go:769-809` memanggil `products.GetByID` per baris (+varian+kategori); legacy mengembalikan `error` per item. |
| 5 | Panjang kolom branch/user | TERBUKTI | Lihat F3b-6. Tambah: `branches.default_stock_location_label`, kota 120 vs 100, `product_variants.code` 80 vs 50, `variant name` 255 vs 150, `product_categories.name` 255 vs 150. |
| 6 | Setting company yang berpengaruh | SEBAGIAN | dari 5 kolom JSON legacy hanya 3 benar-benar dibaca kode: `ui_preferences.documentProfile` (sudah dipadankan: key `document_profile`), `operational_rules.stockAdjustmentApprovalMode` (BELUM), `assistant_preferences.{whatsapp_mode,ai_daily_limit,behavior_preferences}` (AI/RAG by-design dibuang; `behavior_preferences` perlu dicek). Sisanya (`business_labels`, `defaultDueDays`, `defaultOrderKind`, `criticalStockFocus`, `compactTable`, `dashboardHighlight`, `reporting_preferences`) tidak dikonsumsi kode mana pun -> tidak dibawa. |
| 7 | Audit before/after, lintas cabang, best-effort | TERBUKTI | lihat F3b-8. |
| 8 | Assistant | SEBAGIAN | Rule engine: urutan intent beda ("penjualan hari ini" jadi performa), `order_status` daftar-per-status hilang, `product_info` rapuh (hanya kata pertama, nama hanya produk terlacak, stok via valuasi seluruh cabang). Whitelist/rate-limit/pairing setara. Audit 7 operasi channel/authorization/config nol. Lihat F3b-9. |

---

## F3b-1 Login: username ATAU email, izin = role aktif, switch-role UI, jejak login

Ukuran: M (backend S, UI S, test M).

### Masalah + bukti
1. Login username saja: R `user/infrastructure/repository.go:132` (`WHERE username = ?`), `user/application/service.go:94-110 VerifyCredentials`, `auth/presentation/handler.go:39-46` (field `username`), `web/.../login.schema.ts`. Legacy: `auth.service.ts:40-43` `where: [{username},{email}]`. Email di R unik dan NULL-able (KI-22), sehingga pencarian email aman.
2. Izin sesi: R `resolveSession` `auth/application/service.go:556-601` memanggil `GetPermissionCodes(roleIDs)` (semua role) dan hanya memakai `roleID` aktif untuk `RoleCode`. `Me` `:403-409` sama. `SwitchRole` `:476-504` valid tetapi tak mengubah izin. Legacy: izin hanya dari `session.idActiveRole`.
3. Jejak: legacy `users.last_login_at` (`auth.service.ts:72`), `user_sessions.user_agent/ip_address` (`:205-206`). `users.phone_e164`, `display_title` ikut di form pengguna legacy (`user.service.ts:101-110`).
4. Catatan perilaku legacy yang TIDAK dibawa karena by-design/lebih baik di revamp: refresh token bcrypt (revamp SHA-256 rotasi + replay detection), `switchBranch` legacy tak memvalidasi `status` cabang akses (revamp memvalidasi).

### Rancangan
**Backend**
- `user/contracts/client.go`: tambah `GetByLogin(ctx, identifier) (*User, error)` pada `UserClient`; `VerifyCredentials(ctx, identifier, password)` pakai identifier. Implementasi `user/infrastructure/repository.go`: `GetByLogin`: `SELECT ... FROM users WHERE username = ? OR email = ? LIMIT 2`; bila 2 baris berbeda (username A = email B) pilih yang cocok `username` dulu (urutan: `ORDER BY (username = ?) DESC LIMIT 1`). Trim + case-insensitive (collation MySQL default `*_ci`, tetap bandingkan `LOWER` hanya jika kolom bukan `_ci`; cek di test integrasi).
- `auth/presentation/handler.go Login`: terima `identifier` (alias `username` dan `email` tetap diterima untuk kompatibilitas klien lama: ambil `identifier`, jika kosong `username`, jika kosong `email`). Rate-limit key tetap `ip|identifier` (jangan bocorkan mana yang cocok; pesan error sama).
- Izin role aktif: `auth/application/service.go`
  - `resolveSession`: hitung `perms` dari `[]int64{roleID}` (role aktif setelah fallback), bukan `roleIDs`. Hapus pemanggilan union.
  - `Me`: pakai `sess.RoleID`; tambahkan ke respons `activeRole {id, code, name}` dan `availableRoles [{id, code, name}]` (dari `s.roles.List` difilter `roleIDs`; 1 grouped read, bukan per role). `userView` tetap menyertakan `roleIds`.
  - `Login`: pilih role aktif awal = role pertama (id terkecil, deterministik, KI-05). Respons login menyertakan `activeRole` dan `availableRoles`.
  - `SwitchRole`: sudah ada; tambahkan pengembalian `permissions` bukan hanya token, atau biarkan klien memanggil `loadMe` (pola yang sama dengan switch-branch). Pilih: biarkan `loadMe`.
  - `InvalidateUser` sudah dipanggil `UpdateUser`; `Verify` fallback role drift tetap.
  - Dampak: user multi-role yang sebelumnya melihat gabungan izin kini hanya role aktif. ETL: role legacy per-user dipetakan apa adanya (urutan `user_roles` legacy tidak dijamin; ETL mengurutkan id role menaik agar pilihan awal deterministik).
- Kolom baru (migrasi `000032_user_login_trail.up.sql`, down kebalikannya):
  - `ALTER TABLE users ADD COLUMN phone VARCHAR(30) NULL AFTER email, ADD COLUMN display_title VARCHAR(100) NULL AFTER full_name, ADD COLUMN last_login_at DATETIME(6) NULL;`
  - `ALTER TABLE user_sessions ADD COLUMN user_agent VARCHAR(255) NULL AFTER id_active_role, ADD COLUMN ip_address VARCHAR(64) NULL AFTER user_agent;` (255 cukup; potong UA di service, jangan TEXT).
  - `last_login_at` ditulis di `Login` lewat metode baru `UserClient.TouchLogin(ctx, userID)` (UPDATE ringan, best-effort, bukan dalam transaksi login). `CreateSession` menerima `UserAgent`, `IP` dari handler (`clientIP(r)` + header `User-Agent`).
  - `phone`/`display_title`: tambahkan ke `CreateUserInput/UpdateUserInput`, handler `user/presentation/handler.go` dan `UsersPage.tsx` (field opsional). Validasi telepon pakai fungsi yang sama dengan party (`validPhone`) disalin ke `shared/validator` bila belum ada (dua modul butuh; tidak boleh impor lintas modul).
  - Tidak ada `phone_e164` format E.164 di revamp: simpan apa adanya (ETL membawa nilai legacy mentah).
- Daftar sesi aktif per user / revoke dari admin = F1 (hardening); skema `ip_address,user_agent` yang saya tambahkan dipakai F1.

**Frontend**
- `modules/auth/schemas/login.schema.ts`: field `identifier` (label "Username atau Email"), `LoginPage.tsx` kirim `{identifier, password}`; `auth.service.ts login` ikut.
- `modules/auth/types.ts`: `AuthUser` + `activeRole?: {id,code,name}`, `availableRoles?: ...`.
- `auth.service.ts`: `switchRole: (roleId) => apiPost("/api/v1/auth/switch-role", {roleId})`.
- `auth.store.ts`: `switchRole(roleId)`: panggil endpoint, `tokenStore.set(accessToken, refresh)`, `loadMe()`; setelah itu jika rute aktif tidak lagi punya izin -> `navigate(dashboard)` (legacy `avatar-menu.tsx:108-120`). Taruh logika navigasi di `AppLayout` setelah `await`.
- `shared/layouts/AppLayout.tsx AvatarMenu`: tampilkan "Ganti Role" bila `availableRoles.length > 1` (sub-daftar seperti legacy), tampilkan role aktif di header menu.
- Keputusan UI (diambil, tidak perlu ditanya): tampilkan "Ganti Role" hanya bila user punya >1 role (sama dengan legacy).

### Reuse terverifikasi
`SwitchRole` + `repo.UpdateBranchRole` + `cache.drop` sudah ada; `AuthzInvalidator`; `userView`; `useAccessibleBranches` (pola serupa); rate limiter di auth.

### Test
- Go unit: `GetByLogin` (username, email, keduanya cocok orang berbeda, email NULL tak cocok string kosong); `resolveSession` izin hanya role aktif (dua role dengan izin berbeda); `SwitchRole` mengubah izin pada `Verify` berikutnya; `Login` mengisi `last_login_at` dan IP/UA.
- Integrasi `internal/integration`: login by email -> 200; role ganti -> `GET /audit-logs` 403 lalu 200 setelah kembali.
- Vitest: `LoginPage` kirim identifier; `AvatarMenu` menampilkan Ganti Role hanya multi-role.

### Cek volume/performa
Verify tetap 1 cache-miss per 30 dtk per sesi; `GetPermissionCodes([]int64{roleID})` lebih murah dari union. `users` ~ puluhan baris: `OR` tanpa indeks tambahan aman (kedua kolom sudah unique-indexed).

Dependensi: tidak ada. F1 menyentuh `auth/application/service.go` (rate-limit, sesi); koordinasi: perubahan saya di `resolveSession`/`Me`/`Login` signature saja.

---

## F3b-2 Izin: granularitas legacy, guard rute, kebijakan role reserved

Ukuran: L (banyak rute, tetapi mekanis). Pemilik katalog awal + seeding = F1; saya menyerahkan DAFTAR KODE dan PETA RUTE (Lampiran A, C). F1 yang memasukkannya ke seed/migrasi data `permissions`.

### Masalah + bukti
Lihat Lampiran A (peta kode) dan C (peta rute). Ringkasan perbedaan perilaku nyata (bukan sekadar nama):
1. `member_type.view` legacy (daftar jenis member untuk dropdown order/POS/customer) vs R `GET /member-types` dijaga `member_types.manage` (`party/presentation/routes.go:30`): kasir tak bisa memuat dropdown tanpa izin kelola.
2. `order.export`: R `GET /sales-orders/export`, `/purchase-orders/export` dijaga `*.view`.
3. Refund retur tanpa `*.refund` (F3b-2 temuan 2e).
4. Finance: legacy 14 kode, R 3 (`finance.view/post/manage`). Contoh perbedaan: `finance.manage` R mengizinkan posting jurnal manual, buka/tutup periode, penyesuaian pajak sekaligus; legacy memisahkan `journal.reverse`, `tax_adjustment.manage`, `close.force`; `finance.close.force` tak ada di R sehingga `POST /finance/periods/close` (tutup periode tanpa readiness, `finance/application/readiness.go:83-98` hanya SafeClose yang memeriksa) dijaga `finance.manage` biasa. Legacy `finance-close.service.ts:198` mewajibkan `finance.close.force` untuk force.
5. Stok: legacy `stock.update` (CRUD lokasi) `stock.transfer` (transfer + pindah satu-langkah) `stock.adjust` (koreksi + saldo awal + rusak/pulihkan/hapus) `order.create` (hold/release reservasi) vs R satu `stock.manage` untuk lokasi, reservasi, transfer, rusak, saldo awal (`stock/presentation/routes.go:22-52`) dan `stock.adjust` hanya koreksi. Efek: role `staff` legacy (stock.adjust saja) bisa catat rusak & saldo awal; di R harus diberi `stock.manage` yang sekaligus membuka lokasi/transfer.
6. `whatsapp.simulate` (hanya superadmin) vs R `assistant.manage` untuk `/assistant/simulate` (`assistant/presentation/handler.go:268`).
7. Legacy `order.*` membungkus: penjualan+pembelian+customer+supplier+goods receipt+delivery+alamat kirim+status-history. R memecah (lebih granular). Pemetaan role legacy -> R harus MENGUNDANG semua pecahan (Lampiran A) agar tak ada kehilangan akses; pengetatan yang disengaja (mis. `customers.archive` terpisah) dilakukan admin setelah cutover.
8. `sales.approve` R (approve-credit) lebih ketat daripada legacy (`order.update`). Pemetaan: pemegang `order.update` mendapat `sales.approve` saat migrasi role (menjaga siapa yang bisa approve).

### Rancangan
**Katalog kode baru/berubah (F1 meng-seed; saya menetapkan namanya):**
- Tambah: `member_types.view`, `sales.export`, `purchasing.export`, `salesreturn.refund`, `purchasereturn.refund`, `stock.update`, `stock.transfer`, `assistant.simulate`, finance granular (nama sama dengan legacy agar peta jelas): `finance.close.force`, `finance.posting.view`, `finance.posting.post`, `finance.journal.view`, `finance.journal.reverse`, `finance.report.view`, `finance.tax_report.view`, `finance.expense.view`, `finance.expense.create`, `finance.expense.cancel`, `finance.tax_adjustment.view`, `finance.tax_adjustment.manage`.
- Hapus (setelah rute dipindah dan migrasi data role): `stock.manage` (diganti `stock.update`/`stock.transfer`/`stock.adjust`), `finance.post` (diganti `finance.posting.post` + `finance.expense.create/cancel`).
- Dipertahankan: `stock.approve`, `finance.view`, `finance.manage`, semua kode revamp lain.
- Penamaan: kode finance mempertahankan format legacy (dot bersarang) karena `PermissionCatalog()` hanya string; tidak ada aturan format.

**Perubahan rute** (file: `<modul>/presentation/*.go`, bergantung pada Lampiran C):
- `party/presentation/routes.go:30`: `GET /member-types` -> `member_types.view`; CUD/archive/restore tetap `member_types.manage`.
- `sales/purchasing` export -> `sales.export` / `purchasing.export`.
- `salesreturn/purchasereturn` handler `AddSettlement`: jika `type == refund`, cek `authcontracts.SessionFromContext(ctx).HasPermission("salesreturn.refund")` di handler (pola: handler yang memegang session; service tak melihat session). 403 dengan pesan legacy: "Anda tidak memiliki izin memproses refund". Juga berlaku untuk settlement `refund` yang dibuat implisit saat create/confirm bila ada (cek `salesreturn/application/service.go` jalur create membawa settlement; tutup juga).
- `stock/presentation/routes.go`: ikuti Lampiran C.
- `finance/presentation/*`: ikuti Lampiran C; `POST /finance/periods/close` -> `finance.close.force` (rute ini adalah jalur "tutup tanpa readiness"; `safe-close` tetap `finance.manage`); tidak ada flag `force` di `safe-close` R (readiness selalu wajib) sehingga `finance.close.force` hanya melekat pada rute `periods/close`.
- `delivery/presentation/handler.go:305`: bungkus `ProofUpload` dengan `guarded(...)`: `perm("delivery.create", guarded(func(w,r){ h.ProofUpload(w,r,media) }, branchGuard))`. Tambah test rute (tabel di bawah).
- `assistant/presentation/handler.go:268`: `simulate` -> `assistant.simulate`.
- `user/presentation/routes.go:32` `GET /permissions` tetap `roles.manage` (UI role perlu), tapi `GET /roles` `roles.view` (sudah).

**Guard sistematis (anti-regresi):** tambah test `cmd/server` atau `internal/router` yang mem-parse seluruh pola `mux.Handle` (reflection pada `http.ServeMux` tidak mungkin; gunakan test statis: `internal/integration/route_guard_test.go` membuat server penuh via `server.New`+semua `RegisterRoutes`, lalu untuk SETIAP rute terdaftar (daftar dari wrapper `perm` yang dicatat: ubah `authMod.Permission` menjadi pencatat ketika `GUARD_AUDIT=1`) memastikan 401 tanpa token, 403 tanpa izin, 403 "Pilih cabang aktif" untuk sesi tanpa cabang pada rute `needsBranch`). Ringkas: perlu `authMod.Permission` mencatat `(method, path, perm)` ke registry in-memory; test membaca registry. [rincian implementasi: S-M]

**Kebijakan role reserved (superadmin)** — saya yang menangani kebijakan peran; F1 menangani lock-out:
- Legacy: `reserved-role-policy.ts` + migrasi 042: role `superadmin` (id 1, global), hanya user id 1; tak bisa di-assign lewat API (`user.service.ts:91,140,309-313`); disembunyikan dari daftar user/role untuk non-superadmin (`user.service.ts:39-50`, `role.service.ts:34`); `updatePermissions` atas role itu 404 untuk non-superadmin (`role.service.ts:164`); `assignRoles` memaksa user 1 selalu memegangnya (`:291-298`); user yang memegangnya tak bisa diubah/dinonaktifkan/ganti sandi oleh non-superadmin (`assertTargetUserManageable`, 404).
- Revamp saat ini: tak ada role superadmin; seed membuat `owner`. Kebijakan diputuskan: **dibawa sebagai "role reserved" generik** (bukan hard-code id 1): kolom `roles.is_reserved TINYINT(1) NOT NULL DEFAULT 0` (migrasi `000033_roles_reserved.up.sql`). Semantik:
  1. role `is_reserved=1` tak muncul di `GET /roles`, `GET /users` (user pemegangnya) dan `GET /users/{id}` bagi sesi yang bukan pemegang role reserved (`session.RoleCode` tak reserved); `PUT/DELETE /roles/{id}*` dan mutasi user pemegangnya -> 404 bagi mereka.
  2. role reserved tak dapat di-assign/dilepas lewat `POST/PUT /users` (validasi `requireRolesExist` menolak id reserved: "Role ini khusus akun pengembang default"; saat `PUT /users/{id}` untuk user pemegangnya, role reserved dipertahankan otomatis seperti `assignRoles` legacy).
  3. `DeleteRole` sudah menolak `is_system`; reserved ikut `is_system=1`.
  4. ETL: role `superadmin` legacy -> `roles(code='superadmin', is_system=1, is_reserved=1)`, user id 1 -> user pemegang (username mempertahankan nilai legacy).
  5. Koordinasi F1: lock-out guard ("tak boleh menghilangkan izin `roles.manage` terakhir / menonaktifkan pemegang izin terakhir") ditulis F1; saya menyediakan helper `UserClient`-sisi-role `CountHoldersOfPermission` jika F1 butuh (S).
- Perlu `userview.go`: `contracts.Role` + `IsReserved`; `repo.ListRoles(ctx, includeReserved bool)`; `ListUsers` memakai `NOT EXISTS` ke `user_roles` JOIN `roles` (intra-modul, aman).
- Test: sesi owner (non-reserved) tak melihat/mengubah user reserved; sesi reserved melihat; assign reserved ditolak.

### Reuse
`PermissionCatalog()` per modul + `EnsurePermissions` (seed); `AuthzInvalidator.InvalidateAll` setelah perubahan katalog/role.

### Test
Tabel rute (Lampiran C) -> test tabel `TestRouteGuards` memeriksa tiap rute berubah: 403 tanpa izin baru, 200 dengan izin baru; refund tanpa izin 403; proof tanpa cabang 403.

### Cek volume
Hanya konfigurasi; `GetPermissionCodes` satu role ~ <70 baris.

Dependensi: F1 (seed + lock-out), modul pemilik rute (sales, purchasing, salesreturn, purchasereturn, stock, finance, delivery, assistant): perubahan 1-3 baris per rute, tidak ada logika bisnis.

---

## F3b-3 Produk: guard riwayat stok, varian, scan, tabrakan kode, status, tipe

Ukuran: L.

### 3.1 Wiring StockCheck lewat kontrak (arah dependensi: product L2 <- stock L3)
Bukti: lihat 3a/3b di tabel. Tidak boleh product mengimpor stock. Desain (pola `SetPayments`):
- Pindahkan tipe `StockState` dari `product/application` ke `product/contracts` dan tambahkan antarmuka milik product (stock yang mengimplementasikannya karena stock sudah boleh mengimpor `product/contracts`):
  ```go
  // product/contracts/stockstate.go
  type StockState struct { HasHistory bool; Available float64; HasHolds bool }
  type VariantStockState struct { OnHand float64; HasHistory bool }
  type StockStateProvider interface {
      ProductStockState(ctx context.Context, productID int64) (StockState, error)
      VariantStockStates(ctx context.Context, productID int64) (map[int64]VariantStockState, error) // 1 grouped read
  }
  ```
- `product/application/service.go`: ganti field `StockCheck func` dengan `stock contracts.StockStateProvider` + `SetStockState(p)`; `product.Module.SetStockState(...)`.
- `stock/application`: implement `ProductStockState`: `HasHistory` = `repo.HasMovementsAnywhere(productID)` (sudah ada, `repository.go:480`; ubah ke `SELECT EXISTS(... LIMIT 1)`); `Available` = `SELECT COALESCE(SUM(on_hand),0) FROM stock_balances WHERE product_id=?` (SELURUH cabang; legacy guard `inventory_balances.on_hand_qty` > 0.0001 lintas branch perusahaan); `HasHolds` = `EXISTS stock_reservations(status='active' AND qty_remaining>0)` OR `EXISTS stock_transfer_items JOIN stock_transfers status IN ('draft','dispatched')` (intra-modul). `VariantStockStates`: `SELECT variant_id, SUM(on_hand), (EXISTS movement)` group by variant dalam satu kueri. Indeks: `stock_movements` punya `ix_stock_movements_lookup (branch_id, product_id, ...)` yang tak melayani `WHERE product_id = ?` (kolom depan branch). Tambahkan migrasi `000034_stock_product_idx.up.sql`: `ALTER TABLE stock_movements ADD KEY ix_stock_movements_product (product_id, variant_id);` `ALTER TABLE stock_balances ADD KEY ix_stock_balances_product (product_id, variant_id);` (serahkan finalisasi ke agen schema/perf; saya mendaftar kebutuhan).
- `cmd/server/main.go`: setelah `stockMod := stock.NewModule(...)`: `productMod.SetStockState(stockMod.ProductStockState())`. `cmd/seed/main.go` tak perlu (tak menjalankan update produk).
- Guard yang harus aktif (legacy `product.service.ts`):
  - `UpdateProduct`: `HasHistory` -> UOM dasar/beli/jual, faktor, tipe terkunci (kode ada, `service.go:480-489`); tambah: produk `Tracked=true` tak boleh diubah menjadi `Tracked=false`/tipe non-barang bila `HasHistory || Available>0` (legacy `assertTypeTransitionAllowed:569-573`, lebih ketat: tanpa syarat history); mengubah `Tracked` dari false->true diizinkan.
  - Varian: transisi punya-varian <-> tidak (di R: set varian eksplisit vs hanya default). Aturan: mengarsipkan/menonaktifkan varian dengan `OnHand>0` ditolak (pesan legacy "Varian masih punya stok. Kosongkan stok varian sebelum dihapus atau disembunyikan."), mengubah kode varian ber-`HasHistory` ditolak (kode varian = kunci scan & ID stok stabil; rename = varian baru), mengaktifkan varian eksplisit pada produk yang varian default-nya masih ber-stok ditolak (`assertVariantTransitionAllowed`).
  - `ArchiveProduct` `Available>0 || HasHolds` ditolak (`service.go:528-535`, sudah benar; legacy menolak `on_hand>0`).
- Perbaiki `replaceVariants` (`product/infrastructure/repository.go:246-301`): ubah signature menerima `states map[int64]VariantStockState` atau, lebih bersih, pindahkan keputusan ke service: service memanggil `VariantStockStates` sebelum `UpdateProduct`, menyusun daftar "varian yang akan diarsipkan" (kode tak ada di payload), menolak bila ada yang `OnHand>0`; repo hanya mengeksekusi. Tambah juga: ubah `buildVariants` agar pada `UpdateProduct` tanpa `variants` TIDAK membuat default baru bila produk sudah punya varian aktif (pertahankan set varian yang ada); kode produk berubah -> varian default ikut berubah kode HANYA bila varian default tak punya riwayat (`HasHistory` false), kalau punya riwayat: kode varian default dibekukan (kode produk berbeda dari kode varian default diperbolehkan). Test regresi untuk skenario ini (ubah kode produk berstok).
- Legacy memegang flag `has_variants` + `is_hidden`. Di R varian default = `is_default=1` dan UI memuat semua varian aktif ke form (`ProductFormPage.tsx:102-110`), sehingga varian default tampak sebagai "varian". Rancangan: tambah `product_variants.is_hidden TINYINT(1)` dan `sort_order INT`, `attributes JSON NULL`, status `inactive` (sub-poin 3.4). API `productView.variants[]` menambah `isHidden`, `sortOrder`, `attributes`. UI form hanya menampilkan varian `!isHidden`; toggle "Produk punya varian" (legacy `has_variants`) mengatur apakah default disembunyikan.

### 3.2 Scan: cocokkan kode varian, barcode, kode produk
- Kontrak baru `product/contracts/client.go`: `ResolveScanCode(ctx, code string) (*ScanMatch, error)` dengan
  ```go
  type ScanMatch struct { Product *Product; Variant *Variant; Via string /* variant_code|barcode|product_code */; VariantRequired bool }
  ```
  Urutan: 1) `product_variants` aktif dengan `code = ? OR barcode = ?` (satu kueri, indeks: unique (product_id,code) tidak melayani `code` saja -> migrasi indeks `ix_product_variants_code (code)`), 2) `products.code = ?`: jika produk punya varian tak-tersembunyi >1 -> `VariantRequired=true` (legacy status `variant_required`); jika tidak -> varian default. Hanya produk `status='active'`; produk inactive/archived -> hasil `NotFound`+pesan khusus (legacy `inactive` tetap mengembalikan produk; di R: 409 "Produk tidak aktif").
- Pertahankan `GetVariantByBarcode` (dipakai stock/POS) tetapi pindahkan `stock/application/service.go:647 Scan` ke `ResolveScanCode`; respons API `POST /stock/scan-product` menambah `status` (`ok|variant_required|inactive`) dan `variants[]` bila `variant_required`; POS (`PosPage.tsx:332-350`) menangani `variant_required` dengan memilih varian.
- Opsi data: ETL menyetel barcode varian default hidden = NULL (kode produk sudah di-resolve via langkah 2), jadi tak menggandakan indeks barcode.

### 3.3 Tabrakan kode produk <-> kode/barcode varian
- `validateProduct` (`service.go:301`): tambah `repo.VariantCodeOrBarcodeTaken(ctx, code, excludeProductID)` -> 409 "Kode produk 'X' sudah digunakan sebagai kode atau barcode varian" (kecuali varian default milik produk itu sendiri), dan untuk tiap varian: kode/barcode tak boleh sama dengan `products.code` produk lain, kode varian tak boleh sama dengan barcode varian lain, antar-varian dalam payload (kode+barcode satu namespace; legacy `seenScanKeys`). Satu kueri IN untuk semua kunci payload (bukan per varian). Panjang: `code` produk 50, varian 80 (lihat 3.5).
- Constraint DB: `uq_product_variants_barcode` global sudah ada; tambahkan pemeriksaan service saja untuk lintas-tabel (tak ada FK lintas tabel yang bisa menegakkan ini).

### 3.4 Status, tipe, atribut, ringkasan
- Migrasi `000035_product_parity.up.sql`:
  - `ALTER TABLE products ADD COLUMN summary TEXT NULL, ADD COLUMN attributes JSON NULL;` (`products.status` sudah VARCHAR(20): terima `active|inactive|archived`; tidak butuh DDL).
  - `ALTER TABLE product_variants ADD COLUMN attributes JSON NULL, ADD COLUMN is_hidden TINYINT(1) NOT NULL DEFAULT 0, ADD COLUMN sort_order INT NOT NULL DEFAULT 0;` + `MODIFY name VARCHAR(255)`, `MODIFY code VARCHAR(80)`.
  - `ALTER TABLE product_categories ADD COLUMN sort_order INT NOT NULL DEFAULT 0, MODIFY name VARCHAR(255);`
  - `ALTER TABLE products` panjang: `code` tetap 50 (legacy 50), `name` 255 (sama).
- Tipe: izinkan `barang|jasa|paket|non_stok` (padanan `physical|service|bundle|non_stock`); hanya `barang` boleh `Tracked=true`; selain itu `Tracked=false`, faktor 1, ketiga UOM sama (aturan legacy `resolveProductUomConfig:487-505`). Titik ubah: `validateProduct` (`service.go:332`), `import.go:228`, `ProductFormPage.tsx:265-268`, `product.schema.ts:22`, `ProductDetailPage.tsx:143,188` (label), `qr`: QR bulk tetap `type='barang'`.
- Status: `PUT /products/{id}` menerima `status` (`active|inactive`); `ArchiveProduct` tetap; tambah `POST /products/{id}/restore` (`products.archive`) karena legacy punya Sampah untuk party saja—untuk produk legacy `archived_at` tidak punya restore API: **tidak ditambahkan** (hindari fitur baru). `inactive` memblokir transaksi baru (modul lain sudah menolak `Status != "active"`) tetapi tetap tampil di daftar; filter daftar menambah `status` (`active|inactive|archived`) dan default UI menyembunyikan `archived`.
- `summary`, `attributes` ("Info Tambahan" key-value) -> `productPayload`, `productView`, `ProductFormPage` (blok seperti `customers-pages.tsx` legacy), `ProductDetailPage`.
- Filter list: tambah `type`, `tracked`; `search-options` menambah `preferStockAvailable` (legacy `addStockAvailabilityRank`) melalui `ProductAvailability` stok via kontrak (stock -> product tidak boleh; maka lakukan ranking di lapis handler ASSISTANT/POS? **diputuskan**: tidak dibawa; di legacy dipakai pada pencarian order/POS untuk mengurutkan produk bersaldo di atas; R memakai `GET /stock/balances` + daftar produk di sisi POS. Dicatat sebagai selisih kecil yang disetujui, bukan gap fungsi).
- `search-options` R (`handler.go:212-225`) hanya mengembalikan `id, code, name, sellingPrice` dan selalu `limit 20`, `q` saja; legacy mengembalikan UOM, faktor, harga beli/jual/minimum, tipe, tracked, varian (`product.service.ts:197-230`). Perluas respons (`baseUom, purchaseUom, salesUom, purchaseFactor, salesFactor, purchasePrice, minSellingPrice, type, tracked, variants[]`) dan terima `limit` (1-30), `status`, `type`, `tracked`, `productId`; tanpa ini form PO/SO harus memanggil `GET /products/{id}` per pilihan.
- Thumbnail daftar: `productView` + `primaryImageUrl` lewat `MediaClient.PrimaryByOwners(ctx, ownerType, ids) (map[int64]string)` (kontrak baru batch; satu kueri `WHERE owner_type='product' AND owner_id IN (...) AND is_primary=1`), diisi di handler list (product handler sudah memegang `media` client untuk rute media; untuk `List` perlu diteruskan: ubah `RegisterRoutes` agar `List` menjadi closure berisi `media`).

### 3.5 Pelebaran kolom (daftar kebutuhan; keputusan tipe final = agen schema)
`users.username 50->100`, `users.full_name 150->255`; `branches.code 10->50`, `name 150->255`, `address VARCHAR(255)->TEXT`, `city 100->120`; `product_variants.code 50->80`, `name 150->255`; `product_categories.name 150->255`; `parties.code` 50 (sama), `phone 30` (sama); `member_types.name` 120 (sama); `roles.name 100` (sama). Catatan: nomor dokumen `VARCHAR(60)`: kode cabang 50 menghasilkan `ORD-<50>/PB/2026/09/00001` = 80 karakter, tak muat 60. Jika kode cabang dilebarkan, kolom `number` di 9 tabel dokumen harus ikut ke VARCHAR(100), atau batasi kode cabang ke 20 dan migrasikan hanya jika data legacy memang >10. **Perlu cek data legacy**: `SELECT MAX(CHAR_LENGTH(code)) FROM branches` (diminta sebelum memutuskan; lihat Pertanyaan Terbuka).

### 3.6 Media produk (optimasi gambar)
- `media/application/service.go Upload`: untuk owner `product` (dan `payment_proof`, `delivery_proof`): decode (stdlib `image/jpeg`, `image/png`; WebP decode + encode butuh dependensi: `golang.org/x/image/webp` hanya decode, tidak ada encoder WebP pure-Go yang matang. Keputusan: dukung input jpeg/png/webp (decode via `golang.org/x/image/webp`), **keluaran JPEG q85 (atau PNG bila alpha)** dengan resize `fit inside` 1600 (produk) / 2400 (bukti), `withoutEnlargement`, EXIF orientation diterapkan lalu seluruh EXIF dibuang (re-encode otomatis menghapus metadata; orientasi dibaca dengan `github.com/rwcarlsen/goexif/exif` atau `github.com/disintegration/imaging` `AutoOrientation(true)`), batas piksel input 48.000.000 (cek `image.DecodeConfig` sebelum decode penuh). Output WebP persis legacy butuh cgo/ binary; ditolak karena Docker satu container tanpa cgo. Konsekuensi: ukuran berkas sedikit lebih besar dari WebP; diterima.
- Deteksi tipe: whitelist `image/jpeg|png|webp` dari `DetectContentType` + ekstensi; buang `.pdf` dari whitelist (`service.go:71`) dan `Accept ["image/"]` -> whitelist eksak (gif/bmp lolos prefix `image/` saat ini).
- Promosi primer setelah hapus (`media/infrastructure/repository.go:108 Delete`): dalam transaksi, jika baris yang dihapus `is_primary=1`, set `is_primary=1` pada baris `ORDER BY id ASC LIMIT 1`. Perlombaan `IsPrimary: len(existing)==0` (`service.go:87`) -> hitung di dalam transaksi `INSERT ... SELECT` atau `SELECT ... FOR UPDATE`.
- `media_files` tambah `width INT NULL, height INT NULL, original_size_bytes BIGINT NULL, sort_order INT NOT NULL DEFAULT 0` (padanan metadata legacy) dalam migrasi `000036_media_meta.up.sql`. Legacy `purpose` (`product_image`) = `owner_type` R; tak perlu kolom.
- Dependensi go.mod baru: `golang.org/x/image` (sudah ada excelize yang menarik `x/image`? cek `go.mod`), `github.com/disintegration/imaging`.
- Test: gambar 4000x3000 -> <=1600; PNG berEXIF-bomb piksel besar ditolak; file `.png` isi GIF ditolak; hapus primer -> pengganti.

Cek volume: foto 5 MB -> proses CPU di request; batasi konkurensi dengan semaphore global (N=2) agar upload massal tak menjatuhkan container.

### Reuse
`ProductClient`, `QRLabel` (`product/application/qr.go`) untuk payload; `ExistingCodes` (impor), `httpx.MultipartFile`.

### Test
Unit service dengan fake `StockStateProvider`; integrasi MySQL: update kode produk berstok; arsip varian ber-stok ditolak; scan via kode varian/produk; tabrakan kode; status inactive diblok sales (`sales` create dengan produk inactive 4xx).

---

## F3b-4 Impor produk: template Excel lama harus bisa dipakai

Ukuran: L.

### Bukti perbedaan
Legacy `product-import.service.ts`: 4 sheet dengan alias nama sheet (`CATEGORY_SHEET_ALIASES` dst, baris 107-110), alias header Indonesia (`CATEGORY_HEADERS`... baris 112-150), `.xlsx` dan `.xls` (SheetJS), aksi per-baris `create|update` (upsert berdasarkan kode, `validateProductReferences:672-700`), hierarki kategori dari kolom "Subkategori Dari" (nama ATAU kode, `resolveCategoryRef`), "Info Tambahan Produk" -> `attributes`, kolom `Status`, `Pantau Stok?`, `Satuan Stok/Beli/Jual`, `Isi per Satuan Beli`, `Isi Jual per Satuan Stok` (kebalikan faktor; `parseSalesToBaseFactor:1156-1175`: faktor = 1/nilai) ATAU `Isi per Satuan Jual` (alias lama), `Minimum Stok`, harga, `Catatan Produk` -> `summary`; angka diparse longgar `parseFlexibleNumber` (`"10,000"`, `"10.000"`, `"1.234,50"`); varian: `Kode Produk, Kode Varian, Nama Varian, Status, Urutan Tampil` (tanpa barcode); gerbang: seluruh commit ditolak bila ada error validasi; kategori satu transaksi, produk chunk 200 + fallback per-baris, varian satu transaksi; audit satu entri ringkas.
Revamp `product/application/import.go`: 2 sheet (`products`, `variants`) header Inggris (`:24-33`), create-only (`:206-208` kode ada = error saat preview, tetapi `:409` skip saat commit — tak konsisten), tanpa kategori/atribut, commit parsial per baris, `parseIntCell` `ParseInt` (`:102`) menolak `"10,000"` dan harga desimal ("10000.5"), `excelize GetRows` mengembalikan teks berformat sel (mis. "10,000" bila format angka) bukan nilai mentah -> gagal. `.xls`: UI menerima (`ImportPage.tsx:111`) tetapi `excelize.OpenReader` hanya OOXML -> "bukan file Excel yang valid". Preview melakukan 2 kueri per baris (`:206,215`) -> N+1.

### Rancangan
- Ganti `import.go` menjadi paket impor baru di modul product: `application/import/` (tetap di module product; tak ada lintas modul): file `workbook.go` (baca xlsx/xls -> `[]sheet{name, rows [][]string}`), `parse.go` (alias, normalisasi header `lower, hapus "?", spasi tunggal`), `plan.go` (validasi + rencana create/update), `commit.go` (tulis), `template.go` (template Indonesia).
- Pembaca file: `.xlsx` via excelize dengan opsi `excelize.Options{RawCellValue: true}` bila sel numerik untuk mendapat nilai mentah, kemudian `parseFlexibleNumber` (port persis dari legacy `product-import.service.ts:1201-1220`) untuk sel teks. `.xls` (BIOS BIFF8): tambahkan `github.com/extrame/xls` (pure Go, read-only). Bila sulit/berisiko, fallback keputusan: tolak `.xls` dengan pesan jelas "Simpan ulang sebagai .xlsx" dan hapus `.xls` dari `accept`. **Rekomendasi: implementasi `.xls` via extrame/xls** karena pengguna legacy punya file lama (diuji dengan fixture .xls); jika uji gagal dalam M, turun ke fallback.
- Nama sheet & header: tabel alias dipindahkan apa adanya (konstanta). Sheet wajib: Kategori Produk + Daftar Produk (sama legacy); Varian & Info Tambahan opsional. Sheet "Panduan" diabaikan.
- Aksi: upsert berdasar kode produk (case-insensitive): ada -> `update` (menimpa kolom yang disebutkan; kolom kosong pada update TIDAK menimpa nilai lama kecuali legacy menimpa semuanya (`applyProductDraft` menimpa semua)). **Pertahankan perilaku legacy (menimpa semua)** karena file legacy adalah "sumber kebenaran". Namun jalankan guard yang sama dengan form untuk update: UOM/faktor/tipe terkunci bila `HasHistory` (legacy TIDAK menjaga di impor `applyProductDraft`: itu celah legacy; revamp menjaga, dilaporkan sebagai error baris).
- Kategori: upsert berdasar kode (kategori aktif), induk dari nama/kode (draft atau existing), deteksi siklus, kode di data arsip -> error "ada di data arsip, pulihkan dulu" (legacy) — di R kategori terarsip tak punya restore API; ubah menjadi: kategori terarsip dengan kode sama DIPULIHKAN otomatis (status active) + dicatat di ringkasan (keputusan: lebih ramah, tidak ada fitur restore kategori). `[Konfirmasi dengan pemilik tidak perlu]`.
- Varian: `Kode Produk, Kode Varian, Nama Varian, Status, Urutan Tampil` + kolom opsional `Barcode` (tambahan revamp tetap diterima). Aturan legacy: produk harus `barang`+`Pantau Stok`, minimal satu varian aktif, kode varian tak boleh sama dengan kode produk/varian milik produk lain/barcode lain, varian default tersembunyi diarsipkan (di R: `is_hidden` dipertahankan tapi `status='archived'` hanya bila tanpa riwayat), produk existing punya stok di varian default -> error.
- Gerbang: `Preview` mengembalikan `ok`, `summary {categories_total, products_total, *_to_create, *_to_update, variants_*, errors, warnings}`, `issues[{severity, sheet, row, column, message}]`, daftar baris (bentuk legacy `toPreview:1010-1050`, field camelCase R: `okToCommit`, `summary`, `issues`, `products[]`, ...). `Commit` menolak (400 + hitungan) bila ada error (gerbang seluruh-file seperti legacy), kecuali perilaku yang saya pertahankan: **tidak** memakai commit parsial lagi. Penulisan: kategori satu tx; produk chunk 200 per tx dengan fallback per-baris (port `runChunked`); varian satu tx; audit satu entri `products.bulk_import` dengan JSON ringkasan (`audit` perlu `after_json`; lihat F3b-8). Respons commit `imported{categories_created,...}`, `failures[]`.
- Performa: ganti N+1 dengan satu kueri `SELECT code,... FROM products WHERE code IN (...)` (chunk 1000) dan satu kueri kategori + varian; `CategoryByCode` batch. Volume: katalog nyata <= 5000 baris/file (batas 10 MB tetap; legacy 5 MB: naikkan tidak perlu, pertahankan 5 MB? R 10 MB, biarkan).
- Template: `GET /product-imports/template` menghasilkan workbook 5 sheet (Panduan, Kategori Produk, Daftar Produk, Varian Produk, Info Tambahan Produk) dengan header Indonesia + contoh + gaya header wajib/opsional, meniru `web/src/modules/products/components/bulk-upload-template.tsx` legacy (header di `CATEGORY_HEADERS`... baris 8-30; contoh baris 33-52; panduan baris 54+). excelize mendukung style/lebar kolom. Header Inggris lama R tetap dikenali sebagai alias (agar template R generasi lalu tetap bisa dipakai).
- Respons/FE: `ImportPage.tsx` menampilkan ringkasan per sheet, tabel issue (sheet,baris,kolom,pesan) dan tombol commit nonaktif bila `errors>0`.

### Test
Fixture: salin satu template legacy nyata (xlsx dan xls) ke `internal/modules/product/application/testdata/`; test: sheet alias, header alias, "10,000", "1.234,50", `Isi Jual per Satuan Stok`=4 -> faktor jual 0.25, kategori hierarkis induk-belakangan, siklus, upsert (update harga), gerbang (1 error -> 0 tertulis), UOM terkunci pada update (stok fake), `.xls`.

Dependensi: F3b-3 (guard `StockStateProvider`, kolom summary/attributes/is_hidden/sort_order, status inactive, tipe).

---

## F3b-5 Party: Informasi Tambahan, member pricing, kode otomatis

Ukuran: M.

### 5.1 Informasi Tambahan (customer & supplier)
- Migrasi `000037_party_parity.up.sql`: `ALTER TABLE parties ADD COLUMN attributes JSON NULL;`.
- Kontrak/DTO: `party/contracts/client.go Party.Attributes map[string]string` (kontrak publik hanya bila modul lain butuh; cetak faktur memakai? Legacy `toBusinessParty` menyertakan metadata ke UI cetak? tidak; kontrak tidak perlu dulu -> simpan di repo/handler saja). `party/presentation/handler.go` DTO `attributes` (map string->string, maks 20 kunci, kunci 1-60 char, nilai 1-255; legacy tak membatasi: batas hanya pengaman); `PartyInput.Attributes`; `repo` baca/tulis JSON (`json.Marshal`). Tidak menambah NPWP/NIK/termin/limit kredit sebagai kolom (bukan paritas); "NPWP" hanya jadi saran nama atribut di UI.
- UI: `CustomersPage/SuppliersPage` form: blok "Informasi Tambahan" (nama dari opsi saran: NPWP, Bank, Nama PIC, ...; allow create) meniru legacy.
- ETL: `metadata_json` -> `attributes` apa adanya.

### 5.2 Member pricing
- `domain/pricing.go`: ubah `ApplyRule(base float64, ...)`. Basis `purchase_price` di `Quote` (`service.go:801-806`): `base = float64(p.PurchasePrice) / p.PurchaseFactor * p.SalesFactor` (guard faktor <=0 -> error baris "Konversi satuan ... tidak valid"); selling/min tetap per satuan jual. Pembulatan akhir ke rupiah bulat dengan `math.Round` setelah pembulatan kelipatan (urutan legacy: hitung -> `applyRounding` kelipatan -> roundMoney 2 desimal; di R rupiah bulat: hitung -> kelipatan -> `Round`).
- Harga basis kosong: `base <= 0` -> baris galat "Produk 'X' belum memiliki harga jual normal|jual minimum|beli" (padanan legacy `requirePrice`). Catatan: R menyimpan 0 untuk "kosong"; ETL memetakan NULL legacy -> 0.
- Hasil negatif: sudah error ("harga member baris N negatif"); pertahankan.
- Persen: validasi legacy hanya `>= 0`. Ubah `validateMemberType`: `percent` boleh > 100 bila `direction=plus` (markup), `minus` dibatasi <= 100; batas atas teknis `<= 100000` (DECIMAL(18,4) aman). Ambang konfirmasi nominal 1.000.000 (`NominalConfirmThreshold`) dipertahankan sebagai pengaman UI (tidak ada di legacy; hanya berlaku saat membuat/ubah lewat UI, tak memengaruhi data ETL).
- Rounding: enum R (`none|round_100|round_500|round_1000|floor|ceil`) vs legacy (`none|nearest|up|down` + `rounding_increment` bebas). Karena `applyRounding` R memakai `step` bila > 0 (mode hanya menentukan default step & arah), peta lossless (Lampiran B): `nearest`->`round_100` + step = increment; `up`->`ceil`; `down`->`floor`; `none`->`none`, step 0. **Rekomendasi bersih**: ubah nilai enum R menjadi `none|nearest|up|down` (migrasi data `000037`: `UPDATE member_types SET rounding_mode = CASE rounding_mode WHEN 'round_100' THEN 'nearest' ... WHEN 'floor' THEN 'down' WHEN 'ceil' THEN 'up' END`), karena nama `round_100` dengan step 250 menyesatkan; UI memberi tombol cepat 100/500/1000. Direction `minus|plus` dipertahankan (peta ke `subtract|add`). Quote contract tetap string kode.
- Quote per-baris + batch: `QuoteLine.Error string`; `Quote` hanya 400 untuk request tak valid (customer tak ada, produk tak ada tetap per-baris). Konsumen: satu-satunya pemanggil kontrak adalah `sales/application/service.go:164`, yang sekarang membuang pesan galat (`"gagal menghitung harga member"`); ubah agar tiap `QuoteLine.Error` dilaporkan sebagai `fail("baris N: "+msg)` pada loop `:176-`. Handler `POST /pricing/quote` (POS) menampilkan `error` per baris di keranjang.
- N+1: tambahkan `ProductClient.PricingByIDs(ctx, ids) (map[int64]PricingInfo, error)` (satu kueri `WHERE id IN (...)`: id, status, selling/min/purchase, factors, code, name) dan pakai di `Quote`; hapus `GetByID` per baris.
- Status jenis member legacy `active|inactive|archived_at`: R `active|archived` (inactive dilebur). Legacy menolak menonaktifkan/mengarsipkan jenis member yang dipakai pelanggan aktif (`assertNotAssignedToActiveCustomers`); R membiarkan dan degradasi ke harga standar (`memberInactive`). Pertahankan R (lebih longgar tapi aman) — perilaku disengaja KI-72; catat sebagai penyimpangan dokumen.

### 5.3 Lain-lain party
- Kode otomatis: legacy `CUS-###`/`SUP-###` berurutan (maks+1, termasuk arsip); R `PLG-XXXXXXXX` acak (`service.go:130-150`). Paritas: ganti ke `CUS-###`/`SUP-###` dengan `SELECT MAX(CAST(SUBSTRING(code,5) AS UNSIGNED)) ... LIKE 'CUS-%'` (lock tidak diperlukan karena UNIQUE + retry 3x). Ukuran S. Prefix `PLG-` R lama untuk data uji tak masalah.
- Pencarian: legacy mencari juga `email` dan `address_text`; R hanya code/name/phone (`ListParties`). Tambah `email`, `address_text`. Daftar "Sampah" = `status=archived`: R sudah ada (restore setara).
- Legacy `customers/detail` menolak arsip; R `GET /customers/{id}` mengembalikan arsip (lebih longgar, benar untuk dokumen lama).

### Test
Unit domain `ApplyRule`: basis purchase dengan faktor (100.000/dus faktor 12, jual per pcs faktor 1 + plus 10% = 9.167); markup >100% plus; floor/ceil/nearest increment 250; basis kosong galat. Integrasi: customer attributes round trip; quote batch >50 baris (jumlah kueri tetap).

Dependensi: F3b-3 (`PricingByIDs`), `sales`/POS (pemanggil Quote).

---

## F3b-6 Branch: kolom, label lokasi default, status

Ukuran: S-M.
- Pelebaran + `default_stock_location_label` (lihat 3.5). Migrasi `000038_branch_parity.up.sql`: `ALTER TABLE branches MODIFY code VARCHAR(50) NOT NULL, MODIFY name VARCHAR(255) NOT NULL, MODIFY address TEXT NULL, MODIFY city VARCHAR(120) NULL, ADD COLUMN default_location_label VARCHAR(150) NOT NULL DEFAULT 'Gudang Utama';` (legacy default 'Default', ETL membawa nilai nyata).
- Kontrak `branch/contracts Branch.DefaultLocationLabel`. `branch/application/service.go:80-90,140-142`: batas kode berubah 10 -> 50 (`CreateBranch/UpdateBranch`) setelah keputusan lebar nomor dokumen (lihat 3.5). `branch/presentation/handler.go` DTO + `branchView`; UI form cabang.
- `stock/application/service.go ensureDefaults` (`:57-60`): nama lokasi sistem `default` = `branch.DefaultLocationLabel` (via `BranchClient.GetByID`, sudah dimiliki stock) dan rekonsiliasi: bila lokasi sistem `default` ada tetapi namanya berbeda dari label -> `UPDATE name` (satu write idempoten). Kode lokasi legacy `GDG-<kode cabang>` vs R `GDG` per cabang (unique per branch): ETL memetakan, tak ada perubahan kode.
- Legacy: perubahan status cabang menurunkan status ke lokasi default (`branch.service.ts:update`), R: lokasi tak mengikuti status; cabang inactive sudah diblok `RequireBranch`. Tidak dibawa.
- Legacy `reporting_preferences`: dead setting, tidak dibawa. `is_default` legacy branch vs R `is_head`: ETL: `is_default` -> `is_head`.
- Penomoran: dikerjakan F2a.
Test: create/update dengan label; stock rename lokasi default; kode 50 char.

---

## F3b-7 Company: setting yang berpengaruh

Ukuran: S.
- Profil: legacy `companies(code,name,legal_name,timezone,currency_code,locale,status)`; R `company_profile` lebih kaya (alamat/telepon/email/NPWP). `code`, `timezone`, `currency`, `locale` sengaja tak ada (ADR-0006/0009). ETL: `name`, `legal_name`; `address/phone` diisi dari `document_profile` (kop) bila ada, dan `tax_id` kosong.
- Padanan setting (tabel):
  | Legacy | Dibaca oleh | Padanan R | Tindakan |
  |---|---|---|---|
  | `ui_preferences.documentProfile` | `print-shared.tsx` | `company_settings['document_profile']` (JSON string) | ETL: `JSON.stringify(documentProfile)`; bentuk identik (`document-profile.ts`) |
  | `operational_rules.stockAdjustmentApprovalMode` simple/strict | `stock.service.ts:171,1168-1196` (approval WAJIB tiap koreksi, strict = approver != pemohon) | `stock_adjust_approval_threshold` (angka; approval hanya di atas ambang, `transactions.go:346-353`) | BELUM sepadan. Tambah key `stock_adjust_approval_mode` (`simple|strict`, default `simple`); `strict` -> approver harus bukan `actorID`; dan keputusan "approval wajib tiap koreksi (legacy) vs ambang (R)" ditandai sebagai Pertanyaan Terbuka. Titik ubah: `stock/application/transactions.go checkApprover` menerima `actorID`. Pemilik modul stock. |
  | `assistant_preferences.whatsapp_mode` | `whatsapp.service.ts getAssistantConfig` | `assistant` config (`assistant/application/service.go`) | cek F3b-9 |
  | `assistant_preferences.ai_daily_limit`, `mode=ai_assisted` | AI (OpenAI) | tidak ada | by-design dibuang |
  | `business_labels`, `operational_rules.{defaultOrderKind,defaultDueDays,criticalStockFocus}`, `ui_preferences.{compactTable,dashboardHighlight}`, `reporting_preferences` | tidak ada pembaca | - | tidak dibawa |
- Logo/kop: legacy tak punya unggahan logo (kop = teks `documentProfile`). Tidak ada gap.
- Perilaku `SaveSettings` (merge, KI-30) sudah lebih aman dari legacy (legacy `known-issues.md:715` menimpa kelompok).
- Validasi `document_profile`: tolak JSON invalid pada `PUT /company/settings` (R menerima string apa pun, `company/application/service.go:58-70`); tambah validasi per-key terdaftar (`document_profile` JSON objek, `stock_adjust_approval_*`). S.

---

## F3b-8 Audit: before/after, lintas cabang, tidak ditelan, retensi

Ukuran: L.

### Bukti
- R `audit_logs` (000017): `action_key, entity_type, entity_id, branch_id, actor_id, note VARCHAR(255), created_at`. Tanpa `before/after/metadata`, `actor_type`. Legacy `audit-log.entity.ts`: `actor_type, id_actor, before_json, after_json, metadata_json, happened_at`.
- Daftar: R `audit/infrastructure/repository.go:36-40` selalu `(branch_id = ? OR branch_id IS NULL)` untuk cabang sesi: admin tak bisa melihat cabang lain. Legacy `audit-log.service.ts:36-55` filter `id_branch` opsional (tanpa batasan cabang sesi), filter `action_key LIKE %..%`.
- Best-effort `_ =` : lihat hitungan di bagian `[PERLU DIPERLUAS]` di bawah.

### Rancangan
- Migrasi `000039_audit_detail.up.sql`: `ALTER TABLE audit_logs ADD COLUMN actor_type VARCHAR(20) NOT NULL DEFAULT 'user' AFTER branch_id, ADD COLUMN before_json JSON NULL, ADD COLUMN after_json JSON NULL, ADD COLUMN metadata_json JSON NULL, MODIFY note VARCHAR(500) NULL;`
- `audit/contracts.Entry`: tambah `ActorType string` (`user|system`), `Before, After, Metadata any` (di-marshal di `audit/application.Log`, batas 16 KB; lebih -> dipotong dengan penanda `{"_truncated":true}`). Kompatibel mundur: field baru opsional, 100 titik panggilan tak berubah.
- Daftar: `List(ctx, ListFilter{BranchID *int64, AllBranches bool, Action, Entity, ActorID, From, To})`; handler `GET /audit-logs?branchId=&entity=&action=&actorId=&from=&to=`; default tetap cabang sesi + global; `branchId=all` hanya bila sesi memiliki izin baru `audit.view_all` (lintas cabang) — **jangan** membuat izin baru: legacy memberi semua pemegang `audit_log.view` akses lintas cabang; paritas: izinkan `branchId` bebas bagi pemegang `audit.view`. Dikoreksi: ikuti legacy.
- Filter `action` pakai `LIKE`? Legacy `LIKE %x%`; R `=`. Pakai prefix-match `action_key LIKE 'x%'` (memakai indeks `ix_audit_logs_action`); persis legacy tidak dibutuhkan (penelusuran `product.%`).
- Respons tambah `before`, `after`, `metadata`, `actorType`, `actorName` (nama aktor diselesaikan di handler lewat `UserClient` batch `NamesByIDs`, tambahkan metode batch pada `UserClient`: `LabelsByIDs`; hindari N+1), `branchName` (BranchClient.GetByIDs batch). UI `AuditPage` (cek ada) tampil panel diff.
- Cakupan audit saat ini (grep `audit.Log` per modul; hasil dihitung ulang, bukan klaim audit): ~100 titik `_ = s.audit.Log` di 23 berkas: auth 5, branch 2, company 2, delivery 5, finance 16 (expenses 4, opening 1, posting 3, readiness 1, service 6, taxadjust 1), goodsreceipt 1, media 3, party 13, payment 3, product 6, purchasereturn 5, purchasing 4, sales 5, salesreturn 7, stock 15 (opening 1, service 6, transactions 8), user 8. Tanpa satu pun audit: **assistant** (7 operasi tulis, lihat F3b-9), dan **`stock.MoveLocation`** (`stock/application/movelocation.go`: komentar kepala berkata "audit trail" tetapi tidak ada `Log(`; transfer seketika memindah stok antar lokasi tanpa jejak), `stock.Hold/Release` hanya jejak kasar, impor produk menulis N audit `products.create` (satu per baris via `CreateProduct`) sedangkan legacy satu entri ringkas `product.bulk_import`, `company.SaveSettings` mencatat tanpa key/nilai (tak ada before/after), `user.SetRolePermissions` tanpa daftar kode (`Note` kosong) -> sengketa "siapa memberi izin apa" tak terjawab (legacy menyimpan `after: {permissions}`).
- Before/After minimal yang wajib diisi (setelah kolom JSON ada): `users.update` (before username/email/fullName/roleIds), `users.status`, `roles.permissions` (before/after daftar kode), `company.profile/settings` (key diubah), `products.update` (before/after harga, UOM, status), `member_types.update`, `branches.update`, `stock_adjustments.create` (qty sebelum/sesudah, approver), `fiscal_periods.*`, `payments.cancel`.
- Audit tak boleh ditelan untuk uang: `_ = s.audit.Log` pada operasi kas/pembayaran/jurnal/stok. Rancangan: dua metode: `Log` (best-effort, tetap) dan `LogTx(ctx, tx, Entry)` yang menulis ke tabel `audit_logs` DALAM transaksi pemanggil untuk modul yang sudah punya `*sql.Tx` terbuka (payment create/cancel, finance post/manual journal/reverse/period close, stock adjust/write-off/opening, delivery confirm, return confirm/settle, user role/permission). Karena modul tak boleh menyentuh tabel audit milik modul lain, `AuditClient.LogTx` meminta `tx` generik; **atau** pendekatan outbox: tulis ke `audit_logs` melalui `AuditClient` memakai koneksi sendiri tetapi sebelum commit (jaminan lebih lemah). Keputusan: sediakan `contracts.AuditTx interface{ ExecContext(...) }` agar audit modul menulis lewat `tx` pemanggil (database/sql `*sql.Tx` memenuhinya) — memenuhi aturan "transaksi satu modul" karena tx itu hanya dipinjamkan ke audit di dalam modul pemanggil dan tabel audit_logs tetap milik audit. Dicatat sebagai pengecualian terdokumentasi di BACKEND_GUIDE; perlu persetujuan arsitektur (Pertanyaan Terbuka 2).
- Retensi: tabel tumbuh tanpa batas. Rancangan: tombol/penjadwal `cmd/maintenance` perintah `purge-audit --older-than=730d --batch=5000` (hapus bertahap `DELETE ... LIMIT`), default 24 bulan, dan ekspor ke CSV sebelum hapus (opsional). Tidak ada Redis/cron baru: dijalankan manual/crontab host (sesuai aturan monorepo). Indeks: `(created_at)` untuk purge; serahkan ke agen schema/perf.
- ETL: legacy `audit_logs` -> `audit_logs` (`happened_at`->`created_at`, `before_json`...). Aksi legacy `actionKey` (mis. `user.create`, `stock.adjust`) berbeda dari revamp (`users.create`): jangan dipetakan ulang; simpan apa adanya (pencarian lama tetap menemukan).

---

## F3b-9 Assistant / WhatsApp

Ukuran: M.

### Perbandingan (dibaca sendiri)
Legacy `assistant.service.ts:365-382 detectIntent` (urutan, regex): help -> policy_qna -> operational_summary -> sales_summary -> pending_orders -> order_status -> critical_stock -> today_performance -> product_info -> help. 8 intent berfungsi + help. Revamp `assistant/application/engine.go:78-120 Detect` (substring, urutan): help -> `ORD-` nomor -> operational_summary -> today_performance -> sales_summary -> pending_orders -> order_status -> critical_stock -> product_info -> help. `policy_qna` memang dibuang (by-design RAG).

| Intent | Legacy | Revamp | Selisih (status) |
|---|---|---|---|
| help | `/help|help|bantuan|menu|fitur`, "apa saja ... fitur/bisa", "cara pakai" (hanya bila SELURUH pesan `help`-like atau frasa) | substring `/help, bantuan, cara pakai, mulai, menu, help` di mana pun | R terlalu longgar: "mulai"/"menu" di dalam kalimat lain ("menu makan") memicu help; "fitur" tak dikenali. Ringan. |
| operational_summary | `ringkasan|summary|kondisi bisnis|operasional` | `ringkasan|rekap|operasional` | R menambah "rekap", kehilangan "summary/kondisi bisnis". Samakan (union). |
| sales_summary | `penjualan|sales|omzet|revenue` SEBELUM performa | `omzet|omset|penjualan|laba|pendapatan|rugi` SESUDAH `hari ini` | **TERBUKTI beda perilaku**: legacy "Penjualan hari ini" (contoh pertama di teks bantuan) -> sales_summary; R -> today_performance (tes R `engine_test.go:19` memasukkan "omzet hari ini" -> performance sebagai ekspektasi; keputusan R yang tidak terdokumentasi). Pulihkan urutan legacy: sales sebelum performa; "hari ini" menjadi performa hanya bila tak ada kata penjualan. Ubah tes. |
| pending_orders | `pending|menunggu|belum diproses|belum selesai` | `pending, tertunda, menunggu, belum bayar/lunas, jatuh tempo, hutang, piutang, tagihan` | R lebih luas (tambahan piutang/hutang): pertahankan. |
| order_status | pesan menyebut `order|pesanan|transaksi` + kata status -> DAFTAR 5 order terbaru per status (draft/dikonfirmasi/diproses/selesai/dibatalkan) di rentang tanggal | cari satu order per NOMOR `ORD-...` atau "status/cek order/lacak" tanpa daftar | **Gap**: daftar-per-status hilang (status order konfigurable D4 dibuang by-design, tetapi 4 status tetap R: draft/confirmed/completed/cancelled). Tambah mode daftar: `ListOrders(statuses, from, to, limit 5)` yang sudah ada di `SalesOrderClient/PurchaseOrderClient` (dipakai `operationalSummary`). Mapping kata: draft->draft, dikonfirmasi/konfirmasi->confirmed, selesai->completed, batal/dibatalkan->cancelled; "diproses/in progress/aktif" -> confirmed (R tak punya in_progress). Tetap pertahankan pencarian per nomor (nilai tambah R). |
| critical_stock | `stok kritis|menipis|restock|tipis` | `kritis, menipis, habis, restock, kurang, "stok"` | R lebih luas; "kurang" terlalu umum ("kurang bayar"). Hapus "kurang". |
| today_performance | `performa|performance|hari ini` | `hari ini, performa, kinerja, bagaimana hari` | setelah perbaikan urutan sama. |
| product_info | pesan menyebut `produk|barang|item` di mana pun; query = teks dalam tanda kutip, atau pesan tanpa kata (`info|detail|status|stok|produk|barang|item`) | HANYA bila kata pertama = `info|produk|barang|harga|stok|cari` dan sisanya jadi query: "info produk beras" -> query "produk beras" -> tak ketemu; tanda kutip tak didukung; pencarian nama memakai `ListStocked` (hanya produk terlacak, maks 5000) sehingga jasa/non-stok tak pernah ketemu (`engine.go:469-486`); stok per cabang dihitung dari `finance.InventoryValue` seluruh cabang untuk SATU produk (`:487-502`, berat). | **TERBUKTI**. Perbaikan: `extractProductQuery` port legacy (kutip, buang kata kunci); tambah `ProductClient.Search(ctx, q string, limit int) ([]*Product, error)` (aktif saja, kode exact dulu baru nama; satu kueri) menggantikan `ListStocked`; stok per cabang dari kontrak stock baru `StockClient.ProductQty(ctx, branchIDs []int64, productID) (map[int64]float64, error)` (satu SUM per cabang, indeks `ix_stock_balances_product` dari F3b-3). |
| policy_qna | RAG | - | by-design |

Pairing/whitelist/rate-limit:
- Whitelist: legacy `whatsapp_authorizations` (phone_e164, display_name, access_level `owner|authorized_party`, status `active|revoked`, is_primary_owner, id_user); R `Authorization` sama (+ `FindActiveByPhone`). R juga memberi balasan pada nomor dicabut/tak dikenal (KI-137; legacy diam) — dipertahankan. ETL: `phone_e164`, `display_name`, `access_level`, `status`, `is_primary_owner`; `id_user` tak dibawa (R tak menyimpan; bila dibutuhkan audit "siapa", tambahkan kolom nullable `user_id` pada tabel otorisasi `[agen schema]`).
- Rate limit: legacy konstanta 10/menit per nomor (`whatsapp.service.ts:19-20`, in-memory, reset restart). R `allowRate` konfigurabel (`cfg.RateLimitPerMin`, default 10), jendela geser, in-memory — setara/lebih baik. Catatan: peta `s.rates` tak pernah dibersihkan untuk nomor yang berhenti mengirim (`service.go:223-239` hanya memangkas slice per nomor); batasi dengan `len(s.rates) > 10000` reset seperti limiter login (S).
- Pairing: legacy `whatsapp-channel.service.ts` (QR Baileys) vs R `assistant/infrastructure/gateway.go` (QR, `Connect/Disconnect/ResetSession`, `ChannelStatus`). Fungsional setara (13 endpoint). Path sesi WA disk = F1.
- Mode/konfigurasi: legacy `assistant_preferences.whatsapp_mode` + `behavior_preferences`; R `GetConfig/UpdateConfig(mode, rateLimit)`. AI by-design dibuang; `behavior_preferences` (legacy UI memakai? tidak ada pembaca di engine rule) tak dibawa.

### Audit endpoint channel/authorizations/config (legacy juga tidak ada; kewajiban baru)
`assistant/application/service.go` memiliki `Connect:258`, `Disconnect:283`, `ResetSession:290`, `CreateAuthorization:323`, `UpdateAuthorization:363`, `RevokeAuthorization:390`, `UpdateConfig:469` TANPA satu pun `audit.Log` (modul assistant tidak menerima `AuditClient`: `assistant.NewModule(db, storageDir, branches, products, ..., finance, log)` di `cmd/server/main.go`). Rancangan:
- `assistant.NewModule(..., audit auditcontracts.AuditClient)`; `Service` menyimpan `audit`; tiap metode di atas menerima `actorID` (`Connect/Disconnect/ResetSession/UpdateConfig` sekarang tanpa actor; handler meneruskan `actorID(r)`).
- Aksi: `assistant.channel_connect|channel_disconnect|channel_reset` (entity `assistant_channel`, `Note`=phone terhubung jika ada), `assistant.authorization_create|authorization_update|authorization_revoke` (entity `assistant_authorization`, EntityID=id, After={phone masked, accessLevel, status}, Before untuk update), `assistant.config_update` (Before/After {mode, rateLimit}). Nomor telepon di-mask di audit (4 digit terakhir) agar PII tidak tersalin ke log.
- Simulasi (`/assistant/simulate`) menulis `assistant_runs` dengan `actorID`; tak perlu audit tambahan.
- Test: unit fake AuditClient memastikan 7 aksi tercatat dengan actor; integrasi `RevokeAuthorization` menulis audit.

### Test
Perluas `engine_test.go` ke 40+ frasa (urutan legacy; "penjualan hari ini" -> sales_summary; "order draft" -> daftar order; `info produk "Beras Cap X"`; produk jasa ditemukan); fake `ProductClient.Search`.

Dependensi: F3b-3 (indeks stok), kontrak `StockClient.ProductQty` (modul stock), F1 (jalur sesi WA).

---

## Lampiran A. PETA IZIN legacy -> revamp (satu baris per kode legacy)

Aturan migrasi role (ETL): sebuah role revamp mendapat kode R bila role legacy memegang SALAH SATU kode sumber di kolom "Sumber legacy" (dibaca dari `role_permissions` produksi). Kolom "Aksi R" = kode baru/ubah yang harus ada di katalog (oleh F1).

| # | Kode legacy | Kode revamp (target) | Catatan |
|---|---|---|---|
| 1 | `dashboard.view` | `dashboard.view` | sama |
| 2 | `product.view` | `products.view` | juga memberi GET kategori |
| 3 | `product.create` | `products.create`, `product_import.manage` | legacy impor = `product.create` |
| 4 | `product.update` | `products.update`, `product_categories.manage` | legacy kategori create/update = create/update; media upload/hapus/utama = `product.update` |
| 5 | `product.archive` | `products.archive`, `product_categories.manage` | kategori archive = `product.archive` |
| 6 | `member_type.view` | `member_types.view` (BARU) | R menjaga GET dengan `manage` (perlu diubah) |
| 7 | `member_type.manage` | `member_types.manage` | juga implisit view |
| 8 | `order.view` | `sales.view`, `purchasing.view`, `customers.view`, `suppliers.view`, `delivery.view`, `goodsreceipt.view` | list/detail/status-history/work-queue/proof-url |
| 9 | `order.create` | `sales.create`, `purchasing.create`, `customers.create`, `suppliers.create`, `pricing.quote` | juga reservasi stok (R: hold/release -> `sales.create`) dan alamat kirim create |
| 10 | `order.update` | `sales.update`, `sales.approve`, `purchasing.update`, `customers.update`, `suppliers.update`, `delivery.create`, `goodsreceipt.create` | legacy: confirm/update-status/approve-credit/receive-goods/deliver-goods/deliveries create+confirm+archive+proof/alamat kirim update+archive/goods-receipts create semuanya `order.update` |
| 11 | `order.archive` | `sales.archive`, `purchasing.archive`, `customers.archive`, `suppliers.archive` | R `delivery.archive` (cancel SJ) dipetakan dari `order.update` (legacy cancel SJ = `order.update`), tidak dari archive |
| 12 | `order.export` | `sales.export`, `purchasing.export` (BARU) | R memakai `*.view` |
| 13 | `sales_return.view` | `salesreturn.view` | sama |
| 14 | `sales_return.create` | `salesreturn.create` | termasuk pengiriman pengganti + konfirmasi |
| 15 | `sales_return.refund` | `salesreturn.refund` (BARU) | settlement `refund` |
| 16 | `sales_return.cancel` | `salesreturn.archive` | legacy kode ada tetapi TIDAK ditegakkan (tak ada endpoint cancel); R cancel = `salesreturn.archive` |
| 17 | `purchase_return.view` | `purchasereturn.view` | |
| 18 | `purchase_return.create` | `purchasereturn.create` | |
| 19 | `purchase_return.refund` | `purchasereturn.refund` (BARU) | |
| 20 | `purchase_return.cancel` | `purchasereturn.archive` | idem 16 |
| 21 | `stock.view` | `stock.view` | |
| 22 | `stock.update` | `stock.update` (BARU; ganti `stock.manage` lokasi) | CRUD lokasi |
| 23 | `stock.transfer` | `stock.transfer` (BARU; ganti `stock.manage` transfer) | transfer + pindah lokasi |
| 24 | `stock.adjust` | `stock.adjust` | R juga memakai untuk saldo awal + rusak (ubah rute dari `stock.manage`) |
| 25 | `stock.adjust.approve` | `stock.approve` | izin penyetuju (kredensial) |
| 26 | `reporting.view` | `reporting.view` | |
| 27 | `user.view` | `users.view` | |
| 28 | `user.create` | `users.create` | |
| 29 | `user.update` | `users.update` | termasuk ganti sandi |
| 30 | `user.archive` | `users.archive` | |
| 31 | `company_config.view` | `company.view` | legacy juga order-status-definitions (by-design dibuang) |
| 32 | `company_config.manage` | `company.manage` | |
| 33 | `branch.view` | `branches.view` | |
| 34 | `branch.manage` | `branches.manage` | |
| 35 | `role.manage` | `roles.manage`, `roles.view` | |
| 36 | `knowledge.view` | (dibuang) | RAG by-design |
| 37 | `knowledge.create` | (dibuang) | |
| 38 | `knowledge.update` | (dibuang) | |
| 39 | `knowledge.archive` | (dibuang) | |
| 40 | `whatsapp.view` | `assistant.view` | |
| 41 | `whatsapp.manage` | `assistant.manage` | |
| 42 | `whatsapp.simulate` | `assistant.simulate` (BARU) | R `simulate` memakai `assistant.manage` |
| 43 | `audit_log.view` | `audit.view` | |
| 44 | `payment.view` | `payment.view` | |
| 45 | `payment.create` | `payment.create` | juga mencakup `settle-credit` dan bukti |
| 46 | `payment.archive` | `payment.archive` | |
| 47 | `finance.view` | `finance.view` | R: akun, mapping, periode, opening get |
| 48 | `finance.manage` | `finance.manage` | setup, saldo awal, periode, safe-close |
| 49 | `finance.close.force` | `finance.close.force` | rute `periods/close` |
| 50 | `finance.posting.view` | `finance.posting.view` | preview, posting-sources |
| 51 | `finance.posting.post` | `finance.posting.post` | post, batch, close-day, costing sync |
| 52 | `finance.journal.view` | `finance.journal.view` | |
| 53 | `finance.journal.reverse` | `finance.journal.reverse` | |
| 54 | `finance.report.view` | `finance.report.view` | laporan kas/hutang/piutang/margin/GL/TB/LR/NR + export paket |
| 55 | `finance.tax_report.view` | `finance.tax_report.view` | tax-summary/detail, tax-periods list |
| 56 | `finance.expense.view` | `finance.expense.view` | |
| 57 | `finance.expense.create` | `finance.expense.create` | |
| 58 | `finance.expense.cancel` | `finance.expense.cancel` | |
| 59 | `finance.tax_adjustment.view` | `finance.tax_adjustment.view` | |
| 60 | `finance.tax_adjustment.manage` | `finance.tax_adjustment.manage` | |
| (x) | `stock.create` (dihapus legacy 029) | - | abaikan jika ada baris sisa |

Kode revamp tanpa sumber legacy (hanya admin-set): `delivery.archive` (diwarisi `order.update`), `pricing.quote` (diwarisi `order.create`), `product_categories.manage`/`product_import.manage` (diwarisi, lihat 3-5), `roles.view`.
Peta role: `superadmin`(id 1, reserved, hanya user id 1) -> `superadmin` is_reserved; `owner` -> `owner` (R sudah ada; seed menyetel semua kode); `admin` -> `admin`; `staff` -> `staff`; role kustom (mis. `kasir`, legacy `roles.id_company` NOT NULL) -> role kustom `is_system=0`, kode dipertahankan; `description_text` -> `description`, `is_system_role` -> `is_system`. Id role/user/permission TIDAK dipertahankan: ETL membuat peta id (`role_id_map`, `user_id_map`).
Pengguna: `users.id_user` -> baru; `username`, `email`, `password_hash` (bcrypt legacy dibawa apa adanya: R memakai `bcrypt.CompareHashAndPassword` kompatibel dengan hash `$2a$/$2b$` bcryptjs cost 12), `full_name`, `phone_e164`->`phone`, `display_title`, `status`, `last_login_at`. `user_branch_accesses(id_user,id_branch,is_default_branch)` -> `user_branch_access`. Sesi legacy TIDAK dimigrasi (semua pengguna login ulang).

## Lampiran B. PETA ENUM / ID untuk ETL (area ini)

| Domain | Nilai legacy | Nilai revamp | Catatan |
|---|---|---|---|
| `member_types.price_basis` | `purchase_price`/`min_selling_price`/`selling_price` | identik | |
| `member_types.price_adjustment_direction` | `subtract` / `add` | `minus` / `plus` | |
| `member_types.price_adjustment_type` | `percent` / `nominal` | identik | |
| `member_types.rounding_mode` | `none` | `none` (step 0) | |
|  | `nearest` + increment I>0 | `nearest` (usulan F3b-5; sementara `round_100`) + step=I | I null/0 -> `none` |
|  | `up` + I | `up` (sementara `ceil`) + step=I | |
|  | `down` + I | `down` (sementara `floor`) + step=I | |
| `member_types.price_adjustment_value` | DECIMAL(18,4) >=0, persen >100 sah | sama; validasi R harus melonggarkan (F3b-5) | |
| `member_types.status` + `archived_at` | `active`/`inactive` + archived_at | `active` bila active & archived_at NULL; `archived` bila inactive ATAU archived_at tak NULL | inactive dilebur ke archived (R tak punya inactive); pelanggan yang menunjuknya tetap tersambung |
| `business_parties.party_type` | `customer`/`supplier` | identik | |
| party status | `archived_at IS NULL` -> aktif | `status='active'`; `archived_at` -> `status='archived'` | R juga mengisi `archived_at` |
| `business_parties.metadata_json` | JSON key-value | `parties.attributes` | |
| `phone_e164` | teks | `phone` (apa adanya) | |
| `products.product_type` | `physical` | `barang` | |
|  | `service` | `jasa` | |
|  | `bundle` | `paket` (usulan) / `jasa` jika usulan ditolak | |
|  | `non_stock` | `non_stok` (usulan) / `jasa` | |
| `products.status` + `archived_at` | `active` & NULL | `active` | |
|  | `inactive` & NULL | `inactive` (usulan) | |
|  | any & `archived_at` NOT NULL | `archived` | |
| `products.stock_tracked` | 0/1 | `stock_tracked` | non-physical selalu 0 |
| `products.purchase_price/selling_price/min_selling_price` | DECIMAL(18,2) NULL | BIGINT NOT NULL; NULL->0; nilai dibulatkan rupiah (migrasi legacy 043 sudah) | beda: tak lagi bisa membedakan "kosong" |
| `products.min_stock_qty` | DECIMAL NULL | `min_stock_qty` NOT NULL DEFAULT 0 | NULL->0 |
| `products.uom` (legacy kolom lama) | - | dibuang; pakai `base_uom` | |
| `products.has_variants` | 0/1 | tidak ada; turun dari ada-tidaknya varian non-hidden | |
| `product_variants` default | `is_default=1,is_hidden=1`, code=product_code, barcode=product_code | `is_default=1,is_hidden=1`, code=product_code, barcode=NULL | scan via product_code (F3b-3.2) |
| `product_variants` eksplisit | `is_default=0,is_hidden=0`, `status active/inactive`, `archived_at` | `status active/inactive/archived` (archived_at NOT NULL -> `archived`) | peta id varian `variant_id_map` WAJIB (stok/transaksi menunjuk id varian) |
| `product_variants.variant_code` | VARCHAR(80), unik hanya via service | `code` VARCHAR(80); ETL harus mendeteksi duplikat `(product,code)` dan barcode ganda (unique global R) | beri laporan konflik |
| `product_categories` | `id_parent_category`, `sort_order`, `archived_at` | `parent_id`, `sort_order`, `status` | archived_at -> archived; urutan impor induk dulu (topologis) |
| UOM / konversi | `base_uom, purchase_uom, sales_uom, purchase_to_base_factor (1 purchase = N base), sales_to_base_factor (1 sales = N base)` | sama arti (`ResolveUOM` mengembalikan factor base-per-unit) | kolom `uom` lama diabaikan; faktor DECIMAL(24,12) sama |
| `order.*` role perms | lihat Lampiran A | | |
| `users.status` | `active`/`inactive` | identik | |
| `user_sessions` | - | tidak dimigrasi | |
| `audit_logs.actor_type` | `user`/`system` (30) | `user`/`system` (20) | |
| `audit_logs.action_key` | mis. `user.create`, `stock.adjust`, `member_type.update` | dipertahankan apa adanya | |
| `media_files` | `owner_type='product'`, `purpose='product_image'`, `storage_driver/bucket/object_key/visibility/is_primary/sort_order/archived_at` | `owner_type='product'`, `file_key`=object_key, `is_primary`, `sort_order` (baru) | `archived_at` NOT NULL tak dibawa; bila driver S3 kunci tetap valid, jika lokal salin berkas |

## Lampiran C. Peta RUTE -> izin (perubahan yang diperlukan)

| Rute revamp | Sekarang | Seharusnya | Alasan |
|---|---|---|---|
| `GET /api/v1/member-types` | `member_types.manage` | `member_types.view` | dropdown POS/customer |
| `GET /api/v1/sales-orders/export` | `sales.view` | `sales.export` | legacy `order.export` |
| `GET /api/v1/purchase-orders/export` | `purchasing.view` | `purchasing.export` | idem |
| `POST /api/v1/sales-returns/{id}/settlements` (type=refund) | `salesreturn.create` | + cek `salesreturn.refund` | legacy `sales-return.service.ts:904` |
| `POST /api/v1/purchase-returns/{id}/settlements` (refund) | `purchasereturn.create` | + cek `purchasereturn.refund` | legacy `:569` |
| `POST /api/v1/deliveries/{id}/proof` | tanpa branchGuard | + `guarded(..., branchGuard)` | bug rute |
| `POST /api/v1/assistant/simulate` | `assistant.manage` | `assistant.simulate` | legacy `whatsapp.simulate` |
| `POST /api/v1/stock/locations`, `PUT .../{id}`, `.../archive` | `stock.manage` | `stock.update` | |
| `POST /api/v1/stock/reservations/hold|release` | `stock.manage` | `sales.create` | legacy `order.create` |
| `POST /api/v1/stock/transfers`, `move-location`, `{id}/dispatch|receive|cancel` | `stock.manage` | `stock.transfer` | |
| `POST /api/v1/stock/damaged/move-in|restore|write-off` | `stock.manage` | `stock.adjust` | |
| `GET /api/v1/stock/opening/template`, `POST opening/preview|commit` | `stock.manage` | `stock.adjust` | |
| `POST /api/v1/finance/periods/close` | `finance.manage` | `finance.close.force` | |
| `POST /api/v1/finance/periods/reopen`, `tax-periods/close|reopen`, `opening/*`, `account*`, `journals/manual`, `safe-close` | `finance.manage` | tetap `finance.manage` | legacy sama |
| `GET /api/v1/finance/posting/preview`, `posting-sources` | `finance.post` | `finance.posting.view` | |
| `POST /api/v1/finance/posting/post`, `post-batch`, `close-day`, `costing/sync` | `finance.post` | `finance.posting.post` | |
| `GET /api/v1/finance/journals`, `{id}` | `finance.view` | `finance.journal.view` | |
| `POST /api/v1/finance/journals/{id}/reverse` | `finance.manage` | `finance.journal.reverse` | |
| `GET /api/v1/finance/expenses`, `{id}` | `finance.view` | `finance.expense.view` | |
| `POST /api/v1/finance/expenses` | `finance.post` | `finance.expense.create` | |
| `POST /api/v1/finance/expenses/{id}/cancel` | `finance.post` | `finance.expense.cancel` | |
| `GET /finance/reports/{trial-balance,profit-loss,balance-sheet,general-ledger,inventory-value,margin,margin-by-product,receivables,payables,cash-summary,fiscal-summary}`, `GET /finance/export/tax-package` | `finance.view` | `finance.report.view` | |
| `GET /finance/reports/tax-summary`, `tax-detail`, `GET /finance/tax-periods` | `finance.view` | `finance.tax_report.view` | |
| `GET /finance/tax-adjustments` | `finance.view` | `finance.tax_adjustment.view` | |
| `POST /finance/tax-adjustments` | `finance.manage` | `finance.tax_adjustment.manage` | |

---

---

## Risiko (final)
1. Izin sesi = role aktif mengubah akses user multi-role; kumpulkan daftar user multi-role dari data produksi sebelum cutover dan umumkan.
2. Perombakan kode izin (finance 3 -> 14, stock 1 -> 3, 7 kode baru) menyentuh >60 rute di 8 modul; tanpa `TestRouteGuards` (registry rute) mudah ada rute terlepas. Jadikan test ini gerbang CI.
3. Migrasi data role/permission lama -> baru (peta Lampiran A) harus idempoten dan dapat diulang; kode lama `stock.manage`/`finance.post` dihapus hanya setelah role dipetakan.
4. Resize gambar tanpa encoder WebP: berkas keluaran JPEG/PNG lebih besar; CPU di request upload (batasi dengan semaphore). Alternatif WebP butuh cgo -> ditolak (Docker satu container).
5. `.xls` bergantung pustaka pihak ketiga (extrame/xls); fallback "simpan ulang .xlsx" disiapkan. Template lama pengguna adalah risiko adopsi utama; fixture nyata wajib.
6. Lebar kode cabang memengaruhi lebar nomor dokumen di 9 tabel (`VARCHAR(60)`).
7. `replaceVariants` + ubah kode produk berstok saat ini dapat menyembunyikan stok (varian ber-stok diarsipkan): perbaikan F3b-3 harus masuk sebelum data produksi dimigrasi/dipakai edit massal.
8. ETL produk: kode/barcode varian legacy tidak dijamin unik oleh DB legacy (hanya service) sedangkan R memakai unique barcode global dan `(product_id, code)`; ETL wajib menghasilkan laporan konflik, bukan gagal diam-diam.
9. ETL harga: NULL legacy -> 0 menghilangkan pembeda "belum diisi"; aturan "basis kosong = galat" memakai 0 sebagai kosong sehingga produk gratis (harga jual 0) berjenis member basis jual akan galat. Diterima.
10. `audit_logs` lebar (JSON before/after) menambah ukuran tabel; wajib retensi (purge) dan batas 16 KB per entri.

## Pertanyaan terbuka (hanya yang butuh keputusan pemilik)
1. Stok approval: legacy mewajibkan otorisasi penanggung jawab untuk SETIAP koreksi stok (mode `simple`/`strict` = approver wajib orang lain), revamp hanya di atas ambang (default 100 unit dasar, tanpa mode strict). Pulihkan "selalu wajib + mode strict" atau pertahankan ambang? (menyangkut kontrol internal, bukan sekadar tampilan).
2. Boleh tidaknya pengecualian arsitektur: modul lain menulis audit di DALAM transaksinya lewat `AuditClient.LogTx(tx, ...)` agar jejak operasi uang/stok tidak pernah hilang (tabel tetap milik audit)? Jika tidak, jejak uang tetap best-effort.
3. Nilai nyata `MAX(CHAR_LENGTH(code))` pada `branches` produksi (dan apakah ada cabang berkode >10): menentukan apakah lebar kode cabang + kolom `number` dokumen dilebarkan (VARCHAR 50/100) atau cukup 20.
4. Tipe produk `bundle` dan `non_stock`: dipertahankan sebagai nilai terpisah (`paket`, `non_stok`, rekomendasi saya) atau dilebur ke `jasa`? Perilaku keduanya identik; ini hanya soal label/laporan.

## Urutan & ukuran (saran implementasi)
1. F3b-2 sub-bagian rute (bug `proof` tanpa branchGuard, export, refund, member_types.view) = S, bisa langsung; lalu `TestRouteGuards`.
2. F3b-1 (login/role aktif/switch-role UI) = M. Tergantung F1 hanya pada file `auth/application/service.go` (koordinasi merge).
3. F3b-3.1/3.2/3.3 (wiring stok, scan, tabrakan kode) = M, prioritas tinggi (bug harian POS + risiko kehilangan stok varian).
4. F3b-5 member pricing (konversi UOM, per-baris error, batch) + attributes party = M.
5. F3b-3.4/3.5/3.6 (kolom, tipe/status, media resize) + F3b-6 branch = M.
6. F3b-4 impor produk = L (setelah 3.4 karena kolom/sheet baru).
7. F3b-2 katalog finance/stok penuh + role reserved = L (menunggu F1 untuk seed).
8. F3b-8 audit (kolom JSON, filter lintas cabang, retensi) = L; F3b-9 assistant = M; F3b-7 company = S.
Migrasi bernomor: 000032 login trail, 000033 roles.is_reserved, 000034 indeks stok, 000035 product parity, 000036 media meta, 000037 party parity (+enum rounding), 000038 branch parity, 000039 audit detail. Nomor boleh bergeser; urutan tidak boleh (000034 sebelum fitur stok-state aktif).
