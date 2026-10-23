# PLAN — Redesain UI/UX mini-erp ("DS v2 · Bingkai Navy")

**Proyek:** mini-erp revamp — frontend `apps/web` (React 19 + Tailwind 4)
**Tanggal:** 2026-10-16
**Status:** sebagian besar sudah dieksekusi — lihat "Status eksekusi" dan "Pembaruan eksekusi 2026-10-21" di bawah untuk yang selesai dan yang tersisa.
**Keputusan pemilik 2026-10-16** (sudah dimasukkan ke seluruh dokumen; ringkasan di §12):
- brand tetap **navy**, sidebar tetap **gelap navy** tetapi dimodernkan
- primitif **Radix**
- tema gelap **ditunda**
- penempatan alur diserahkan ke analis (U9 berlaku)
- kasir memakai **tablet layar sentuh, landscape, pemindai = kamera tablet**; tablet hanya
  untuk POS
- logo resmi menyusul di akhir (U7-7)
**Lingkup:** seluruh tampilan di 22 folder modul frontend (±70 halaman + modal), kerangka aplikasi,
halaman auth, dan mode kasir POS. **Tidak** mengubah perilaku bisnis, endpoint API, kunci izin,
atau hasil cetak di kertas.
**Basis analisis:**
- inventaris halaman per modul (dibaca langsung dari `apps/web/src`, lihat §1)
- screenshot halaman login yang sedang berjalan
- riset pola dashboard modern per Oktober 2026 (sumber di Lampiran D)

---

## Status eksekusi (2026-10-16)

**Selesai dan terverifikasi** (`npm run verify:fast` hijau; vitest 611 test; e2e 47 lulus termasuk audit axe
semua rute menu tanpa pelanggaran serious/critical; kontras warna lolos):
Fase 0 (baseline 53 screenshot, `scripts/ui-ratchet.mjs`), Fase 1 (token navy, preflight, font lokal, 40 primitif),
Fase 2 (fasad + pola baru), Fase 3 (AppShell, command palette, auth Indonesia), Fase 4 (semua modul, rute baru:
`/deliveries/new`, `/sales-returns/new`, `/purchase-returns/new`, `/goods-receipts/new`, `/stock/transfers/new`,
`/finance/journals/new`, `/finance/reports/cash`), Fase 5 (POS Mode Kasir, kecuali U5-5), Fase 7 sebagian
(U7-1 axe, U7-4 bundle, U7-5 dokumen: ADR-0017 + `FRONTEND_GUIDE.md`, U7-6 screenshot di `dist/visual/after`).
Ratchet UI: ukuran arbitrer 242→1, `Memuat…` manual 58→1, input mentah 30→0, `<Link><Button>` 6→0, hex 5→0.

**Belum / perlu tindakan pemilik:**
- **U5-5** uji tablet sungguhan (pindai 20 barcode, HTTPS kamera, cetak struk Android, E02 `POS_TABLET=1`).
- **U7-7** logo resmi (menyusul dari pemilik).
- **Fase 6** tersisa: command palette "lompat ke dokumen" dan saved view per daftar (kepadatan tabel sudah ada).
- **U7-2/U7-3**: uji keyboard-only dan responsif 390/768 hanya sebagian (screenshot mobile 5 rute; `LineItemsEditor` di layar
  sempit hanya menggulir horizontal, belum mode kartu).
- Tema gelap tetap ditunda (U5).

**Penyimpangan yang disengaja dari rencana:**
1. **TanStack Table tidak dipakai.** Sortir/paginasi berjalan di server; `DataTable` mengimplementasikan sendiri
   header sticky, sortir header, seleksi + aksi massal, pilihan kolom (localStorage), loading/galat/klik baris.
2. **Combobox (U2-5):** logika dalam `SearchSelect`/`AsyncSearchSelect`/`HierarchicalSelect` dipertahankan (teruji);
   yang diganti adalah cangkangnya menjadi Popover Radix (`select-popover.tsx`) agar bisa hidup di dalam Dialog.
3. **`ConfirmDialog` kini `role=alertdialog`**; sheet dan dialog menutup dengan label "Tutup"/"Tutup panel".
4. **Ratchet** hanya mengukur kode fitur (bukan `shared/components`, `shared/layouts`, `primitives`, `print`).
5. **Tile POS** tanpa stok (tanpa endpoint baru, U11). *Catatan 2026-10-23:* foto **sudah** tampil (temuan pengguna 4;
   API daftar produk ternyata mengirim `primaryImageUrl`, cadangan inisial bila tidak ada/gagal); popover diskon tanpa
   kolom "alasan" (tidak ada di model keranjang); tanpa pintasan F-key (POS lama tidak punya).
6. **Rel Dokumen** mengikuti status nyata backend (mis. retur: Draf → Barang kembali → Diselesaikan) dan hanya menampilkan
   dokumen terkait yang sudah ada di data halaman.
7. **Pohon akun** dua tingkat berdasarkan tipe akun (API tak punya induk); kolom "Keterangan" jurnal manual berisi info turunan
   (API hanya menerima memo tingkat dokumen).
8. **Koreksi stok** mempertahankan mode set/in/out lama (sempat dihapus agen, dikembalikan).

**Bug lama yang ditemukan dan diperbaiki sepanjang jalan:** Neraca crash saat pembukuan kosong (backend mengirim `null`;
kini `?? []` dan `DataTable` menerima `null`); `CriticalItem` (dashboard stok kritis) tanpa tag JSON sehingga API
mengirim PascalCase dan UI menampilkan `undefined` (ditambah tag `json:"…"` di `stock/contracts/client.go`).

### Pembaruan eksekusi 2026-10-21

**Selesai:** Fase 6 seluruhnya dan bagian U7-2/U7-3 yang bisa dikerjakan tanpa perangkat fisik.
- **U6-1** kepadatan: sudah ada sebelumnya (menu akun).
- **U6-2** lompat ke dokumen: `app/routes/document-search.ts` (dimuat malas) memakai endpoint daftar ber-`search` untuk
  SO/PO (`ORD-…/PJ|PB`), `PAY-`, `SJ-`, `RTR-`, `RTB-`. GR- dan TRF- dilewati karena daftarnya tidak punya `search` (U11).
  Label "Terakhir dibuka" kini memakai nomor dokumen.
- **U6-3** tampilan tersimpan: `SavedViews` (patterns) + `shared/lib/list-views.ts`, dipasang di 15 daftar ber-filter
  URL. Daftar itu kini juga punya pilihan kolom (`columnsKey`). Kolom, tampilan, dan "terakhir dibuka" disimpan per pengguna.
- **U6-4** audit token: `ui-ratchet` kini juga menolak warna mentah di seluruh `src` (nol toleransi). `text-white`
  dan bayangan `rgba()` diganti token (`--destructive-foreground`, `--shadow-glow`, `--shadow-pop|md`).
- **U7-2** keyboard: spec e2e `E15` (SO → SJ → bayar, PO → terima, posting tutup hari, Ctrl+K lompat dokumen) tanpa
  `.click()`. Ditemukan dan diperbaiki sepanjang jalan:
  - `ConfirmDialog` tidak memindahkan fokus ke dalam dialog (fokus tertinggal di belakang overlay);
  - pemilih tidak bisa dipilih dengan Enter/panah, dan Enter memilih dari daftar basi;
  - Enter di pemilih menambah baris kosong di `LineItemsEditor` (event portal merambat);
  - opsi `SearchSelect` tanpa `role="option"`;
  - lima pemilih di baris barang tanpa nama aksesibel.
- **U7-3** responsif: spec e2e `V02` memeriksa gulir menyamping halaman di beberapa ukuran:
  - semua rute menu (dari registry) + form/detail pada 390 dan 768, dengan data nyata;
  - 7 template pada 390/768/1366/1920;
  - POS pada 1024/1280/1340.

  `LineItemsEditor` berganti ke mode kartu bila wadah < 600px. Ditemukan dan diperbaiki:
  - dasbor melebar 268px di 390 bila tabel "Produk bulan ini" berisi (grid tanpa kolom dasar `grid-cols-1`; pola yang
    sama dirapikan di ProductForm dan Perusahaan);
  - slot kanan `Toolbar` tidak membungkus;
  - teks 404 dengan path panjang tidak membungkus.
- Anggaran bundle (U7-4): chunk awal naik 0,6 KB gzip.

**Masih menunggu pemilik / perangkat:** U5-5 dan bagian "uji sentuh POS di tablet" U7-2 (tablet kasir sungguhan),
U7-7 logo resmi, tema gelap (ditunda).

---

## 0. Ringkasan untuk pengambil keputusan

Desain sekarang dinilai kuno, berantakan, dan tidak nyaman. Penyebabnya **bukan** warna yang
berserakan: token warna sudah terpusat, dan tidak ada satu pun warna palet Tailwind yang
di-hardcode di modul. Masalahnya ada di empat tempat:

1. **Kerangka terasa lama.** Sidebar hampir hitam yang menempel rata ke konten, chip
   "Perusahaan/Cabang" di topbar, gradient pada logo dan avatar, dan dua judul `<h1>` di setiap
   halaman. Warna navy-nya sendiri bukan masalah: yang kuno adalah eksekusinya.
2. **Tidak ada primitif modern.** Belum ada popover, command palette, sheet/panel samping, tabs,
   checkbox, tooltip, dan skeleton. Akibatnya tiap halaman merakitnya sendiri, dengan gaya yang
   berbeda-beda.
3. **Komposisi halaman tidak konsisten.**
   - Tombol aksi ada di 3 tempat berbeda.
   - Tombol "kembali" punya 4 gaya.
   - Alur "buat baru" kadang berupa halaman, kadang modal xl.
   - Halaman detail berupa tumpukan kartu satu kolom.
4. **Kepadatan informasi tidak dikelola.**
   - Dashboard menampilkan 17 kartu angka.
   - Filter daftar berisi 8 kontrol yang selalu terbuka.
   - POS masih tampil di dalam sidebar, bukan layar kasir.

**Usulan.** Pindah ke **Design System v2 "Bingkai Navy"**, dibangun di atas **shadcn/ui**
(pondasi yang sama dengan dashboard buatan Lovable), lalu dibedakan dengan tiga hal:

- warna brand **navy** yang dipertahankan, menjadi "bingkai": sidebar navy gelap varian *inset*
  memeluk kanvas konten putih. Semua pasangan warna divalidasi kontras WCAG AA
- pasangan huruf **Plus Jakarta Sans** (karya desainer Indonesia) dengan **Inter**
- satu elemen khas: **Rel Dokumen**, yang menampilkan rantai dokumen PO → Penerimaan → Retur →
  Pembayaran di setiap halaman dokumen

**Strategi eksekusi: "ganti mesin di balik fasad".** Komponen `shared/components/ui/*`
dipertahankan nama dan props-nya, tetapi isinya diganti dengan primitif shadcn. Dengan begitu
seluruh ±70 halaman langsung berganti rupa tanpa disentuh satu per satu. Setelah itu tiap modul
dirapikan ke template halaman baku, gelombang demi gelombang. Totalnya 8 fase (§9); setiap fase
bisa dirilis sendiri.

---

## 1. Diagnosis — kenapa terasa kuno dan berantakan

Yang sudah benar perlu dicatat dulu, karena inilah yang membuat redesain ini murah:

- **Token warna terpusat** di `src/theme/theme.css`. Tidak ada `bg-blue-600` di modul mana pun.
  Hex hanya muncul di halaman cetak, dan itu wajar.
- **`DataTable` sudah dipakai** di semua daftar. `<table>` mentah hanya ada di `AuditPage` dan
  halaman cetak.
- **Test vitest berbasis role/teks** (`getByText` 39, `getByRole` 27, `querySelector` hanya 2).
  Mengganti markup aman selama accessible name dipertahankan.
- **Menu dan izin terpusat** di `src/app/routes/registry.tsx`.

| # | Akar masalah | Bukti |
|---|---|---|
| R1 | **Kerangka gaya lama.** Sidebar hampir hitam `bg-sidebar #0c1626` yang menempel rata ke konten, teks abu redup, garis aktif `#6aa3dd` hardcode; gradient + hex lokal pada logo dan avatar; chip Perusahaan/Cabang memakan topbar | `shared/layouts/AppLayout.tsx:326-341`, `:120` |
| R2 | **Dua `<h1>` per halaman.** Judul topbar dan `PageHeader` sering berteks sama | `AppLayout.tsx:470`, `shared/components/ui/page-header.tsx:19` |
| R3 | **Preflight Tailwind dimatikan.** Diganti reset manual, sehingga ekosistem shadcn tidak bisa dipakai apa adanya | `theme/theme.css:5-7,70` |
| R4 | **Tidak ada skala tipografi.** Ukuran arbitrer (`text-[0.8125rem]`, `[0.6875rem]`, `rounded-[10px]`) tersebar di mana-mana | `AppLayout.tsx`, halaman auth, halaman detail |
| R5 | **Primitif yang hilang dirakit ulang di tiap halaman** (rincian di bawah tabel) | — |
| R6 | **State tidak dirancang.** Sekitar 90 teks `Memuat…` dengan 2 gaya, tanpa skeleton; sekitar 40 "Notice + tombol Coba lagi" yang ditulis manual | seluruh modul |
| R7 | **Penempatan alur tidak konsisten** (rincian di bawah tabel) | — |
| R8 | **Kepadatan tidak dikelola.** Dashboard 17 `SummaryCard` dalam 5 baris; filter SO/PO 8 kontrol; sortir lewat dua select "Urutkan/Arah", bukan header kolom | `DashboardPage.tsx`, `SalesOrdersPage.tsx` |
| R9 | **POS bukan layar kasir.** Tampil di dalam sidebar sebagai grid 2 kolom; hasil produk berupa daftar teks; keranjang tidak sticky; hingga 3 Notice izin bertumpuk di atas area kerja | `pos/pages/PosPage.tsx:925` (1517 baris) |
| R10 | **Status bermakna salah.** `draft`/`pending`/`open` diberi nada *warning* (oranye) dan `cancelled` diberi *danger* (merah). Daftar penuh draf jadi terlihat seperti daftar masalah | `shared/lib/format.ts:53-79` |
| R11 | **Bahasa campur.** Login dan pilih cabang berbahasa Inggris ("Welcome back", "Sign In", "Select Branch"); `LoadingState` "Loading…"; eyebrow `PageHeader` ("Pembelian", "Master", "Laporan") tidak cocok dengan grup sidebar | `auth/pages/LoginPage.tsx:95,147` |
| R12 | **Navigasi kabur** (rincian di bawah tabel) | `registry.tsx` |
| R13 | **File raksasa.** Tampilan dan logika menyatu sehingga sulit dirapikan (rincian di bawah tabel) | — |

**R5 — primitif yang dirakit ulang:**
- Checkbox/radio mentah di Users, Roles, Posting, dan Balances.
- `<input type=file>` mentah di 5 tempat.
- `Segmented` dipakai sebagai tab di 5 halaman.
- 21 blok `<dl>` dengan 6 varian grid berbeda.
- Stepper `FlowSteps` lokal di `PurchaseOrderDetailPage.tsx:52`.

**R7 — penempatan alur:**
- Alur buat baru: SO/PO/Produk berupa halaman, sedangkan Retur, Surat Jalan, Penerimaan, Transfer,
  Pembayaran, dan Jurnal Manual berupa modal xl. `SalesReturnCreateModal` saja 824 baris.
- Aksi detail tersebar di `PageHeader`, kartu "Aksi Order", dan `ActionRow` di bawah.
- Tombol kembali punya 4 gaya.
- Ada 5 pola `<Link><Button>` bersarang.

