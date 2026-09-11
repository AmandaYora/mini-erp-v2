import { apiGet, apiPage, apiPostFull, apiPutFull, httpClient } from "@/shared/services/http-client";
import type {
  ProductOption,
  SalesOrder,
  SalesOrderPayload,
} from "@/modules/sales/types";

export interface SalesListParams {
  search?: string;
  status?: string;
  channel?: string;
  page: number;
  limit: number;
}

function cleanParams(p: SalesListParams): Record<string, unknown> {
  const out: Record<string, unknown> = { page: p.page, limit: p.limit };
  if (p.search && p.search.trim() !== "") out.search = p.search.trim();
  if (p.status && p.status !== "") out.status = p.status;
  if (p.channel && p.channel !== "") out.channel = p.channel;
  return out;
}

// Rute + permission backend (sales/presentation/handler.go RegisterRoutes):
// list/get sales.view, create sales.create, update/confirm sales.update,
// cancel sales.archive, approve-credit sales.approve.

export const salesService = {
  list: (p: SalesListParams) =>
    apiPage<SalesOrder>("/api/v1/sales-orders", cleanParams(p)),

  get: (id: number) => apiGet<SalesOrder>(`/api/v1/sales-orders/${id}`),

  create: (body: SalesOrderPayload) =>
    apiPostFull<SalesOrder>("/api/v1/sales-orders", body),

  update: (id: number, body: SalesOrderPayload) =>
    apiPutFull<SalesOrder>(`/api/v1/sales-orders/${id}`, body),

  confirm: (id: number) =>
    apiPostFull<SalesOrder>(`/api/v1/sales-orders/${id}/confirm`),

  cancel: (id: number) =>
    apiPostFull<SalesOrder>(`/api/v1/sales-orders/${id}/cancel`),

  approveCredit: (id: number, dueDate: string) =>
    apiPostFull<SalesOrder>(`/api/v1/sales-orders/${id}/approve-credit`, { dueDate }),

  /** Unduh daftar order (J1) — blob xlsx/pdf dengan filter yang sama seperti list. */
  downloadExport: async (
    format: "xlsx" | "pdf",
    p: { search?: string; status?: string; channel?: string },
  ): Promise<void> => {
    const params: Record<string, unknown> = { format };
    if (p.search?.trim()) params.search = p.search.trim();
    if (p.status) params.status = p.status;
    if (p.channel) params.channel = p.channel;
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
