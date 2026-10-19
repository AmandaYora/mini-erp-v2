// Tipe respons backend modul cabang — diverifikasi dari
// apps/api/internal/modules/branch/presentation/handler.go (branchView).
// Semua key camelCase.

export interface BranchItem {
  id: number;
  code: string;
  name: string;
  address: string;
  city: string;
  phone: string;
  status: string; // active | inactive
  isHead: boolean;
  /** Nama lokasi stok bawaan cabang. */
  defaultLocationLabel?: string;
}

export interface CreateBranchInput {
  code: string;
  name: string;
  address: string;
  city: string;
  phone: string;
  isHead: boolean;
  defaultLocationLabel?: string;
}

export interface UpdateBranchInput {
  code: string;
  name: string;
  address: string;
  city: string;
  phone: string;
  status: string;
  isHead: boolean;
  defaultLocationLabel?: string;
}
