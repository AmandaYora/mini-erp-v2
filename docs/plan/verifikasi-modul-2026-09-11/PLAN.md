# PLAN — Penyempurnaan Pasca-Verifikasi Modul

**Proyek:** mini-erp revamp (`E:/dimasprasetio/SAAS/mini-erp-revamp`)
**Sistem lama (pembanding):** `E:/dimasprasetio/SAAS/mini-erp` (NestJS + TypeORM)
**Tanggal:** 2026-09-11
**Basis:** verifikasi independen sesi ini — build, vet, test, integrasi (MySQL nyata), tsc, eslint,
vitest, arch-check, sapuan endpoint legacy↔revamp, dan pembacaan kode per modul.
**Bentuk:** 3 scope, **batas direktori saling lepas**, dirancang untuk dikerjakan **2 agent paralel**.

---

## 1. Hasil Verifikasi — Kondisi Nyata Hari Ini

Semua angka di bawah dihasilkan ulang sesi ini, bukan dikutip dari audit sebelumnya.

### 1.1 Kesehatan build — hijau seluruhnya

| Pemeriksaan | Perintah | Hasil |
|---|---|---|
| Kompilasi backend | `go build ./...` | ✅ |
| Analisis statis Go | `go vet ./...` | ✅ bersih |
| Unit test backend | `go test ./...` | ✅ 23 file, 57 `func Test` |
| **Integrasi backend** | `go test -count=1 ./internal/integration/...` | ✅ **19 test, 67 detik, MySQL nyata** |
| Tipe frontend | `tsc --noEmit` | ✅ |
| Lint frontend | `eslint "src/**/*.{ts,tsx}"` | ✅ **0 error**, 101 warning |
| Test frontend | `vitest run` | ✅ 11 file, 60 test |
| Guardrail arsitektur | `node scripts/arch-check.mjs` | ✅ A/B/C/D lolos |

Integrasi benar-benar menjalankan MySQL (bukan skip diam-diam): `TestFinancePostIdempotent`,
`TestDeliveryConfirmMovesStock`, `TestPaymentCreateAllocation`, `TestSettlementCap`,
`TestReturnPartialProRata`, dst. **Jalur uang end-to-end terverifikasi.**

### 1.2 Paritas kapabilitas — terkonfirmasi, dan lebih tinggi dari yang tercatat

Sapuan endpoint legacy↔revamp dijalankan ulang dengan skrip (legacy RPC-over-POST vs revamp REST,
jadi dibandingkan per-kapabilitas, bukan per method+path):

| Ukuran | Legacy | Revamp |
|---|---|---|
| Endpoint | 220 | **217** |
| Backend non-test | 37.218 baris TS | 32.835 baris Go |
| Frontend non-test | — | 37.864 baris TS/TSX |
| Halaman | 54 | **65 — seluruhnya terhubung rute** |

- **CRUD lengkap**: tidak ada resource yang kehilangan create/update/archive/restore.
- **Tidak ada `PlaceholderPage` yang tersisa** — 0 dari 42 item menu jatuh ke "Modul segera hadir".
- Sisa selisih endpoint = yang **sengaja dibuang** (knowledge/RAG, `company/features`,
  `order-status-*`) atau berganti bentuk (`cash-accounts` → flag `is_cash`,
  `opening/validate` → `opening/preview`, `whatsapp/*` → `assistant/*`).

### 1.3 Guardrail arsitektur — benar-benar ditegakkan, bukan slogan

- **Permission fail-closed nyata**: `RequirePermission` *panic saat wiring* bila permission kosong
  (`auth/contracts/session.go:126-141`) — rute tanpa deklarasi tidak mungkin lolos diam-diam.
- Hanya 7 rute yang sengaja di luar guard permission: `auth/login`, `auth/refresh` (publik) dan
  `logout`/`me`/`switch-branch`/`switch-role`/`branches/my-access` (login-only, memang by design).
- **Nol join lintas modul, nol FK lintas modul, nol `companyId`** — arch-check lolos.
- **Nol SQL string-concat** dari input pengguna; semua parameterized.
- Penomoran dokumen memakai idiom atomik MySQL `LAST_INSERT_ID(current_value + 1)`
  (`branch/infrastructure/repository.go:251-268`) — aman dari balapan.
