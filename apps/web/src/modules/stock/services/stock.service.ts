import {
  apiGet,
  apiPage,
  apiPostFull,
  apiPutFull,
  apiUploadFull,
  httpClient,
} from "@/shared/services/http-client";
import type {
  AdjustmentPayload,
  BranchOption,
  DamagedMovePayload,
  DamagedWriteOffPayload,
  LocationPayload,
  OpeningPreview,
  OpeningResult,
  MoveLocationPayload,
  ProductDetail,
  ProductOption,
  StockBalance,
  StockLocation,
  StockMovement,
  StockTransfer,
  TransferPayload,
} from "@/modules/stock/types";

export interface MovementParams {
  productId?: number | null;
  variantId?: number | null;
  locationId?: number | null;
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  limit?: number;
}

function cleanParams(p: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(p)) {
    if (v === undefined || v === null) continue;
    if (typeof v === "string" && v.trim() === "") continue;
    if (typeof v === "number" && v <= 0 && k.endsWith("Id")) continue;
    out[k] = v;
  }
  return out;
}

function toForm(file: File): FormData {
  const form = new FormData();
  form.append("file", file);
  return form;
}

export const stockService = {
  // GET /api/v1/stock/locations?status= — array langsung (bukan paginasi).
  listLocations: (status?: string) =>
    apiGet<StockLocation[]>(
      "/api/v1/stock/locations",
      status ? { status } : undefined,
    ),

  createLocation: (body: LocationPayload) =>
    apiPostFull<StockLocation>("/api/v1/stock/locations", body),

  // Update backend hanya menerima {name, parentId} — kode tidak bisa diubah.
  updateLocation: (id: number, body: { name: string; parentId: number | null }) =>
    apiPutFull<StockLocation>(`/api/v1/stock/locations/${id}`, body),

  archiveLocation: (id: number) =>
    apiPostFull<unknown>(`/api/v1/stock/locations/${id}/archive`),

  // GET /api/v1/stock/balances?productId= — tanpa productId backend balas [].
  // Tidak ada param locationId/limit di backend: filter lokasi sisi klien.
  balancesByProduct: (productId: number) =>
    apiGet<StockBalance[]>("/api/v1/stock/balances", { productId }),

  // GET /api/v1/stock/balances/{productId} (J5) — alias path-style dengan
  // bentuk respons yang sama, untuk halaman detail deep-link.
  balanceDetail: (productId: number) =>
    apiGet<StockBalance[]>(`/api/v1/stock/balances/${productId}`),

  // GET /api/v1/stock/movements — paginasi. Param backend:
  // productId, variantId, locationId, type, dateFrom, dateTo, page, limit.
  listMovements: (params: MovementParams) =>
    apiPage<StockMovement>(
      "/api/v1/stock/movements",
      cleanParams({
        productId: params.productId,
        variantId: params.variantId,
        locationId: params.locationId,
        dateFrom: params.dateFrom,
        dateTo: params.dateTo,
        page: params.page ?? 1,
        limit: params.limit ?? 20,
      }),
    ),

  createAdjustment: (body: AdjustmentPayload) =>
    apiPostFull<unknown>("/api/v1/stock/adjustments", body),

  // GET /api/v1/stock/transfers?status=&page=&limit= — paginasi.
  listTransfers: (params: { status?: string; page?: number; limit?: number }) =>
    apiPage<StockTransfer>(
      "/api/v1/stock/transfers",
      cleanParams({
        status: params.status,
        page: params.page ?? 1,
        limit: params.limit ?? 20,
      }),
    ),

  createTransfer: (body: TransferPayload) =>
    apiPostFull<StockTransfer>("/api/v1/stock/transfers", body),

  /** Pindah lokasi satu langkah (J4) — langsung diterima, tanpa toBranchId. */
  moveLocation: (body: MoveLocationPayload) =>
    apiPostFull<StockTransfer>("/api/v1/stock/transfers/move-location", body),

  getTransfer: (id: number) => apiGet<StockTransfer>(`/api/v1/stock/transfers/${id}`),

  dispatchTransfer: (id: number) =>
    apiPostFull<StockTransfer>(`/api/v1/stock/transfers/${id}/dispatch`),

  receiveTransfer: (id: number) =>
    apiPostFull<StockTransfer>(`/api/v1/stock/transfers/${id}/receive`),

  cancelTransfer: (id: number) =>
    apiPostFull<StockTransfer>(`/api/v1/stock/transfers/${id}/cancel`),

  // GET /api/v1/stock/damaged — saldo di lokasi RUSAK (bentuk balanceView).
  damagedList: () => apiGet<StockBalance[]>("/api/v1/stock/damaged"),

  damagedMoveIn: (body: DamagedMovePayload) =>
    apiPostFull<unknown>("/api/v1/stock/damaged/move-in", body),

  damagedRestore: (body: DamagedMovePayload) =>
    apiPostFull<unknown>("/api/v1/stock/damaged/restore", body),

  // Write-off tanpa locationId: selalu memakan lokasi rusak itu sendiri.
  damagedWriteOff: (body: DamagedWriteOffPayload) =>
    apiPostFull<unknown>("/api/v1/stock/damaged/write-off", body),

  // GET /api/v1/products/search-options?q= — maks 20 baris aktif,
  // bentuk {id,code,name,sellingPrice} (tanpa varian).
  searchProductOptions: (q?: string) =>
    apiGet<ProductOption[]>(
      "/api/v1/products/search-options",
      q && q.trim() !== "" ? { q: q.trim() } : undefined,
    ),

  // Detail produk untuk daftar varian (productView backend memuat variants[]).
  getProduct: (id: number) => apiGet<ProductDetail>(`/api/v1/products/${id}`),

  // GET /api/v1/branches — paginasi; diambil 100 untuk dropdown.
  listBranches: async (): Promise<BranchOption[]> => {
    const res = await apiPage<BranchOption>("/api/v1/branches", {
      page: 1,
      limit: 100,
    });
    return res.items;
  },

  // POST multipart field "file" (httpx.FormFile("file")) →
  // {validRows, errors[{row,column,value,reason}]}. Tanpa commit.
  previewOpening: (file: File) =>
    apiUploadFull<OpeningPreview>("/api/v1/stock/opening/preview", toForm(file)),

  // POST multipart field "file" → {posted,skipped,failures[]}.
  commitOpening: (file: File) =>
    apiUploadFull<OpeningResult>("/api/v1/stock/opening/commit", toForm(file)),

  /** Unduh template Excel saldo awal — blob, bukan envelope JSON. */
  downloadOpeningTemplate: async (): Promise<void> => {
    const res = await httpClient.get("/api/v1/stock/opening/template", {
      responseType: "blob",
    });
    const blob = new Blob([res.data], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "template-saldo-awal.xlsx";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },
};
