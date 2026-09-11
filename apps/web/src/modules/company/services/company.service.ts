import { apiGet, apiPutFull } from "@/shared/services/http-client";
import type {
  CompanyProfile,
  CompanySettings,
  SaveCompanyProfileInput,
} from "@/modules/company/types";

// Endpoint diverifikasi dari
// apps/api/internal/modules/company/presentation/handler.go (RegisterRoutes).
// GET /company/profile dapat mengembalikan data null (belum pernah disimpan).
export const companyService = {
  getProfile: () => apiGet<CompanyProfile | null>("/api/v1/company/profile"),
  saveProfile: (body: SaveCompanyProfileInput) =>
    apiPutFull<CompanyProfile>("/api/v1/company/profile", body),
  getSettings: async () => {
    const data = await apiGet<CompanySettings | null>(
      "/api/v1/company/settings",
    );
    return data ?? {};
  },
  // Save bersifat merge di server (bukan replace).
  saveSettings: (settings: CompanySettings) =>
    apiPutFull<CompanySettings>("/api/v1/company/settings", { settings }),
};