**R12 — navigasi:**
- 8 ikon dipakai ganda: Boxes ×3, Scale ×3, FileSpreadsheet ×3, Cog, Building2, Stethoscope,
  ClipboardList, ClipboardCheck.
- Label "Saldo Awal" ganda.
- Judul topbar jatuh ke "Halaman" untuk `tax-detail`, `inventory-value`, dan `margin`.
- Tab `ReportsPage` tidak tersinkron ke URL, sehingga sorotan sidebar salah.

**R13 — file raksasa:**
- Di atas 1000 baris: `PosPage` 1517, `ReportsPage` 1305, `SalesOrderFormPage` 1062.
- 600–1000 baris: 10 file lain.

**Kesimpulan diagnosis:** masalahnya **sistemik**, bukan per halaman. Karena itu solusinya juga
harus sistemik: pondasi primitif, template halaman, dan aturan penempatan, baru kemudian
penerapan per modul.

---

## 2. Riset & arah desain

### 2.1 Apa yang diambil dari referensi

| Referensi | Yang diambil | Yang **tidak** diambil |
|---|---|---|
| **Lovable / shadcn/ui** (`dashboard-01`, `sidebar-07`) | Sistem token CSS (`background/foreground`, `card`, `muted`, `primary`, `border`, `ring`, `sidebar-*`, `chart-1..5`) dalam OKLCH; sidebar ber-varian *inset* yang bisa diciutkan ke ikon; header berisi breadcrumb; Data Table berbasis TanStack; Sheet; Command (cmdk); Sonner; Skeleton; Empty; Field; Kbd | Tema netral bawaan apa adanya. Ini yang membuat semua app Lovable terlihat sama |
| **Linear** (refresh 2025–26) | Chrome tenang: abu-abu kurang jenuh, sidebar diredupkan, border lembut, *tidak semua elemen sama berat*; keyboard-first (Ctrl+K) | Kontras abu-abu yang terlalu tipis (diukur ±3,6:1, gagal AA) |
| **Stripe Dashboard** | Halaman dokumen: header berisi nomor + status + aksi di kanan; dua kolom (isi kiri, metadata kanan); garis waktu aktivitas; angka uang rata kanan dengan `tabular-nums` | — |
| **NN/g & IBM Carbon** (tabel data) | Header sticky; pilihan kepadatan baris (32/36/44); pemilih kolom; chip filter aktif yang bisa dihapus; bar aksi massal; pagination, bukan infinite scroll; buka rekaman di **panel samping**, bukan baris yang membuka ke bawah | Ekspansi baris *accordion* (seperti `AuditPage` sekarang) |
| **Riset POS** | Layar penuh tanpa sidebar; target sentuh ≥ 44px (POS tablet: 48px); keranjang tetap di kanan dengan total besar; tombol utama di jangkauan ibu jari (sisi kanan bawah pada landscape); pindai kamera satu ketukan | Pintasan F-key sebagai jalur utama: tablet tanpa keyboard fisik, jadi F-key hanya bonus bila keyboard terpasang |

### 2.2 Prinsip desain DS v2

Setiap keputusan di dokumen ini harus bisa ditelusuri ke salah satu prinsip ini.

1. **Tenang dulu, tegas di tempat yang perlu.**
   - Kerangka (sidebar, header, border) berbobot rendah.
   - Warna kuat hanya untuk aksi utama, status, dan angka penting.
   - Dalam satu layar hanya ada **satu** tombol `primary`.
2. **Satu tempat untuk setiap hal.**
   - Aksi dokumen selalu di header halaman: aksi utama di kanan, sisanya di menu `⋯`.
   - Kembali selalu lewat breadcrumb.
   - Filter selalu di toolbar tabel.
3. **Angka yang jujur dan rata.**
   - Semua uang dan qty memakai `tabular-nums` dan rata kanan.
   - Format IDR terkunci (ADR-0005/0006).
   - Total selalu terlihat (sticky) saat mengisi baris.
4. **Status punya arti tetap.** Abu = belum jalan, Navy lembut = berjalan, Kuning = perlu perhatian,
   Hijau = selesai, Merah = masalah. Batal *bukan* masalah, jadi tampil abu bergaris.
5. **Keyboard-first di back-office, sentuh-first di POS.** Ctrl+K di mana saja; Enter/Tab di grid
   baris. POS dirancang untuk tablet layar sentuh: target 48px, tanpa hover, tanpa ketergantungan
   keyboard.
6. **State adalah desain.**
   - Loading memakai skeleton yang berbentuk seperti hasil akhirnya.
   - Kosong memakai ajakan bertindak, misalnya "Buat PO pertama".
   - Galat menampilkan `message` asli dari API ditambah tombol "Coba lagi". Ini menegakkan
     `knowledge/FRONTEND_GUIDE.md`, bagian "Umpan Balik ke Pengguna".

### 2.3 Yang sengaja dihindari

- **Glassmorphism dan blur dekoratif**, karena merusak kontras. Ini termasuk `backdrop-blur` di
  topbar sekarang.
- **Gradient dekoratif** di logo, avatar, dan hero login.
- **Bento grid sebagai hiasan.** Grid hanya dipakai bila mengikuti hierarki informasi.
- **Chart baru yang aneh-aneh.** Cukup line, bar, dan donut.
- **Chat bubble AI melayang.** Asisten WA tetap halaman pengaturan.

---

## 3. Keputusan terkunci

Semua keputusan di bawah **sudah dikonfirmasi pemilik pada 2026-10-16** (lihat §12). Yang ditandai
*(analis)* adalah keputusan yang diserahkan pemilik kepada perancang.

| # | Keputusan | Alasan |
|---|---|---|
| **U1** | **shadcn/ui sebagai pondasi**, dengan primitif **Radix** (`shadcn init -b radix`) — *dipilih pemilik* | (1) Sebagian besar contoh blok, Data Table, Sidebar, dan dokumentasi komunitas ditulis untuk Radix. (2) `cmdk` berbasis Radix. (3) shadcn tidak memaksa migrasi meski Base UI menjadi default sejak Juli 2026. Saat `shadcn add`, selalu pilih varian Radix |
| **U2** | **Preflight Tailwind dinyalakan.** Reset manual di `theme.css` dihapus | Syarat agar shadcn bekerja tanpa tambalan. Halaman cetak diverifikasi khusus (risiko K2, §11) |
| **U3** | **Fasad dipertahankan.** Modul tetap mengimpor dari `@/shared/components/ui`; primitif shadcn tinggal di `@/shared/components/primitives` dan **tidak boleh** diimpor modul (aturan ESLint `no-restricted-imports`) | 70 halaman berganti rupa sekaligus. Props lama tetap jalan. Shared tetap domain-agnostic (`.claude/rules/frontend-react.md`) |
| **U4** | **"Bingkai Navy": sidebar gelap navy, varian *inset*** — *dipilih pemilik*. Seluruh kerangka (sidebar + latar di sekeliling konten) navy gelap; konten berada di kanvas putih membulat yang "mengambang" di dalamnya | Identitas navy dipertahankan dan justru dijadikan ciri. Bedanya dengan sekarang: navy yang lebih kaya (bukan hampir hitam), teks sidebar kontras 8,95:1 (sekarang redup), item aktif berupa pil terang, tanpa gradient/hex lokal, dan kanvas inset yang memisahkan kerangka dari kerja |
| **U5** | **Tema gelap DITUNDA** — *keputusan pemilik*. Hanya tema terang yang dibangun. Aturan yang tetap berlaku: komponen **hanya** memakai token semantik (`bg-card`, `text-muted-foreground`, …), tidak pernah warna mentah | Agar tema gelap kelak cukup menambah satu blok `.dark` di `theme.css` tanpa menyentuh halaman. Nilai acuan gelap yang sudah divalidasi disimpan di Lampiran E |
| **U6** | **Font di-host sendiri** lewat `@fontsource-variable/*`; preconnect Google Fonts di `index.html` dihapus | Aplikasi berjalan dalam satu kontainer, sering di LAN toko; POS tidak boleh menunggu CDN. Juga tanpa kebocoran IP ke pihak ketiga |
| **U7** | **UI 100% Bahasa Indonesia**, termasuk login | R11. Konsekuensi: `apps/e2e/helpers/labels.ts` (`L.login`) diperbarui |
| **U8** | **POS menjadi "Mode Kasir" layar penuh untuk tablet sentuh landscape** di luar `AppLayout` (tetap di bawah `RequireAuth` → `RequireBranch` → `RequirePerm sales.create`). Pemindaian barcode lewat **kamera tablet** (`PosScannerModal` yang ada). Tablet **hanya** dipakai untuk POS; halaman lain cukup responsif standar dengan prioritas desktop | R9 + keputusan pemilik. POS bukan modul backend (`FRONTEND_GUIDE.md` §POS); ini murni tata letak |
| **U9** | **Aturan penempatan alur** (§7.0) *(analis — diserahkan pemilik)* | Dialog untuk ≤ 6 isian satu langkah; Sheet untuk CRUD master data dan aksi dokumen; Halaman penuh untuk dokumen ber-baris barang. Retur, Surat Jalan, Penerimaan, Transfer, dan Jurnal Manual pindah dari modal ke halaman |
| **U10** | **`<select>` native dipertahankan untuk pilihan pendek** (shadcn *Native Select*); Combobox (Popover + Command) untuk pilihan yang bisa dicari; **input tanggal tetap native** (`type=date`, locale `id-ID` → dd/mm/yyyy), ditambah popover preset rentang | Pengguna ERP mengetik tanggal. Native = cepat di keyboard dan mobile, dan test `selectOptions` tetap jalan |
| **U11** | **Tanpa endpoint backend baru.** Saved view dan "terakhir dibuka" disimpan di `localStorage` per pengguna; command palette memakai endpoint daftar yang sudah ada | Redesain = frontend saja. Kebutuhan data baru → tiket terpisah, bukan bagian plan ini |
| **U12** | **Tidak ada perubahan pada hasil cetak di kertas.** Halaman `/print/*` hanya dirapikan toolbar layarnya | Cetak fisik sedang dalam verifikasi (CLAUDE.md: *physical print verification*) |
| **U13** | **Ratchet UI** ala `scripts/sqlc-migration-status.json`: `scripts/ui-ratchet.mjs` + `scripts/ui-migration-status.json`, ikut `npm run verify` | Budaya repo ini: kemajuan migrasi diukur dan tidak boleh mundur |

---

## 4. Design System v2 "Bingkai Navy" — token

### 4.1 Palet

**Navy `#1e3a5f`** tetap menjadi brand (keputusan pemilik), persis nilai lama
(`oklch(0.346 0.074 256)`). Hue-nya terpisah jelas dari hijau-sukses, kuning-perhatian, dan
merah-masalah.

Navy dipakai di **dua peran**:
- **Bingkai.** Sidebar dan latar di sekeliling kanvas memakai navy gelap yang kaya
  (`#101e33`). Ini menggantikan `#0c1626` sekarang yang hampir hitam dan terasa berat.
- **Aksi.** Tombol utama, tautan, dan item terpilih memakai navy brand di atas kanvas putih.

Basis netralnya abu dingin ber-tint navy sangat tipis (hue 256, chroma ≤ 0,025), bukan krem atau
stone. Ini menghindari tampilan "AI default" dan menyatu dengan brand.

Semua nilai di bawah **sudah divalidasi**: dikonversi OKLCH→sRGB dan diukur rasio kontras
WCAG 2.2. Script validasi ada di Lampiran C. Kolom "Kontras" mengukur teks/elemen terhadap latar
pasangannya.

**Kanvas & aksi (`:root`, tema terang)**

| Token | Nama | OKLCH | ≈ Hex | Kontras |
|---|---|---|---|---|
| `--background` | Kanvas | `oklch(0.985 0.003 256)` | `#f9fafc` | — |
| `--card` / `--popover` | Permukaan | `oklch(1 0 0)` | `#ffffff` | — |
| `--foreground` | Tinta | `oklch(0.21 0.025 262)` | `#121824` | 16,98 : 1 di kanvas |
| `--muted` / `--secondary` | Redup | `oklch(0.965 0.005 256)` | `#f1f4f7` | — |
| `--muted-foreground` | Teks sekunder | `oklch(0.50 0.022 258)` | `#5c6470` | 5,99 di kartu · 5,42 di redup ✔ AA |
| `--border` | Garis | `oklch(0.915 0.007 256)` | `#e0e3e8` | dekoratif (pemisah) |
| `--input` | Garis kontrol | `oklch(0.66 0.016 256)` | `#8c939c` | **3,11** di kartu ✔ 1.4.11 |
| `--primary` | **Navy** (brand) | `oklch(0.346 0.074 256)` | `#1e3a5f` | putih di atasnya **11,51** ✔ |
| `--primary-hover` | Navy tua | `oklch(0.30 0.066 256)` | `#162e4e` | — |
| `--accent` | Navy lembut (hover, baris terpilih, "berjalan") | `oklch(0.955 0.018 256)` | `#e8f1fc` | — |
| `--accent-foreground` | Teks di navy lembut | `oklch(0.346 0.074 256)` | `#1e3a5f` | 10,11 ✔ |
| `--ring` | Cincin fokus (biru baja) | `oklch(0.55 0.11 254)` | `#4173b0` | 4,86 di kartu · 4,65 di kanvas ✔ (≥ 3) |
| `--success` / `-soft` | Hijau "Selesai" | `0.50 0.12 155` / `0.96 0.035 155` | `#0b7643` / `#e0f9e8` | 5,10 ✔ |
| `--warning` / `-soft` | Kuning "Perhatian" | `0.53 0.12 60` / `0.965 0.045 85` | `#9d580c` / `#fff2d2` | 4,92 ✔ |
| `--destructive` / `-soft` | Merah "Masalah" | `0.52 0.19 27` / `0.96 0.025 25` | `#be2323` / `#ffece9` | 5,33 ✔ · putih 6,08 ✔ |
| `--info` / `-soft` | Biru laut "Info" (Alert informatif) | `0.50 0.10 225` / `0.96 0.025 225` | `#006e8e` / `#e1f6ff` | 5,18 ✔ |

**Bingkai navy (`--sidebar-*`)**

| Token | Nama | OKLCH | ≈ Hex | Kontras |
|---|---|---|---|---|
| `--sidebar` | Bingkai | `oklch(0.235 0.045 258)` | `#101e33` | — |
| `--sidebar-foreground` | Teks menu | `oklch(0.80 0.025 256)` | `#b4bfce` | **8,95** ✔ (sekarang `#93a1b5` di `#0c1626`, lebih redup) |
| `--sidebar-muted` (baru) | Label grup | `oklch(0.68 0.03 256)` | `#8c99ab` | 5,80 ✔ |
| `--sidebar-accent` | Hover item | `oklch(0.30 0.05 258)` | `#1e2e47` | teks menu 7,32 ✔ |
| `--sidebar-active` (baru) | Pil item aktif | `oklch(0.33 0.06 257)` | `#213654` | teks putih 11,22 ✔ |
| `--sidebar-primary` | Ikon item aktif / indikator | `oklch(0.78 0.09 250)` | `#8bbcef` | 6,15 di pil aktif ✔ |
| `--sidebar-strong` (baru) | Teks aktif / nama | `oklch(0.97 0.006 256)` | `#f2f5f9` | — |
| `--sidebar-border` | Garis bingkai | `oklch(0.30 0.04 258)` | `#212e42` | dekoratif |
| `--sidebar-ring` | Fokus di bingkai | `oklch(0.72 0.10 250)` | `#73a9e1` | 6,77 ✔ |

**Palet chart** (`--chart-1..5`, kategorikal, urutan tetap):

