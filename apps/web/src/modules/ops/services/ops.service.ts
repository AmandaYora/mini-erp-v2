import { apiGet } from "@/shared/services/http-client";
import type { OpsIntegrityReport } from "@/modules/ops/types";

export const opsService = {
  // GET /api/v1/ops/integrity — dokumen setengah jadi (perm stock.update).
  integrity: (signal?: AbortSignal) =>
    apiGet<OpsIntegrityReport>("/api/v1/ops/integrity", undefined, signal),
};
