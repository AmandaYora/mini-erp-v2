// Koreksi harga modal — HANYA via helper http-client. Endpoint (correction_handlers.go):
// - POST /api/v1/finance/cost-restatement/preview {date?} → RestateReport (dry run, perm finance.manage).
// - POST /api/v1/finance/cost-restatement/commit {date?, reason} → RestateReport (perm finance.close.force).
// - GET  /api/v1/finance/inventory-revaluations?page&limit → paginasi RevaluationResult (perm finance.view).
// - GET  /api/v1/finance/inventory-revaluations/positions?productId → RevaluationPosition[] (perm finance.manage).
// - POST /api/v1/finance/inventory-revaluations/preview {productId, variantId, newUnitCost, date?} → RevaluationResult.
// - POST /api/v1/finance/inventory-revaluations/commit {…, reason, clientRef} → RevaluationResult
//   (201; replay clientRef yang sama → 200 `replayed`; perm finance.close.force).

import { apiGet, apiPage, apiPostFull } from "@/shared/services/http-client";
import type { RestateReport, RevaluationPosition, RevaluationResult } from "@/modules/finance/types";

export interface RevaluationRequest {
  productId: number;
  variantId: number;
  newUnitCost: number;
  date?: string;
  reason?: string;
  clientRef?: string;
}

export const correctionsService = {
  restatePreview: (date?: string) =>
    apiPostFull<RestateReport>("/api/v1/finance/cost-restatement/preview", date ? { date } : {}),

  restateCommit: (reason: string, date?: string) =>
    apiPostFull<RestateReport>("/api/v1/finance/cost-restatement/commit", date ? { date, reason } : { reason }),

  revaluationPositions: (productId: number) =>
    apiGet<RevaluationPosition[] | null>("/api/v1/finance/inventory-revaluations/positions", { productId }),

  revaluationPreview: (req: RevaluationRequest) =>
    apiPostFull<RevaluationResult>("/api/v1/finance/inventory-revaluations/preview", req),

  revaluationCommit: (req: RevaluationRequest) =>
    apiPostFull<RevaluationResult>("/api/v1/finance/inventory-revaluations/commit", req),

  listRevaluations: (page = 1, limit = 20) =>
    apiPage<RevaluationResult>("/api/v1/finance/inventory-revaluations", { page, limit }),
};
