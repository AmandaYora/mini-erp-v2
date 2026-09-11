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

export interface PrintProfileState {
  profile: DocumentProfile;
  /** Kalibrasi kertas kontinu yang sudah dinormalisasi (siap ke @page). */
  paper: Required<ContinuousPaperProfile>;
  identity: PrintIdentity;
  /** true = nilai tersimpan rusak — default dipakai, cetak tetap jalan. */
  corrupt: boolean;
  loading: boolean;
}

/**
 * Baca `document_profile` dari company_settings (C6) dengan fallback aman.
 * Kunci hilang/rusak -> default, halaman cetak tidak boleh gagal total.
 */
export function usePrintProfile(): PrintProfileState {
  const company = useAuthStore((s) => s.company);
  const [raw, setRaw] = useState<string | undefined>(undefined);
  const [corrupt, setCorrupt] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    companyService
      .getSettings()
      .then((settings) => {
        if (!alive) return;
        const value = settings?.[DOCUMENT_PROFILE_KEY];
        const parsed = parseDocumentProfile(value);
        setRaw(value);
        setCorrupt(parsed.corrupt);
      })
      .catch(() => {
        if (!alive) return;
        setRaw(undefined);
        setCorrupt(false);
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
