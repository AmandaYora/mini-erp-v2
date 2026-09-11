import { apiGet, apiPage, apiPostFull, apiPutFull } from "@/shared/services/http-client";
import type {
  BranchItem,
  CreateBranchInput,
  UpdateBranchInput,
} from "@/modules/branches/types";

export interface ListBranchesParams {
  search?: string;
  status?: string;
  page: number;
  limit: number;
}

// Endpoint diverifikasi dari
// apps/api/internal/modules/branch/presentation/handler.go (RegisterRoutes).
export const branchService = {
  list: (params: ListBranchesParams) =>
    apiPage<BranchItem>("/api/v1/branches", { ...params }),
  get: (id: number) => apiGet<BranchItem>(`/api/v1/branches/${id}`),
  create: (body: CreateBranchInput) =>
    apiPostFull<BranchItem>("/api/v1/branches", body),
  update: (id: number, body: UpdateBranchInput) =>
    apiPutFull<BranchItem>(`/api/v1/branches/${id}`, body),
};
