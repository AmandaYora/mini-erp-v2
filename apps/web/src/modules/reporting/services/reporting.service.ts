import { apiGet } from "@/shared/services/http-client";
import type {
  InventoryReport,
  OrderReport,
  OrderReportQuery,
  StockReport,
  StockReportQuery,
  TrendPoint,
} from "@/modules/reporting/types";
import { normalizeOrderReport } from "@/modules/reporting/lib/order-report";

export const reportingService = {
  // GET /api/v1/reporting/sales-trend?from&to — YYYY-MM-DD inklusif,
  // rentang maksimal 366 hari (ditegakkan backend).
  salesTrend: (from: string, to: string) =>
    apiGet<TrendPoint[]>("/api/v1/reporting/sales-trend", { from, to }),

  // GET /api/v1/reporting/inventory — posisi stok bernilai + total.
  inventory: () => apiGet<InventoryReport>("/api/v1/reporting/inventory"),

  // GET /api/v1/reporting/orders — laporan order per arus (sales|purchasing).
  orders: async (q: OrderReportQuery, signal?: AbortSignal): Promise<OrderReport> => {
    const raw = await apiGet<Parameters<typeof normalizeOrderReport>[0]>(
      "/api/v1/reporting/orders",
      {
        type: q.type,
        status: q.status || undefined,
        from: q.from || undefined,
        to: q.to || undefined,
        page: q.page,
        limit: q.limit,
      },
      signal,
    );
    return normalizeOrderReport(raw);
  },

  // GET /api/v1/reporting/stock — posisi stok produk terpantau.
  stock: (q: StockReportQuery, signal?: AbortSignal) =>
    apiGet<StockReport>(
      "/api/v1/reporting/stock",
      { criticalOnly: q.criticalOnly ? "true" : undefined, page: q.page, limit: q.limit },
      signal,
    ),
};
