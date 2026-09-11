// Kertas & profil dokumen untuk cetak (Tahap E).
//
// Port dari pengetahuan cetak sistem lama yang sudah diverifikasi lewat tes
// cetak fisik (types/shared.ts:58 + print-shared.tsx + sales-print-shared.tsx):
// - Kertas kontinu 9,5" x 11"/2 -> widthMm 241.3, heightMm 139.7.
// - marginRightMm 37.91 BUKAN margin visual, melainkan kompensasi jangkauan
//   print head narrow-carriage (Epson LQ-310, ±7,85" dari tepi kiri).
// - widthMm TIDAK BOLEH disempitkan: @page harus identik dengan ukuran kertas
//   custom di driver, atau sebagian browser jatuh ke fallback yang salah.
//
// Modul ini self-contained (tidak mengimpor modul company): runtime cetak
// hanya butuh nilai mentah `document_profile` dari company_settings.

export const DOCUMENT_PROFILE_KEY = "document_profile";

export interface ContinuousPaperProfile {
  isDefault?: boolean;
  widthMm?: number;
  heightMm?: number;
  marginTopMm?: number;
  marginRightMm?: number;
  marginBottomMm?: number;
  marginLeftMm?: number;
  /** false = kop/rekening sudah preprinted di form fisik. */
  showLetterhead?: boolean;
}

export interface DocumentPhone {
  label: string;
  number: string;
}

export interface DocumentBankAccount {
  bank: string;
  holder: string;
  number: string;
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

/** Kalibrasi bawaan kertas kontinu 9,5" x 11"/2 (hasil tes cetak fisik lama). */
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

function positiveOr(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) && value > 0
    ? value
    : fallback;
}

function nonNegativeOr(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? value
    : fallback;
}

/** Normalisasi satu profil kertas: angka rusak -> default per field. */
export function resolvePaperProfile(
  raw: ContinuousPaperProfile | null | undefined,
): Required<ContinuousPaperProfile> {
  const r = raw ?? {};
  return {
    isDefault:
      typeof r.isDefault === "boolean"
        ? r.isDefault
        : CONTINUOUS_PAPER_DEFAULTS.isDefault,
    widthMm: positiveOr(r.widthMm, CONTINUOUS_PAPER_DEFAULTS.widthMm),
    heightMm: positiveOr(r.heightMm, CONTINUOUS_PAPER_DEFAULTS.heightMm),
    marginTopMm: nonNegativeOr(
      r.marginTopMm,
      CONTINUOUS_PAPER_DEFAULTS.marginTopMm,
    ),
    marginRightMm: nonNegativeOr(
      r.marginRightMm,
      CONTINUOUS_PAPER_DEFAULTS.marginRightMm,
    ),
    marginBottomMm: nonNegativeOr(
      r.marginBottomMm,
      CONTINUOUS_PAPER_DEFAULTS.marginBottomMm,
    ),
    marginLeftMm: nonNegativeOr(
      r.marginLeftMm,
      CONTINUOUS_PAPER_DEFAULTS.marginLeftMm,
    ),
    showLetterhead:
      typeof r.showLetterhead === "boolean"
        ? r.showLetterhead
        : CONTINUOUS_PAPER_DEFAULTS.showLetterhead,
  };
}

export interface ParsedDocumentProfile {
  profile: DocumentProfile;
  /** true = nilai tersimpan ada tapi bukan JSON valid / bukan objek. */
  corrupt: boolean;
}

/**
 * Parse toleran nilai `document_profile`: kosong/rusak -> default, cetak
 * tetap jalan (test §7 Tahap E). Tidak pernah melempar.
 */
