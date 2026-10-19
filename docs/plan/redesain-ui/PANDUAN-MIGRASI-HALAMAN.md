# Panduan migrasi halaman ke DS v2 "Bingkai Navy"

Dokumen kerja untuk Fase 4/5 di [PLAN.md](PLAN.md). Fase 0–3 selesai: token, fasad komponen, AppShell,
auth. Panduan ini menjelaskan **cara memigrasi satu halaman** supaya semua modul tampak dan terasa sama.

## 0. Baca dulu

- [PLAN.md](PLAN.md): §2.2 prinsip, §4.2 arti status, §7 template (T1–T12), §8 isi modul Anda, §10 definisi selesai.
- `apps/web/src/shared/components/` — **baca file komponennya** sebelum memakainya; API di bawah hanya ringkasan.

## 1. Kit komponen

Impor **hanya** dari fasad. ESLint melarang `@/shared/components/primitives/*` di `modules/` dan `app/`.

```ts
import { Button, Badge, StatusBadge, PageHeader, SectionCard, SummaryCard, DataTable, Pagination, Modal,
  ConfirmDialog, FormSheet, FormField, TextInput, TextArea, SelectInput, DateInput, SearchSelect,
  AsyncSearchSelect, HierarchicalSelect, SearchInput, MultiToggle, DateRangeFields, Tabs, TabsList,
  TabsTrigger, TabsContent, Checkbox, RadioGroup, RadioGroupItem, Switch, Tooltip, TooltipTrigger,
  TooltipContent, Popover, PopoverTrigger, PopoverContent, Skeleton, Spinner, Kbd, Separator, ErrorState,
  TableSkeleton, CardSkeleton, DetailSkeleton } from "@/shared/components/ui";
import { Page, PageColumns, DescriptionList, DocumentRail, Stepper, FormSection, StickyFooter, FileDropzone,
  Money, Qty, DocNumber, Toolbar, LineItemsEditor, NavTabs, useUrlTab, ActionMenu } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { usePageCrumb } from "@/shared/stores/page-meta.store"; // halaman detail: usePageCrumb(order.number)
```

Penting:
- `Button` punya `loading`, `asChild`, ukuran `sm | md | lg | icon | icon-sm`. **Tautan bergaya tombol** =
  `<Button asChild><Link to="…">Tambah</Link></Button>` — bukan `<Link><Button/></Link>` (ratchet melarang).
- `DataTable` menerima: `loading`, `error`, `onRetry`, `onRowClick`, `sort` + `onSortChange` (kolom ber-`sortKey`),
  `selectable` + `selectedKeys` + `onSelectionChange` + `bulkActions`, `columnsKey` (pilihan kolom), `emptyAction`,
  `rows` boleh `null`. Header sticky otomatis.
- `SectionCard` untuk tabel: `<SectionCard flush>` (tanpa padding ganda).
- `PageHeader`: `title`, `description`, `meta` (badge di samping judul), `subtitle`, `actions`, dan `children`
  (pita di bawah header, mis. `DocumentRail`). `eyebrow` **diabaikan** — jangan diisi.
- `ConfirmDialog` kini `role="alertdialog"`. `Modal` = Dialog Radix (`role="dialog"`).
- `toast.success/danger/info/warning/fromServer` tidak berubah (Sonner di baliknya).
- Warna: **hanya kelas semantik** — `bg-card`, `bg-background`, `bg-muted`, `bg-accent`, `bg-primary`,
  `text-foreground`, `text-muted-foreground`, `border-border`, `border-input`, `text-success|warning|destructive`,
  `bg-success-soft|warning-soft|destructive-soft`. Tidak ada hex, tidak ada palet Tailwind (`bg-blue-500`).
- Tipografi: skala `text-2xs … text-4xl`. **Tidak ada** `text-[0.8125rem]`, `rounded-[10px]`, `max-w-[…]`
  baru. Angka besar/judul memakai `font-display`; kode/nomor dokumen `font-mono` (`<DocNumber/>`).
- Spasi antarseksi halaman: bungkus dengan `<Page>` (gap 24px); jangan `mb-4`/`mt-6` ad-hoc.

## 2. Resep per template

### T1 — Daftar