- Unggahan media: whitelist ekstensi + `http.DetectContentType` + nama file UUID
  (`media/application/service.go:63-114`) — tidak ada path traversal.
- N+1 tren penjualan yang dicatat audit §5.2 **sudah diperbaiki** (`finance/application/trend.go` —
  3 query konstan, bukan 366).

### 1.4 Temuan — 1 bug nyata + 8 utang kualitas

Inilah yang benar-benar perlu dikerjakan. Semuanya diverifikasi dengan `file:baris`.

| # | Temuan | Tingkat | Bukti |
|---|---|---|---|
| **T1** | **Transfer stok tidak memeriksa kepemilikan cabang** | **P0 — keamanan** | `stock/application/transactions.go:75,89,138,173` |
| T2 | Frontend membuang `message` dari envelope backend | P1 — aturan proyek | `shared/services/http-client.ts` + 105 sisi panggil |
| T3 | Repo **tidak berada di bawah version control**, dan `storage/` (berisi sesi WA + kunci) belum di-ignore | P1 — prasyarat kerja paralel | `git rev-parse` → *not a git repository*; `.gitignore` tanpa `storage/` |
| T4 | 10 dari 20 modul backend nol unit test; E2E nol | P1 | `stock` 3.330 brs, `salesreturn` 1.889, `payment` 1.558, `user` 1.518, `purchasereturn` 1.362 |
| T5 | N+1 resolusi nama lintas modul | P2 — efisiensi | `sales:526`, `purchasing:448`, `payment:729,770` |
| T6 | 9 komponen UI bersama belum diport → duplikasi di halaman | P2 | `shared/components/ui/` |
| T7 | 89 warning `react-hooks/set-state-in-effect` | P2 | pola fetch-in-`useEffect` di ±47 file |
| T8 | `go.mod` minta `go 1.26.0`, toolchain lokal `go1.21.5` | P2 — reprodusibilitas | `apps/api/go.mod:3` |
| T9 | `.env` lokal masih `JWT_SECRET=change-me` | P2 — jebakan deploy | guard hanya menyala saat `APP_ENV=production` (`config:108`) |

#### T1 — rincian (satu-satunya bug fungsional yang ditemukan)

Empat endpoint transfer-per-ID **tidak pernah membandingkan cabang dokumen dengan cabang sesi**:

```
GET  /api/v1/stock/transfers/{id}           → routes.go:40
POST /api/v1/stock/transfers/{id}/dispatch  → routes.go:41
POST /api/v1/stock/transfers/{id}/receive   → routes.go:42
POST /api/v1/stock/transfers/{id}/cancel    → routes.go:43
```

`GetTransfer(ctx, id)` tidak menerima `branchID` sama sekali, dan `guarded` hanyalah
`RequireBranch` — ia memastikan *ada* cabang aktif, bukan cabang **yang mana**.

**Dampak:** pengguna di cabang B yang punya `stock.manage` dapat **mengirim, menerima, atau
membatalkan transfer milik cabang A** dengan menebak ID numerik — memindahkan stok cabang A tanpa
hak. `GET` membocorkan isi dokumen cabang lain.

**Ini melanggar aturan inti `CLAUDE.md`:** *"Session scope (`branchId`/`userId`/role/permission)
always comes from the authenticated session"*. Di sini cabang tidak dikonsultasikan sama sekali.

**Bukti bahwa ini anomali, bukan desain:** `stock` adalah **satu-satunya** dari 8 modul
berdokumen yang tidak melakukannya.

| Modul | `BranchID != branchID(r)` di handler |
|---|---|
| delivery, sales, purchasing, payment, purchasereturn, salesreturn, goodsreceipt | ✅ ada |
| **stock** | ❌ **nol** |

