// Kalibrasi kertas (E7) — ubah lebar/tinggi/margin lalu cetak halaman uji
// tanpa deploy ulang. Menyimpan ke kunci `document_profile` di
// company_settings (merge di server, seperti halaman Perusahaan).

import { useMemo, useState } from "react";
import { Printer } from "lucide-react";
import {
  Button,
  Checkbox,
  FormField,
  PageHeader,
  SectionCard,
  TextInput,
} from "@/shared/components/ui";
import { Page } from "@/shared/components/patterns";
import { CardSkeleton } from "@/shared/components/feedback/skeleton";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { companyService } from "@/modules/company/services/company.service";
import {
  CONTINUOUS_PAPER_DEFAULTS,
  DOCUMENT_PROFILE_KEY,
  contentHeightMm,
  contentWidthMm,
  continuousPageRule,
  fmtMm,
  resolvePaperProfile,
  type ContinuousPaperProfile,
  type DocumentProfile,
} from "@/modules/print/paper";
import { useEditableDocumentProfile } from "@/modules/print/hooks/usePrintProfile";
import { calibrationTestStyles } from "@/modules/print/styles";

function parseNum(raw: string): number | null {
  const t = raw.trim().replace(",", ".");
  if (t === "") return null;
  const v = Number(t);
  return Number.isFinite(v) ? v : null;
}

function str(v: unknown): string {
  if (typeof v === "number") return String(v);
  if (typeof v === "string") return v;
  return "";
}

