// Bentuk JSON backend: apps/api/internal/modules/dashboard/contracts/client.go
// (Summary: branchId, today, monthToDate, balances, inventoryValue — camelCase).

export interface DashboardPeriod {
  from: string;
  to: string;
  revenue: number;
  expense: number;
  profit: number;
}

export interface DashboardBalances {
  cash: number;
  receivable: number;
  payable: number;
  inventory: number;
}

export interface DashboardSummary {
  branchId: number;
  today: DashboardPeriod;
  monthToDate: DashboardPeriod;
  balances: DashboardBalances;
  inventoryValue: number;
}

// Bentuk JSON backend: reporting/contracts TrendPoint (date, revenue, expense,
// profit). Didefinisikan ulang di sini agar modul dashboard tidak mengimpor
// modul lain.
export interface DashboardTrendPoint {
  date: string;
  revenue: number;
  expense: number;
  profit: number;
}

// Bentuk JSON backend: stock/contracts CriticalItem (productId, productCode,
// productName, available, minStock).
export interface CriticalStockItem {
  productId: number;
  productCode: string;
  productName: string;
  available: number;
  minStock: number;
}

// Bentuk JSON backend: sales/contracts OrderSummary (id, number, grandTotal,
// status, partyId, partyName, orderDate, dueDate).
export interface PriorityOrder {
  id: number;
  number: string;
  grandTotal: number;
  status: string;
  partyId: number;
  partyName: string;
  orderDate: string;
  dueDate: string;
}

// Bentuk JSON backend: dashboard/contracts OrderStatus — hitungan order per
// status turunan (draft/confirmed/completed/cancelled) untuk kedua arus order.
export interface OrderStatusCounts {
  sales: Record<string, number>;
  purchasing: Record<string, number>;
}

// Bentuk JSON backend: sales/contracts TopProduct — terlaris bulan berjalan
// berdasarkan qty basis.
export interface TopSeller {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  qtyBase: number;
  revenue: number;
  orders: number;
}

// Bentuk JSON backend: finance/contracts ProductMargin — margin dari buku,
// bukan dari baris order.
export interface TopMarginProduct {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  revenue: number;
  cogs: number;
  gross: number;
  percent: number;
}

// Rentang tren penjualan (F7): 30 hari … 12 bulan.
export type TrendRangeKey = "30d" | "90d" | "12m";

export const TREND_RANGES: { value: TrendRangeKey; label: string; days: number }[] = [
  { value: "30d", label: "30 hari", days: 30 },
  { value: "90d", label: "90 hari", days: 90 },
  { value: "12m", label: "12 bulan", days: 365 },
];