> **Catatan semantik yang wajib dihormati saat memperbaiki:** transfer antar-cabang itu sah.
> Aturan yang benar bukan "harus sama dengan cabang sesi", melainkan:
> `dispatch`/`cancel` → hanya `FromBranchID`; `receive` → hanya `ToBranchID`;
> `GET` → `FromBranchID` **atau** `ToBranchID`. Menyamaratakan jadi satu perbandingan akan
> **mematahkan transfer antar-cabang** — itu regresi, bukan perbaikan.

---

## 2. Prinsip Pembagian Scope

Syarat pemilik: **maksimal 3 scope, dapat dikerjakan 2 agent secara bersamaan.**

Pembagian di bawah memakai **batas direktori**, bukan batas fitur, supaya dua agent tidak pernah
menyentuh file yang sama:

| Scope | Direktori yang boleh disentuh | Agent |
|---|---|---|
| **A** | `apps/api/**` | Agent 1 |
| **B** | `apps/web/**` | Agent 2 |
| **C** | root repo + `docs/**` + `knowledge/**` | *prasyarat & penutup — bukan paralel* |

**Irisan file antara A dan B: nol.** Itu yang membuat paralel aman.

### Urutan jalannya

```
   C0 (git init)  ──►  ┌─ A (Agent 1, apps/api) ─┐  ──►  C1 (docs) ──► verify
     ~20 menit         └─ B (Agent 2, apps/web) ─┘
                          BERJALAN BERSAMAAN
```

`C0` wajib lebih dulu (T3): tanpa git, dua agent yang menulis bersamaan tidak bisa di-review,
di-merge, atau di-rollback. Ini prasyarat keselamatan, bukan formalitas.

### Kontrak antar-scope (dibekukan sebelum kerja dimulai)

Hanya ada **satu** titik singgung A↔B — T2. Bekukan bentuknya di muka supaya keduanya bisa jalan
tanpa saling menunggu:

> Backend **sudah** mengirim `message` di setiap respons sukses
> (`shared/response/response.go` — `OK`/`Created`/`Paginated`). **Scope A tidak perlu mengubah
> apa pun untuk ini.** Scope B cukup berhenti membuangnya di sisi klien. Kontraknya sudah
> berjalan hari ini — B hanya memakai yang sudah ada.

---

## 3. SCOPE A — Backend: perbaikan bug + jaring pengaman + efisiensi

**Agent 1 · hanya `apps/api/**` · tidak menyentuh `apps/web/`, `docs/`, root.**

### A1 — Perbaiki kebocoran otorisasi transfer stok  · **P0, kerjakan pertama**

1. Ubah tanda tangan menjadi `GetTransfer(ctx, branchID, id)`,
   `DispatchTransfer(ctx, actorID, branchID, id)`, `ReceiveTransfer(...)`, `CancelTransfer(...)`
   — `transactions.go:75,89,138,173`.
2. Terapkan aturan arah (lihat catatan semantik §1.4):

   | Operasi | Cabang sesi yang sah |
   |---|---|
   | `GET` | `FromBranchID` **atau** `ToBranchID` |
   | `dispatch`, `cancel` | `FromBranchID` saja |
   | `receive` | `ToBranchID` saja |

3. Gagal → `apperror.NotFound("Transfer")`, **bukan** `Forbidden` — jangan membocorkan
   keberadaan dokumen cabang lain lewat beda pesan.
4. Teruskan `branchID(r)` dari handler (`handler.go:371,452`).
5. Sapu modul `stock` untuk metode per-ID lain yang senasib sebelum menyatakan selesai.

**Test wajib** (`internal/integration/`, ikuti pola `movelocation_test.go`):
- cabang asing ditolak untuk keempat operasi;
- `receive` oleh cabang tujuan **berhasil** (menjaga transfer antar-cabang tetap hidup);
- `dispatch` oleh cabang tujuan ditolak.

> Test ketiga itu yang mencegah "perbaikan" berubah jadi regresi.

### A2 — Jaring pengaman untuk modul yang nol test

Urut berdasarkan risiko uang/stok, bukan ukuran file:

