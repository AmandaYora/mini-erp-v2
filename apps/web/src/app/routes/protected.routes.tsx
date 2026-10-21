import type { RouteObject } from "react-router-dom";
import AppShell from "@/shared/layouts/AppShell";
import FocusLayout from "@/shared/layouts/FocusLayout";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { RequireAuth, RequireBranch, RequirePerm } from "@/app/routes/guards";
import { MENU_GROUPS } from "@/app/routes/registry";
import { RouteErrorBoundary } from "@/app/routes/RouteErrorBoundary";
import {
  MODULE_PAGES,
  PlaceholderPage,
  DashboardPage,
  NotFoundPage,
  DeliveryNotePrintPage,
  StockTransferPrintPage,
  SalesInvoicePrintPage,
  PaymentReceiptPrintPage,
  PosThermalReceiptPage,
  ProductLabelsPrintPage,
} from "@/app/routes/route-pages";

// Rute anak (di luar path menu persis) dengan kunci permission induknya.
const EXTRA_ROUTES: { path: string; perm: string; page: keyof typeof MODULE_PAGES & string }[] = [
  { path: "products/new", perm: "products.create", page: "/products/new" },
  { path: "products/:id", perm: "products.view", page: "/products/:id" },
  { path: "products/:id/edit", perm: "products.update", page: "/products/:id/edit" },
  { path: "customers/:id", perm: "customers.view", page: "/customers/:id" },
  { path: "suppliers/:id", perm: "suppliers.view", page: "/suppliers/:id" },
  { path: "stock/transfers/new", perm: "stock.transfer", page: "/stock/transfers/new" },
  { path: "stock/transfers/:id", perm: "stock.view", page: "/stock/transfers/:id" },
  { path: "stock/items/:productId", perm: "stock.view", page: "/stock/items/:productId" },
  { path: "purchase-orders/new", perm: "purchasing.create", page: "/purchase-orders/new" },
  { path: "purchase-orders/:id", perm: "purchasing.view", page: "/purchase-orders/:id" },
  { path: "purchase-orders/:id/edit", perm: "purchasing.update", page: "/purchase-orders/:id/edit" },
  { path: "sales-orders/new", perm: "sales.create", page: "/sales-orders/new" },
  { path: "sales-orders/:id", perm: "sales.view", page: "/sales-orders/:id" },
  { path: "sales-orders/:id/edit", perm: "sales.update", page: "/sales-orders/:id/edit" },
  { path: "goods-receipts/new", perm: "goodsreceipt.create", page: "/goods-receipts/new" },
  { path: "goods-receipts/:id", perm: "goodsreceipt.view", page: "/goods-receipts/:id" },
  { path: "deliveries/new", perm: "delivery.create", page: "/deliveries/new" },
  { path: "deliveries/new-direct", perm: "delivery.create_direct", page: "/deliveries/new-direct" },
  { path: "deliveries/:id", perm: "delivery.view", page: "/deliveries/:id" },
  { path: "payments/:id", perm: "payment.view", page: "/payments/:id" },
  { path: "sales-returns/new", perm: "salesreturn.create", page: "/sales-returns/new" },
  { path: "sales-returns/:id", perm: "salesreturn.view", page: "/sales-returns/:id" },
  { path: "purchase-returns/new", perm: "purchasereturn.create", page: "/purchase-returns/new" },
  { path: "purchase-returns/:id", perm: "purchasereturn.view", page: "/purchase-returns/:id" },
  { path: "finance/journals/new", perm: "finance.manage", page: "/finance/journals/new" },
  { path: "finance/reports/cash", perm: "finance.report.view", page: "/finance/reports/cash" },
  { path: "finance/journals/:id", perm: "finance.journal.view", page: "/finance/journals/:id" },
  { path: "finance/expenses/:id", perm: "finance.expense.view", page: "/finance/expenses/:id" },
  { path: "finance/reports/tax-detail", perm: "finance.tax_report.view", page: "/finance/reports/tax-detail" },
  { path: "finance/reports/inventory-value", perm: "finance.report.view", page: "/finance/reports/inventory-value" },
  { path: "finance/reports/margin", perm: "finance.report.view", page: "/finance/reports/margin" },
  { path: "finance/reports/receivables-payables", perm: "finance.report.view", page: "/finance/reports/receivables-payables" },
  { path: "print/calibration", perm: "company.manage", page: "/print/calibration" },
];