| Token | OKLCH | ≈ Hex | Peran |
|---|---|---|---|
| `--chart-1` | `0.48 0.10 256` | `#355e95` | Navy (seri utama). 6,56:1 di kartu |
| `--chart-2` | `0.65 0.11 185` | `#1aa598` | Toska |
| `--chart-3` | `0.75 0.14 75` | `#e1a035` | Kunyit |
| `--chart-4` | `0.58 0.15 320` | `#a059b1` | Ungu terong |
| `--chart-5` | `0.66 0.03 256` | `#8693a5` | Abu (pembanding / "periode lalu"). 3,11:1 ✔ 1.4.11 |

Recharts menerima `var(--chart-n)` langsung, sehingga `theme/colors.ts` menyusut menjadi cermin
hex untuk kanvas dan cetak saja.

**Pemetaan nama lama → baru.** Fase 1 mempertahankan kelas lama sebagai alias sementara di
`@theme inline`, lalu dihapus di akhir Fase 4.

| Lama | Baru |
|---|---|
| `ink` | `foreground` |
| `heading` / `muted` | `muted-foreground` |
| `hairline` | `border` |
| `line-strong` | `input` |
| `surface` | `card` |
| `surface-subtle` | `background` |
| `surface-sunken` | `muted` |
| `brand` | `primary` |
| `brand-soft` | `accent` |
| `ok` | `success` |
| `warn` | `warning` |
| `bad` | `destructive` |
| `accent` (terracotta) | **dihapus** — perhatian memakai `warning` |

### 4.2 Arti status — satu peta untuk seluruh aplikasi

Memperbaiki R10. `statusTone()` di `shared/lib/format.ts` diganti dengan peta di bawah, dan
`format.test.ts` diperbarui.

| Nada | Arti | Status | Rupa badge |
|---|---|---|---|
| `neutral` | Belum berjalan | `draft` | abu, titik abu |
| `info` | Sedang berjalan | `confirmed`, `open`, `pending`, `dispatched`, `partial`* | navy lembut (`accent`) |
| `warning` | Perlu perhatian | `pending_approval`*, `partially_paid`*, jatuh tempo ≤ 3 hari | kuning lembut |
| `success` | Selesai | `completed`, `paid`, `received`, `closed`, `posted`, `active` | hijau lembut |
| `muted` (baru) | Berhenti | `cancelled`, `void`, `reversed`, `archived`, `inactive` | abu bergaris, teks dicoret pada nomor dokumen |
| `danger` | Masalah | `expired`, `overdue`, galat integritas | merah lembut |

\* Nama status persis diambil dari tipe modul saat eksekusi (`statusLabel()` menjadi sumber
daftarnya). Status yang belum terdaftar jatuh ke `neutral`.

### 4.3 Tipografi

| Peran | Font | Dipakai untuk | Fitur |
|---|---|---|---|
| **Display** | **Plus Jakarta Sans** Variable (Gumpita Rahayu / Tokotype, Indonesia; OFL) | Judul halaman, angka KPI, total POS, nomor dokumen di header, judul login | `tnum` (ada sejak v2.6), weight 600–700, tracking −0,02em |
| **UI / Body** | **Inter** Variable (4.x) | Semua teks antarmuka, tabel, form | `cv11` (a satu-lantai), `tnum` di sel angka, `opsz` otomatis |
| **Kode** | **Geist Mono** Variable (OFL) | Nomor dokumen di tabel (SO-2026-00012), SKU, kode akun, kode cabang | `tnum`, ukuran −1 step |

Pengekangan: Jakarta **tidak** dipakai di tabel atau form. Huruf geometrisnya yang lebar boros
tempat. Ia hanya muncul di "momen" (judul dan angka besar). Inilah satu-satunya risiko estetik
yang diambil, dan alasannya identitas lokal yang tidak dimiliki template Lovable mana pun.

**Skala.** Kelas default Tailwind didefinisikan ulang di `@theme` sehingga ukuran arbitrer hilang.
Base 14px.

| Kelas | px / line-height | Peran |
|---|---|---|
| `text-2xs` | 11 / 16 | overline, label grup sidebar |
| `text-xs` | 12 / 16 | teks bantu, badge, meta |
| `text-sm` | 13 / 20 | tabel, label, tombol kecil |
| `text-base` | 14 / 22 | teks badan, input |
| `text-lg` | 16 / 24 | judul kartu/seksi |
| `text-xl` | 18 / 26 | judul dialog/sheet |
| `text-2xl` | 22 / 28 | judul halaman (Jakarta 600) |
| `text-3xl` | 28 / 34 | angka KPI (Jakarta 600, tnum) |
| `text-4xl` | 36 / 40 | total POS (Jakarta 700, tnum) |

### 4.4 Bentuk, ruang, kepadatan, gerak

| Aspek | Nilai |
|---|---|
| **Radius** | `--radius: 0.5rem` → `sm` 4 · `md` 6 (input, tombol, badge) · `lg` 8 (kartu) · `xl` 12 (dialog, sheet, kanvas inset). Lebih rapat dari default shadcn (10px) supaya terasa presisi |
| **Elevasi** | Kartu: **border saja, tanpa bayangan**. Bayangan hanya untuk lapisan yang melayang (popover, dropdown, dialog, sheet, toast): `0 8px 24px -6px oklch(0.21 0.025 262 / 0.14), 0 2px 4px oklch(... / 0.06)` |
| **Ruang** | Grid 4px. Gutter halaman 24 (desktop) / 16 (mobile). Jarak antarseksi 24. Padding kartu 20 (`compact`: 16). Lebar konten maks 1440px; form tunggal maks 960px |
| **Kepadatan** | Atribut `data-density` di `<html>`: `compact` / `default` / `comfortable`. Tinggi baris tabel 32 / **36** / 44; tinggi kontrol 32 / **36** / 40. POS memakai skala sentuh sendiri (target ≥ 48, tile ≥ 160×136, lihat §8 pos). Pilihan disimpan per pengguna di `localStorage` |
| **Fokus** | `ring-2 ring-ring ring-offset-2 ring-offset-background` di semua kontrol (memenuhi 2.4.13: 2px, ≥ 3:1). `scroll-padding-top` = tinggi header + 8px, supaya fokus tidak tertutup header sticky (2.4.11) |
| **Gerak** | Hover 120ms; popover/dropdown 160ms fade + 4px slide; sheet 220ms `ease-out`; tanpa animasi masuk halaman. `prefers-reduced-motion` → 0ms (sudah ada, dipertahankan) |
| **Lapisan (z)** | header 30 · sidebar 40 · sheet/dialog 50 · popover/dropdown 60 · toast 70 · global loader 80. Menggantikan `z-[999]` / `z-[1000]` sekarang |

### 4.5 Elemen khas: **Rel Dokumen**

Satu-satunya hal yang membuat aplikasi ini bisa dikenali sebagai mini-erp, bukan template.

Rel Dokumen adalah baris tipis di bawah header setiap halaman dokumen transaksi. Isinya dua
bagian:

- **Langkah siklus dokumen.** Contoh SO: Draf → Terkonfirmasi → Dikirim → Lunas → Selesai.
  Langkah aktif berwarna navy, langkah lewat bercentang, langkah batal abu bergaris.
- **Chip dokumen terkait** yang bisa diklik. Contoh SO: `SJ-0042 Dikirim`, `PAY-0107 Lunas`,
  `RJ-0003 Draf`.

```
 SO-2026-00012  [Terkonfirmasi]                     [Cetak] [Catat Bayar] [Kirim Barang ▸] [⋯]
 ●───────●───────◐───────○───────○        Terkait: ⟨SJ-0042 · Dikirim⟩ ⟨PAY-0107 · Lunas⟩
 Draf    Konfirm Dikirim Lunas   Selesai
```

Kenapa ini pantas jadi elemen khas: ia **mengodekan informasi yang benar**, yaitu rantai dokumen
ERP. Pengguna lama harus membuka 3 halaman untuk menjawab "SO ini sudah dikirim? sudah dibayar?";
Rel Dokumen menjawabnya dalam satu pandangan.

Datanya diambil dari yang **sudah dimuat** halaman, tanpa endpoint baru (U11):
- status dokumen itu sendiri
- `OrderHistoryCard` events di detail SO/PO
- daftar surat jalan, pembayaran, dan retur yang sudah di-fetch halaman detail

`FlowSteps` lokal di `PurchaseOrderDetailPage.tsx:52` menjadi komponen pertama yang digantikannya.

Komponennya generik (`DocumentRail`: `steps[]`, `current`, `related[]`), tanpa istilah domain.
Ia tinggal di `shared/components/patterns/`. Definisi langkah per jenis dokumen tinggal di modul
masing-masing (`modules/<m>/lib/<doc>-rail.ts`).

---

## 5. Arsitektur komponen

### 5.1 Lapisan

```
modules/<m>/pages, components     ← hanya impor dari ui/ dan patterns/
        │
shared/components/patterns/       ← komposisi generik: PageHeader v2, DocumentRail, DataTable,
        │                            Toolbar, FilterChips, DescriptionList, KpiCard, FormSection,
        │                            StickyFooter, FileDropzone, Stepper, ErrorState, …
shared/components/ui/             ← FASAD: nama & props lama dipertahankan (Button, Modal, …)
        │
shared/components/primitives/     ← kode shadcn (hasil `npx shadcn add`), DILARANG diimpor modul
        │
shared/lib/cn.ts                  ← clsx + tailwind-merge
theme/theme.css                   ← token :root + @theme inline (satu-satunya sumber warna; .dark ditunda)
```

`components.json` (shadcn): `style: "new-york"`, `tailwind.css: "src/theme/theme.css"`, `iconLibrary: "lucide"`,
`aliases.ui: "@/shared/components/primitives"`, `aliases.utils: "@/shared/lib/cn"`.

### 5.2 Dependensi baru

Versi persis dikunci saat Fase 1, **wajib** dicek ke dokumentasi terkini lewat Context7 MCP.
Context7 perlu diotorisasi dulu lewat `/mcp`.

| Paket | Untuk |
|---|---|
| `radix-ui` (paket gabungan) | primitif dialog, popover, dropdown, tabs, checkbox, tooltip, dll. |
| `class-variance-authority`, `clsx`, `tailwind-merge` | varian komponen + `cn()` |
| `tw-animate-css` | animasi masuk/keluar primitif (pengganti `tailwindcss-animate`) |
| `cmdk` | Command palette + Combobox |
| `sonner` | Toast (di balik `toast.store` yang ada) |
| `@tanstack/react-table` | mesin DataTable (sorting, visibilitas kolom, seleksi) |
| `@fontsource-variable/inter`, `@fontsource-variable/plus-jakarta-sans`, `@fontsource-variable/geist-mono` | font lokal (U6). Verifikasi nama paket di npm |

**Tidak** ditambahkan:
- `react-hook-form`: form tetap dengan pola state + Zod yang ada; Field shadcn agnostik library.
- `react-day-picker`: input tanggal tetap native (U10).
- Library chart lain: tetap recharts v3 + wrapper Chart shadcn.

### 5.3 Peta fasad: komponen lama → isi baru

| Komponen (`ui/`) | Isi baru | Perubahan API |
|---|---|---|
| `Button`, `buttonClass` | shadcn Button (cva) | **kompatibel**. `primary`→default, `secondary`→outline, `ghost`, `danger`→destructive. **Tambahan**: `size="lg" \| "icon"`, `loading`, `asChild` (mengakhiri `<Link><Button>`) |
| `Badge` | shadcn Badge + titik | kompatibel. Tambahan nada `muted` |
| `TextInput`, `TextArea`, `PasswordInput`, `DateInput` | shadcn Input/Textarea + **Input Group** (ikon/prefiks "Rp"/suffix satuan) | kompatibel. Tambahan `prefix`/`suffix` |
| `SelectInput`, `SelectField` | shadcn **Native Select** | kompatibel (U10) |
| `SearchSelect`, `AsyncSearchSelect`, `HierarchicalSelect` | Popover + Command (Combobox), virtualisasi untuk daftar panjang | props dipertahankan. `async-search-select.test.tsx` & `hierarchical-select.test.tsx` diperbarui (role `combobox`/`option`) |
| `FormField`, `FieldHint` | shadcn **Field** (`FieldLabel`, `FieldDescription`, `FieldError`) | kompatibel. Badge "Wajib" diganti tanda `*` + `aria-required` (lebih tenang) |
| `Modal` | shadcn **Dialog** | kompatibel. Fokus terkunci + Esc + `aria-labelledby` gratis |
| `ConfirmDialog` | shadcn **AlertDialog** | kompatibel. Tambahan `tone="danger"`, `count` (aksi massal) |
| `DataTable` | TanStack + shadcn Table (§7.1) | kompatibel (`columns.header/render/align/width`). **Tambahan opsional**: `loading`, `error`, `onRetry`, `onRowClick`, `sortable` per kolom + `sort`/`onSortChange`, `selectable` + `bulkActions`, `stickyHeader` (default **on**), `columnVisibilityKey`, `density` |
| `Pagination` | shadcn Pagination + pilihan "baris per halaman" | kompatibel. Tambahan `onLimitChange` |
| `PageHeader` | PageHeader v2 | kompatibel. `eyebrow` **diabaikan** (diganti breadcrumb). Tambahan `meta` (badge/status), `secondaryActions` (masuk menu `⋯`), `backTo` |
| `SectionCard` | shadcn Card | kompatibel. Tambahan `variant="plain"` (tanpa kartu, untuk seksi di dalam halaman) |
| `SummaryCard` | **KpiCard** | kompatibel. Tambahan `delta`, `sparkline`, `href` (tombol nyata, bukan `div role=button`) |
| `FilterBar` | **Toolbar** + `FilterChips` | kompatibel. Tambahan `primary` (selalu terlihat) vs `more` (masuk popover "Filter") |
| `ActionRow` | → `StickyFooter` (form) atau dihapus (detail) | dideprekasi bertahap |
| `Segmented` | shadcn **Toggle Group** (pengganti nilai) **dan** **Tabs** (pengganti panel) | dipecah; pemakai `Segmented`-sebagai-tab pindah ke `Tabs` di Fase 4 |
| `MultiToggle` | Toggle Group multiple | kompatibel |
| `DateRangeFields` | 2× native date + popover preset (Hari ini, 7 hari, Bulan ini, Bulan lalu, Tahun ini) | kompatibel |
| `ToastViewport` + `toast.store` | **Sonner** (`<Toaster richColors position="bottom-right">`) | **API `toast.success/danger/...` tetap** — hanya renderer yang berganti |
| `GlobalLoader` | dipertahankan, ditipiskan 2px warna `primary` | — |
| `EmptyState` | shadcn **Empty** (ikon kontekstual + judul + deskripsi + aksi) | kompatibel. Tambahan `icon` |
| `Notice` | shadcn **Alert** | kompatibel |
| `LoadingState` (layar penuh) | logo + Spinner, teks "Memuat…" | teks Indonesia (R11) |

### 5.4 Komponen baru

