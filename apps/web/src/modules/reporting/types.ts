// Bentuk JSON backend: apps/api/internal/modules/reporting/contracts/client.go
// (TrendPoint: date/revenue/expense/profit; InventoryReport: positions/total)
// dan finance/contracts InventoryPosition — camelCase.

export interface TrendPoint {
  date: string;
  revenue: number;
  expense: number;
  profit: number;
}

export interface InventoryPosition {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  qty: number;
  avgCost: number;
  value: number;
}

export interface InventoryReport {
  positions: InventoryPosition[];
  total: number;
}
