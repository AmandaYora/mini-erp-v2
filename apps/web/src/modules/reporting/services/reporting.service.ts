import { apiGet } from "@/shared/services/http-client";
import type {
  InventoryReport,
  TrendPoint,
} from "@/modules/reporting/types";

export const reportingService = {
  // GET /api/v1/reporting/sales-trend?from&to — YYYY-MM-DD inklusif,
  // rentang maksimal 366 hari (ditegakkan backend).
  salesTrend: (from: string, to: string) =>
    apiGet<TrendPoint[]>("/api/v1/reporting/sales-trend", { from, to }),

  // GET /api/v1/reporting/inventory — posisi stok bernilai + total.
  inventory: () => apiGet<InventoryReport>("/api/v1/reporting/inventory"),
};
