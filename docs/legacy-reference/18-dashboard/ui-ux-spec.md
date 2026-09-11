# UI/UX Spec — Modul 18 Dashboard

**Kelompok A.** Satu halaman + perilaku muat. Sumber: `dashboard-page.tsx` (penuh),
hook + test, `summary-card.tsx` (dicek — **tak dipakai halaman ini**; KPI memakai
`KpiCard` lokal!), registry, utils.

---

## 1. Halaman Ringkasan Operasional — `/dashboard` (menu "Utama" → "Dashboard")

- **Akses & registrasi:** halaman default pasca-login/pilih-cabang
  (`FALLBACK_PATH=/dashboard`, `auth-redirect.ts:1`; `resolvePostAuthRedirectPath` selalu
  `/dashboard` untuk `/, /login, /select-branch`, dijaga test!); registry
  `module-registry.tsx:200–207` (`title:Dashboard, access:protected, permission:dashboard.view,
  shell:true, menu Utama→Dashboard, lazy DashboardPage`); butuh `dashboard.view`
  (semua role bawaan termasuk staff!); tanpa perusahaan/cabang → render `null` tanpa error
  (`dashboard-page.tsx:355–357`) dan request dibatalkan (`207–211`).
- **Header:** eyebrow `{kodeCabang} | {kodePerusahaan}`; judul `Ringkasan Operasional`;
  deskripsi `Pantauan cabang {nama} dari transaksi, stok, dan finance yang sudah tercatat.`;
  aksi `Buat Pesanan Baru` (hanya bila `can('order.create')`!) → `/orders/create`
  (`dashboard-page.tsx:361–372`).
- **4 kartu KPI** (tombol! geser-naik saat hover; titik-nada + nilai-tebal + hint):
  Order Perlu Aksi (warn bila >0 else ok → `/orders?status_group=pending`) ·
  Penjualan Net Bulan Ini (info → `?status_group=completed`) ·
  Piutang Terbuka (warn bila >0 else ok → `/finance/receivables-payables`) ·
  Stok Kritis (bad bila >0 else ok → `/stock?critical_only=true`).
  Nominal panjang tak-patah-tengah-digit (hanya setelah "Rp"); mobile sedikit kecil.
- **Strip sekunder** (sel-buku-besar 1→2→4 kolom): Penjualan Net Hari Ini
  (`{n} pesanan tercatat hari ini.`) · Utang Supplier Terbuka (merah bila >0;
  `Saldo pembelian yang belum dilunasi.`) · Produk Stok Dipantau
  (`Produk aktif yang memakai kontrol stok.`) · Kanal WhatsApp (titik hijau/redup +
  label + `{n} SOP siap dipakai.`).
- **Tren Penjualan Net** (bar-chart 6 bulan,标签 `Mon yyyy`, sumbu compact,
  tooltip `Penjualan Net` + nominal penuh; bar 40/56px radius-atas-6) +
  kolom insight: **5 Barang Paling Laku** (peringkat + nama + `kode|Tanpa kode` +
  `{n} transaksi` + qty + satuan else `unit`; klik → `/products/{id}`; kosong →
  info `Belum ada penjualan bulan ini` + kalimat) + **5 Estimasi Margin** (badge
  `Estimasi`; warning bila cost-tak-lengkap; baris `Omzet {x} | Est. HPP {y}` +
  margin + persen; 3 keadaan kosong info/warning/info — teks §4!).
- **Dua kartu bawah:** Pesanan Prioritas (`Lihat Semua` → `/orders`; baris nomor +
  badge-status + pihak else `Pihak terkait belum dipilih` + `Jatuh tempo {tgl|-}` +
  alasan/catatan else `-`; klik → `/orders/{id}`; kosong → success
  `Tidak ada pesanan prioritas` + kalimat) · Stok Kritis (`Buka Stok` →
  `/stock?critical_only=true`; baris nama + badge-merah `Tersisa {n}` + `Minimum
  {n} {satuan} | update terakhir {tgl}`; klik → `/stock/{idProduk}`; kosong →
  success `Tidak ada stok kritis` + kalimat).
- **Tanpa di halaman:** filter tanggal/cabang, refresh manual, ekspor, angka
  completed/cancelled/transaksi-total, penanda fallback (KI-132/KI-134!).
- **Detail tampil per seksi (teks persis!):** 4 KPI tombol (`KpiCard` lokal 39–56, hover
  naik-0.5 + border-brand/30; titik-nada `info/bg-brand, ok/bg-ok, warn/bg-warn, bad/bg-bad`;
  nominal `text-xl→2xl tabular-nums`, tak-patah-tengah-digit!); strip sekunder
  `grid 1→2→4 kolom sel-buku-besar` (407–431); tren `h-[380px] BarChart margin 10/10/0/0,
  grid `#e2e8f0`, tick `#64748b`, bar `#1e3a5f` 40/56px radius-6, cursor `#f1f5f9`
  (24–32, 438–471)`; ranking `RANK_ITEM 32px|1fr|112px` + marker + metrik-kanan
  (17–20, 475–550); daftar `LIST_ITEM/LIST_ROW` + `Badge statusTone` + `Notice
  info/warning/success` + `SectionCard` (554–624). Format: nominal `formatCurrency`
  0-desimal; qty `formatQuantity` lokal ≤2-desimal (180–182); persen `formatQuantity+%`;
  tren-Y compact; waktu `formatDateTime` tanpa detik (`utils.ts:25–37`); warning
  `compactNumber` 1-desimal (`utils.ts:121–126`).
- **Satu-satunya endpoint + halaman (konfirmasi):** `apiPost('dashboard/summary',
  {today_from, today_to, month_from, month_to, trend_months:6})` (`dashboard-page.tsx:217–223`);
  tak ada pemanggil lain di `apps/web/src` (grep!). Tanpa folder `reporting` di web
  sebagai pembanding — dashboard satu-satunya ringkasan visual.

## 2. Pemuatan & Fallback

- Muat saat cabang/timezone berubah: kirim hari + bulan zona-perusahaan + tren-6;
  gagal → ringkasan null → **fallback workspace diam-diam** (6 angka dari store
  lokal dengan logika lebih sederhana — tren tanpa-retur, kritis per-baris,
  prioritas tanpa-skor; lihat KI-132!). Tanpa cabang → tak-request.
- Hook tak memicu preload slice apa pun (dijaga 5 test!); halaman membaca store
  apa-adanya (kosong = fallback kosong, bukan loading!).
- `summary-card.tsx` bersama **tak dipakai** halaman ini (warisan/untuk modul
  lain — [PERLU KONFIRMASI] pemakainya!).