| Urutan | Modul | Fokus |
|---|---|---|
| 1 | `stock` (3.330 brs) | opname/saldo awal, reservasi + kedaluwarsa, adjustment, siklus transfer |
| 2 | `payment` (1.558 brs) | FIFO `fifo()`, tolak overpay, `settle-credit`, batal + hitung ulang |
| 3 | `salesreturn` (1.889 brs) | batas penyelesaian, mode tukar, SJ pengganti |
| 4 | `user` (1.518 brs) | resolusi permission, hapus role yang masih terpakai |
| 5 | `purchasereturn` (1.362 brs) | penyelesaian + `context`/`preview` |

Target: setiap modul punya minimal unit test domain + satu integrasi jalur utama.
**Jangan** kejar angka cakupan — kejar invarian (uang tidak bocor, stok tidak dobel, izin tidak
lolos).

### A3 — Hapus N+1 lintas modul

`sales:526`, `purchasing:448`, `payment:729,770` memanggil `GetByID` **per baris**.
Tidak ada metode batch di `contracts/` mana pun hari ini.

1. Tambah ke `party/contracts` dan `sales`/`purchasing` `contracts`:
   `NamesByIDs(ctx, ids []int64) (map[int64]string, error)`.
2. Ganti loop dengan satu panggilan batch.

Ini **menambah** `contracts/` — tetap patuh aturan modular monolith (justru itu gunanya
`contracts/`). Jangan tergoda membuat join lintas modul.

### A4 — Kunci toolchain Go

`go.mod:3` meminta `go 1.26.0`; `go version` lokal `go1.21.5`. Build hanya berhasil karena
toolchain 1.26 kebetulan sudah ter-cache di mesin ini — **mesin bersih atau CI offline akan gagal**.
`knowledge/BACKEND_GUIDE.md` sudah memperingatkan hal ini dan saat ini dilanggar.

Putuskan salah satu, lalu catat di `BACKEND_GUIDE.md` (lewat Scope C):
- turunkan `go.mod` ke versi yang benar-benar terpasang, **atau**
- naikkan toolchain resmi proyek dan pin `toolchain go1.26.x` secara eksplisit.

### A5 — Perkuat guard `JWT_SECRET` (T9)

`.env` lokal masih memakai `JWT_SECRET=change-me`. Penolakannya benar tapi hanya menyala saat
`APP_ENV=production` (`internal/config/config.go:108`) — satu deploy dengan `APP_ENV` lupa diisi
akan berjalan memakai secret yang tertulis di `.env.example` publik.

Perkuat: tolak nilai contoh **di semua env kecuali `development`**, dan tolak secret yang terlalu
pendek. Satu kondisi `if`, bukan proyek — ditaruh di Scope A karena `internal/config/` milik
`apps/api/`.

### Definisi selesai Scope A

```bash
cd apps/api && go build ./... && go vet ./... && go test ./... \
  && go test -count=1 ./internal/integration/...
```

---

## 4. SCOPE B — Frontend: kontrak envelope + konsolidasi UI

**Agent 2 · hanya `apps/web/**` · tidak menyentuh `apps/api/`, `docs/`, root.**

### B1 — Hentikan pembuangan `message` · **P1, kerjakan pertama**

`CLAUDE.md` menyatakan: *"`message` is never discarded by the frontend."* Hari ini dilanggar —
`apiPost`/`apiPut`/`apiPatch`/`apiDelete`/`apiUpload` mengembalikan `res.data.data` dan membuang
`message`, lalu **105 sisi panggil di 47 file** menuliskan ulang kalimatnya sendiri.

Akibatnya dua sumber kebenaran untuk teks yang dilihat pengguna: backend bilang
`"Transfer dikirim"`, frontend bilang sesuatu yang lain, dan keduanya bisa menyimpang tanpa ada
yang sadar.

1. Tambah varian yang mempertahankan envelope, mis.
   `apiPostFull<T>(): Promise<{ data: T; message: string }>` (biarkan `apiPost` apa adanya supaya
   migrasi bisa bertahap dan tidak mengubah 22 file service sekaligus).
2. Migrasikan sisi panggil tulis agar memakai `message` dari server.
3. Pertahankan teks hardcode **hanya** untuk aksi murni klien (mis. "Disalin ke papan klip").