```tsx
<Page>
  <PageHeader
    title="Order Jual"
    description="…satu kalimat…"
    actions={<>
      {canExport && <Button variant="secondary" size="sm" …>Ekspor</Button>}
      {canCreate && <Button asChild><Link to={`${ROUTE_PATHS.salesOrders}/new`}>Tambah order</Link></Button>}
    </>}
  />
  <Toolbar
    search={<SearchInput aria-label="Cari order jual" … />}
    primary={/* maksimal 3 filter utama: status, tanggal, 1 lagi */}
    more={/* sisa filter di popover "Filter" */} moreActiveCount={…}
    active={/* chip filter aktif: {key,label,onRemove} */} onClearAll={reset}
  />
  <SectionCard flush>
    <DataTable rows={items} rowKey={…} loading={loading} error={fetchError} onRetry={reload}
      onRowClick={(r) => navigate(`${…}/${r.id}`)}
      sort={{ key: filters.sort, dir: filters.dir }} onSortChange={(s) => setFilters({ sort: s.key, dir: s.dir })}
      emptyTitle="Belum ada order jual" emptyDescription="…" emptyAction={canCreate && <Button asChild>…</Button>}
      columns={[ { header: "Nomor", sortKey: "number", render: (r) => <Link …><DocNumber value={r.number} /></Link> },
                 { header: "Total", align: "right", sortKey: "total", render: (r) => <Money value={r.grandTotal} /> },
                 { header: "Status", sortKey: "status", render: (r) => <StatusBadge status={r.status} /> } ]} />
    <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={…} onLimitChange={…opsional} />
  </SectionCard>
</Page>
```
- Select "Urutkan"/"Arah" **dihapus**; sortir lewat header (`sortKey` = nilai `sort` yang diterima API). Pertahankan param URL.
- Kolom "Aksi" berisi tautan "Detail" **dihapus** (baris bisa diklik; nomor tetap tautan). Aksi baris lain → `ActionMenu`.
- Empty karena filter vs kosong sungguhan: beda teks (`emptyTitle`/`emptyDescription`).
- Daftar yang belum berpaginasi dan bisa tumbuh: tambahkan pagination **di klien** bila endpoint belum mendukung.

### T2 — Detail dokumen

```tsx
usePageCrumb(order?.number);
<Page>
  <PageHeader title={order.number} meta={<StatusBadge status={order.status} />} subtitle={…pelanggan · tanggal…}
    actions={<>{/* aksi sekunder */}<ActionMenu items={[…, {…, destructive:true}]} />{/* SATU primary */}</>}>
    <DocumentRail steps={[…]} current={…} stopped={cancelled} related={[…]} />
  </PageHeader>
  <PageColumns main={<>…Barang, Aktivitas…</>} aside={<>…Ringkasan total, Pelanggan, Info dokumen (DescriptionList)…</>} />
</Page>
```
- **Semua aksi di header**; kartu "Aksi Order" dan `ActionRow` "Kembali ke daftar" dihapus (kembali = breadcrumb).
  Maksimal **satu** tombol `primary` (aksi berikut yang paling mungkin menurut status), maksimal 2 `secondary`,
  sisanya `ActionMenu`; aksi merusak paling bawah (`destructive: true`).
- Definisi langkah Rel Dokumen per jenis dokumen: file `modules/<m>/lib/<dokumen>-rail.ts` (fungsi murni + test).
  Dokumen terkait hanya dari data yang **sudah dimuat** halaman (tanpa endpoint baru).
- Tabel barang maksimal 6 kolom; info turunan jadi baris kedua redup (`text-xs text-muted-foreground`).
- Loading = `<DetailSkeleton />`; tidak ditemukan = `NotFoundState` (sudah ada).
- Catatan kebijakan (mis. "Order prabayar: …") = satu `Notice tone="info"` di bawah header, hanya bila relevan.

### T3 — Form dokumen berbaris barang

- Header dokumen ringkas 1–2 baris (grid `sm:grid-cols-2 lg:grid-cols-4`); isian jarang di `Collapsible` "Detail lain".
- Baris barang = `LineItemsEditor` (satu pola untuk SO, PO, retur, penerimaan, surat jalan, transfer, jurnal).
- `StickyFooter` berisi total di `summary` dan `Batal · Simpan draf · Simpan` di kanan.
- Draf otomatis (`use-form-draft`) tetap; tampilkan indikator kecil "Draf tersimpan", bukan Notice besar.
- Galat API: `ErrorState`/`Notice` di atas form dengan `message` asli; galat per sel inline.

### T4 — Master data
Daftar = T1. Buat/ubah = `FormSheet` (API sama dengan `Modal`: ganti nama komponen, `actions` jadi footer).
Detail (pelanggan/pemasok/produk) = header + `Tabs` yang tersinkron URL (`useUrlTab`). Hierarki (kategori, akun) =
tabel pohon (indentasi + chevron lipat), bukan kolom "induk".

### T5 laporan · T7 konsol kerja · T8 pengaturan · T9 wizard/impor
Lihat PLAN §7. Tab antarlaporan = `NavTabs` (tautan; satu URL per laporan). Wizard = `Stepper` + `FileDropzone` +
`StickyFooter`. Pengaturan = `FormSection` per seksi + `StickyFooter` yang muncul **hanya bila ada perubahan**;
mode baca-saja tiga lapis (kontrol nonaktif + teks penjelas + submit diabaikan) **dipertahankan**.

## 3. Aturan penempatan alur (PLAN §7.0) dan kontrak rute baru

Dialog ≤ 6 isian satu langkah; `FormSheet` untuk CRUD master & aksi pada dokumen; **halaman penuh** untuk dokumen
ber-baris barang. Alur "buat" berikut **pindah dari modal ke halaman**, dengan kontrak rute **tetap** (agar agen
modul lain bisa menautkannya tanpa menunggu):

