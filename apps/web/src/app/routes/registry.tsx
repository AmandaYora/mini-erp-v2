import type { LucideIcon } from "lucide-react";
import {
  ArrowLeftRight,
  BookPlus,
  Calculator,
  BadgePercent,
  BookOpen,
  Bot,
  Boxes,
  Building2,
  ChartBar,
  ChartColumnStacked,
  ChartLine,
  ClipboardCheck,
  ClipboardList,
  ClipboardPen,
  Columns2,
  Factory,
  FilePenLine,
  Layers,
  ListChecks,
  ListTree,
  PackagePlus,
  ShieldAlert,
  SlidersHorizontal,
  FileSpreadsheet,
  FileText,
  Landmark,
  LayoutDashboard,
  LockKeyhole,
  MapPin,
  NotebookText,
  Package,
  PackageCheck,
  Receipt,
  RefreshCcw,
  Scale,
  ScrollText,
  ShieldCheck,
  Stethoscope,
  ShoppingBag,
  ShoppingCart,
  Tags,
  TriangleAlert,
  TrendingUp,
  Truck,
  Undo2,
  UserCog,
  Users,
  Wallet,
} from "lucide-react";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

// CATATAN: ClipboardPen dipakai untuk "Koreksi" (ClipboardEdit tidak ada di
// lucide-react 1.43) dan ScrollText untuk "Jejak Audit" (History tidak ada).

export interface MenuItem {
  label: string;
  path: string;
  icon: LucideIcon;
  perm: string;
  /** Sub-seksi collapsible di dalam grup (mengikuti aplikasi lama). */
  section?: string;
}

export interface MenuGroup {
  title: string;
  items: MenuItem[];
}

/** Urutan seksi per grup; item tanpa seksi tampil lebih dulu. */
export const MENU_SECTION_ORDER: Record<string, string[]> = {
  Keuangan: ["Harian", "Cek Bisnis", "Pajak & Akhir Bulan", "Detail"],
};

