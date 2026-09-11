import { apiGet, apiPage, apiPatchFull, apiPostFull, apiPutFull } from "@/shared/services/http-client";
import type {
  BranchOption,
  CreateUserInput,
  UpdateUserInput,
  UserDetail,
  UserItem,
} from "@/modules/users/types";

export interface ListUsersParams {
  search?: string;
  status?: string;
  page: number;
  limit: number;
}

// Endpoint diverifikasi dari
// apps/api/internal/modules/user/presentation/routes.go.
export const userService = {
  list: (params: ListUsersParams) =>
    apiPage<UserItem>("/api/v1/users", { ...params }),
  get: (id: number) => apiGet<UserDetail>(`/api/v1/users/${id}`),
  create: (body: CreateUserInput) =>
    apiPostFull<UserItem>("/api/v1/users", body),
  update: (id: number, body: UpdateUserInput) =>
    apiPutFull<UserItem>(`/api/v1/users/${id}`, body),
  setStatus: (id: number, status: "active" | "inactive") =>
    apiPatchFull<unknown>(`/api/v1/users/${id}/status`, { status }),
  changePassword: (id: number, password: string) =>
    apiPostFull<unknown>(`/api/v1/users/${id}/password`, { password }),
  // Opsi cabang untuk form (endpoint cabang paginasi; ambil banyak sekaligus).
  branchOptions: async () => {
    const { items } = await apiPage<BranchOption>("/api/v1/branches", {
      page: 1,
      limit: 200,
    });
    return items;
  },
};