| Lapisan | Komponen | Menggantikan |
|---|---|---|
| ui | `Checkbox`, `RadioGroup`, `Switch` | input mentah di Users, Roles, Posting, Balances |
| ui | `Tabs` (+ varian `nav` yang tersinkron URL) | `Segmented` sebagai tab |
| ui | `Tooltip`, `DropdownMenu`, `Popover`, `Sheet`, `Separator`, `ScrollArea`, `Avatar`, `Kbd`, `Spinner`, `Skeleton`, `Progress`, `Breadcrumb`, `Collapsible` | — |
| patterns | `Page` (tumpukan vertikal 24px; mengakhiri `mb-4` ×80, `mt-4` ×38) | `<div>` akar + margin ad-hoc |
| patterns | `DescriptionList` (`columns={2\|3\|4}`, `label/value`, salin-ke-clipboard opsional) | 21 `<dl>` + `InfoRow` ganda di Customer/SupplierDetail |
| patterns | `DocumentRail` (§4.5), `Stepper` | `FlowSteps`, judul "Langkah 1/2/3" di ImportPage |
| patterns | `ErrorState` (`message` API asli + "Coba lagi") | ±40 Notice + tombol manual |
| patterns | `TableSkeleton`, `CardSkeleton`, `DetailSkeleton` | ±90 teks `Memuat…` |
| patterns | `FormSection` (judul + deskripsi kiri, isian kanan di ≥ 1024px) | SectionCard bertumpuk di form panjang |
| patterns | `StickyFooter` (Batal · Simpan draf · **Simpan**; ringkasan total di kiri) | `ActionRow` di dasar halaman |
| patterns | `LineItemsEditor` (grid baris barang: Enter = baris baru, Tab = sel berikut, Combobox produk, total langsung) | DataTable baris SO vs kartu-per-baris PO (tidak konsisten) |
| patterns | `FileDropzone` (seret-lepas + pilih + pratinjau nama/ukuran) | 5× `<input type=file>` mentah |
| patterns | `Money`, `Qty`, `DocNumber` (mono + salin) | format ad-hoc di sel |
| patterns | `CommandPalette` (§6.3), `PageMeta` (zustand kecil: judul + breadcrumb dari halaman) | — |
| patterns | `Numpad` (papan angka layar sentuh generik: qty, nominal; dipakai POS) | keyboard sistem tablet |
| layouts | `AppShell` (sidebar + header), `FocusLayout` (POS), `AuthLayout`, `PrintChrome` | `AppLayout.tsx` 511 baris |

---

## 6. Kerangka aplikasi

### 6.1 Tata letak — "Bingkai Navy"

```
▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
▓ [logo] Mini ERP      ⌄ ▓ ╭──────────────────────────────────────────────────────────────╮ ▓
▓        PT Contoh · UTM ▓ │ [≡] Operasional / Order Jual / SO-2026-00012  [⌕ Ctrl K] [+ Buat ▾]│ ▓ ← header DI DALAM kanvas
▓ ────────────────────── ▓ │──────────────────────────────────────────────────────────────│ ▓
▓ [⌕ Cari menu   Ctrl K] ▓ │                                                              │ ▓
▓                        ▓ │   kanvas putih/abu sangat terang (--background),             │ ▓
▓ UTAMA                  ▓ │   radius xl, mengambang 8px dari bingkai                     │ ▓
▓  ▣ Dashboard           ▓ │                                                              │ ▓
▓ OPERASIONAL            ▓ │                                                              │ ▓
▓  ▣ POS                 ▓ │                                                              │ ▓
▓ ╭▣ Order Jual────────╮ ▓ │                                                              │ ▓ ← pil aktif
▓ ╰────────────────────╯ ▓ │                                                              │ ▓   --sidebar-active
▓  ▣ Pengiriman          ▓ │                                                              │ ▓
▓ KEUANGAN               ▓ │                                                              │ ▓
▓  ▸ Harian              ▓ │                                                              │ ▓
▓ ────────────────────── ▓ │                                                              │ ▓
▓ (DP) Dimas P.        ⌃ ▓ ╰──────────────────────────────────────────────────────────────╯ ▓
▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
 ▓ = --sidebar (#101e33). Bingkai mengelilingi kanvas di keempat sisi, dengan celah 8px.
```

Yang membuat bingkai ini terasa modern dan bukan "sidebar gelap kuno":
- **Kanvas inset.** Konten tidak lagi menempel rata ke sidebar, tetapi duduk di kartu besar
  membulat di dalam bingkai navy. Header ikut berada di dalam kanvas.
- **Navy yang kaya, bukan hampir hitam.** `#101e33` menggantikan `#0c1626`. Teks menu jauh lebih
  terbaca: 8,95:1, dibanding teks `#93a1b5` sekarang.
- **Item aktif berupa pil** `--sidebar-active` dengan teks putih dan ikon `--sidebar-primary`.
  Garis biru hardcode `#6aa3dd` dihapus.
- **Tanpa gradient, glow, atau bayangan** di logo dan avatar. Hierarki dibangun dari bobot teks
  dan pil aktif saja.

**Sidebar:**
- Komponen shadcn Sidebar, `variant="inset"`, `collapsible="icon"`, Ctrl/Cmd+B. Lebar 16rem,
  mode ikon 3rem.
- Menggantikan logika manual di `AppLayout.tsx` (lebar, drawer mobile, Esc, `localStorage`).
- **Kepala sidebar:**
  - Logo dan nama aplikasi; logo resmi menyusul (U7-7).
  - Di bawahnya nama perusahaan dan kode cabang. Chip Perusahaan dihapus dari topbar.
  - Pemilih cabang ala *team switcher* menggantikan chip "Ganti Cabang" di topbar. Klik membuka
    dropdown daftar cabang yang bisa diakses; bila hanya satu cabang, tampil statis.
- **Tombol "Cari menu"** membuka command palette.
- **Grup:**
  - Tetap 8 grup `MENU_GROUPS` dengan urutan yang sama.
  - Label grup `text-2xs` uppercase berwarna `--sidebar-muted`.
  - Seksi Keuangan tetap collapsible (Collapsible).
  - Label "Keuangan", "Detail", "Posting", "Jurnal", "Laba Rugi", "POS", dan "Stok" **tidak
    diganti**, karena dipakai e2e (`labels.ts`).
- **Footer:** menu pengguna (`nav-user`) berisi nama, role, ganti role, kepadatan tampilan, dan
  keluar. Menggantikan `AvatarMenu`; avatar polos tanpa gradient.

**Header** (tinggi 48px, sticky di dalam kanvas, tanpa blur):
- `SidebarTrigger`
- **Breadcrumb**: Grup / Menu / [dokumen]. Ini **bukan** `<h1>`; `<h1>` hanya ada di
  `PageHeader` (R2).
- Tombol Cari (Ctrl+K)
- **"+ Buat"** (dropdown aksi cepat sesuai izin)

**Mobile (< 768px):** bingkai inset dilepas (kanvas penuh layar); sidebar menjadi Sheet off-canvas
navy (bawaan shadcn); header menampilkan judul halaman singkat; aksi halaman pindah ke bar bawah
sticky.

### 6.2 Perbaikan registry (`app/routes/registry.tsx`)

**Ikon unik**, menghapus semua ikon ganda:

| Menu | Ikon lama | Ikon baru (lucide) |
|---|---|---|
| Saldo (Stok) | Boxes | `Layers` |
| Laporan Stok | Boxes | `ChartColumnStacked` |
| Penyesuaian Pajak | Scale | `FilePenLine` |
| Neraca Saldo | Scale | `Columns2` |
| Saldo Awal (Stok) | FileSpreadsheet | `PackagePlus` |
| Saldo Awal (Keuangan) | FileSpreadsheet | `BookPlus` |
| Akun | Cog | `ListTree` |
| Konfigurasi (Asisten) | Cog | `SlidersHorizontal` |
| Pemasok | Building2 | `Factory` |
| Konsistensi | Stethoscope | `ShieldAlert` |
| Laporan Order | ClipboardList | `ChartBar` |
| Antrian Kirim | ClipboardCheck | `ListChecks` |

Nama ikon diverifikasi terhadap lucide-react 1.43 saat eksekusi; catatan "ClipboardEdit/History
tidak ada" di registry tetap berlaku.

**Perbaikan lain:**
- **Label ganda.** "Saldo Awal" di grup Stok menjadi **"Saldo Awal Stok"**; di Keuangan menjadi
  **"Saldo Awal Akun"**. Pastikan tidak ada spec e2e yang memakai teks ini; bila ada, perbarui
  `labels.ts`.
- **Judul yang hilang.** Tambahkan `PAGE_TITLES` untuk `taxDetail` ("Rincian Pajak"),
  `inventoryValue` ("Nilai Persediaan"), dan `margin` ("Margin Produk").
- **Breadcrumb.** `getPageTitle` menjadi `getBreadcrumb(pathname)` yang mengembalikan
  `[grup, menu]`. Halaman detail menambahkan segmen ketiga (nomor dokumen) lewat `usePageMeta()`.
- **Aksi cepat.** Tambahkan `QUICK_ACTIONS` (label, path, ikon, perm) untuk tombol "+ Buat" dan
  command palette: Order Jual baru, Order Beli baru, Catat Pembayaran, Catat Biaya, Transfer Stok,
  Produk baru, Pelanggan baru.

### 6.3 Command palette (Ctrl+K)

- **Navigasi:** semua item menu yang diizinkan, dikelompokkan per grup.
- **Aksi:** `QUICK_ACTIONS` yang diizinkan.
- **Terakhir dibuka:** 8 dokumen terakhir (disimpan di `localStorage` per user saat halaman
  detail dibuka).
- **Lompat ke dokumen (Fase 6):** ketik nomor (`SO-…`, `PO-…`, `SJ-…`, `PAY-…`) → cari lewat
  endpoint daftar yang sudah menerima parameter `search`. Verifikasi per service saat eksekusi;
  jenis yang belum mendukung dilewati, tanpa menambah endpoint (U11).

---

## 7. Template halaman

Setiap halaman di §8 dipetakan ke **satu** template ini. Template adalah kontrak: tata letak, letak
aksi, state, dan komponen yang dipakai.

### 7.0 Aturan penempatan alur (U9)

| Bentuk | Kapan | Contoh |
|---|---|---|
| **Dialog** | ≤ 6 isian, satu langkah, tidak butuh konteks halaman di belakangnya | Setujui Kredit, Ganti Kata Sandi, Batalkan + alasan, Konfirmasi |
| **Sheet kanan** (480–640px) | CRUD master data; aksi pada dokumen yang butuh melihat dokumennya; melihat rincian baris tanpa pindah halaman | Pelanggan/Pemasok/Cabang/Kategori/Tipe Member/Akun/Pengguna; Catat Pembayaran; detail Jejak Audit |
| **Halaman penuh** | Dokumen ber-baris barang, alur multi-langkah, atau > 12 isian | SO, PO, **Retur Jual**, **Retur Beli**, **Surat Jalan**, **Penerimaan**, **Transfer**, **Jurnal Manual**, Produk, Impor |

Halaman baru (`…/new`) didaftarkan di `EXTRA_ROUTES` (`protected.routes.tsx`) dengan izin yang
sama dengan tombol pembukanya. Untuk alur yang berawal dari dokumen lain (contoh: retur dari SO),
sumbernya dikirim lewat query `?from=<id>`, menggantikan prop `initialSource`.

### T1 — Daftar (List)

```
 Order Jual                                                   [Ekspor ▾]  [+ Order Jual]
 128 order · 12 draf · 5 jatuh tempo minggu ini                    ← ringkasan satu baris (opsional)
 ┌───────────────────────────────────────────────────────────────────────────────────────┐
 │ [⌕ Cari nomor / pelanggan…] [Status ▾] [Tanggal: Bulan ini ▾] [+ Filter]    [⚙ Kolom] │
 │ Status: Draf ✕   Pelanggan: Toko Maju ✕   Hapus semua                                 │ ← chip filter
 ├───────────────────────────────────────────────────────────────────────────────────────┤
 │ ☐ │ Nomor ↓        │ Tanggal    │ Pelanggan        │ Status        │      Total      │ │ ← header sticky
 │ ☐ │ SO-2026-00012  │ 16 Okt 26  │ Toko Maju        │ ● Dikirim     │  Rp 12.450.000  │⋯│
 │ … baris bisa diklik (seluruh baris → detail); ⋯ = aksi baris                          │
 ├───────────────────────────────────────────────────────────────────────────────────────┤
 │ 1–25 dari 128                                   Baris: [25 ▾]   ‹ 1 2 3 … 6 ›         │
 └───────────────────────────────────────────────────────────────────────────────────────┘
```

**Aturan:**
- **Filter:**
  - Selalu terlihat: maksimal **3 filter utama** + pencarian. Sisanya ada di popover
    "+ Filter".
  - Filter aktif tampil sebagai chip.
  - Semua filter tersinkron ke URL lewat `use-url-filters` yang sudah ada.
- **Kolom:**
  - Sortir dilakukan lewat **header kolom**. Select "Urutkan/Arah" dihapus.
  - Nomor dokumen ditulis dengan `DocNumber` (mono).
  - Uang memakai `Money` (rata kanan); status memakai `StatusBadge`.
- **Baris:**
  - Seluruh baris bisa diklik (`onRowClick`) dengan aksesibilitas yang benar: link di sel
    pertama, baris diberi `cursor-pointer`.
  - Aksi baris ada di menu `⋯`.
- **Aksi massal:** hanya bila ada aksi massal nyata (contoh: posting jurnal, cetak label). Saat
  ada baris terpilih, bar aksi massal menggantikan toolbar.
- **State:**
  - `loading` → `TableSkeleton` sebanyak `limit` baris.
  - `error` → `ErrorState` di dalam kartu.
  - Kosong tanpa filter → Empty + tombol buat ("Belum ada order jual. [+ Buat order pertama]").
  - Kosong karena filter → "Tidak ada yang cocok dengan filter ini" + "Hapus filter".
- **Pagination wajib** untuk semua daftar yang bisa tumbuh. Saat ini belum ada di: MemberTypes,
  Categories, Accounts, Periods, Queue, Damaged, Roles, TaxAdjustments, Stock Lokasi/Saldo,
  kedua Opening, Posting. Bila endpoint belum berpaginasi, pagination dilakukan di klien dengan
  catatan di §8.

### T2 — Detail Dokumen

```
 Operasional / Order Jual / SO-2026-00012
 SO-2026-00012  [● Terkonfirmasi]  COD                  [Cetak] [Catat Bayar] [Kirim Barang] [⋯]
 Toko Maju · 16 Okt 2026 · oleh Rina
 ●──────●──────◐──────○──────○    Terkait: ⟨SJ-0042 · Dikirim⟩ ⟨PAY-0107 · Lunas⟩     ← Rel Dokumen
 ┌──────────────────────────────────────────────────┐ ┌─────────────────────────────────┐
 │ Barang (6)                                       │ │ Ringkasan                       │
 │ Produk              Qty  Terkirim   Harga  Total │ │ Subtotal          Rp 12.000.000 │
 │ Semen 50kg · SMN-01  20   12/20   65.000  1,3 jt │ │ Diskon              − Rp 50.000 │
 │   ↳ Dasar 70.000 · Diskon 5% (promo)  ← baris    │ │ PPN 11%            Rp 1.314.500 │
 │     kedua redup, bukan sel bertingkat            │ │ ─────────────────────────────── │
 │ …                                                │ │ Total             Rp 13.264.500 │
 └──────────────────────────────────────────────────┘ │ Dibayar           Rp 10.000.000 │
 ┌──────────────────────────────────────────────────┐ │ Sisa               Rp 3.264.500 │ ← warning bila lewat tempo
 │ Aktivitas                                        │ ├─────────────────────────────────┤
 │ ○ 16 Okt 10:12  Rina mengonfirmasi order         │ │ Pelanggan                       │
 │ ○ 16 Okt 13:40  SJ-0042 diterbitkan              │ │ Toko Maju · 0812… · Termin 14 h │
 │ …                                                │ ├─────────────────────────────────┤
 └──────────────────────────────────────────────────┘ │ Info dokumen (DescriptionList)  │
                                                      └─────────────────────────────────┘
```

