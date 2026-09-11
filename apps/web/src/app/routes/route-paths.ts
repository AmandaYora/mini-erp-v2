export const ROUTE_PATHS = {
  home: "/",
  login: "/login",
  selectBranch: "/select-branch",
  forbidden: "/403",
  dashboard: "/dashboard",
  pos: "/pos",
  salesOrders: "/sales-orders",
  deliveries: "/deliveries",
  deliveriesQueue: "/deliveries/queue",
  salesReturns: "/sales-returns",
  purchaseReturns: "/purchase-returns",
  purchaseOrders: "/purchase-orders",
  goodsReceipts: "/goods-receipts",
  payments: "/payments",
  stock: "/stock",
  stockTransfers: "/stock/transfers",
  stockAdjustments: "/stock/adjustments",
  stockDamaged: "/stock/damaged",
  stockOpening: "/stock/opening",
  products: "/products",
  productCategories: "/product-categories",
  productImports: "/product-imports",
  customers: "/customers",
  suppliers: "/suppliers",
  memberTypes: "/member-types",
  financePosting: "/finance/posting",
  financeJournals: "/finance/journals",
  financeGeneralLedger: "/finance/general-ledger",
  trialBalance: "/finance/reports/trial-balance",
  profitLoss: "/finance/reports/profit-loss",
  balanceSheet: "/finance/reports/balance-sheet",
  taxSummary: "/finance/reports/tax-summary",
  taxDetail: "/finance/reports/tax-detail",
  inventoryValue: "/finance/reports/inventory-value",
  margin: "/finance/reports/margin",
  receivablesPayables: "/finance/reports/receivables-payables",
  financeExpenses: "/finance/expenses",
  financeAccounts: "/finance/accounts",
  financePeriods: "/finance/periods",
  financeOpening: "/finance/opening",
  taxAdjustments: "/finance/tax-adjustments",
  reporting: "/reporting",
  auditLogs: "/audit-logs",
  assistantSetup: "/assistant/setup",
  assistantConfig: "/assistant/config",
  users: "/users",
  roles: "/roles",
  branches: "/branches",
  company: "/company",
  printDelivery: "/print/deliveries/:id",
  printSales: "/print/sales/:id",
  printPayment: "/print/payments/:id",
  printPos: "/print/pos/:orderId",
  printLabels: "/print/labels",
  printCalibration: "/print/calibration",
} as const;

export function deliveryPrintPath(id: number | string): string {
  return `/print/deliveries/${id}`;
}

export function salesInvoicePrintPath(id: number | string): string {
  return `/print/sales/${id}`;
}

export function paymentReceiptPrintPath(id: number | string): string {
  return `/print/payments/${id}`;
}

export function posReceiptPrintPath(
  orderId: number | string,
  paymentId?: number | null,
): string {
  return paymentId != null
    ? `/print/pos/${orderId}?paymentId=${paymentId}`
    : `/print/pos/${orderId}`;
}

/** Halaman cetak label QR (J2): ?ids=1,2,3, maks 20 produk. */
export function productLabelsPrintPath(ids: (number | string)[]): string {
  return `/print/labels?ids=${ids.join(",")}`;
}

export function stockItemPath(productId: number | string): string {
  return `/stock/items/${productId}`;
}
