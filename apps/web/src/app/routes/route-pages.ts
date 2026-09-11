import { lazy } from "react";

/**
 * Definisi halaman lazy untuk tabel rute. File sendiri yang HANYA mengekspor
 * komponen (lazy exotic) agar protected.routes.tsx / public.routes.tsx hanya
 * mengekspor objek rute (data) — nol peringatan
 * react-refresh/only-export-components di kedua sisi.
 *
 * Halaman nyata per path menu. Modul yang belum dibangun UI-nya tidak ada di
 * sini → otomatis placeholder. Daftarkan halaman baru di tabel ini (satu
 * tempat), JANGAN ubah generator di protected.routes.tsx.
 */
export const MODULE_PAGES: Record<string, React.LazyExoticComponent<React.ComponentType>> = {
  "/users": lazy(() => import("@/modules/users/pages/UsersPage")),
  "/roles": lazy(() => import("@/modules/users/pages/RolesPage")),
  "/branches": lazy(() => import("@/modules/branches/pages/BranchesPage")),
  "/company": lazy(() => import("@/modules/company/pages/CompanyPage")),
  "/products": lazy(() => import("@/modules/products/pages/ProductsPage")),
  "/products/new": lazy(() => import("@/modules/products/pages/ProductFormPage")),
  "/products/:id": lazy(() => import("@/modules/products/pages/ProductDetailPage")),
  "/products/:id/edit": lazy(() => import("@/modules/products/pages/ProductFormPage")),
  "/product-categories": lazy(() => import("@/modules/products/pages/CategoriesPage")),
  "/product-imports": lazy(() => import("@/modules/products/pages/ImportPage")),
  "/customers": lazy(() => import("@/modules/party/pages/CustomersPage")),
  "/customers/:id": lazy(() => import("@/modules/party/pages/CustomerDetailPage")),
  "/suppliers": lazy(() => import("@/modules/party/pages/SuppliersPage")),
  "/suppliers/:id": lazy(() => import("@/modules/party/pages/SupplierDetailPage")),
  "/member-types": lazy(() => import("@/modules/party/pages/MemberTypesPage")),
  "/stock": lazy(() => import("@/modules/stock/pages/StockPage")),
  "/stock/transfers": lazy(() => import("@/modules/stock/pages/TransfersPage")),
  "/stock/transfers/:id": lazy(() => import("@/modules/stock/pages/TransferDetailPage")),
  "/stock/adjustments": lazy(() => import("@/modules/stock/pages/AdjustmentsPage")),
  "/stock/damaged": lazy(() => import("@/modules/stock/pages/DamagedPage")),
  "/stock/opening": lazy(() => import("@/modules/stock/pages/OpeningPage")),
  "/stock/items/:productId": lazy(
    () => import("@/modules/stock/pages/StockItemPage"),
  ),
  "/purchase-orders": lazy(() => import("@/modules/purchasing/pages/PurchaseOrdersPage")),
  "/purchase-orders/new": lazy(() => import("@/modules/purchasing/pages/PurchaseOrderFormPage")),
  "/purchase-orders/:id": lazy(() => import("@/modules/purchasing/pages/PurchaseOrderDetailPage")),
  "/purchase-orders/:id/edit": lazy(() => import("@/modules/purchasing/pages/PurchaseOrderFormPage")),
  "/sales-orders": lazy(() => import("@/modules/sales/pages/SalesOrdersPage")),
  "/sales-orders/new": lazy(() => import("@/modules/sales/pages/SalesOrderFormPage")),
  "/sales-orders/:id": lazy(() => import("@/modules/sales/pages/SalesOrderDetailPage")),
  "/sales-orders/:id/edit": lazy(() => import("@/modules/sales/pages/SalesOrderFormPage")),
  "/goods-receipts": lazy(() => import("@/modules/goodsreceipt/pages/GoodsReceiptsPage")),
  "/goods-receipts/:id": lazy(() => import("@/modules/goodsreceipt/pages/GoodsReceiptDetailPage")),
  "/deliveries": lazy(() => import("@/modules/delivery/pages/DeliveriesPage")),
  "/deliveries/queue": lazy(() => import("@/modules/delivery/pages/QueuePage")),
  "/deliveries/:id": lazy(() => import("@/modules/delivery/pages/DeliveryDetailPage")),
  "/payments": lazy(() => import("@/modules/payment/pages/PaymentsPage")),
  "/payments/:id": lazy(() => import("@/modules/payment/pages/PaymentDetailPage")),
  "/sales-returns": lazy(() => import("@/modules/salesreturn/pages/SalesReturnsPage")),
  "/sales-returns/:id": lazy(() => import("@/modules/salesreturn/pages/SalesReturnDetailPage")),
  "/purchase-returns": lazy(() => import("@/modules/purchasereturn/pages/PurchaseReturnsPage")),
  "/purchase-returns/:id": lazy(() => import("@/modules/purchasereturn/pages/PurchaseReturnDetailPage")),
  "/finance/posting": lazy(() => import("@/modules/finance/pages/PostingPage")),
  "/finance/journals": lazy(() => import("@/modules/finance/pages/JournalsPage")),
  "/finance/journals/:id": lazy(() => import("@/modules/finance/pages/JournalDetailPage")),
  "/finance/expenses": lazy(() => import("@/modules/finance/pages/ExpensesPage")),
  "/finance/expenses/:id": lazy(() => import("@/modules/finance/pages/ExpenseDetailPage")),
  "/finance/accounts": lazy(() => import("@/modules/finance/pages/AccountsPage")),
  "/finance/periods": lazy(() => import("@/modules/finance/pages/PeriodsPage")),
  "/finance/opening": lazy(() => import("@/modules/finance/pages/OpeningPage")),
  "/finance/tax-adjustments": lazy(
    () => import("@/modules/finance/pages/TaxAdjustmentsPage"),
  ),
  "/finance/reports/trial-balance": lazy(() => import("@/modules/finance/pages/ReportsPage")),
  "/finance/reports/profit-loss": lazy(() => import("@/modules/finance/pages/ReportsPage")),
  "/finance/reports/balance-sheet": lazy(() => import("@/modules/finance/pages/ReportsPage")),
  "/finance/reports/tax-summary": lazy(() => import("@/modules/finance/pages/ReportsPage")),
  "/finance/reports/inventory-value": lazy(() => import("@/modules/finance/pages/ReportsPage")),
  "/finance/reports/margin": lazy(() => import("@/modules/finance/pages/ReportsPage")),
  "/finance/reports/receivables-payables": lazy(() => import("@/modules/finance/pages/ReportsPage")),
  "/finance/general-ledger": lazy(() => import("@/modules/finance/pages/GeneralLedgerPage")),
  "/finance/reports/tax-detail": lazy(() => import("@/modules/finance/pages/TaxDetailPage")),
  "/reporting": lazy(() => import("@/modules/reporting/pages/ReportingPage")),
  "/audit-logs": lazy(() => import("@/modules/audit/pages/AuditPage")),
  "/assistant/setup": lazy(() => import("@/modules/assistant/pages/SetupPage")),
  "/assistant/config": lazy(() => import("@/modules/assistant/pages/ConfigPage")),
  "/pos": lazy(() => import("@/modules/pos/pages/PosPage")),
  "/print/calibration": lazy(
    () => import("@/modules/print/pages/PaperCalibrationPage"),
  ),
};

export const PlaceholderPage = lazy(() => import("@/app/routes/placeholder"));

export const DashboardPage = lazy(() => import("@/modules/dashboard/pages/DashboardPage"));

export const LoginPage = lazy(() => import("@/modules/auth/pages/LoginPage"));

export const SelectBranchPage = lazy(
  () => import("@/modules/auth/pages/SelectBranchPage"),
);

export const ForbiddenPage = lazy(() => import("@/modules/auth/pages/ForbiddenPage"));

// Halaman dokumen cetak (Tahap E): standalone TANPA AppLayout agar sidebar /
// topbar tidak ikut tercetak.
export const DeliveryNotePrintPage = lazy(
  () => import("@/modules/print/pages/DeliveryNotePrintPage"),
);

export const SalesInvoicePrintPage = lazy(
  () => import("@/modules/print/pages/SalesInvoicePrintPage"),
);

export const PaymentReceiptPrintPage = lazy(
  () => import("@/modules/print/pages/PaymentReceiptPrintPage"),
);

export const PosThermalReceiptPage = lazy(
  () => import("@/modules/print/pages/PosThermalReceiptPage"),
);

export const ProductLabelsPrintPage = lazy(
  () => import("@/modules/print/pages/ProductLabelsPrintPage"),
);
