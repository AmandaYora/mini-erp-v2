import { apiGet } from "@/shared/services/http-client";
import type {
  CriticalStockItem,
  DashboardSummary,
  DashboardTrendPoint,
  OpenDirectSummary,
  OrderStatusCounts,
  PriorityOrder,
  TopMarginProduct,
  TopSeller,
} from "@/modules/dashboard/types";

// Status kanal WhatsApp asisten (DB-01) — izin assistant.view, bukan dashboard.view.
export interface WhatsAppChannel {
  connected: boolean;
  phone?: string;
  qrDataUrl?: string;
  lastError?: string;
}

export const dashboardService = {
  // GET /api/v1/assistant/channel — terhubung / menghubungkan (ada QR) / belum.
  whatsappChannel: () => apiGet<WhatsAppChannel>("/api/v1/assistant/channel"),

  // GET /api/v1/dashboard/summary — snapshot operasional cabang aktif.
  summary: () => apiGet<DashboardSummary>("/api/v1/dashboard/summary"),

  // GET /api/v1/reporting/sales-trend?from&to — angka harian untuk grafik.
  salesTrend: (from: string, to: string) =>
    apiGet<DashboardTrendPoint[]>("/api/v1/reporting/sales-trend", {
      from,
      to,
    }),

  // GET /api/v1/dashboard/critical-stock — stok di/lewat batas minimum.
  criticalStock: (limit = 20) =>
    apiGet<CriticalStockItem[]>("/api/v1/dashboard/critical-stock", { limit }),

  // GET /api/v1/dashboard/priority-orders — SO terkonfirmasi dekat/lewat tempo.
  priorityOrders: (limit = 10) =>
    apiGet<PriorityOrder[]>("/api/v1/dashboard/priority-orders", { limit }),

  // GET /api/v1/dashboard/order-status — hitungan order per status turunan.
  orderStatus: () => apiGet<OrderStatusCounts>("/api/v1/dashboard/order-status"),

  // GET /api/v1/dashboard/top-sellers — 5 terlaris bulan berjalan.
  topSellers: (limit = 5) =>
    apiGet<TopSeller[]>("/api/v1/dashboard/top-sellers", { limit }),

  // GET /api/v1/dashboard/top-margin — 5 margin tertinggi bulan berjalan.
  topMargin: (limit = 5) =>
    apiGet<TopMarginProduct[]>("/api/v1/dashboard/top-margin", { limit }),

  // GET /api/v1/deliveries/open-direct?limit=0 — hitungan SJ Langsung yang belum
  // diorder (ADR-0022); izin delivery.view, bukan dashboard.view.
  openDirect: () =>
    apiGet<OpenDirectSummary>("/api/v1/deliveries/open-direct", { limit: 0 }),
};
