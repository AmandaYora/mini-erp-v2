// Profil dokumen (kop + kalibrasi kertas) — satu kunci JSON `document_profile`
// di company_settings (Tahap C6). Bentuk meniru DocumentProfile sistem lama
// (types/shared.ts), dengan fallback default bila kunci kosong/rusak — cetak
// tidak boleh gagal total gara-gara setting (test §7 Tahap E).
export interface DocumentPhone {
  label: string;
  number: string;
}

export interface DocumentBankAccount {
  bank: string;
  holder: string;
  number: string;
}

export interface ContinuousPaperProfile {
  isDefault?: boolean;
  widthMm?: number;
  heightMm?: number;
  marginTopMm?: number;
  marginRightMm?: number;
  marginBottomMm?: number;
  marginLeftMm?: number;
  showLetterhead?: boolean;
}

export interface DocumentProfile {
  headerName?: string;
  addressLine?: string;
  tagline?: string;
  cityLine?: string;
  phones?: DocumentPhone[];
  bankAccounts?: DocumentBankAccount[];
  sellingPoints?: string[];
  returnNote?: string;
  thanksNote?: string;
  continuousNota?: ContinuousPaperProfile;
}

/** Kalibrasi bawaan kertas kontinu 9,5" × 11"/2 (hasil tes cetak fisik lama). */
export const CONTINUOUS_PAPER_DEFAULTS: Required<ContinuousPaperProfile> = {
  isDefault: false,
  widthMm: 241.3,
  heightMm: 139.7,
  marginTopMm: 4,
  marginRightMm: 37.91,
  marginBottomMm: 7,
  marginLeftMm: 6,
  showLetterhead: true,
};

export const EMPTY_PROFILE: DocumentProfile = {
  headerName: "",
  addressLine: "",
  tagline: "",
  cityLine: "",
  phones: [],
  bankAccounts: [],
  sellingPoints: [],
  returnNote: "",
  thanksNote: "",
  continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS },
};

/** Parse toleran: rusak/kosong → default (prinsip: cetak tetap jalan). */
export function parseDocumentProfile(raw: string | undefined): { profile: DocumentProfile; corrupt: boolean } {
  if (!raw || raw.trim() === "") return { profile: structuredClone(EMPTY_PROFILE), corrupt: false };
  try {
    const parsed = JSON.parse(raw) as DocumentProfile;
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("bukan objek");
    }
    return {
      profile: {
        ...structuredClone(EMPTY_PROFILE),
        ...parsed,
        phones: Array.isArray(parsed.phones) ? parsed.phones : [],
        bankAccounts: Array.isArray(parsed.bankAccounts) ? parsed.bankAccounts : [],
        sellingPoints: Array.isArray(parsed.sellingPoints) ? parsed.sellingPoints : [],
        continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS, ...(parsed.continuousNota ?? {}) },
      },
      corrupt: false,
    };
  } catch {
    return { profile: structuredClone(EMPTY_PROFILE), corrupt: true };
  }
}

/** Lebar konten aman = lebar kertas − margin kiri − margin kanan. */
export function contentWidthMm(p: ContinuousPaperProfile): number {
  const d = { ...CONTINUOUS_PAPER_DEFAULTS, ...p };
  return d.widthMm - d.marginLeftMm - d.marginRightMm;
}
