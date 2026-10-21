import { apiGet, apiPage, apiPostFull, apiPutFull, httpClient } from "@/shared/services/http-client";
import { compactParams, positiveId, sortParams } from "@/shared/lib/list-query";
import type { SortDir } from "@/shared/lib/list-query";
import type {
  ProductOption,
  SalesClientRefState,
  SalesOrder,
  SalesOrderPayload,
  ShortClosePlan,
} from "@/modules/sales/types";

export interface SalesListParams {
  search?: string;
  /** CSV: "confirmed,completed". */
  status?: string;
  channel?: string;
  paymentTerms?: string;
  /** unpaid | partial | paid | overdue — disaring server (data bayar ada di modul payment). */
  paymentStatus?: string;
  partyId?: number;
  /** YYYY-MM-DD (WIB), inklusif. */
  from?: string;
  to?: string;
  sort?: string;
  dir?: SortDir | "";
  page: number;
  limit: number;
}

/** Filter saja (tanpa halaman) — dipakai bersama oleh list dan ekspor. */
export type SalesFilterParams = Omit<SalesListParams, "page" | "limit">;

/** SATU sumber query untuk GET /sales-orders dan /sales-orders/export. */
export function buildSalesFilterQuery(p: SalesFilterParams): Record<string, unknown> {
  return compactParams({
    search: p.search,
    status: p.status,
    channel: p.channel,
    paymentTerms: p.paymentTerms,
    paymentStatus: p.paymentStatus,
    partyId: positiveId(p.partyId),
    from: p.from,
    to: p.to,
    ...sortParams(p.sort ?? "", p.dir ?? ""),
  });
}

export function buildSalesListQuery(p: SalesListParams): Record<string, unknown> {
  const { page, limit, ...filter } = p;
  return { ...buildSalesFilterQuery(filter), page, limit };
}

// Rute + permission backend (sales/presentation/handler.go RegisterRoutes):
// list/get sales.view, create sales.create, update/confirm sales.update,
// cancel + close-short (+ pratinjau) sales.archive, approve-credit sales.approve.

export const salesService = {
  list: (p: SalesListParams, signal?: AbortSignal) =>
    apiPage<SalesOrder>("/api/v1/sales-orders", buildSalesListQuery(p), signal),

  get: (id: number) => apiGet<SalesOrder>(`/api/v1/sales-orders/${id}`),

  /** Posisi keranjang klien (POS) untuk melanjutkan dari nextStep. */
  stateByClientRef: (ref: string) =>
    apiGet<SalesClientRefState>(`/api/v1/sales-orders/by-client-ref/${encodeURIComponent(ref)}/state`),

  create: (body: SalesOrderPayload) =>
    apiPostFull<SalesOrder>("/api/v1/sales-orders", body),

  update: (id: number, body: SalesOrderPayload) =>
    apiPutFull<SalesOrder>(`/api/v1/sales-orders/${id}`, body),

  confirm: (id: number) =>
    apiPostFull<SalesOrder>(`/api/v1/sales-orders/${id}/confirm`),

  /** Alasan disimpan di order (cancel_reason) dan jejak audit. */
  cancel: (id: number, reason: string) =>
    apiPostFull<SalesOrder>(`/api/v1/sales-orders/${id}/cancel`, { reason }),

  /** Pratinjau tutup sisa order (tanpa tulis). */
  closeShortPreview: (id: number) =>
    apiGet<ShortClosePlan>(`/api/v1/sales-orders/${id}/close-short/preview`),

  /** Tutup sisa order: baris dipotong ke qty terkirim, order selesai. */
  closeShort: (id: number, reason: string) =>
    apiPostFull<SalesOrder>(`/api/v1/sales-orders/${id}/close-short`, { reason }),

  approveCredit: (id: number, dueDate: string) =>
    apiPostFull<SalesOrder>(`/api/v1/sales-orders/${id}/approve-credit`, { dueDate }),

  /** Unduh daftar order (J1) — blob xlsx/pdf dengan filter yang sama seperti list. */
  downloadExport: async (
    format: "xlsx" | "pdf",
    p: SalesFilterParams,
  ): Promise<void> => {
    const params: Record<string, unknown> = { ...buildSalesFilterQuery(p), format };
    const res = await httpClient.get("/api/v1/sales-orders/export", {
      params,
      responseType: "blob",
    });
    const blob = new Blob([res.data], {
      type:
        format === "pdf"
          ? "application/pdf"
          : "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = format === "pdf" ? "order-jual.pdf" : "order-jual.xlsx";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },
};

/** Dropdown produk (product/presentation/routes.go: search-options, q + aktif). */
export const productOptionService = {
  search: (q?: string) =>
    apiGet<ProductOption[]>("/api/v1/products/search-options", {
      q: q?.trim() ? q.trim() : undefined,
    }),
};