export default function PaperCalibrationPage() {
  const canManage = useAuthStore((s) => s.can("company.manage"));
  const { profile, setProfile, corrupt, loading, reload } = useEditableDocumentProfile();

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const paper: ContinuousPaperProfile = profile.continuousNota ?? {};
  // Primitif didestruktur agar deps useMemo stabil (objek paper identitasnya
  // bisa baru tiap render).
  const {
    widthMm,
    heightMm,
    marginTopMm,
    marginRightMm,
    marginBottomMm,
    marginLeftMm,
    showLetterhead,
    isDefault,
  } = paper;

  // Pratinjau live: field kosong/tak valid jatuh ke default per field.
  const preview = useMemo(() => {
    const width = parseNum(str(widthMm));
    const height = parseNum(str(heightMm));
    const top = parseNum(str(marginTopMm));
    const right = parseNum(str(marginRightMm));
    const bottom = parseNum(str(marginBottomMm));
    const left = parseNum(str(marginLeftMm));
    return resolvePaperProfile({
      isDefault,
      widthMm: width !== null && width > 0 ? width : undefined,
      heightMm: height !== null && height > 0 ? height : undefined,
      marginTopMm: top !== null && top >= 0 ? top : undefined,
      marginRightMm: right !== null && right >= 0 ? right : undefined,
      marginBottomMm: bottom !== null && bottom >= 0 ? bottom : undefined,
      marginLeftMm: left !== null && left >= 0 ? left : undefined,
      showLetterhead,
    });
  }, [widthMm, heightMm, marginTopMm, marginRightMm, marginBottomMm, marginLeftMm, showLetterhead, isDefault]);

  function setPaper(patch: Partial<ContinuousPaperProfile>) {
    setProfile((prev) => ({
      ...prev,
      continuousNota: { ...(prev.continuousNota ?? {}), ...patch },
    }));
  }

  function setPaperText(key: keyof ContinuousPaperProfile, value: string) {
    const v = parseNum(value);
    // Simpan angka bila valid; biarkan string mentah bila belum valid agar
    // pengguna bisa mengetik (validasi final saat simpan).
    setPaper({ [key]: v === null && value.trim() !== "" ? value : v } as Partial<ContinuousPaperProfile>);
  }

  async function handleSave() {
    const width = Number(widthMm);
    const height = Number(heightMm);
    if (!Number.isFinite(width) || width <= 0) {
      setError("Lebar kertas harus angka lebih dari 0.");
      return;
    }
    if (!Number.isFinite(height) || height <= 0) {
      setError("Tinggi kertas harus angka lebih dari 0.");
      return;
    }
    const margins: Array<[unknown, string]> = [
      [marginTopMm, "Margin atas"],
      [marginRightMm, "Margin kanan"],
      [marginBottomMm, "Margin bawah"],
      [marginLeftMm, "Margin kiri"],
    ];
    for (const [rawValue, label] of margins) {
      const v = typeof rawValue === "number" ? rawValue : Number(rawValue);
      if (!Number.isFinite(v) || v < 0) {
        setError(`${label} harus angka 0 atau lebih.`);
        return;
      }
    }
    setError(null);
    setSaving(true);
    try {
      const next: DocumentProfile = {
        ...profile,
        continuousNota: {
          ...(profile.continuousNota ?? {}),
          widthMm: width,
          heightMm: height,
          marginTopMm: Number(marginTopMm),
          marginRightMm: Number(marginRightMm),
          marginBottomMm: Number(marginBottomMm),
          marginLeftMm: Number(marginLeftMm),
        },
      };
      const res = await companyService.saveSettings({
        [DOCUMENT_PROFILE_KEY]: JSON.stringify(next),
      });
      toast.fromServer(res.message, "Kalibrasi kertas disimpan");
      void reload();
    } catch (err) {
      const msg = toApiError(err).message;
      setError(msg);
      toast.danger("Gagal menyimpan kalibrasi", msg);
    } finally {
      setSaving(false);
    }
  }

  const readOnlyHint = !canManage ? (
    <Notice tone="info" title="Mode baca saja">
      Anda tidak memiliki izin mengubah kalibrasi. Isian terkunci dan tidak dapat disimpan; pratinjau dan cetak uji tetap bisa dipakai.
    </Notice>
  ) : null;

  return (
    <Page className="print:block">
      <style>{calibrationTestStyles(preview)}</style>
      <PageHeader
        title="Kalibrasi kertas"
        description="Ubah ukuran form lalu cetak halaman uji — tanpa deploy ulang."
        actions={
          <Button onClick={() => window.print()}>
            <Printer aria-hidden /> Cetak halaman uji
          </Button>
        }
      />

      {loading ? (
        <div className="no-print grid gap-6 lg:grid-cols-[22rem_minmax(0,1fr)]">
          <CardSkeleton />
          <CardSkeleton />
        </div>
      ) : (
        <div className="grid items-start gap-6 lg:grid-cols-[22rem_minmax(0,1fr)] print:block">
          <div className="no-print flex flex-col gap-6">
            {readOnlyHint}
            {corrupt ? (
              <Notice tone="warning" title="Profil tersimpan rusak">
                Nilai {DOCUMENT_PROFILE_KEY} bukan JSON valid — form menampilkan default. Simpan untuk menimpanya.
              </Notice>
            ) : null}
            {error !== null ? <Notice tone="danger" title={error} /> : null}

            <SectionCard
              title="Ukuran form kontinu (mm)"
              description="Ukuran harus identik dengan ukuran kertas custom di driver printer. Margin kanan 37,91 mengkompensasi jangkauan print head, bukan margin visual."
            >
              <fieldset disabled={!canManage || saving} className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <FormField label="Lebar" htmlFor="calib-width">
                    <TextInput id="calib-width" inputMode="decimal" value={str(widthMm)} onChange={(e) => setPaperText("widthMm", e.target.value)} />
                  </FormField>
                  <FormField label="Tinggi" htmlFor="calib-height">
                    <TextInput id="calib-height" inputMode="decimal" value={str(heightMm)} onChange={(e) => setPaperText("heightMm", e.target.value)} />
                  </FormField>
                  <FormField label="Margin atas" htmlFor="calib-mt">
                    <TextInput id="calib-mt" inputMode="decimal" value={str(marginTopMm)} onChange={(e) => setPaperText("marginTopMm", e.target.value)} />
                  </FormField>
                  <FormField label="Margin kanan" htmlFor="calib-mr">
                    <TextInput id="calib-mr" inputMode="decimal" value={str(marginRightMm)} onChange={(e) => setPaperText("marginRightMm", e.target.value)} />
                  </FormField>
                  <FormField label="Margin bawah" htmlFor="calib-mb">
                    <TextInput id="calib-mb" inputMode="decimal" value={str(marginBottomMm)} onChange={(e) => setPaperText("marginBottomMm", e.target.value)} />
                  </FormField>
                  <FormField label="Margin kiri" htmlFor="calib-ml">
                    <TextInput id="calib-ml" inputMode="decimal" value={str(marginLeftMm)} onChange={(e) => setPaperText("marginLeftMm", e.target.value)} />
                  </FormField>
                </div>
                <label htmlFor="calib-letterhead" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
                  <Checkbox
                    id="calib-letterhead"
                    checked={showLetterhead ?? CONTINUOUS_PAPER_DEFAULTS.showLetterhead}
                    onCheckedChange={(v) => setPaper({ showLetterhead: v === true })}
                  />
                  Tampilkan kop (matikan untuk form preprinted)
                </label>
                <label htmlFor="calib-default" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
                  <Checkbox
                    id="calib-default"
                    checked={isDefault ?? false}
                    onCheckedChange={(v) => setPaper({ isDefault: v === true })}
                  />
                  Jadikan mode cetak bawaan
                </label>
              </fieldset>
              {canManage ? (
                <div className="mt-5 flex justify-end border-t border-border pt-4">
                  <Button onClick={() => void handleSave()} loading={saving}>
                    {saving ? "Menyimpan…" : "Simpan kalibrasi"}
                  </Button>
                </div>
              ) : null}
            </SectionCard>

            <SectionCard title="Hasil @page" description="Aturan yang akan dipakai halaman cetak mode kontinu.">
              <code className="block text-xs break-all text-muted-foreground">{continuousPageRule(preview)}</code>
              <p className="mt-2 text-xs text-muted-foreground">
                Lebar konten: {fmtMm(contentWidthMm(preview))} mm · Tinggi konten: {fmtMm(contentHeightMm(preview))} mm
              </p>
            </SectionCard>
          </div>

          {/* Pratinjau = lembar uji yang sama dengan yang dicetak (kontrol di kiri .no-print). */}
          <div className="min-w-0 overflow-x-auto rounded-xl border border-border bg-muted p-4 print:overflow-visible print:rounded-none print:border-0 print:bg-transparent print:p-0">
            <CalibrationSheet preview={preview} />
          </div>
        </div>
      )}
    </Page>
  );
}