> Mulai dari modul bernilai uang (finance, payment, stock, retur) — di situlah pesan server paling
> spesifik dan paling mahal bila menyimpang.

### B2 — Port 9 komponen bersama yang hilang

Belum ada di `shared/components/ui/`, sehingga perilakunya ditulis ulang per halaman:

`async-search-select` · `hierarchical-select` · `password-input` · `field-hint` · `select-field`
· `global-loader` · `party-search-select` · `product-search-select` · `stock-location-select`

Aturan yang tetap berlaku: **komponen bersama harus domain-agnostik.** Tiga varian
`party`/`product`/`stock-location` dibangun **di atas** `async-search-select` di dalam modulnya
masing-masing, bukan ditanam ke `shared/` dengan pengetahuan domain di dalamnya.

Gunakan skill `frontend-design` (diwajibkan `.claude/rules/frontend-react.md`).

### B3 — Tuntaskan 89 warning `set-state-in-effect`

Bukan sekadar kosmetik lint: polanya adalah `setLoading(true)` sinkron di dalam `useEffect`
(mis. `UsersPage.tsx:95`, `AppLayout.tsx:132`) yang memicu render beruntun di hampir setiap
halaman daftar.

Perbaiki di akarnya dengan satu hook `useAsyncData` di `shared/hooks/` (fetch + loading + error +
pembatalan saat unmount), lalu pakai ulang. Satu perbaikan, ±47 halaman ikut sembuh — sekaligus
menghapus duplikasi terbesar di frontend.

Sisa 12 warning `react-refresh/only-export-components` ditangani dengan memindahkan konstanta
non-komponen ke file sendiri (mis. `button.tsx:21`).

### B4 — Uji yang baru dikonsolidasi

Tambah test Vitest untuk `useAsyncData` dan komponen select baru. Naikkan dari 11 file;
prioritaskan yang dipakai banyak halaman.

### Definisi selesai Scope B

```bash
cd apps/web && npx tsc --noEmit && npm run lint && npm run test && npx vite build
```
Target: **0 error, 0 warning.**

---

## 5. SCOPE C — Prasyarat & penutup (bukan pekerjaan paralel)

### C0 — Inisialisasi version control · **WAJIB SEBELUM A & B MULAI**

`git rev-parse` → *fatal: not a git repository*. Sekitar **70.000 baris kode di dua aplikasi tidak
punya riwayat sama sekali** — tidak ada diff, tidak ada rollback, tidak ada review.

Dua agent yang menulis bersamaan tanpa git adalah kondisi yang tidak bisa dipulihkan bila salah.

1. `git init` di root repo.
2. `.gitignore` saat ini menutup `.env`, `node_modules/`, `dist/`, `apps/api/tmp/` — **tetapi
   belum menutup `apps/api/storage/`**. Tambahkan sebelum commit pertama.
3. **Ini bukan kehati-hatian teoretis.** `apps/api/storage/` saat ini berisi:
   - `whatsapp_session.db` — **sesi WhatsApp yang sudah terautentikasi**. Bocor = siapa pun bisa
     mengirim pesan sebagai nomor bisnis itu.
   - 4 foto produk yang diunggah pengguna.

   Keduanya data runtime, bukan kode. Verifikasi `git status` bersih dari keduanya **sebelum**
   `git add`, dan `.env` (berisi `JWT_SECRET` asli) juga tidak ikut.
4. Commit dasar: `chore: baseline mini-erp revamp sebelum penyempurnaan`.
5. Cabang per scope: `scope-a-backend`, `scope-b-frontend`.

> Commit pertama menyerap 70k baris sekaligus — itu tidak terhindarkan dan tidak apa-apa. Yang
> penting setiap perubahan **setelahnya** bisa dibaca sebagai diff.

### C1 — Selaraskan dokumen dengan kenyataan · **setelah A & B selesai**

Dokumen saat ini saling bertentangan dan beberapa menggambarkan masalah yang sudah diperbaiki.
Dokumen yang salah lebih berbahaya daripada tidak ada dokumen — ia membuat orang berikutnya
mengerjakan ulang hal yang sudah beres.