// Struktur grup mengikuti aplikasi lama (Utama, Operasional, Stok & Gudang,
// Produk & Mitra, Keuangan, Pantauan, Asisten WA, Pengaturan).
export const MENU_GROUPS: MenuGroup[] = [
  {
    title: "Utama",
    items: [
      { label: "Dashboard", path: ROUTE_PATHS.dashboard, icon: LayoutDashboard, perm: "dashboard.view" },
    ],
  },
  {
    title: "Operasional",
    items: [
      { label: "POS", path: ROUTE_PATHS.pos, icon: ShoppingCart, perm: "sales.create" },
      { label: "Order Jual", path: ROUTE_PATHS.salesOrders, icon: ClipboardList, perm: "sales.view" },
      { label: "Pengiriman", path: ROUTE_PATHS.deliveries, icon: Truck, perm: "delivery.view" },
      { label: "Antrian Kirim", path: ROUTE_PATHS.deliveriesQueue, icon: ListChecks, perm: "delivery.view" },
      { label: "Retur Jual", path: ROUTE_PATHS.salesReturns, icon: RefreshCcw, perm: "salesreturn.view" },
      { label: "Order Beli", path: ROUTE_PATHS.purchaseOrders, icon: ShoppingBag, perm: "purchasing.view" },
      { label: "Penerimaan", path: ROUTE_PATHS.goodsReceipts, icon: PackageCheck, perm: "goodsreceipt.view" },
      { label: "Retur Beli", path: ROUTE_PATHS.purchaseReturns, icon: Undo2, perm: "purchasereturn.view" },
      { label: "Pembayaran", path: ROUTE_PATHS.payments, icon: Wallet, perm: "payment.view" },
    ],
  },
  {
    title: "Stok & Gudang",
    items: [
      { label: "Stok", path: ROUTE_PATHS.stock, icon: Boxes, perm: "stock.view" },
      { label: "Saldo", path: ROUTE_PATHS.stockBalances, icon: Layers, perm: "stock.view" },
      { label: "Transfer", path: ROUTE_PATHS.stockTransfers, icon: ArrowLeftRight, perm: "stock.transfer" },
      { label: "Koreksi", path: ROUTE_PATHS.stockAdjustments, icon: ClipboardPen, perm: "stock.adjust" },
      { label: "Rusak", path: ROUTE_PATHS.stockDamaged, icon: TriangleAlert, perm: "stock.adjust" },
      { label: "Saldo Awal Stok", path: ROUTE_PATHS.stockOpening, icon: PackagePlus, perm: "stock.adjust" },
    ],
  },
  {
    title: "Produk & Mitra",
    items: [
      { label: "Produk", path: ROUTE_PATHS.products, icon: Package, perm: "products.view" },
      { label: "Kategori", path: ROUTE_PATHS.productCategories, icon: Tags, perm: "product_categories.manage" },
      { label: "Impor Produk", path: ROUTE_PATHS.productImports, icon: FileSpreadsheet, perm: "product_import.manage" },
      { label: "Pelanggan", path: ROUTE_PATHS.customers, icon: Users, perm: "customers.view" },
      { label: "Pemasok", path: ROUTE_PATHS.suppliers, icon: Factory, perm: "suppliers.view" },
      { label: "Tipe Member", path: ROUTE_PATHS.memberTypes, icon: BadgePercent, perm: "member_types.view" },
    ],
  },
  {
    title: "Keuangan",
    items: [
      { label: "Posting", path: ROUTE_PATHS.financePosting, icon: ClipboardCheck, perm: "finance.posting.view", section: "Harian" },
      { label: "Biaya", path: ROUTE_PATHS.financeExpenses, icon: Receipt, perm: "finance.expense.view", section: "Harian" },
      { label: "Piutang & Hutang", path: ROUTE_PATHS.receivablesPayables, icon: Scale, perm: "finance.report.view", section: "Cek Bisnis" },
      { label: "Laba Rugi", path: ROUTE_PATHS.profitLoss, icon: TrendingUp, perm: "finance.report.view", section: "Cek Bisnis" },
      { label: "Neraca", path: ROUTE_PATHS.balanceSheet, icon: Landmark, perm: "finance.report.view", section: "Cek Bisnis" },
      { label: "Pajak", path: ROUTE_PATHS.taxSummary, icon: FileText, perm: "finance.tax_report.view", section: "Pajak & Akhir Bulan" },
      { label: "Penyesuaian Pajak", path: ROUTE_PATHS.taxAdjustments, icon: FilePenLine, perm: "finance.tax_adjustment.view", section: "Pajak & Akhir Bulan" },
      { label: "Periode", path: ROUTE_PATHS.financePeriods, icon: LockKeyhole, perm: "finance.manage", section: "Pajak & Akhir Bulan" },
      { label: "Jurnal", path: ROUTE_PATHS.financeJournals, icon: BookOpen, perm: "finance.journal.view", section: "Detail" },
      { label: "Buku Besar", path: ROUTE_PATHS.financeGeneralLedger, icon: NotebookText, perm: "finance.report.view", section: "Detail" },
      { label: "Neraca Saldo", path: ROUTE_PATHS.trialBalance, icon: Columns2, perm: "finance.report.view", section: "Detail" },
      { label: "Saldo Awal Akun", path: ROUTE_PATHS.financeOpening, icon: BookPlus, perm: "finance.view", section: "Detail" },
      { label: "Akun", path: ROUTE_PATHS.financeAccounts, icon: ListTree, perm: "finance.manage", section: "Detail" },
      { label: "Integritas", path: ROUTE_PATHS.financeIntegrity, icon: Stethoscope, perm: "finance.manage", section: "Detail" },
      { label: "Hitung Ulang HPP", path: ROUTE_PATHS.financeCostRestatement, icon: Calculator, perm: "finance.manage", section: "Detail" },
      { label: "Revaluasi Harga Modal", path: ROUTE_PATHS.financeRevaluations, icon: Tags, perm: "finance.manage", section: "Detail" },
    ],
  },
  {
    title: "Pantauan",
    items: [
      { label: "Tren & Stok", path: ROUTE_PATHS.reporting, icon: ChartLine, perm: "reporting.view" },
      { label: "Laporan Order", path: ROUTE_PATHS.reportingOrders, icon: ChartBar, perm: "reporting.view" },
      { label: "Laporan Stok", path: ROUTE_PATHS.reportingStock, icon: ChartColumnStacked, perm: "reporting.view" },
      { label: "Rekonsiliasi Pihak", path: ROUTE_PATHS.partyReconciliation, icon: Scale, perm: "finance.report.view" },
      { label: "Konsistensi", path: ROUTE_PATHS.opsIntegrity, icon: ShieldAlert, perm: "stock.update" },
      { label: "Jejak Audit", path: ROUTE_PATHS.auditLogs, icon: ScrollText, perm: "audit.view" },
    ],
  },
  {
    title: "Asisten WA",
    items: [
      { label: "Penyiapan WA", path: ROUTE_PATHS.assistantSetup, icon: Bot, perm: "assistant.view" },
      { label: "Konfigurasi", path: ROUTE_PATHS.assistantConfig, icon: SlidersHorizontal, perm: "assistant.view" },
    ],
  },
  {
    title: "Pengaturan",
    items: [
      { label: "Pengguna", path: ROUTE_PATHS.users, icon: UserCog, perm: "users.view" },
      { label: "Role", path: ROUTE_PATHS.roles, icon: ShieldCheck, perm: "roles.view" },
      { label: "Cabang", path: ROUTE_PATHS.branches, icon: MapPin, perm: "branches.view" },
      { label: "Perusahaan", path: ROUTE_PATHS.company, icon: Building2, perm: "company.view" },
    ],
  },
];

