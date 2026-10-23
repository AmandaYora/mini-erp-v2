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
  deliveriesNew: "/deliveries/new",
  /** SJ Langsung (ADR-0022): surat jalan tanpa order, izin delivery.create_direct. */
  deliveriesNewDirect: "/deliveries/new-direct",
  salesReturns: "/sales-returns",
  purchaseReturns: "/purchase-returns",
  purchaseOrders: "/purchase-orders",
  goodsReceipts: "/goods-receipts",
  goodsReceiptsNew: "/goods-receipts/new",
  payments: "/payments",
  stock: "/stock",
  stockBalances: "/stock/balances",
  stockTransfers: "/stock/transfers",
  stockTransfersNew: "/stock/transfers/new",
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
  cashReport: "/finance/reports/cash",
  financeJournalNew: "/finance/journals/new",
  financeExpenses: "/finance/expenses",
  financeAccounts: "/finance/accounts",
  financePeriods: "/finance/periods",
  financeIntegrity: "/finance/integrity",
  financeCostRestatement: "/finance/cost-restatement",
  financeRevaluations: "/finance/inventory-revaluations",
  opsIntegrity: "/ops/integrity",
  /** Alert kesehatan data (ADR-0025), izin dashboard.alerts.view_all. */
  dataHealth: "/data-health",
  financeOpening: "/finance/opening",
  taxAdjustments: "/finance/tax-adjustments",
  reporting: "/reporting",
  reportingOrders: "/reporting/orders",
  reportingStock: "/reporting/stock",
  partyReconciliation: "/reporting/party-reconciliation",
  auditLogs: "/audit-logs",
  assistantSetup: "/assistant/setup",
  assistantConfig: "/assistant/config",
  users: "/users",
  roles: "/roles",
  branches: "/branches",
  company: "/company",
  printDelivery: "/print/deliveries/:id",
  printTransfer: "/print/transfers/:id",
  printSales: "/print/sales/:id",
  printPayment: "/print/payments/:id",
  printPos: "/print/pos/:orderId",
  printLabels: "/print/labels",
  printCalibration: "/print/calibration",
} as const;

export function deliveryPrintPath(id: number | string): string {
  return `/print/deliveries/${id}`;
}

export function transferPrintPath(id: number | string): string {
  return `/print/transfers/${id}`;
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