| Berkas | Yang tertulis | Kenyataan terverifikasi |
|---|---|---|
| `CLAUDE.md` | paritas ~75%, cakupan endpoint 84%, "Stage 1 selesai" | ±98%, 217 endpoint, Tahap A–J selesai |
| `PARITY_AUDIT.md` §1 | "Endpoint revamp 180" | **217** |
| `PARITY_AUDIT.md` §5.2 | N+1 tren penjualan terbuka | **sudah diperbaiki** (`trend.go`) |
| `PARITY_AUDIT.md` §10.3 | "eslint tidak menjangkau `src/`" | **sudah jalan** — 0 error |
| `PARITY_AUDIT.md` §1 | "File test: 9" | 23 backend + 11 frontend |
| `MODULE_MAP.md` | "±30k baris Go" | 32.835 baris (jumlah migrasi 26 — sudah benar) |
| `CLAUDE.md` | "38 migration files" | **26** (`000001`–`000030`, bernomor renggang) |

Tambahkan juga: keputusan toolchain A4, dan catatan bahwa `doclock` **hanya berlaku satu proses**
(sudah terdokumentasi di `shared/doclock`) — bila suatu hari di-scale horizontal, invarian uang
patah diam-diam. Itu batasan deployment yang pantas naik ke `DEPLOYMENT.md`, bukan terkubur di
komentar paket.

### C2 — E2E (opsional, keputusan pemilik)

Legacy punya 27 spec Playwright; revamp nol. Skill `webapp-testing` tersedia.
**Saran:** jangan kejar 27. Ambil 3 alur yang paling mahal bila rusak —
POS checkout → bayar, PO → terima → jurnal, SO → kirim → retur.

> Ini dipisah sebagai C2 karena butuh keputusan pemilik soal biaya perawatan, bukan karena
> kurang penting.

---

## 5b. Status Pelaksanaan (diverifikasi 2026-09-11)

| Butir | Status |
|---|---|
| A1 kebocoran otorisasi transfer | ✅ selesai — aturan arah benar, `NotFound` bukan `Forbidden`, dijaga `transfer_scope_test.go` termasuk kasus *receive di cabang tujuan berhasil*. Sapuan A1.5 ikut menemukan `TestLocationBranchScope` |
| A2 jaring pengaman | ✅ selesai — ditempuh lewat integrasi (bukan unit test per modul): 23 → 31 file, integrasi 19 → 35 test |
| A3 N+1 | ✅ selesai — `NamesByIDs` (party) + `OrderParties`/`PaidForOrders` (payment), dijaga `batch_reads_test.go` |
| A4 toolchain | ✅ selesai — `toolchain go1.26.7` di-pin |
| A5 rahasia JWT | ✅ selesai — tolak nilai contoh di luar `development` + minimum 32 karakter |
| B1 envelope | ✅ **selesai** — 98 sisi panggil memakai pesan server; 8 sisa memang bukan tulis-server (unduhan `void`, agregat batch, pesan komposit POS) |
| B2 komponen bersama | ✅ selesai — 6 generik di `shared/ui`, 3 varian domain di modulnya |
| B3 render beruntun | ✅ selesai — `useAsyncData` (65 file), eslint 101 → **0** |
| B4 uji frontend | ✅ selesai — 11 → 17 file, 60 → 85 test |
| **C0 git init** | ⏸️ **ditunda atas keputusan pemilik** sampai repo target disiapkan |
| C1 selaraskan dokumen | ✅ selesai — `CLAUDE.md`, `PARITY_AUDIT.md` §1/§5.1/§5.2/§10.3 + §11 baru |
| C2 E2E | ⏸️ menunggu keputusan pemilik |

**Temuan tambahan saat verifikasi, sudah dikunci:** `React.act` hanya ada di
`react.development.js`, sedangkan `react/index.js` memilih build dari `process.env.NODE_ENV`.
Menjalankan test dengan `NODE_ENV=production` (lazim di CI dan Docker build) membuat seluruh test
yang me-render komponen gagal dengan *"React.act is not a function"* — bukan karena kodenya salah.
Dikunci lewat `env: { NODE_ENV: "development" }` di `apps/web/vitest.config.ts`, diverifikasi
lulus 85/85 dengan `NODE_ENV=production`.