/** Bagi item grup menjadi seksi berurutan (item tanpa seksi → seksi tanpa label). */
export function groupSections(
  groupTitle: string,
  items: MenuItem[],
): { label?: string; items: MenuItem[] }[] {
  const order = MENU_SECTION_ORDER[groupTitle] ?? [];
  const labels = [
    "",
    ...order,
    ...Array.from(new Set(items.map((i) => i.section ?? ""))).filter(
      (l) => l !== "" && !order.includes(l),
    ),
  ];
  return labels
    .map((label) => ({
      label: label || undefined,
      items: items.filter((i) => (i.section ?? "") === label),
    }))
    .filter((s) => s.items.length > 0);
}

/** Judul halaman per path menu + rute khusus. */
export const PAGE_TITLES: Record<string, string> = Object.fromEntries(
  MENU_GROUPS.flatMap((group) => group.items.map((item) => [item.path, item.label] as const)),
);
PAGE_TITLES[ROUTE_PATHS.taxDetail] = "Rincian Pajak";
PAGE_TITLES[ROUTE_PATHS.inventoryValue] = "Nilai Persediaan";
PAGE_TITLES[ROUTE_PATHS.margin] = "Margin Produk";
PAGE_TITLES[ROUTE_PATHS.cashReport] = "Laporan Kas";
PAGE_TITLES[ROUTE_PATHS.selectBranch] = "Pilih Cabang";
PAGE_TITLES[ROUTE_PATHS.forbidden] = "Akses Ditolak";
PAGE_TITLES["/print/deliveries"] = "Cetak Surat Jalan";
PAGE_TITLES["/print/transfers"] = "Cetak Surat Transfer";
PAGE_TITLES["/print/sales"] = "Cetak Faktur";
PAGE_TITLES["/print/payments"] = "Cetak Kwitansi";
PAGE_TITLES["/print/pos"] = "Struk POS";
PAGE_TITLES["/print/labels"] = "Cetak Label QR";
PAGE_TITLES[ROUTE_PATHS.printCalibration] = "Kalibrasi Kertas";

/** Judul topbar: cocokkan prefiks terpanjang agar rute anak ikut bernama. */
export function getPageTitle(pathname: string): string {
  const entries = Object.entries(PAGE_TITLES).sort((a, b) => b[0].length - a[0].length);
  for (const [path, title] of entries) {
    if (pathname === path || pathname.startsWith(`${path}/`)) return title;
  }
  return "Halaman";
}

/**
 * Breadcrumb "Grup / Menu" untuk kerangka (bukan <h1>; <h1> hanya di PageHeader).
 * Halaman detail menambah segmen ketiga (nomor dokumen) lewat page-meta store.
 */
export function getBreadcrumb(pathname: string): { group?: string; title: string; path?: string } {
  const title = getPageTitle(pathname);
  const best = MENU_GROUPS.flatMap((g) => g.items.map((i) => ({ g: g.title, i })))
    .filter(({ i }) => pathname === i.path || pathname.startsWith(`${i.path}/`))
    .sort((a, b) => b.i.path.length - a.i.path.length)[0];
  return best ? { group: best.g, title, path: best.i.path } : { title };
}

export interface QuickAction {
  label: string;
  path: string;
  icon: LucideIcon;
  perm: string;
}

/** Aksi cepat tombol "+ Buat" dan command palette; hanya yang diizinkan peran yang ditampilkan. */
export const QUICK_ACTIONS: QuickAction[] = [
  { label: "Order jual baru", path: `${ROUTE_PATHS.salesOrders}/new`, icon: ClipboardList, perm: "sales.create" },
  { label: "Order beli baru", path: `${ROUTE_PATHS.purchaseOrders}/new`, icon: ShoppingBag, perm: "purchasing.create" },
  { label: "Catat biaya", path: ROUTE_PATHS.financeExpenses, icon: Receipt, perm: "finance.expense.create" },
  { label: "Produk baru", path: `${ROUTE_PATHS.products}/new`, icon: Package, perm: "products.create" },
  { label: "Transfer stok", path: ROUTE_PATHS.stockTransfers, icon: ArrowLeftRight, perm: "stock.transfer" },
];
