// Tipe respons backend modul perusahaan — diverifikasi dari
// apps/api/internal/modules/company/presentation/handler.go
// (profileView, GetSettings/SaveSettings). Semua key camelCase.

export interface CompanyProfile {
  id: number;
  name: string;
  legalName: string;
  address: string;
  city: string;
  phone: string;
  email: string;
  taxId: string;
}

export interface SaveCompanyProfileInput {
  name: string;
  legalName: string;
  address: string;
  city: string;
  phone: string;
  email: string;
  taxId: string;
}

export type CompanySettings = Record<string, string>;

/** GET /company/settings/definitions — pengaturan terdaftar di backend (F3b-7). */
export type SettingKind = "boolean" | "enum" | "number" | "json";

export interface SettingDefinition {
  key: string;
  kind: SettingKind;
  default: string;
  enum?: string[];
  label: string;
}
