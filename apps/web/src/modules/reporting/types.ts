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

// GET /reporting/orders. CATATAN: OrderSummary di backend (sales/purchasing
// contracts) belum memakai tag json, sehingga kunci baris bisa PascalCase
// (ID, Number, ...). normalizeOrderRow() menerima kedua bentuk.
export type OrderReportType = "sales" | "purchasing";

export interface OrderReportRow {
  id: number;
  number: string;
  grandTotal: number;
  status: string;
  partyId: number;
  partyName: string;
  orderDate: string;
  dueDate: string;
}

export interface OrderReport {
  type: OrderReportType;
  rows: OrderReportRow[];
  total: number;
  grandTotal: number;
}

export interface OrderReportQuery {
  type: OrderReportType;
  status: string;
  from: string;
  to: string;
  page: number;
  limit: number;
}

// GET /reporting/stock (stock/contracts StockReport).
export interface StockReportRow {
  productId: number;
  productCode: string;
  productName: string;
  onHand: number;
  reserved: number;
  available: number;
  minStock: number;
  critical: boolean;
}

export interface StockReport {
  rows: StockReportRow[];
  total: number;
}

export interface StockReportQuery {
  criticalOnly: boolean;
  page: number;
  limit: number;
}

/** Satu pihak yang buku (akun kendali) dan dokumennya tidak sama. */
export interface PartyReconRow {
  partyId: number;
  partyName: string;
  /** Saldo akun kendali: Piutang + Uang Muka Pelanggan, atau Hutang + Uang Muka Supplier. */
  books: number;
  /** Saldo dokumen: order − pembayaran − retur + refund. */
  documents: number;
  /** books − documents */
  difference: number;
  openOrders: number;
  /** Masih ada order berjalan: selisih bisa karena waktu (buku per pengiriman, dokumen per order). */
  timing: boolean;
}

export type PartyReconSide = "receivable" | "payable";

export interface PartyRecon {
  side: PartyReconSide;
  partiesChecked: number;
  matched: number;
  rows: PartyReconRow[];
  totalDifference: number;
  unexplained: number;
}