// Semua path MENU → halaman nyata bila ada, placeholder bila belum. Bunyi
// permission dari registry. Suspense fallback "Memuat halaman…" ada di
// AppShell di sekitar <Outlet/>.
const menuRoutes: RouteObject[] = MENU_GROUPS.flatMap((group) =>
  group.items
    .filter((item) => item.path !== "/dashboard" && item.path !== ROUTE_PATHS.pos)
    .map((item) => {
      const Page = MODULE_PAGES[item.path] ?? PlaceholderPage;
      return {
        path: item.path.replace(/^\//, ""),
        element: (
          <RequirePerm perm={item.perm}>
            <Page />
          </RequirePerm>
        ),
      };
    }),
);

const PosPage = MODULE_PAGES[ROUTE_PATHS.pos];

export const protectedRoutes: RouteObject[] = [
  // Mode Kasir (U8): layar penuh tanpa sidebar. Penjaga sama: RequireAuth -> RequireBranch -> RequirePerm.
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <FocusLayout title="Kasir" />
        </RequireBranch>
      </RequireAuth>
    ),
    children: [
      {
        // errorElement di rute anak (bukan layout) → topbar mode kasir tetap tampil.
        errorElement: <RouteErrorBoundary />,
        children: [
          {
            path: ROUTE_PATHS.pos.replace(/^\//, ""),
            element: (
              <RequirePerm perm="sales.create">
                <PosPage />
              </RequirePerm>
            ),
          },
        ],
      },
    ],
  },
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <AppShell />
        </RequireBranch>
      </RequireAuth>
    ),
    children: [
      {
        // OP-01: galat halaman tampil sebagai kartu di dalam AppShell (sidebar tetap ada).
        errorElement: <RouteErrorBoundary />,
        children: [
          {
            path: "dashboard",
            element: (
              <RequirePerm perm="dashboard.view">
                <DashboardPage />
              </RequirePerm>
            ),
          },
          ...EXTRA_ROUTES.map((r) => {
            const Page = MODULE_PAGES[r.page];
            return {
              path: r.path,
              element: (
                <RequirePerm perm={r.perm}>
                  <Page />
                </RequirePerm>
              ),
            };
          }),
          ...menuRoutes,
          // Rute tak dikenal: 404 di dalam layout (tamu diarahkan ke login oleh RequireAuth).
          { path: "*", element: <NotFoundPage /> },
        ],
      },
    ],
  },
];

// Halaman dokumen cetak (Tahap E): standalone TANPA AppShell agar sidebar /
// topbar tidak ikut tercetak. Auth + cabang + permission tetap dijaga —
// yang dihilangkan hanya chrome layout, bukan pengamanannya.
export const printRoutes: RouteObject[] = [
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <RequirePerm perm="delivery.view">
            <DeliveryNotePrintPage />
          </RequirePerm>
        </RequireBranch>
      </RequireAuth>
    ),
    path: "print/deliveries/:id",
  },
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <RequirePerm perm="stock.view">
            <StockTransferPrintPage />
          </RequirePerm>
        </RequireBranch>
      </RequireAuth>
    ),
    path: "print/transfers/:id",
  },
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <RequirePerm perm="sales.view">
            <SalesInvoicePrintPage />
          </RequirePerm>
        </RequireBranch>
      </RequireAuth>
    ),
    path: "print/sales/:id",
  },
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <RequirePerm perm="payment.view">
            <PaymentReceiptPrintPage />
          </RequirePerm>
        </RequireBranch>
      </RequireAuth>
    ),
    path: "print/payments/:id",
  },
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <RequirePerm perm="sales.create">
            <PosThermalReceiptPage />
          </RequirePerm>
        </RequireBranch>
      </RequireAuth>
    ),
    path: "print/pos/:orderId",
  },
  {
    element: (
      <RequireAuth>
        <RequireBranch>
          <RequirePerm perm="products.view">
            <ProductLabelsPrintPage />
          </RequirePerm>
        </RequireBranch>
      </RequireAuth>
    ),
    path: "print/labels",
  },
];
