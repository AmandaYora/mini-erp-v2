import type { LucideIcon } from "lucide-react";
import {
  ArrowLeftRight,
  BadgePercent,
  BookOpen,
  Bot,
  Boxes,
  Building2,
  ChartLine,
  ClipboardCheck,
  ClipboardList,
  ClipboardPen,
  Cog,
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
}

export interface MenuGroup {
  title: string;
  items: MenuItem[];
}

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
      { label: "Antrian Kirim", path: ROUTE_PATHS.deliveriesQueue, icon: ClipboardCheck, perm: "delivery.view" },
      { label: "Retur Jual", path: ROUTE_PATHS.salesReturns, icon: RefreshCcw, perm: "salesreturn.view" },
      { label: "Retur Beli", path: ROUTE_PATHS.purchaseReturns, icon: Undo2, perm: "purchasereturn.view" },
    ],
  },
  {
    title: "Pembelian",
    items: [
      { label: "Order Beli", path: ROUTE_PATHS.purchaseOrders, icon: ShoppingBag, perm: "purchasing.view" },
      { label: "Penerimaan", path: ROUTE_PATHS.goodsReceipts, icon: PackageCheck, perm: "goodsreceipt.view" },
      { label: "Pembayaran", path: ROUTE_PATHS.payments, icon: Wallet, perm: "payment.view" },
    ],
  },
  {
    title: "Stok & Gudang",
    items: [
      { label: "Stok", path: ROUTE_PATHS.stock, icon: Boxes, perm: "stock.view" },
      { label: "Transfer", path: ROUTE_PATHS.stockTransfers, icon: ArrowLeftRight, perm: "stock.manage" },
      { label: "Koreksi", path: ROUTE_PATHS.stockAdjustments, icon: ClipboardPen, perm: "stock.adjust" },
      { label: "Rusak", path: ROUTE_PATHS.stockDamaged, icon: TriangleAlert, perm: "stock.manage" },
      { label: "Saldo Awal", path: ROUTE_PATHS.stockOpening, icon: FileSpreadsheet, perm: "stock.manage" },
    ],
  },
  {
    title: "Master",
    items: [
      { label: "Produk", path: ROUTE_PATHS.products, icon: Package, perm: "products.view" },
      { label: "Kategori", path: ROUTE_PATHS.productCategories, icon: Tags, perm: "product_categories.manage" },
      { label: "Impor Produk", path: ROUTE_PATHS.productImports, icon: FileSpreadsheet, perm: "product_import.manage" },
      { label: "Pelanggan", path: ROUTE_PATHS.customers, icon: Users, perm: "customers.view" },
      { label: "Pemasok", path: ROUTE_PATHS.suppliers, icon: Building2, perm: "suppliers.view" },
      { label: "Tipe Member", path: ROUTE_PATHS.memberTypes, icon: BadgePercent, perm: "member_types.manage" },
    ],
  },
  {
    title: "Keuangan",
    items: [
      { label: "Posting", path: ROUTE_PATHS.financePosting, icon: ClipboardCheck, perm: "finance.post" },
      { label: "Jurnal", path: ROUTE_PATHS.financeJournals, icon: BookOpen, perm: "finance.view" },
      { label: "Buku Besar", path: ROUTE_PATHS.financeGeneralLedger, icon: NotebookText, perm: "finance.view" },
      { label: "Neraca Saldo", path: ROUTE_PATHS.trialBalance, icon: Scale, perm: "finance.view" },
      { label: "Laba Rugi", path: ROUTE_PATHS.profitLoss, icon: TrendingUp, perm: "finance.view" },
      { label: "Neraca", path: ROUTE_PATHS.balanceSheet, icon: Landmark, perm: "finance.view" },
      { label: "Pajak", path: ROUTE_PATHS.taxSummary, icon: FileText, perm: "finance.view" },
      { label: "Piutang & Hutang", path: ROUTE_PATHS.receivablesPayables, icon: Scale, perm: "finance.view" },
      { label: "Biaya", path: ROUTE_PATHS.financeExpenses, icon: Receipt, perm: "finance.post" },
      { label: "Saldo Awal", path: ROUTE_PATHS.financeOpening, icon: FileSpreadsheet, perm: "finance.view" },
      { label: "Penyesuaian Pajak", path: ROUTE_PATHS.taxAdjustments, icon: Scale, perm: "finance.view" },
      { label: "Akun", path: ROUTE_PATHS.financeAccounts, icon: Cog, perm: "finance.manage" },
      { label: "Periode", path: ROUTE_PATHS.financePeriods, icon: LockKeyhole, perm: "finance.manage" },
    ],
  },
  {
    title: "Laporan",
    items: [
      { label: "Tren & Stok", path: ROUTE_PATHS.reporting, icon: ChartLine, perm: "reporting.view" },
      { label: "Jejak Audit", path: ROUTE_PATHS.auditLogs, icon: ScrollText, perm: "audit.view" },
    ],
  },
  {
    title: "Asisten",
    items: [
      { label: "Penyiapan WA", path: ROUTE_PATHS.assistantSetup, icon: Bot, perm: "assistant.view" },
      { label: "Konfigurasi", path: ROUTE_PATHS.assistantConfig, icon: Cog, perm: "assistant.view" },
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

/** Judul halaman per path menu + rute khusus. */
export const PAGE_TITLES: Record<string, string> = Object.fromEntries(
  MENU_GROUPS.flatMap((group) => group.items.map((item) => [item.path, item.label] as const)),
);
PAGE_TITLES[ROUTE_PATHS.selectBranch] = "Pilih Cabang";
PAGE_TITLES[ROUTE_PATHS.forbidden] = "Akses Ditolak";
PAGE_TITLES["/print/deliveries"] = "Cetak Surat Jalan";
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