**Aturan:**
- **Aksi:**
  - **Semua aksi ada di header.** Maksimal 1 `primary` (aksi berikut yang paling mungkin menurut
    status) ditambah 2 `outline`. Sisanya di `⋯`, dan aksi destruktif paling bawah, berwarna
    merah.
  - Kartu "Aksi Order" dan `ActionRow` "Kembali ke daftar" dihapus.
- **Kolom:**
  - ≥ 1280px: dua kolom, `2fr / 1fr`. Kolom kanan sticky saat menggulir.
  - < 1280px: kolom kanan turun ke bawah ringkasan.
- **Tabel barang:**
  - Maksimal 6 kolom yang terlihat.
  - Info turunan (dasar harga, sumber harga, DPP, catatan) pindah ke **baris kedua redup** atau
    Tooltip. Ini memperbaiki 8 kolom bertingkat di `SalesOrderDetailPage.tsx:346-445`.
- **Catatan kebijakan:** catatan seperti "Order prabayar: surat jalan baru bisa…" menjadi Alert
  `info` tunggal di bawah Rel Dokumen, hanya bila relevan.
- **Loading:** `DetailSkeleton`. **Tidak ditemukan:** `NotFoundState` dengan breadcrumb tetap
  tampil.

### T3 — Form Dokumen (dengan baris barang)

```
 Order Beli / Baru                                                     Draf tersimpan otomatis ✓
 ┌─────────────────────────────────────────────────────────────────────────────────────────┐
 │ Pemasok*  [Combobox…………………]   Tanggal* [16/10/2026]   Termin [14 hari ▾]   Gudang [▾] │ ← header ringkas, 1 baris
 │ ▸ Detail lain (No. ref pemasok, catatan, alamat kirim)                     ← Collapsible │
 ├─────────────────────────────────────────────────────────────────────────────────────────┤
 │ #  Produk                     Satuan   Qty     Harga      Disk%   PPN   Total        ✕ │
 │ 1  [Semen 50kg ▾]             [sak ▾]  [20]    [65.000]   [0]     ☑     1.300.000    ✕ │
 │ 2  [Cari produk atau scan…]                                                            │ ← Enter = baris baru
 │ [+ Tambah baris]   Tip: Enter menambah baris · Tab berpindah sel · Alt+↑↓ pindah baris  │
 └─────────────────────────────────────────────────────────────────────────────────────────┘
 ═══════════════════════════════════════════════════════════════════════════════════════════
  6 barang · Subtotal Rp 12.000.000 · PPN Rp 1.320.000 · Total Rp 13.320.000   [Batal] [Simpan draf] [Simpan & Konfirmasi]
 ═══════════════════════════════════════════════════════════════════════════════════════════ ← StickyFooter
```

**Aturan:**
- Header dokumen ringkas dalam 1–2 baris. Isian jarang dipakai disimpan di Collapsible
  "Detail lain".
- Baris barang selalu memakai `LineItemsEditor`, sama untuk SO, PO, retur, penerimaan, surat
  jalan, dan transfer. Di layar sempit, baris menjadi kartu yang dilipat (ringkasan + "Ubah").
- `StickyFooter` selalu terlihat dan memuat total.
- Galat validasi:
  - tampil inline per sel
  - ringkasan galat di footer ("2 isian perlu diperbaiki" → klik fokus ke yang pertama)
  - `message` API tampil sebagai Alert di atas form
- Draf otomatis (`use-form-draft` yang ada) ditampilkan sebagai indikator kecil di kanan atas,
  bukan Notice besar.

### T4 — Master Data

- **Daftar:** template T1.
- **Buat/ubah:** Sheet kanan (U9), memakai `FormSection` tanpa kartu. Footer Sheet berisi Batal
  dan Simpan.
- **Detail (Pelanggan/Pemasok/Produk):** header berisi identitas (avatar/foto, nama, kode, status)
  dan aksi, lalu **Tabs** yang tersinkron ke URL (`?tab=`): Ringkasan · Alamat · Transaksi ·
  Saldo · Riwayat. Tidak lagi semua kartu ditumpuk.
- **Hierarki** (Kategori, Akun): tabel pohon (indentasi + chevron lipat), bukan tabel datar
  dengan kolom "induk".

### T5 — Laporan

```
 Laba Rugi                                                      [Ekspor ▾] [Cetak]
 [Periode: Okt 2026 ▾] [Bandingkan: Sep 2026 ▾] [Cabang: Aktif]          ← toolbar filter
 ┌ KPI ─────────┐┌──────────────┐┌──────────────┐┌──────────────┐
 │ Pendapatan   ││ HPP          ││ Laba kotor   ││ Laba bersih  │  ← maks 4, dengan delta vs pembanding
 └──────────────┘└──────────────┘└──────────────┘└──────────────┘
 ┌ Grafik (bila laporan bersifat tren) ───────────────────────────────────────┐
 └────────────────────────────────────────────────────────────────────────────┘
 ┌ Tabel laporan: header sticky, baris grup tebal, subtotal ber-garis atas, ─┐
 │ kolom angka tnum rata kanan, negatif dalam (kurung) merah                 │
 └────────────────────────────────────────────────────────────────────────────┘
```

**Aturan:**
- Laporan keuangan memakai format akuntansi: negatif ditulis `(1.250.000)` dan diberi warna
  `destructive`; nol ditulis `–`.
- Setiap laporan punya **URL sendiri**. Tab antarlaporan adalah `Tabs variant="nav"` yang
  sebenarnya berupa link (memperbaiki R12 di `ReportsPage`).

### T6 — Dashboard (§8, modul dashboard)

### T7 — Konsol kerja (antrian / workbench)

Dipakai Posting, Antrian Kirim, Setup WA, Integritas, dan Konsistensi.

- **Atas:** baris status, maksimal 4 `KpiCard` kecil yang bisa diklik untuk memfilter.
- **Tengah:** `Tabs` per antrian (tersinkron URL).
- **Isi tab:** T1 dengan aksi massal.
- **Aksi besar** (Tutup Hari, Posting semua) ada di header dengan `ConfirmDialog` yang menyebut
  jumlah.

### T8 — Pengaturan

Dipakai Perusahaan, Konfigurasi Asisten, dan Periode.

- **≥ 1024px:** navigasi seksi vertikal di kiri (anchor, sticky), isian di kanan, memakai
  `FormSection` dengan pola "label + deskripsi | kontrol".
- **Simpan:** `StickyFooter` muncul **hanya bila ada perubahan** ("3 perubahan belum disimpan").
- **Mode baca-saja tiga lapis** dari sistem lama **dipertahankan**: kontrol nonaktif, teks
  penjelas, dan submit diabaikan (`FRONTEND_GUIDE.md`). Rupanya diganti menjadi Alert `info` di
  atas plus ikon gembok di setiap seksi.

### T9 — Wizard / Impor

Dipakai Impor Produk, Saldo Awal Stok, dan Saldo Awal Akun.

- `Stepper` horizontal di atas: Unduh templat → Unggah → Tinjau → Selesai.
- `FileDropzone` untuk unggahan.
- Langkah Tinjau: tabel dengan tab "Valid (n) | Bermasalah (n)", baris bermasalah diberi sorotan
  dan tooltip alasan.
- Tombol lanjut di `StickyFooter`.
- **Satu komponen `ImportWizard`** di `patterns/` (generik: `steps`, `onUpload`, `preview`,
  `onCommit`) dipakai ketiganya. Alur yang sama tidak lagi dibangun 3 kali.

### T10 — Mode Fokus (POS, pemindai)

Lihat §8, modul pos.

### T11 — Auth & halaman status

- **Login:**
  - Layout terbelah 5:7. Kiri panel `--sidebar` (bukan gradient), berisi logo dan satu kalimat
    produk dalam Jakarta.
  - Kanan form tanpa kartu: label "Nama pengguna atau email" dan "Kata sandi", tombol "Masuk".
  - Galat dari API tampil di Alert.
- **Pilih Cabang:** daftar Item (shadcn Item) yang berisi kode, nama, dan badge "Pusat", dengan
  navigasi panah. Judul "Pilih Cabang".
- **403/404:** Empty dengan ikon, judul, satu tautan ("Ke Dashboard").

### T12 — Cetak

Hasil cetak **tidak diubah** (U12). Yang berubah hanya `PrintLayout` (toolbar layar):
- memakai Button/Native Select/Input baru
- sticky di atas
- disembunyikan saat `@media print`

Halaman cetak memuat font lokal yang sama. Profil kertas dan `@page` tidak disentuh.

---

## 8. Redesain per modul

Kolom **T** = template (§7). Kolom **Gel.** = gelombang eksekusi (§9, Fase 4). "Bentuk baru" hanya
mencantumkan perubahan khusus di luar penerapan template.

### auth
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `LoginPage` | T11 | Bahasa Indonesia (U7); tanpa gradient; `PasswordInput` baru; tombol `size=lg` penuh | Fase 3 |
| `SelectBranchPage` | T11 | Daftar Item + keyboard; badge "Pusat"; pencarian bila > 8 cabang | Fase 3 |
| `ForbiddenPage`, `NotFoundPage` | T11 | Empty; hapus `rounded-[10px]` | Fase 3 |

### dashboard
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `DashboardPage` | T6 | Lihat di bawah | 4f |

**Komposisi dashboard (17 kartu → 4 + konteks):**

```
 Selamat siang, Rina                                    Cabang Utama · Jumat, 16 Okt 2026
 Sumber angka: [Pembukuan | Order]  ⓘ                    ← ToggleGroup; tooltip menjelaskan beda
 ┌ Omzet hari ini ┐┌ Laba hari ini ┐┌ Kas ──────────┐┌ Piutang ──────┐
 │ Rp 18,2 jt     ││ Rp 3,1 jt     ││ Rp 142 jt     ││ Rp 64 jt      │
 │ Bulan ini 412jt││ Bulan ini 81jt││ Hutang 38 jt  ││ 5 lewat tempo │   ← angka sekunder, bukan kartu baru
 └────────────────┘└───────────────┘└───────────────┘└───────────────┘
 ┌ Tren penjualan            [7h|30h|90h] ┐ ┌ Perlu tindakan (7) ──────────────┐
 │ (area chart, chart-1, pembanding chart-5)│ │ ● SO-0012 jatuh tempo 2 hari    │
 │                                          │ │ ● Stok kritis: 4 produk  →       │
 │                                          │ │ ● 3 SJ menunggu konfirmasi →     │
 └──────────────────────────────────────────┘ └──────────────────────────────────┘
 ┌ Terlaris | Margin tertinggi  (Tabs) ─────┐ ┌ Status order (donut / bar 100%) ─┐
 └──────────────────────────────────────────┘ └──────────────────────────────────┘
```

- **Dua sumber angka tetap disajikan, dengan cara yang berbeda.**
  - Sekarang "Saldo Buku (pembukuan)" dan "Dari Order" adalah dua seksi panjang berdeskripsi
    teknis.
  - Keduanya menjadi satu **ToggleGroup** "Sumber angka" ditambah tooltip penjelasan.
  - Perbedaan semantik pembukuan vs order adalah keputusan domain, jadi **tidak dihapus**.
- **"Perlu tindakan"** menggabungkan `priorityOrders` dan `criticalStock` yang sudah di-fetch.
  Setiap baris adalah tautan ke daftar yang sudah difilter.
- **Data dan izin tidak berubah.** Seluruh angka dari endpoint yang sudah ada. Widget tren tetap
  digerbangi `reporting.view`.
- **Delta "vs kemarin" hanya bila data tersedia**; tidak menebak (U11).
- **State:** galat per widget memakai `ErrorState` ringkas; loading memakai `CardSkeleton`.

### pos — Mode Kasir untuk tablet sentuh landscape (U8)

**Perangkat (keputusan pemilik):**
- Tablet layar sentuh, orientasi **landscape**, dipakai **hanya** untuk POS.
- Pemindai barcode = **kamera tablet**, lewat `PosScannerModal` + `@zxing/browser` yang ada.
- Struk lewat jembatan printer Android yang ada (D6 plan paritas).