| Rute | Query | Izin | Pemilik modul |
|---|---|---|---|
| `/deliveries/new` | `?from=<salesOrderId>` | `delivery.create` | delivery |
| `/sales-returns/new` | `?from=<salesOrderId>` | izin yang dipakai tombol pembukanya sekarang | salesreturn |
| `/purchase-returns/new` | `?from=<purchaseOrderId>` | idem | purchasereturn |
| `/goods-receipts/new` | `?from=<purchaseOrderId>` | idem | goodsreceipt |
| `/stock/transfers/new` | — | `stock.transfer` | stock |
| `/finance/journals/new` | — | izin tombol "Jurnal manual" sekarang | finance |

Mendaftarkan rute: tambah konstanta di `app/routes/route-paths.ts`, halaman lazy di `route-pages.ts`, dan entri
`EXTRA_ROUTES` di `protected.routes.tsx` — **hanya dengan `Edit` (penyisipan kecil), jangan `Write`/menimpa
file itu**, karena agen lain juga menyuntingnya. Tombol pembuka tetap di tempat yang sama dan menavigasi ke rute
baru. Prop `initialSource`/`initialSalesOrder` lama diganti `?from=`. Komponen modal lama yang tak terpakai lagi
**dihapus** (beserta test yang hanya menguji modal itu — pindahkan perilakunya ke test halaman baru).

## 4. Batas kerja (agar agen paralel tidak bentrok)

1. Ubah hanya file di **modul Anda** (`apps/web/src/modules/<modul>/…`) + test-nya + spec e2e yang menyangkut modul Anda.
2. **Jangan** mengubah `apps/web/src/shared/**`, `theme/**`, `AppShell`. Butuh perubahan di sana? Jangan paksa — pakai
   yang ada atau bungkus di modul Anda, lalu laporkan kebutuhannya di ringkasan akhir.
3. Komponen/ekspor yang **dipakai modul lain** (mis. `OrderPaymentModal`, `OrderBillingCard`, `OrderHistoryCard`,
   `PartySearchSelect`, `ProductPicker`, `LocationSelect`, `ReturnLinesTable`) boleh berubah isi, tetapi **nama ekspor dan
   props tidak boleh berubah**. Cek pemakainya dengan grep sebelum mengubah.
4. Perilaku bisnis, endpoint, kunci izin, skema Zod, dan format cetak **tidak berubah**. Ini redesain tampilan.
5. Jangan menjalankan `vite build`, `npm run test:e2e`, atau server — itu dijalankan terpusat di akhir.
   Yang boleh dan wajib: `npx tsc --noEmit` (abaikan galat di file modul lain yang sedang diubah agen lain),
   `npx eslint <file-Anda>`, `npx vitest run src/modules/<modul>` (dari `apps/web`).
6. Teks UI Bahasa Indonesia, *sentence case*, tombol berkata kerja konsisten ("Simpan" → toast "Tersimpan").
   Galat tulis selalu menampilkan `message` asli API (tidak diganti teks generik).
7. **Ratchet UI** (`node scripts/ui-ratchet.mjs --report`) tidak boleh naik di modul Anda; target = turun ke 0 untuk
   `arbitrary`, `loadingText`, `rawInput`, `linkButton`, `segmented` (sebagai tab), `hex`. Input `type=checkbox|radio|file`
   mentah → `Checkbox`/`RadioGroup`/`FileDropzone`. `Memuat…` tulis-tangan → skeleton/`loading` DataTable.
8. Test yang berubah karena markup/teks baru **diperbarui dengan sadar** (pertahankan niat uji, bukan dihapus).
   Spec e2e: grep `apps/e2e/specs` untuk teks/rute modul Anda; bila teks yang dipakai spec berubah, perbarui spec dan
   `apps/e2e/helpers/labels.ts` di perubahan yang sama. Nama menu sidebar tidak boleh Anda ubah.
9. Aksesibilitas: setiap kontrol berlabel, fokus terlihat, target sentuh wajar, hanya satu `<h1>` (dari `PageHeader`).

## 5. Selesai berarti (ringkas dari PLAN §10)

Satu template, aksi di tempat template, satu `<h1>`, skeleton/ErrorState/Empty+aksi, tanpa ukuran arbitrer/hex/tabel
mentah/input mentah/`<Link><Button>`, uang/qty/nomor memakai `Money`/`Qty`/`DocNumber`, status memakai `StatusBadge`,
layak di 390px, Bahasa Indonesia, `tsc` + `eslint` + `vitest` modul hijau.

## 6. Laporan akhir agen

Tulis ringkasan singkat: halaman yang selesai, rute baru, komponen yang dihapus, spec/test yang diubah, angka ratchet
modul sebelum→sesudah, kebutuhan di `shared/` yang **tidak** Anda kerjakan, dan hal yang belum selesai (jujur).