### Gerbang mutu setelah seluruh pekerjaan

`go build` ✅ · `go vet` ✅ · `go test` ✅ (73 test) · integrasi ✅ 35 test / MySQL nyata ·
`tsc` ✅ · `eslint` ✅ **0 masalah** · `vitest` ✅ 17 file / 85 test · `vite build` ✅ ·
`arch-check` ✅

---

## 6. Ringkasan Penugasan

| Scope | Agent | Direktori | Isi | Blokir |
|---|---|---|---|---|
| **C0** | siapa saja | root | git init | **memblokir A & B** |
| **A** | Agent 1 | `apps/api/**` | A1 bug P0 · A2 test · A3 N+1 · A4 toolchain · A5 secret | butuh C0 |
| **B** | Agent 2 | `apps/web/**` | B1 envelope · B2 komponen · B3 hook · B4 test | butuh C0 |
| **C1** | siapa saja | `docs/`, `knowledge/` | selaraskan dokumen | butuh A & B |
| **C2** | — | `apps/e2e/` (baru) | 3 alur E2E | keputusan pemilik |

### Verifikasi akhir (setelah merge A + B)

```bash
npm run verify   # go build + vet + test + tsc + lint + vitest + arch-check
cd apps/api && go test -count=1 ./internal/integration/...
```

---

## 7. Yang Sengaja **Tidak** Masuk Rencana Ini

Supaya scope tidak melar tanpa alasan:

| Tidak dikerjakan | Alasan |
|---|---|
| Rewrite modul `finance` | Terverifikasi paling sehat di repo — posting idempoten, preview tak bisa meleset, 3 bug uang sudah tertangkap & ada test-nya |
| Mengganti `doclock` dengan lock DB | Benar untuk satu container (deployment saat ini). Ganti hanya bila benar-benar multi-instance |
| Tabel `finance_*` yang dibuang | Keputusan terkunci `DB_SCHEMA.md` §6 — kapabilitasnya sudah tertutup jalur lain |
| Menaikkan cakupan test ke level legacy (1.254 `it`) | Legacy menguji banyak hal dangkal. Revamp menguji invarian di integrasi — arah yang lebih baik. Tutup celah nyata (A2), bukan kejar angka |
| Duplikasi `lineEconomics`/`prorate` antar modul | Disengaja per standar monorepo (tercatat di PLAN sebelumnya §5.3) |
| Riwayat status order, PWA | Dibuang by-design (D4) |

---

## 8. Penilaian Jujur

Pemilik meminta revamp yang **setara kapabilitas, sama UI, tapi lebih ideal, optimal, efisien, dan
maintainable.** Berdasarkan verifikasi sesi ini:

- **Setara kapabilitas** — ✅ tercapai. 217 endpoint, 65 halaman, nol placeholder, CRUD utuh.
- **UI sama** — ✅ tercapai. Token desain diport identik (diverifikasi `diff`, nol selisih).
- **Lebih optimal & efisien** — ✅ sebagian besar. Uang sebagai integer, sqlc di 19/20 modul,
  N+1 tren sudah mati, ekspor streaming di server. Sisa: T5 (N+1 lintas modul), T7 (render beruntun).
- **Lebih maintainable** — ✅ struktur, ⚠️ jaring pengaman. Batas modul benar-benar ditegakkan
  compiler + arch-check; tapi 10 modul nol unit test (T4) dan tidak ada version control (T3).

**Yang benar-benar mendesak hanya satu: T1.** Itu lubang otorisasi yang bisa memindahkan stok
cabang lain, dan perbaikannya kecil. Sisanya adalah utang kualitas yang nyata tapi tidak
mendarurat — kecuali T3, yang harus dibereskan sebelum dua agent menulis bersamaan.

**Revamp ini dalam kondisi jauh lebih baik daripada yang digambarkan `CLAUDE.md`.** Yang paling
usang di repo ini bukan kodenya, melainkan dokumen yang menilai kodenya.
