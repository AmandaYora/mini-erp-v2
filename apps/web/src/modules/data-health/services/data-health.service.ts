import { apiDeleteFull, apiGet, apiPage, apiPostFull } from "@/shared/services/http-client";
import type { AlertItemRow, AlertSummary, DataHealthReport, HealthAlert } from "@/modules/data-health/types";
import type { SnoozeValues } from "@/modules/data-health/schemas/snooze.schema";

// Izin semua rute: dashboard.alerts.view_all; area keamanan disaring backend
// menurut dashboard.alerts.security.
export const dataHealthService = {
  // GET /api/v1/dashboard/alerts?limit= — kartu "Perlu tindakan" (critical/warning, tidak ditunda).
  alerts: (limit = 5) => apiGet<AlertSummary>("/api/v1/dashboard/alerts", { limit }),

  // GET /api/v1/data-health?refresh=1 — halaman penuh; refresh dibatasi backend 1×/30 dtk.
  report: (refresh = false, signal?: AbortSignal) =>
    apiGet<DataHealthReport>("/api/v1/data-health", refresh ? { refresh: 1 } : undefined, signal),

  // GET /api/v1/data-health/:id/items — daftar rinci (A2, A7).
  items: (alertId: string, page: number, limit = 20, signal?: AbortSignal) =>
    apiPage<AlertItemRow>(`/api/v1/data-health/${encodeURIComponent(alertId)}/items`, { page, limit }, signal),

  // POST /api/v1/data-health/:id/snooze — tunda per cabang, beralasan, teraudit.
  snooze: (alertId: string, body: SnoozeValues) =>
    apiPostFull<HealthAlert>(`/api/v1/data-health/${encodeURIComponent(alertId)}/snooze`, body),

  // DELETE /api/v1/data-health/:id/snooze — aktifkan lagi (idempoten).
  unsnooze: (alertId: string) =>
    apiDeleteFull<{ alertId: string; snoozed: boolean }>(`/api/v1/data-health/${encodeURIComponent(alertId)}/snooze`),
};
