import { useEffect, useState } from "react";
import { companyService } from "@/modules/company/services/company.service";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import {
  CONTINUOUS_PAPER_DEFAULTS,
  DOCUMENT_PROFILE_KEY,
  parseDocumentProfile,
  resolvePaperProfile,
  type ContinuousPaperProfile,
  type DocumentProfile,
} from "@/modules/print/paper";
import { resolveIdentity, type PrintIdentity } from "@/modules/print/document";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toApiError } from "@/shared/services/http-client";
import type { ApiError } from "@/shared/services/http-client";

export interface PrintProfileState {
  profile: DocumentProfile;
  /** Kalibrasi kertas kontinu yang sudah dinormalisasi (siap ke @page). */
  paper: Required<ContinuousPaperProfile>;
  identity: PrintIdentity;
  /** true = nilai tersimpan rusak — default dipakai, cetak tetap jalan. */
  corrupt: boolean;
  /** Pesan bila profil perusahaan gagal dimuat (mis. peran tanpa company.view) — tidak ditelan. */
  warning: string | null;
  loading: boolean;
}

/**
 * Pesan peringatan bila profil cetak gagal dimuat. Peran tanpa izin
 * company.view (403) tetap bisa mencetak, tetapi tanpa kop/rekening/kalibrasi
 * tersimpan; peringatan menjelaskan itu alih-alih menelan galat.
 */
export function printProfileWarning(err: ApiError): string {
  if (err.status === 403) {
    return "Kop, rekening, dan kalibrasi kertas perusahaan tidak termuat karena akun ini tidak punya izin Perusahaan (lihat). Dokumen tetap bisa dicetak dengan tampilan bawaan.";
  }
  return `Kop, rekening, dan kalibrasi kertas perusahaan tidak termuat: ${err.message}. Dokumen tetap bisa dicetak dengan tampilan bawaan.`;
}

/**
 * Baca `document_profile` dari company_settings (C6) dengan fallback aman.
 * Kunci hilang/rusak -> default, halaman cetak tidak boleh gagal total.
 */
export function usePrintProfile(): PrintProfileState {
  const company = useAuthStore((s) => s.company);
  const [raw, setRaw] = useState<string | undefined>(undefined);
  const [corrupt, setCorrupt] = useState(false);
  const [warning, setWarning] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    companyService
      .getPrintProfile()
      .then((res) => {
        if (!alive) return;
        const value = res?.documentProfile;
        const parsed = parseDocumentProfile(value);
        setRaw(value);
        setCorrupt(parsed.corrupt);
      })
      .catch((err) => {
        if (!alive) return;
        setRaw(undefined);
        setCorrupt(false);
        setWarning(printProfileWarning(toApiError(err)));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  const { profile } = parseDocumentProfile(raw);
  return {
    profile,
    paper: resolvePaperProfile(profile.continuousNota),
    identity: resolveIdentity(profile, {
      companyName: company?.name,
      companyCity: company?.city,
    }),
    corrupt,
    warning,
    loading,
  };
}

/** Kalibrasi kertas kontinu saja — untuk halaman cetak dokumen. */
export function useContinuousPaperProfile(): Required<ContinuousPaperProfile> {
  const { paper } = usePrintProfile();
  return paper;
}

/** Nilai mentah + simpan untuk halaman kalibrasi (E7). */
export function useEditableDocumentProfile() {
  const { data, loading, reload } = useAsyncData(
    async () => {
      try {
        const settings = await companyService.getSettings();
        return parseDocumentProfile(settings?.[DOCUMENT_PROFILE_KEY]);
      } catch {
        return parseDocumentProfile(undefined);
      }
    },
    [],
  );

  const [profile, setProfile] = useState<DocumentProfile>({
    continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS },
  });
  const [prevData, setPrevData] = useState(data);
  if (data !== prevData) {
    setPrevData(data);
    if (data) {
      setProfile(data.profile);
    }
  }
  const corrupt = data?.corrupt ?? false;

  return { profile, setProfile, corrupt, loading, reload };
}
