import {
  apiGet,
  apiPage,
  apiPost,
  apiPostFull,
  apiPutFull,
  apiUploadFull,
  httpClient,
} from "@/shared/services/http-client";
import type {
  AdjustmentPayload,
  BranchOption,
  DamagedMovePayload,
  ScanResult,
  DamagedPage,
  DamagedWriteOffPayload,
  LocationPayload,
  OpeningPreview,
  OpeningResult,
  MoveLocationPayload,
  ProductDetail,
  StockBalance,
  StockBalanceRow,
  StockLocation,
  StockMovement,
  StockTransfer,
  TransferPayload,
} from "@/modules/stock/types";
import { isStrictAdjust } from "@/modules/stock/schemas/stock.schema";

export interface MovementParams {
  productId?: number | null;
  variantId?: number | null;
  locationId?: number | null;
  /** "in" | "out" (tipe mutasi, ST-04). */
  type?: string;
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  limit?: number;
}

export interface BalanceListParams {
  locationId?: number | null;
  negative?: boolean;
  /** Cari kode/nama produk (server). */
  search?: string;
  /** Hanya produk di/bawah batas minimum. */
  criticalOnly?: boolean;
  /** Sertakan lokasi rusak (default server: tidak). */
  includeDamaged?: boolean;
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
  // GET /api/v1/stock/locations?status=&branchId= — array langsung (bukan paginasi).
  // branchId (K-06) meminta lokasi cabang lain (tujuan transfer); backend
  // menjawab 403 bila user tak punya akses ke cabang itu.
  listLocations: (status?: string, branchId?: number | null) =>
    apiGet<StockLocation[]>("/api/v1/stock/locations", {
      ...(status ? { status } : {}),
      ...(branchId ? { branchId } : {}),
    }),

  createLocation: (body: LocationPayload) =>
    apiPostFull<StockLocation>("/api/v1/stock/locations", body),

  // Update menerima {code?, name, parentId, forceTransfer?, isPickingArea?, isPrimary?, sortOrder?}.
  updateLocation: (
    id: number,
    body: {
      code?: string;
      name: string;
      parentId: number | null;
      forceTransfer?: boolean;
      isPickingArea?: boolean;
      isPrimary?: boolean;
      sortOrder?: number;
    },
  ) =>
    apiPutFull<StockLocation>(`/api/v1/stock/locations/${id}`, body),

  archiveLocation: (id: number) =>
    apiPostFull<unknown>(`/api/v1/stock/locations/${id}/archive`),

  // GET /api/v1/stock/balances?productId= — tanpa productId backend balas [].
  // Tidak ada param locationId/limit di backend: filter lokasi sisi klien.
  balancesByProduct: (productId: number) =>
    apiGet<StockBalance[]>("/api/v1/stock/balances", { productId }),

  // GET /api/v1/stock/balances TANPA productId — saldo seluruh cabang,
  // paginasi {page,limit,total,total_pages}; filter locationId, negative=true.
  listBalances: (params: BalanceListParams) =>
    apiPage<StockBalanceRow>(
      "/api/v1/stock/balances",
      cleanParams({
        locationId: params.locationId,
        negative: params.negative ? true : undefined,
        search: params.search,
        criticalOnly: params.criticalOnly ? true : undefined,
        includeDamaged: params.includeDamaged ? true : undefined,
        page: params.page ?? 1,
        limit: params.limit ?? 20,
      }),
    ),

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
        type: params.type,
        dateFrom: params.dateFrom,
        dateTo: params.dateTo,
        page: params.page ?? 1,
        limit: params.limit ?? 20,
      }),
    ),

  // Kebijakan stock_adjust_always_approve dari GET /company/settings (merge
  // key-value string). Ketat secara default: hanya "false" eksplisit yang
  // melonggarkan. Gagal baca (tanpa izin) = ketat; server tetap otoritas.
  adjustAlwaysApprove: async (): Promise<boolean> => {
    const s = await apiGet<Record<string, string> | null>(
      "/api/v1/company/settings",
    );
    return isStrictAdjust(s?.stock_adjust_always_approve);
  },

  createAdjustment: (body: AdjustmentPayload) =>
    apiPostFull<unknown>("/api/v1/stock/adjustments", body),

  // GET /api/v1/stock/transfers?status=&direction=out|in|internal&page=&limit= — paginasi.
  listTransfers: (
    params: { status?: string; direction?: string; page?: number; limit?: number },
    signal?: AbortSignal,
  ) =>
    apiPage<StockTransfer>(
      "/api/v1/stock/transfers",
      cleanParams({
        status: params.status,
        direction: params.direction,
        page: params.page ?? 1,
        limit: params.limit ?? 20,
      }),
      signal,
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

  /** Batalkan transfer draf/dikirim — alasan wajib {reason}. */
  cancelTransfer: (id: number, reason: string) =>
    apiPostFull<StockTransfer>(`/api/v1/stock/transfers/${id}/cancel`, { reason }),

  // GET /api/v1/stock/damaged — saldo di lokasi RUSAK (bentuk balanceView).
  // Respons objek {items,total,withStock,empty,totalEstimatedLoss,...}; paginasi server (ST-03).
  damagedList: (p: { page: number; limit: number; empty?: boolean }) =>
    apiGet<DamagedPage>("/api/v1/stock/damaged", {
      page: p.page,
      limit: p.limit,
      empty: p.empty ? true : undefined,
    }),

  damagedMoveIn: (body: DamagedMovePayload) =>
    apiPostFull<unknown>("/api/v1/stock/damaged/move-in", body),

  damagedRestore: (body: DamagedMovePayload) =>
    apiPostFull<unknown>("/api/v1/stock/damaged/restore", body),

  // Write-off tanpa locationId: selalu memakan lokasi rusak itu sendiri.
  damagedWriteOff: (body: DamagedWriteOffPayload) =>
    apiPostFull<unknown>("/api/v1/stock/damaged/write-off", body),

  // POST /api/v1/stock/scan-product {barcode} — barcode, kode varian, atau kode produk (ST-06).
  scan: (barcode: string) => apiPost<ScanResult>("/api/v1/stock/scan-product", { barcode }),

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

  // GET /api/v1/branches/my-access — cabang aktif yang boleh diakses user
  // (login saja; tujuan transfer hanya cabang yang boleh dijangkau, K-06).
  listAccessibleBranches: (): Promise<BranchOption[]> =>
    apiGet<BranchOption[]>("/api/v1/branches/my-access"),

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