**Viewport sasaran** (CSS px):
- utama 1280×800 (tablet Android 10–11")
- harus tetap layak di 1024×768 dan 1340×800
- di bawah lebar 1024 (salah orientasi), tampilkan ajakan "Putar tablet ke posisi mendatar";
  layout portrait tidak dibangun

| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `PosPage` (1517 baris) | T10 | Layout di bawah + dipecah menjadi komponen | Fase 5 |
| `PosScannerModal` | T10 | Dialog **layar penuh** dengan bidikan besar; tetap terbuka untuk pindai beruntun (lihat "Alur pindai"); input manual tetap ada sebagai cadangan | Fase 5 |
| `PosCategoryDrilldown` | T10 | Chip kategori 44px, bergulir horizontal | Fase 5 |
| `PosCustomerPanel` | T10 | Kepala keranjang; ganti pelanggan lewat Sheet dengan pencarian | Fase 5 |

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│ [←] Kasir · UTM · Rina      [⌕ Cari produk…          ]  [▣ PINDAI]       14:32   [⛶] [⋯]  │ ← 64px, navy bingkai
├────────────────────────────────────────────────────────┬─────────────────────────────────┤
│ [Semua] [Semen] [Besi] [Cat] [Pipa] [Lainnya] →        │ Pelanggan: Umum        [Ganti]  │
│ ┌──────────┐┌──────────┐┌──────────┐┌──────────┐       │ ─────────────────────────────── │
│ │  [foto]  ││  [foto]  ││          ││          │       │ Semen 50kg                   ✕  │
│ │ Semen    ││ Besi 10  ││  …       ││          │       │ 65.000   [ − ] [ 20 ] [ + ]     │
│ │ 50kg     ││          ││          ││          │       │          Diskon ›  1.300.000    │
│ │ 65.000   ││ 98.000   ││          ││          │       │ Cat 5L                       ✕  │
│ │ Stok 120 ││ Stok 8 ⚠ ││          ││          │       │ 210.000  [ − ] [  2 ] [ + ]     │
│ └──────────┘└──────────┘└──────────┘└──────────┘       │ ─────────────────────────────── │
│  tile ≥ 160×136, grid auto-fill, gulir vertikal        │ Subtotal               1.450.000 │
│                                                        │ Diskon                   −50.000 │
│                                                        │ TOTAL         Rp 1.400.000       │ ← text-4xl Jakarta
│                                                        │ [ Kosongkan ]  [  BAYAR  ▸  ]   │ ← 56px, kanan-bawah
└────────────────────────────────────────────────────────┴─────────────────────────────────┘
  Panel keranjang: 400px di ≥ 1280, 360px di 1024–1279. Topbar POS memakai warna bingkai navy.
```

**Ergonomi sentuh** (aturan khusus `FocusLayout`):
- **Ukuran:**
  - Semua target ≥ **48px**; jarak antartarget ≥ 8px.
  - Tombol "BAYAR" 56px, di **kanan bawah** (zona ibu jari tangan kanan pada landscape).
- **Tanpa ketergantungan hover.**
  - Setiap info yang sekarang di Tooltip punya padanan ketukan (Popover).
  - Status tekan memakai `active:` (latar + `scale-[0.98]`), bukan `hover:`.
- **Keyboard layar tidak boleh muncul tanpa diminta.** Ini membalik pola "input selalu fokus"
  dari riset scanner-HID, karena di tablet fokus otomatis memunculkan keyboard yang menutupi
  separuh layar.
  - Input cari **tidak** auto-focus; baru fokus saat diketuk.
  - Qty dan nominal bayar memakai **Numpad di layar** (komponen generik `patterns/Numpad`) yang
    muncul di Popover, bukan keyboard sistem.
  - Input yang memang perlu keyboard memakai `inputmode` yang tepat (`numeric` / `search`).
- **Cegah gangguan sentuh:**
  - `touch-action: manipulation` (tanpa zoom ketuk-ganda / jeda 300ms).
  - `overscroll-behavior: none` (tanpa tarik-untuk-muat-ulang yang menghapus keranjang).
  - `user-select: none` di tile dan tombol.
  - Meta viewport `interactive-widget=resizes-content`.
- **Layar tetap menyala:** pakai Screen Wake Lock API bila tersedia, dilepas saat keluar dari POS.
- **Tombol layar penuh** (`⛶`, Fullscreen API) di topbar. *Tidak* memakai PWA, karena PWA sengaja
  dibuang (D4 plan paritas).
- **`Kbd` disembunyikan** di perangkat `(pointer: coarse)`. Pintasan F-key lama tetap berfungsi
  bila keyboard fisik dipasang, tetapi tidak lagi menjadi jalur utama.

**Alur pindai (kamera):**
1. Kasir mengetuk **PINDAI**. Dialog kamera layar penuh terbuka dengan kamera belakang (perilaku
   pemilihan kamera yang ada dipertahankan).
2. Setiap kode terbaca memberi umpan balik:
   - kilatan hijau di bingkai bidikan
   - getar singkat (`navigator.vibrate(40)` bila didukung)
   - chip "＋ Semen 50kg (21)" di bawah bidikan
   - barang masuk keranjang

   Kode yang sama dalam ±1,5 detik diabaikan (*debounce*), supaya satu bidikan tidak menambah
   berkali-kali.
3. Dialog **tetap terbuka** untuk barang berikutnya. Tombol besar "Selesai (n barang)" menutupnya.
   Kode tidak dikenal → chip merah "Kode 899… tidak ditemukan" tanpa menutup kamera.
4. Bila `PosScannerModal` saat ini menutup diri setelah satu pindaian, perubahan ke pindai
   beruntun adalah **perubahan alur di frontend saja** (tanpa API). Ia masuk U5-3 dan diuji di
   `pos/lib` (debounce) + E02.
5. **Syarat deployment:** kamera hanya jalan di *secure context* (HTTPS atau localhost),
   sesuai `pos/lib/camera.ts`. Tablet kasir **wajib** mengakses aplikasi lewat HTTPS. Pesan
   fallback yang ada tetap dipakai (risiko K10).

**Bagian lain:**
- **Tile produk:**
  - Isinya foto (bila ada; jika tidak, inisial di latar `muted`), nama 2 baris, harga
    `text-lg` tnum, dan stok.
  - Stok kritis diberi badge `warning`; stok habis membuat tile redup dan nonaktif, sesuai
    aturan stok yang ada.
  - Ketuk tile = tambah 1. Produk bervarian atau bersatuan ganda → Sheet pilihan (perilaku sama
    dengan sekarang).
- **Baris keranjang:**
  - Stepper `−`/`+` 48px. Ketuk angka qty membuka Numpad. Hapus lewat tombol `✕` 48px, tanpa
    gestur geser yang tersembunyi.
  - "Diskon ›" membuka Popover diskon (persen/nominal + alasan), bukan input inline di setiap
    baris.
- **Dialog Bayar** (Sheet kanan selebar panel keranjang, agar total tetap terlihat):
  - Metode bayar sebagai ToggleGroup besar.
  - Numpad untuk "Uang diterima" dengan tombol nominal cepat (Uang pas · 50.000 · 100.000 ·
    pembulatan ke atas).
  - Kembalian `text-3xl`.
  - Ini tampilan saja; logika `pos/lib/checkout.ts` tidak diubah.
- **Selesai:** layar konfirmasi dengan dua tombol besar, "Cetak struk" dan "Transaksi baru".
- **Notice izin** (sekarang bisa 3 bertumpuk) menjadi satu baris status ringkas di kepala
  keranjang; ketuk untuk rincian (Popover).
- **Refactor tanpa ubah perilaku:**
  - Logika keranjang dipecah ke `modules/pos/hooks/use-pos-cart.ts`, memakai `pos/lib/cart.ts`,
    `checkout.ts`, `holds.ts`, dan `cart-draft.ts` yang sudah teruji.
  - Tampilan dipecah ke `PosTopbar`, `ProductGrid`, `CartPanel`, `CheckoutSheet`, dan
    `ScanDialog`.
  - Spec e2e `E02-pos.spec.ts` menjadi penjaga; tambahkan proyek Playwright dengan viewport
    1280×800 + `hasTouch: true`.

### sales
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `SalesOrdersPage` | T1 | Filter utama: cari, status, tanggal; sisanya (pelanggan, kanal, termin) ke "+ Filter"; sortir di header; ekspor XLSX/PDF di dropdown "Ekspor" | 4a |
| `SalesOrderDetailPage` | T2 | Rel: Draf → Terkonfirmasi → Dikirim → Lunas → Selesai, terkait SJ/PAY/RJ; aksi utama dinamis (Konfirmasi / Kirim Barang / Catat Bayar); "Setujui Kredit" tetap Dialog; tabel barang 6 kolom + baris kedua | 4a |
| `SalesOrderFormPage` (1062) | T3 | `LineItemsEditor`; info harga (sumber harga, harga dasar) sebagai teks bantu di bawah sel harga; `StickyFooter` dengan total | 4a |

### delivery
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `DeliveriesPage` | T1 | — | 4a |
| `QueuePage` | T7 | Tabs "Siap dibuat (n) / Menunggu konfirmasi (n)" tersinkron URL; pagination | 4a |
| `DeliveryDetailPage` (776) | T2 | Rel: Draf → Dikirim → Diterima, terkait SO/PAY; seksi "Dokumen & Serah Terima" menjadi kartu kolom kanan dengan `FileDropzone` (2 input file mentah) dan pratinjau foto | 4a |
| `CreateDeliveryModal` (474, xl) | T3 | **Menjadi halaman** `/deliveries/new?from=<soId>` (U9) | 4a |

### payment
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `PaymentsPage` | T1 | `Segmented` arah (masuk/keluar) menjadi Tabs; total periode di ringkasan satu baris | 4a |
| `PaymentDetailPage` | T2 | Rel: Tercatat → Dialokasikan → (Dibatalkan), terkait dokumen yang dibayar; "Bukti Bayar" memakai `FileDropzone`; 3 gaya "Kembali" menjadi breadcrumb | 4a |
| `PaymentFormModal` (434), `OrderPaymentModal`, `SettleCreditModal` | — | Menjadi **Sheet** (butuh konteks dokumen di belakangnya); nominal memakai Input Group prefiks "Rp" + tombol "Lunasi sisa" | 4a |
| `PartyBalanceSection`, `OrderBillingCard`, `OrderHistoryCard` | — | Billing menjadi kartu "Ringkasan" T2; History menjadi kartu "Aktivitas" (timeline) | 4a |

### salesreturn / purchasereturn
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `SalesReturnsPage`, `PurchaseReturnsPage` | T1 | — | 4a / 4b |
| `SalesReturnDetailPage` (809) | T2 | Rel: Draf → Disetujui → Barang kembali → Diselesaikan, terkait SO asal/pengganti/PAY; 3 modal alur menjadi Dialog kecil atau Sheet sesuai §7.0 | 4a |
| `SalesReturnCreateModal` (824) | T3 | **Menjadi halaman** `/sales-returns/new?from=<soId>` dengan `Stepper` 3 langkah: Pilih dokumen → Pilih barang & mode (retur saja / tukar) → Tinjau | 4a |
| `PurchaseReturnDetailPage` | T2 | Rel analog; terkait PO/GR | 4b |
| `PurchaseReturnCreateModal` (459) | T3 | **Menjadi halaman** `/purchase-returns/new?from=<poId>` | 4b |
| `ReturnLinesTable` (shared) | — | Dibangun ulang di atas `LineItemsEditor` (mode "pilih qty dari sumber") | 4a |

### purchasing / goodsreceipt
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `PurchaseOrdersPage` | T1 | Sama dengan SO | 4b |
| `PurchaseOrderDetailPage` | T2 | `FlowSteps` lokal digantikan Rel Dokumen; terkait GR/PR/PAY | 4b |
| `PurchaseOrderFormPage` (994) | T3 | Kartu-per-baris 6 kolom × 9 isian menjadi `LineItemsEditor` (konsisten dengan SO) | 4b |
| `GoodsReceiptsPage` | T1 | — | 4b |
| `GoodsReceiptDetailPage` | T2 | Rel: Draf → Diterima → (Dibatalkan), terkait PO | 4b |
| `ReceiveModal` (460, xl), `SettlementFields` | T3 | **Menjadi halaman** `/goods-receipts/new?from=<poId>`; kolom "Dipesan / Sudah diterima / Terima sekarang" | 4b |

### stock
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `StockPage` (694, 3 tabel bertumpuk) | T1 + Tabs | Tabs **Saldo · Mutasi · Lokasi** tersinkron URL; tiap tab punya toolbar dan pagination sendiri | 4c |
| `BalancesPage` | T1 | Checkbox mentah menjadi `Checkbox`/chip filter ("Hanya stok kritis", "Sembunyikan nol") | 4c |
| `TransfersPage` (611) | T1 | Modal "Buat Transfer" menjadi **halaman** `/stock/transfers/new` (T3); "Pindah Lokasi" tetap **Sheet** (satu langkah) | 4c |
| `TransferDetailPage` | T2 | Rel: Draf → Dikirim → Diterima | 4c |
| `AdjustmentsPage` | T3 ringkas | Form berbaris (`LineItemsEditor` kolom: produk, lokasi, qty sistem, qty fisik, selisih); tambah state loading. Bila endpoint riwayat koreksi sudah ada, tampilkan tab "Riwayat"; bila tidak, **jangan** menambah endpoint | 4c |
| `DamagedPage` (496) | T7 | Tabs status; 3 modal aksi (Catat, Pulihkan, Hapusbukukan) menjadi Sheet/Dialog sesuai §7.0; pagination | 4c |
| `OpeningPage` (stok) | T9 | `ImportWizard` | 4c |
| `StockItemPage` | T2 varian | Header produk + 3 KpiCard + Tabs (Per lokasi · Kartu stok) | 4c |
| `ProductPicker`, `LocationSelect`, `StockLocationSelect` | — | Di atas Combobox baru | 4c |

### products / party
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `ProductsPage` | T1 | Toggle tampilan **Tabel / Galeri** (foto sudah ada); thumbnail 32px di tabel | 4d |
| `ProductDetailPage` | T4 | Tabs: Ringkasan · Harga · Varian · Foto · Stok; QR tetap Dialog | 4d |
| `ProductFormPage` (693, ±19 isian) | T3 tanpa baris | `FormSection`: Identitas · Satuan · Harga · Varian · Info tambahan; navigasi seksi sticky di kiri; `StickyFooter` | 4d |
| `CategoriesPage` | T4 | Tabel **pohon**; buat/ubah lewat Sheet | 4d |
| `ImportPage` | T9 | `ImportWizard` | 4d |
| `CustomersPage`, `SuppliersPage` | T1/T4 | `PartyFormModal` menjadi Sheet | 4d |
| `CustomerDetailPage`, `SupplierDetailPage` | T4 | Tabs: Ringkasan · Alamat · Transaksi · Saldo; `InfoRow` ganda menjadi `DescriptionList`; empty state yang hilang ditambahkan | 4d |
| `MemberTypesPage` | T4 | Sheet; pagination | 4d |

### finance
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `PostingPage` (614, 5 panel) | T7 | Tabs: **Antrian · Tutup Hari · Hasil · Posting Manual**; checkbox mentah menjadi DataTable `selectable` + bar aksi massal "Posting n dokumen"; pratinjau jurnal memakai Sheet | 4e |
| `ExpensesPage`, `ExpenseDetailPage` | T1/T2 | `ExpenseFormModal` menjadi Sheet; `CancelExpenseModal` menjadi Dialog | 4e |
| `JournalsPage`, `JournalDetailPage` | T1/T2 | Detail jurnal: tabel Debit/Kredit seimbang + baris total; badge "Seimbang ✓" | 4e |
| `ManualJournalModal` (xl) | T3 | **Menjadi halaman** `/finance/journals/new`; `LineItemsEditor` kolom Akun/Debit/Kredit/Keterangan; footer menampilkan selisih D−K (merah bila ≠ 0, tombol simpan nonaktif) | 4e |
| `GeneralLedgerPage` | T5 | Header sticky (200 baris); saldo berjalan; pilih akun lewat Combobox | 4e |
| `ReportsPage` (1305, 8 seksi) | T5 | **Dipecah per laporan**: `reports/TrialBalance`, `ProfitLoss`, `BalanceSheet`, `TaxSummary`, `InventoryValue`, `Margin`, `ReceivablesPayables`, masing-masing di rute yang **sudah ada**; `Tabs variant="nav"` di atas berisi link, sehingga sorotan sidebar benar. Seksi "kas" tanpa rute: **putuskan saat eksekusi**, beri rute `/finance/reports/cash` atau gabungkan ke Neraca | 4e |
| `TaxDetailPage` | T5 | Judul topbar diperbaiki (§6.2) | 4e |
| `TaxAdjustmentsPage` | T1 + Sheet | Form menjadi Sheet; pagination | 4e |
| `PeriodsPage` | T8 | Daftar periode sebagai timeline bulan (terbuka/terkunci, ikon gembok); kunci/buka lewat ConfirmDialog | 4e |
| `OpeningPage` (keuangan), `CutoverCard` | T9 | `Stepper` cutover | 4e |
| `AccountsPage` | T4 | **Pohon akun** (bagan akun); `AccountFormModal`/`MappingModal` menjadi Sheet | 4e |
| `IntegrityPage`, `OrphansCard` | T7 | KpiCard status (hijau "Semua seimbang" / merah "n temuan") + tabel temuan | 4e |

### reporting / ops / audit
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `ReportingPage` "Tren & Stok" | T5 | **Tambah grafik tren** (data sudah ada, sekarang hanya tabel) memakai Chart shadcn | 4f |
| `OrdersReportPage`, `StockReportPage` | T5 | `Segmented` menjadi Tabs/ToggleGroup | 4f |
| `OpsIntegrityPage` | T7 | Analog `IntegrityPage` (pola sama, komponen sama) | 4f |
| `AuditPage` | T1 | `<table>` mentah menjadi DataTable; baris expand menjadi **Sheet detail** (NN/g) berisi diff sebelum/sesudah ala kode | 4f |

### assistant
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `SetupPage` (633) | T7 | Stepper status koneksi (Belum terhubung → Pindai QR → Terhubung); Tabs: Nomor diizinkan · Konsol uji; konsol uji ala chat sederhana di kartu (bukan bubble melayang); hapus inline `style` | 4g |
| `ConfigPage` | T8 | — | 4g |

### users / branches / company
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `UsersPage` (723) | T4 | Modal user menjadi Sheet; role memakai Checkbox list; cabang memakai RadioGroup "Semua cabang / Pilih cabang" + Checkbox; ganti kata sandi tetap Dialog; limit 10 menjadi 25 | 4g |
| `RolesPage` (413) | T4 | **Matriks izin dikelompokkan per modul** (kelompok dari prefiks kunci, mis. `sales.*`), dengan pencarian, "Pilih semua di modul", dan hitungan "12/40 izin"; tampil di Sheet lebar (640px) atau halaman `/roles/:id`. Label kunci izin tetap dari katalog API | 4g |
| `BranchesPage` | T4 | Modal 7 isian menjadi Sheet | 4g |
| `CompanyPage` (22 isian), `DocumentProfileForm`, `OperationalSettingsForm` | T8 | Navigasi seksi: Profil · Dokumen & Cetak · Operasional; tautan ke Kalibrasi Kertas sebagai Item berikon, bukan tombol | 4g |

### print
| Halaman | T | Bentuk baru | Gel. |
|---|---|---|---|
| `SalesInvoicePrintPage`, `DeliveryNotePrintPage`, `PaymentReceiptPrintPage`, `PosThermalReceiptPage`, `ProductLabelsPrintPage` | T12 | **Kertas tidak berubah.** Toolbar `PrintLayout` memakai komponen baru; loading memakai Spinner | 4g |
| `PaperCalibrationPage` (di dalam AppShell) | T8 | Pratinjau kertas di kanan, kontrol di kiri; `<table>` mentah menjadi DataTable | 4g |

---

## 9. Rencana eksekusi

Setiap fase **bisa dirilis sendiri** dan diakhiri `npm run verify` hijau + e2e hijau. Ukuran: S ≤ ½
hari, M ≈ 1–2 hari, L ≈ 3–5 hari (perkiraan kasar satu pengembang).

### Fase 0 — Baseline & pagar (M)

| ID | Task | Hasil / kriteria terima |
|---|---|---|
| U0-1 | Spec `apps/e2e/specs/V00-visual-baseline.spec.ts`: login dengan seed e2e yang ada, kunjungi setiap rute menu + satu detail per jenis dokumen, simpan screenshot 1366×800 dan 390×844 | Folder `docs/plan/redesain-ui/baseline/` berisi screenshot "sebelum" |
| U0-2 | `scripts/ui-ratchet.mjs` + `scripts/ui-migration-status.json`, dihitung per modul: `text-[`/`rounded-[`/`max-w-[` arbitrer, `Memuat…` manual, `<table` di luar `print/`, `type="checkbox"`/`"radio"`/`"file"` mentah, `<Link…><Button`, `Segmented` di halaman, hex di luar `print/` dan `theme/` | Angka hari ini tercatat; skrip gagal bila angka naik |
| U0-3 | Daftarkan ratchet di `scripts/verify.mjs` (langkah web) | `npm run verify` menjalankannya |

### Fase 1 — Pondasi (L)

| ID | Task | Kriteria terima |
|---|---|---|
| U1-1 | ~~Konfirmasi keputusan dengan pemilik~~ **Selesai 2026-10-16** (§12) | Jawaban sudah tercatat di §3 dan §12 |
| U1-2 | Pasang dependensi §5.2. `components.json`. `shared/lib/cn.ts` | `npm run build -w apps/web` lulus |
| U1-3 | Tulis ulang `theme/theme.css`: `@import "tailwindcss"` (**preflight ON**), `@import "tw-animate-css"`, `:root` (§4.1, termasuk `--sidebar-*` bingkai navy), `@theme inline` (warna, radius, font, skala teks §4.3, z-index), **alias kelas lama** (§4.1 tabel pemetaan), `data-density`, `scroll-padding-top` | Semua halaman tetap ter-render, dibandingkan dengan baseline U0-1. Selisih hanya karena preflight/font dan dicatat |
| U1-4 | Font lokal (U6): import `@fontsource-variable/*` di `main.tsx`; hapus preconnect Google Fonts di `index.html`; `theme-color` mengikuti `--sidebar` | Tidak ada request ke `fonts.googleapis.com` (cek tab Network) |
| U1-5 | `theme/colors.ts` diselaraskan ke token baru (untuk kanvas/cetak) | — |
| U1-6 | `npx shadcn add` primitif ke `shared/components/primitives/`: button, badge, input, textarea, native-select, field, input-group, label, checkbox, radio-group, switch, dialog, alert-dialog, sheet, popover, command, dropdown-menu, tooltip, tabs, toggle-group, table, pagination, breadcrumb, sidebar, skeleton, spinner, empty, alert, kbd, item, separator, scroll-area, avatar, progress, collapsible, chart, sonner | Primitif tersedia, belum dipakai modul |
| U1-7 | ESLint `no-restricted-imports`: `@/shared/components/primitives/*` dilarang dari `src/modules/**` dan `src/app/**` | `npm run lint` menegakkannya |
| U1-8 | Halaman cetak: verifikasi 5 halaman `/print/*` setelah preflight ON (margin `h1`/`p`/`table`, ukuran `@page`). Bila bergeser, tambahkan reset lokal di `print/styles.ts` | Screenshot cetak = baseline (selisih 0 pada area kertas) |

### Fase 2 — Ganti mesin fasad (L)

| ID | Task | Kriteria terima |
|---|---|---|
| U2-1 | Button/Badge/Input/Native Select/Field/PasswordInput/Notice ke primitif (§5.3) | 16 file test vitest lulus tanpa diubah |
| U2-2 | Modal → Dialog, ConfirmDialog → AlertDialog | Fokus terkunci + Esc + label aksesibel |
| U2-3 | Toast → Sonner di balik `toast.store` (API tetap) | Semua pemanggil `toast.*` tidak diubah |
| U2-4 | DataTable di atas TanStack: header sticky, `loading`/`error`/`onRetry`, `onRowClick`, `sortable`, `density`, visibilitas kolom (persist `localStorage`), `selectable` + `bulkActions`; Pagination + "baris per halaman" | Semua daftar menampilkan header sticky + skeleton otomatis bila `loading` diteruskan |
| U2-5 | SearchSelect / AsyncSearchSelect / HierarchicalSelect → Combobox | Test kedua komponen diperbarui; navigasi keyboard ↑↓ Enter Esc |
| U2-6 | Segmented → ToggleGroup (fasad); `Tabs` baru (ui) | — |
| U2-7 | Komponen baru §5.4 lapisan `ui` dan `patterns`: Page, DescriptionList, DocumentRail, Stepper, ErrorState, Skeleton set, FormSection, StickyFooter, FileDropzone, Money/Qty/DocNumber, KpiCard, Toolbar + FilterChips | Masing-masing punya test vitest dasar (render + a11y role) |
| U2-8 | `statusTone()` peta baru §4.2 + nada `muted` | `format.test.ts` diperbarui |
| U2-9 | `LineItemsEditor` (generik, keyboard) + test interaksi (Enter baris baru, Tab, hapus, total) | Test lulus |

> Setelah Fase 2, seluruh aplikasi **sudah tampil dengan DS v2** (warna, huruf, kontrol, tabel,
> dialog, toast), meski komposisi halaman belum dirapikan. Ini titik rilis pertama yang terasa
> bagi pengguna.

### Fase 3 — Kerangka aplikasi (M)

| ID | Task | Kriteria terima |
|---|---|---|
| U3-1 | `AppShell` (shadcn Sidebar inset) menggantikan `AppLayout.tsx`; pemilih cabang di header sidebar; `nav-user` di footer; header 48px + breadcrumb | Satu `<h1>` per halaman; e2e E01 & E10 lulus |
| U3-2 | `registry.tsx`: ikon unik, label "Saldo Awal Stok/Akun", `PAGE_TITLES` yang hilang, `getBreadcrumb`, `QUICK_ACTIONS`; `usePageMeta` | Tidak ada judul "Halaman" lagi |
| U3-3 | Command palette (navigasi + aksi + terakhir dibuka) | Ctrl+K dari halaman mana pun; hanya item berizin |
| U3-4 | Auth: Login, Pilih Cabang, 403/404 (T11); `LoadingState` Indonesia; `apps/e2e/helpers/labels.ts` `L.login` diperbarui | E01 lulus |
| U3-5 | Mobile: sidebar off-canvas, header ringkas | Diuji di 390×844 |

### Fase 4 — Penerapan template per modul (L × 7 gelombang)

Urutan mengikuti frekuensi pakai harian. Setiap gelombang: terapkan template §7 + perubahan §8,
pindahkan alur modal ke halaman/sheet sesuai U9, ratchet turun, e2e terkait lulus, dan screenshot
"sesudah" disimpan berdampingan dengan baseline.

| Gel. | Modul | Penjaga e2e |
|---|---|---|
| **4a** | sales, delivery, payment, salesreturn (+ `ReturnLinesTable`) | E03, E05 |
| **4b** | purchasing, goodsreceipt, purchasereturn | E04, E05 |
| **4c** | stock | E06 |
| **4d** | products, party | E07 |
| **4e** | finance (termasuk pemecahan `ReportsPage`) | E09 |
| **4f** | dashboard, reporting, ops, audit | E10 |
| **4g** | users, branches, company, assistant, print (toolbar), kalibrasi | E01, E08 |

Akhir Fase 4:
- alias kelas warna lama dihapus dari `theme.css`
- `ActionRow` dan fasad `Segmented` dihapus
- ratchet untuk semua kategori = 0, kecuali pengecualian yang dicatat di
  `ui-migration-status.json`

### Fase 5 — Mode Kasir POS untuk tablet (L)

| ID | Task | Kriteria terima |
|---|---|---|
| U5-1 | `FocusLayout` + rute `/pos` dipindah keluar `AppShell` (penjaga sama); topbar POS berwarna bingkai navy; ajakan "Putar tablet" di lebar < 1024 | Tidak ada sidebar di POS; tombol "←" kembali ke dashboard |
| U5-2 | Pecah `PosPage` → `use-pos-cart` + `PosTopbar`, `ProductGrid`, `CartPanel`, `CheckoutSheet`, `ScanDialog` (tanpa ubah perilaku) | E02 lulus **sebelum** perubahan visual |
| U5-3 | Tampilan §8 pos: tile, keranjang tetap di kanan, total besar, `Numpad`, Sheet bayar + nominal cepat, layar selesai, pindai kamera beruntun + debounce + umpan balik getar | E02 lulus; test `pos/lib` untuk debounce pindai |
| U5-4 | Ergonomi sentuh: target ≥ 48px, `touch-action`, `overscroll-behavior`, Wake Lock, tombol layar penuh, `Kbd` disembunyikan di `pointer: coarse`, tanpa auto-focus input | Proyek Playwright 1280×800 + 1024×768 dengan `hasTouch: true` lulus E02 |
| U5-5 | **Uji lapangan di tablet kasir sungguhan** via HTTPS: pindai 20 barcode campuran (EAN-13, Code128, QR label `/print/labels`) dalam cahaya toko; cetak struk lewat jembatan Android | Waktu rata-rata per pindaian dicatat; 0 tambah ganda; struk tercetak |

### Fase 6 — Penyempurnaan (M)

Tema gelap **tidak** dikerjakan (U5, ditunda pemilik).

| ID | Task |
|---|---|
| U6-1 | Pilihan kepadatan (Ringkas / Standar / Lega) di `nav-user` |
| U6-2 | Command palette "lompat ke dokumen" (§6.3) |
| U6-3 | Saved view per daftar (simpan kombinasi filter + kolom sebagai "Tampilan", `localStorage`) |
| U6-4 | Audit token: `grep` memastikan tidak ada warna mentah di komponen, sehingga tema gelap kelak cukup menambah `.dark` (U5) |

### Fase 7 — QA & penutupan (M)

| ID | Task | Kriteria terima |
|---|---|---|
| U7-1 | `@axe-core/playwright` di spec V00 untuk setiap rute | 0 pelanggaran *serious/critical* |
| U7-2 | Uji keyboard penuh di 3 alur back-office (buat SO → kirim → bayar; posting; buat PO → terima) + uji **sentuh** penuh POS di tablet | Back-office tanpa mouse; POS tanpa keyboard fisik |
| U7-3 | Responsif 390 / 768 / 1366 / 1920 untuk setiap template back-office; POS 1024×768 / 1280×800 / 1340×800 landscape | Tanpa scroll horizontal halaman (tabel boleh scroll di dalam kartu) |
| U7-4 | Anggaran bundle: bandingkan `vite build` sebelum/sesudah; primitif berat (Command, Chart) hanya di chunk lazy yang memakainya | Kenaikan chunk awal ≤ 60 KB gzip |
| U7-5 | Perbarui `knowledge/FRONTEND_GUIDE.md` (DS v2, aturan U3/U9, template), ADR baru `knowledge/decisions/ADR-00xx-design-system-v2.md` | Dokumen digabung |
| U7-6 | Screenshot "sesudah" lengkap + catatan perbandingan di folder plan | Dapat ditinjau pemilik |
| U7-7 | **Pasang logo resmi** (diserahkan pemilik di akhir): varian terang untuk bingkai navy dan varian gelap untuk kanvas/login; `favicon.svg` + PNG 32/180/512; ganti monogram sementara di kepala sidebar, login, `LoadingState`, topbar POS; perbarui `theme-color`. Kop dokumen cetak **tidak** disentuh (logo perusahaan di cetak berasal dari pengaturan Perusahaan) | Logo tampil tajam di 1× dan 2×; kontras logo ≥ 3:1 terhadap bingkai navy |

---

## 10. Definisi selesai per halaman

Sebuah halaman dianggap selesai bila **semua** butir terpenuhi:

- [ ] Memakai tepat satu template §7. Aksi berada di tempat yang ditetapkan template.
- [ ] Hanya satu `<h1>`. Breadcrumb benar. Tidak ada tombol "Kembali" terpisah.
- [ ] Loading memakai skeleton. Galat memakai `ErrorState` dengan `message` API asli. Kosong
      memakai Empty + aksi.
- [ ] Tidak ada ukuran arbitrer, hex, `<table>` mentah, input checkbox/radio/file mentah, atau
      `<Link><Button>` (ratchet = 0 untuk file itu).
- [ ] Uang/qty memakai `Money`/`Qty` (tnum, rata kanan). Nomor dokumen memakai `DocNumber`.
- [ ] Status memakai `StatusBadge` dengan peta §4.2.
- [ ] Bisa dioperasikan penuh dengan keyboard. Fokus terlihat dan tidak tertutup header/footer
      sticky.
- [ ] Layak di 390px (tanpa scroll horizontal halaman).
- [ ] Teks 100% Bahasa Indonesia, kalimat biasa (*sentence case*), tombol berbentuk kata kerja
      yang konsisten. Contoh: "Simpan" menghasilkan toast "Tersimpan"; "Konfirmasi order"
      menghasilkan "Order dikonfirmasi".
- [ ] Test vitest dan e2e terkait lulus. Screenshot sebelum/sesudah tersimpan.

---

## 11. Risiko & mitigasi

| # | Risiko | Mitigasi |
|---|---|---|
| K1 | **Preflight ON menggeser tampilan** di halaman yang belum dimigrasi (margin heading, list, tombol) | Fase 1 berhenti untuk membandingkan screenshot baseline; alias kelas lama; perbaikan kecil di fasad, bukan di halaman |
| K2 | **Cetak fisik bergeser** karena preflight/font | U1-8: reset lokal di `print/styles.ts`; bandingkan piksel area kertas; uji cetak fisik ulang 1 dokumen per profil kertas sebelum rilis Fase 1 |
| K3 | **Test rusak** karena Combobox, Checkbox Radix (`role=checkbox` di `<button>`), atau Dialog | Native select dipertahankan (U10); test berbasis role tetap valid; hanya 2 test komponen select yang diperbarui secara sadar |
| K4 | **E2E bergantung teks** | Label menu dipertahankan; hanya `L.login` berubah dan diperbarui dalam PR yang sama (U3-4) |
| K5 | **Memindah modal ke halaman mengubah kebiasaan pengguna** | Tombol pembuka tetap di tempat yang sama, hanya tujuannya halaman; `?from=` mengisi sumber otomatis; draf otomatis tetap berlaku. Diputuskan analis atas delegasi pemilik (§12, Q4) |
| K6 | **Ruang lingkup merembet ke fitur baru** (delta KPI, riwayat koreksi, tahan transaksi, cari dokumen) | U11: hanya data yang sudah ada; kebutuhan data baru dicatat sebagai tiket terpisah, tidak dikerjakan di plan ini |
| K7 | **Bundle membesar** | Primitif hanya di chunk lazy pemakainya; U7-4 |
| K8 | **Kode shadcn menyimpang dari upstream** setelah dimodifikasi | Modifikasi hanya di lapisan fasad/patterns; `primitives/` dibiarkan dekat aslinya agar `shadcn diff` tetap berguna |
| K9 | **Pengerjaan bentrok dengan program paritas** (ETL, cutover) | Redesain hanya menyentuh `apps/web` (+ `apps/e2e` dan `scripts/`); tidak ada perubahan API/DB. Jadwalkan gelombang 4e (finance) di luar minggu gladi cutover |
| K10 | **Kamera tablet tidak bisa dibuka** karena aplikasi diakses lewat `http://` IP LAN (bukan *secure context*) | Syarat deployment: tablet kasir mengakses lewat **HTTPS** (sertifikat internal / reverse proxy di host, sesuai `docs/DEPLOYMENT.md`). Fallback input manual yang ada tetap tampil dengan pesan dari `pos/lib/camera.ts`. Dicek di U5-5 |
| K11 | **Pindai kamera lambat atau gagal** di cahaya toko / barcode kecil | Bidikan besar layar penuh, kamera belakang, debounce; uji lapangan U5-5 sebelum rilis; input manual + pencarian tetap jadi jalur cadangan |
| K12 | **Sidebar gelap terasa "kuno" lagi** bila dieksekusi setengah hati | Elemen pembeda wajib ada semua: kanvas inset (bukan menempel), navy `#101e33` (bukan hampir hitam), pil aktif, tanpa gradient. Dibandingkan dengan baseline di U0-1/U7-6 |

---

## 12. Keputusan pemilik (2026-10-16)

| # | Pertanyaan | Jawaban pemilik | Dampak di plan |
|---|---|---|---|
| Q1 | Warna brand | **Tetap navy `#1e3a5f`** | §4.1 dihitung ulang di atas navy; nama DS menjadi "Bingkai Navy" |
| Q1b | Sidebar terang atau gelap? | **Gelap navy, dimodernkan** | U4, §6.1: bingkai navy *inset*, `#101e33`, pil aktif, teks 8,95:1; risiko K12 |
| Q2 | Radix atau Base UI | **Radix** | U1 |
| Q3 | Tema gelap | **Ditunda** | U5; Fase 6 tanpa tema gelap; nilai acuan di Lampiran E |
| Q4 | Pindah alur buat dari modal ke halaman? | **Diserahkan ke analis**, "pilih yang terbaik menurut riset" | U9 + §7.0 dipertahankan (lihat alasan di bawah) |
| Q5 | Perangkat kasir | **Tablet layar sentuh, landscape, pemindai kamera tablet; tablet hanya untuk POS** | U8, §8 pos, Fase 5 (U5-1…U5-5), risiko K10–K11 |
| Q6 | Logo | **Sudah ada, menyusul di akhir** | Monogram sementara selama Fase 1–6; U7-7 memasang logo resmi |

**Alasan keputusan Q4 (analis).** Riset NN/g dan pola Stripe/Linear sepakat pada tiga hal:
- Modal cocok untuk tugas pendek satu langkah yang tidak membutuhkan konteks.
- Panel samping (Sheet) cocok untuk melihat atau mengubah satu rekaman sambil tetap melihat
  daftar atau dokumennya.
- Alur multi-langkah dengan tabel baris barang butuh **halaman sendiri**.

Halaman sendiri memberi empat hal yang tidak bisa diberikan modal xl:
- ruang untuk `LineItemsEditor` dan total sticky
- URL yang bisa dibagikan dan dibuka ulang
- draf otomatis per rute
- tombol Kembali browser yang berperilaku benar

`SalesReturnCreateModal` (824 baris, mode retur/tukar + pratinjau) dan `ReceiveModal` (460 baris)
adalah contoh alur yang sudah melampaui ukuran modal. Karena itu:
- Retur Jual/Beli, Surat Jalan, Penerimaan, Transfer, dan Jurnal Manual pindah ke halaman.
- CRUD master data dan Catat Pembayaran memakai Sheet.
- Konfirmasi dan isian ≤ 6 tetap Dialog.

**Belum ada pertanyaan terbuka.** Hal kecil yang memang diputuskan saat eksekusi dicatat di
tempatnya:
- rute seksi "kas" di `ReportsPage` (§8 finance)
- nama ikon lucide final (§6.2)
- versi paket (§5.2)

---

## Lampiran A — Struktur file target

```
apps/web/src/
├─ theme/
│  ├─ theme.css            ← token :root + @theme inline (DS v2)
│  └─ colors.ts            ← cermin hex (kanvas/cetak saja)
├─ shared/
│  ├─ lib/cn.ts
│  ├─ components/
│  │  ├─ primitives/       ← shadcn (dilarang diimpor modul)
│  │  ├─ ui/               ← fasad (API lama + tambahan)
│  │  ├─ patterns/         ← Page, DocumentRail, DataTable, LineItemsEditor, ImportWizard, …
│  │  └─ feedback/         ← ErrorState, Empty, Skeleton set
│  ├─ layouts/
│  │  ├─ AppShell.tsx      ← pengganti AppLayout.tsx
│  │  ├─ FocusLayout.tsx   ← POS
│  │  └─ AuthLayout.tsx
│  └─ stores/page-meta.store.ts
└─ modules/<m>/lib/<doc>-rail.ts   ← definisi langkah Rel Dokumen per jenis dokumen
components.json
scripts/ui-ratchet.mjs, scripts/ui-migration-status.json
apps/e2e/specs/V00-visual-baseline.spec.ts
```

## Lampiran B — Cuplikan `theme.css` (kerangka, bukan final)

```css
@import "tailwindcss";
@import "tw-animate-css";

:root {
  --radius: 0.5rem;
  /* Kanvas & teks */
  --background: oklch(0.985 0.003 256);
  --foreground: oklch(0.21 0.025 262);
  --card: oklch(1 0 0);
  --card-foreground: var(--foreground);
  --popover: oklch(1 0 0);
  --popover-foreground: var(--foreground);
  --muted: oklch(0.965 0.005 256);
  --muted-foreground: oklch(0.5 0.022 258);
  --secondary: var(--muted);
  --secondary-foreground: var(--foreground);
  --border: oklch(0.915 0.007 256);
  --input: oklch(0.66 0.016 256);
  /* Brand navy */
  --primary: oklch(0.346 0.074 256);          /* #1e3a5f */
  --primary-hover: oklch(0.3 0.066 256);
  --primary-foreground: oklch(1 0 0);
  --accent: oklch(0.955 0.018 256);
  --accent-foreground: var(--primary);
  --ring: oklch(0.55 0.11 254);
  /* Status */
  --success: oklch(0.5 0.12 155);   --success-soft: oklch(0.96 0.035 155);
  --warning: oklch(0.53 0.12 60);   --warning-soft: oklch(0.965 0.045 85);
  --destructive: oklch(0.52 0.19 27); --destructive-soft: oklch(0.96 0.025 25);
  --info: oklch(0.5 0.1 225);       --info-soft: oklch(0.96 0.025 225);
  /* Chart */
  --chart-1: oklch(0.48 0.1 256);   --chart-2: oklch(0.65 0.11 185);
  --chart-3: oklch(0.75 0.14 75);   --chart-4: oklch(0.58 0.15 320);
  --chart-5: oklch(0.66 0.03 256);
  /* Bingkai navy */
  --sidebar: oklch(0.235 0.045 258);           /* #101e33 */
  --sidebar-foreground: oklch(0.8 0.025 256);
  --sidebar-muted: oklch(0.68 0.03 256);
  --sidebar-strong: oklch(0.97 0.006 256);
  --sidebar-accent: oklch(0.3 0.05 258);
  --sidebar-accent-foreground: var(--sidebar-strong);
  --sidebar-active: oklch(0.33 0.06 257);
  --sidebar-primary: oklch(0.78 0.09 250);
  --sidebar-primary-foreground: var(--sidebar);
  --sidebar-border: oklch(0.3 0.04 258);
  --sidebar-ring: oklch(0.72 0.1 250);
}
/* .dark { … } — DITUNDA (U5). Acuan nilai: Lampiran E. */

@theme inline {
  --font-sans: "Inter Variable", ui-sans-serif, system-ui, sans-serif;
  --font-display: "Plus Jakarta Sans Variable", "Inter Variable", sans-serif;
  --font-mono: "Geist Mono Variable", ui-monospace, monospace;
  --text-2xs: 0.6875rem; --text-2xs--line-height: 1rem;
  --text-xs: 0.75rem;    --text-xs--line-height: 1rem;
  --text-sm: 0.8125rem;  --text-sm--line-height: 1.25rem;
  --text-base: 0.875rem; --text-base--line-height: 1.375rem;
  /* … lg, xl, 2xl, 3xl, 4xl sesuai §4.3 */
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  /* … semua token di atas dipetakan ke --color-* … */
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
  /* Alias sementara (dihapus akhir Fase 4): */
  --color-ink: var(--foreground);
  --color-brand: var(--primary);
  --color-hairline: var(--border);
  /* … tabel pemetaan §4.1 … */
}
```

## Lampiran C — Validasi kontras

Nilai §4.1 dihasilkan dan diukur dengan skrip Node kecil: konversi OKLCH → OKLab → sRGB linear →
sRGB, lalu luminans relatif dan rasio kontras WCAG 2.x. Saat Fase 1 (dan kelak saat tema gelap dibuka), ulangi pengukuran untuk
setiap pasangan teks/latar yang benar-benar dipakai (badge, alert, chart, bingkai navy), atau
gunakan axe (U7-1). Ambang:
- teks normal ≥ 4,5:1
- teks besar dan batas komponen UI (border input, cincin fokus) ≥ 3:1

## Lampiran D — Sumber riset

- shadcn/ui:
  - [Theming](https://ui.shadcn.com/docs/theming)
  - [Tailwind v4](https://ui.shadcn.com/docs/tailwind-v4)
  - [Changelog](https://ui.shadcn.com/docs/changelog), termasuk
    [Base UI default (Jul 2026)](https://ui.shadcn.com/docs/changelog/2026-07-base-ui-default),
    [shadcn create (Des 2025)](https://ui.shadcn.com/docs/changelog/2025-12-shadcn-create), dan
    [komponen baru (Okt 2025)](https://ui.shadcn.com/docs/changelog/2025-10-new-components)
  - Komponen: [Sidebar](https://ui.shadcn.com/docs/components/sidebar),
    [Data Table](https://ui.shadcn.com/docs/components/data-table),
    [Chart](https://ui.shadcn.com/docs/components/chart)
  - [Blocks](https://ui.shadcn.com/blocks)
- Estetika Lovable/v0/Bolt:
  [annaarteeva.substack.com](https://annaarteeva.substack.com/p/design-systems-lovable-bolt-v0-and)
  · [designrevision.com](https://designrevision.com/shadcn-themes/default)
- Linear:
  [How we redesigned the Linear UI](https://linear.app/blog/how-we-redesigned-the-linear-ui)
  · [Behind the latest design refresh](https://linear.app/now/behind-the-latest-design-refresh)
- Tabel data:
  [NN/g — Data Tables](https://www.nngroup.com/articles/data-tables/)
  · [Carbon — Data table usage](https://carbondesignsystem.com/components/data-table/usage/)
  · [SaaS data table UX patterns](https://www.saasui.design/blog/saas-data-table-ux-patterns)
- Angka & visualisasi:
  [Uxcel — numbers alignment](https://app.uxcel.com/courses/ui-components-best-practices/tables-best-practices-356/numbers-alignment-1984)
  · [dash.fi data-viz](https://brand.dash.fi/foundations/data-viz)
- WCAG 2.2:
  [TetraLogical](https://tetralogical.com/blog/2023/10/05/whats-new-wcag-2.2)
  · [Deque](https://dequeuniversity.com/resources/wcag-2.2)
- POS:
  [UXmatters — Designing point-of-sale systems](https://www.uxmatters.com/mt/archives/2018/04/designing-point-of-sale-systems.php)
- Huruf:
  [Inter releases](https://github.com/rsms/inter/releases)
  · [Geist Mono](https://vercel.com/font/mono)
  · [Plus Jakarta Sans — Tokotype](https://www.tokotype.com/custom-fonts/plusjakarta)
  · [Fontsource](https://fontsource.org/fonts/plus-jakarta-sans/about)
- Tren 2026:
  [saasframe.io](https://www.saasframe.io/blog/the-anatomy-of-high-performance-saas-dashboard-design-2026-trends-patterns)
  · [925studios.co](https://925studios.co/blog/saas-dashboard-design-examples-2026)

**Belum terverifikasi dari sumber primer**, dan wajib dicek ulang lewat Context7 saat Fase 1:
- versi TanStack Table yang dipakai panduan Data Table shadcn
- perilaku `cv11` Inter
- tanggal refresh Linear

Context7 MCP belum terotorisasi di sesi riset ini.

## Lampiran E — Acuan tema gelap (DITUNDA, U5)

Tidak dibangun dalam plan ini. Nilai netral dan status di bawah sudah divalidasi kontrasnya dan
disimpan agar tema gelap kelak cukup menambah blok `.dark`.

**Wajib dihitung ulang saat tema gelap dibuka:**
- `--primary`, `--accent`, dan `--ring` gelap. Nilai di bawah masih memakai hue nila (262) dari
  draf awal; turunkan ke navy-terang (hue ±250–256), lalu ukur ulang.
- `--chart-*` versi gelap.

**Draf nilai `.dark`**

| Token | OKLCH | ≈ Hex | Kontras |
|---|---|---|---|
| `--background` | `oklch(0.17 0.012 262)` | `#0d0f15` | — |
| `--card` | `oklch(0.205 0.014 262)` | `#14171e` | — |
| `--sidebar` | `oklch(0.155 0.012 262)` | `#090c11` | — |
| `--foreground` | `oklch(0.96 0.005 255)` | `#eff2f5` | 17,03 ✔ |
| `--muted` | `oklch(0.25 0.015 262)` | `#1e2229` | — |
| `--muted-foreground` | `oklch(0.72 0.02 258)` | `#9da5b1` | 7,23 ✔ |
| `--border` | `oklch(0.30 0.016 262)` | `#292e36` | dekoratif |
| `--input` | `oklch(0.53 0.02 262)` | `#666c78` | 3,39 ✔ |
| `--primary` | `oklch(0.68 0.15 262)` | `#6496f4` | teks gelap di atasnya 6,55 ✔ |
| `--primary-foreground` | `oklch(0.17 0.03 262)` | `#080f1c` | — |
| `--accent` / `-foreground` | `0.29 0.06 262` / `0.84 0.08 262` | `#1a2b49` / `#aecbff` | 8,66 ✔ |
| `--ring` | `oklch(0.70 0.14 262)` | `#6d9df5` | 6,63 ✔ |
| `--success` / `-soft` | `0.75 0.14 155` / `0.28 0.05 155` | `#59c886` / `#12301e` | 6,81 ✔ |
| `--warning` / `-soft` | `0.80 0.13 80` / `0.30 0.05 70` | `#e9b452` / `#3e290f` | 7,27 ✔ |
| `--destructive` / `-soft` | `0.72 0.17 25` / `0.30 0.07 25` | `#fd736d` / `#4b1d1b` | 5,25 ✔ |
| `--info` / `-soft` | `0.76 0.10 225` / `0.29 0.045 225` | `#63bfe1` / `#0d303c` | 6,67 ✔ |