export function parseDocumentProfile(
  raw: string | null | undefined,
): ParsedDocumentProfile {
  const empty: ParsedDocumentProfile = {
    profile: { continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS } },
    corrupt: false,
  };
  if (raw === null || raw === undefined || raw.trim() === "") return empty;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    return { profile: empty.profile, corrupt: true };
  }
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    return { profile: empty.profile, corrupt: true };
  }
  const p = parsed as DocumentProfile;
  return {
    profile: {
      headerName: typeof p.headerName === "string" ? p.headerName : undefined,
      addressLine:
        typeof p.addressLine === "string" ? p.addressLine : undefined,
      tagline: typeof p.tagline === "string" ? p.tagline : undefined,
      cityLine: typeof p.cityLine === "string" ? p.cityLine : undefined,
      phones: Array.isArray(p.phones)
        ? p.phones.filter(
            (x): x is DocumentPhone =>
              typeof x === "object" &&
              x !== null &&
              typeof (x as DocumentPhone).number === "string",
          )
        : [],
      bankAccounts: Array.isArray(p.bankAccounts)
        ? p.bankAccounts.filter(
            (x): x is DocumentBankAccount =>
              typeof x === "object" &&
              x !== null &&
              typeof (x as DocumentBankAccount).number === "string",
          )
        : [],
      sellingPoints: Array.isArray(p.sellingPoints)
        ? p.sellingPoints.filter(
            (x): x is string => typeof x === "string",
          )
        : [],
      returnNote: typeof p.returnNote === "string" ? p.returnNote : undefined,
      thanksNote: typeof p.thanksNote === "string" ? p.thanksNote : undefined,
      continuousNota: resolvePaperProfile(
        p.continuousNota &&
        typeof p.continuousNota === "object" &&
        !Array.isArray(p.continuousNota)
          ? (p.continuousNota as ContinuousPaperProfile)
          : {},
      ),
    },
    corrupt: false,
  };
}

/** Lebar konten aman = lebar kertas - margin kiri - margin kanan. */
export function contentWidthMm(p: ContinuousPaperProfile): number {
  const d = resolvePaperProfile(p);
  return d.widthMm - d.marginLeftMm - d.marginRightMm;
}

/** Tinggi konten aman = tinggi form - margin atas - margin bawah. */
export function contentHeightMm(p: ContinuousPaperProfile): number {
  const d = resolvePaperProfile(p);
  return d.heightMm - d.marginTopMm - d.marginBottomMm;
}

/** Format angka mm untuk CSS: 2 desimal maks, tanpa nol ekor ("37.91", "241.3"). */
export function fmtMm(n: number): string {
  return String(Math.round(n * 100) / 100);
}

/**
 * Aturan `@page` untuk kertas kontinu — ukuran HARUS persis profil fisik
 * (jangan disempitkan; lihat catatan di atas).
 */
export function continuousPageRule(p: ContinuousPaperProfile): string {
  const d = resolvePaperProfile(p);
  return (
    `@page { size: ${fmtMm(d.widthMm)}mm ${fmtMm(d.heightMm)}mm; ` +
    `margin: ${fmtMm(d.marginTopMm)}mm ${fmtMm(d.marginRightMm)}mm ` +
    `${fmtMm(d.marginBottomMm)}mm ${fmtMm(d.marginLeftMm)}mm; }`
  );
}

// --- mode kertas via query param (dibagi Nota, Surat Jalan, Kwitansi) ---

export type PrintPaperMode = "a4" | "continuous";
export const PAPER_MODE_PARAM = "paper";
export const HIDE_LETTERHEAD_PARAM = "hide_letterhead";

export function readPaperMode(
  searchParams: URLSearchParams,
  defaultMode: PrintPaperMode,
): PrintPaperMode {
  const raw = searchParams.get(PAPER_MODE_PARAM);
  return raw === "a4" || raw === "continuous" ? raw : defaultMode;
}

export function withPaperModeParam(
  searchParams: URLSearchParams,
  mode: PrintPaperMode,
): URLSearchParams {
  const next = new URLSearchParams(searchParams);
  next.set(PAPER_MODE_PARAM, mode);
  return next;
}

/** Toggle "kop/rekening sudah preprinted di kertas" — default dari profil. */
export function readHideLetterhead(
  searchParams: URLSearchParams,
  fallback: boolean,
): boolean {
  const raw = searchParams.get(HIDE_LETTERHEAD_PARAM);
  if (raw === "1") return true;
  if (raw === "0") return false;
  return fallback;
}

export function withHideLetterheadParam(
  searchParams: URLSearchParams,
  hidden: boolean,
): URLSearchParams {
  const next = new URLSearchParams(searchParams);
  next.set(HIDE_LETTERHEAD_PARAM, hidden ? "1" : "0");
  return next;
}
