import { apiGet, apiPutFull } from "@/shared/services/http-client";
import type {
  CompanyProfile,
  CompanySettings,
  SettingDefinition,
  SaveCompanyProfileInput,
} from "@/modules/company/types";

// Endpoint diverifikasi dari
// apps/api/internal/modules/company/presentation/handler.go (RegisterRoutes).
// GET /company/profile dapat mengembalikan data null (belum pernah disimpan).
export const companyService = {
  getProfile: () => apiGet<CompanyProfile | null>("/api/v1/company/profile"),
  saveProfile: (body: SaveCompanyProfileInput) =>
    apiPutFull<CompanyProfile>("/api/v1/company/profile", body),
  // Isi minimal untuk mencetak; terbuka untuk semua pengguna berpeluang cabang
  // (tanpa izin company.view).
  getPrintProfile: async () => {
    const data = await apiGet<{ name?: string; legalName?: string; address?: string; city?: string; phone?: string; documentProfile?: string } | null>(
      "/api/v1/company/print-profile",
    );
    return data ?? {};
  },
  getSettings: async () => {
    const data = await apiGet<CompanySettings | null>(
      "/api/v1/company/settings",
    );
    return data ?? {};
  },
  // Kunci pengaturan terdaftar (jenis, bawaan, nilai enum, label).
  getSettingDefinitions: async () =>
    (await apiGet<SettingDefinition[] | null>("/api/v1/company/settings/definitions")) ?? [],
  // Save bersifat merge di server (bukan replace).
  saveSettings: (settings: CompanySettings) =>
    apiPutFull<CompanySettings>("/api/v1/company/settings", { settings }),
};
