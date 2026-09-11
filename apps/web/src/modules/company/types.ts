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