function CalibrationSheet({ preview }: { preview: ReturnType<typeof resolvePaperProfile> }) {
  return (
    <div className="calib-sheet" aria-label="Halaman uji cetak">
      <div style={{ fontWeight: 800, fontSize: "11pt", marginBottom: "4px" }}>
        HALAMAN UJI CETAK
      </div>
      <div style={{ marginBottom: "6px" }}>
        {fmtMm(preview.widthMm)} × {fmtMm(preview.heightMm)} mm · margin{" "}
        {fmtMm(preview.marginTopMm)}/{fmtMm(preview.marginRightMm)}/
        {fmtMm(preview.marginBottomMm)}/{fmtMm(preview.marginLeftMm)} mm
      </div>
      <table>
        <thead>
          <tr>
            <th style={{ width: "8%" }}>No</th>
            <th style={{ width: "52%" }}>Kolom teks</th>
            <th style={{ width: "20%" }}>Qty</th>
            <th style={{ width: "20%" }}>Nominal</th>
          </tr>
        </thead>
        <tbody>
          <tr><td>1</td><td>ABCDEFGHIJKLMNOPQRSTUVWXYZ 0123456789</td><td>12</td><td>1.250.000</td></tr>
          <tr><td>2</td><td>abcdefghijklmnopqrstuvw xyz — uji huruf kecil</td><td>3,5</td><td>175.000</td></tr>
          <tr><td>3</td><td>Batas kanan tabel ini = batas jangkauan print head</td><td>1</td><td>50.000</td></tr>
        </tbody>
      </table>
      <div style={{ marginTop: "6px" }}>
        ||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||
      </div>
      <div style={{ marginTop: "4px" }}>
        Tepi kanan garis di atas harus masih tercetak penuh — bila terpotong,
        naikkan margin kanan lalu cetak uji lagi.
      </div>
    </div>
  );
}
