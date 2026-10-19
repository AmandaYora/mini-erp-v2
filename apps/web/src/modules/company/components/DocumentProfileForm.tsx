import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { ChevronRight, Printer } from "lucide-react";
import {
  Button,
  Checkbox,
  FormField,
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemMedia,
  ItemTitle,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { FormSection } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { companyService } from "@/modules/company/services/company.service";
import {
  CONTINUOUS_PAPER_DEFAULTS,
  parseDocumentProfile,
  type ContinuousPaperProfile,
  type DocumentBankAccount,
  type DocumentPhone,
  type DocumentProfile,
} from "@/modules/company/document-profile";
import { countChanged } from "@/modules/company/lib/dirty";
import SettingsFooter from "@/modules/company/components/SettingsFooter";
import ReadOnlyHint from "@/modules/company/components/ReadOnlyHint";

function num(raw: string): number | undefined {
  const t = raw.trim().replace(",", ".");
  if (t === "") return undefined;
  const v = Number(t);
  return Number.isFinite(v) ? v : NaN;
}

interface Draft {
  headerName: string;
  addressLine: string;
  tagline: string;
  cityLine: string;
  phones: DocumentPhone[];
  banks: DocumentBankAccount[];
  sellingPoints: string;
  returnNote: string;
  thanksNote: string;
  paper: ContinuousPaperProfile;
}

function toDraft(profile: DocumentProfile): Draft {
  return {
    headerName: profile.headerName ?? "",
    addressLine: profile.addressLine ?? "",
    tagline: profile.tagline ?? "",
    cityLine: profile.cityLine ?? "",
    phones: profile.phones ?? [],
    banks: profile.bankAccounts ?? [],
    sellingPoints: (profile.sellingPoints ?? []).join("\n"),
    returnNote: profile.returnNote ?? "",
    thanksNote: profile.thanksNote ?? "",
    paper: { ...CONTINUOUS_PAPER_DEFAULTS, ...(profile.continuousNota ?? {}) },
  };
}

export default function DocumentProfileForm({
  raw,
  canManage,
  onSaved,
}: {
  raw: string | undefined;
  canManage: boolean;
  onSaved: () => void;
}) {
  const parsed = useMemo(() => parseDocumentProfile(raw), [raw]);

  const [baseline, setBaseline] = useState<Draft>(() => toDraft(parsed.profile));
  const [draft, setDraft] = useState<Draft>(() => toDraft(parsed.profile));
  const [corrupt, setCorrupt] = useState(() => parsed.corrupt);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Sinkron ulang saat identitas raw berubah (pengganti effect parse).
  const [prevRaw, setPrevRaw] = useState<string | undefined>(raw);
  if (raw !== prevRaw) {
    setPrevRaw(raw);
    setCorrupt(parsed.corrupt);
    const fresh = toDraft(parsed.profile);
    setBaseline(fresh);
    setDraft(fresh);
    setError(null);
  }

  const changed = countChanged(draft, baseline);
  const paper = draft.paper;

  function patch(p: Partial<Draft>) {
    setDraft((d) => ({ ...d, ...p }));
  }

  function setPaperNum(key: keyof ContinuousPaperProfile, value: string) {
    const v = num(value);
    setDraft(
      (d) => ({ ...d, paper: { ...d.paper, [key]: Number.isNaN(v) ? value : v } }) as Draft,
    );
  }

  async function handleSave() {
    // Lapis ke-3 mode baca-saja: simpan diabaikan.
    if (!canManage) return;
    const width = Number(paper.widthMm);
    const height = Number(paper.heightMm);
    if (!Number.isFinite(width) || width <= 0) {
      setError("Lebar kertas harus angka lebih dari 0.");
      return;
    }
    if (!Number.isFinite(height) || height <= 0) {
      setError("Tinggi kertas harus angka lebih dari 0.");
      return;
    }
    for (const [k, label] of [
      ["marginTopMm", "Margin atas"],
      ["marginRightMm", "Margin kanan"],
      ["marginBottomMm", "Margin bawah"],
      ["marginLeftMm", "Margin kiri"],
    ] as const) {
      const v = Number(paper[k]);
      if (!Number.isFinite(v) || v < 0) {
        setError(`${label} harus angka 0 atau lebih.`);
        return;
      }
    }
    const profile: DocumentProfile = {
      headerName: draft.headerName.trim(),
      addressLine: draft.addressLine.trim(),
      tagline: draft.tagline.trim(),
      cityLine: draft.cityLine.trim(),
      phones: draft.phones
        .filter((p) => p.label.trim() !== "" || p.number.trim() !== "")
        .map((p) => ({ label: p.label.trim(), number: p.number.trim() })),
      bankAccounts: draft.banks
        .filter((b) => b.bank.trim() !== "" || b.number.trim() !== "")
        .map((b) => ({ bank: b.bank.trim(), holder: b.holder.trim(), number: b.number.trim() })),
      sellingPoints: draft.sellingPoints.split("\n").map((s) => s.trim()).filter(Boolean),
      returnNote: draft.returnNote.trim(),
      thanksNote: draft.thanksNote.trim(),
      continuousNota: { ...paper, widthMm: width, heightMm: height } as ContinuousPaperProfile,
    };
    setError(null);
    setSaving(true);
    try {
      const res = await companyService.saveSettings({ document_profile: JSON.stringify(profile) });
      toast.fromServer(res.message, "Profil dokumen disimpan");
      // Muat ulang dari nilai yang baru dinormalisasi agar footer hilang walau JSON tersimpan identik.
      const saved = toDraft(profile);
      setBaseline(saved);
      setDraft(saved);
      onSaved();
    } catch (err) {
      const msg = toApiError(err).message;
      setError(msg);
      toast.danger("Gagal menyimpan profil dokumen", msg);
    } finally {
      setSaving(false);
    }
  }

  const locked = !canManage || saving;

  return (
    <div className="py-6 first:pt-0">
      <div id="dokumen">
        <FormSection
          className="scroll-mt-20"
          title="Kop dan teks dokumen"
          description="Kop, rekening, dan kalibrasi kertas untuk halaman cetak. Disimpan sebagai satu kunci JSON document_profile."
        >
          <ReadOnlyHint show={!canManage} />
          {corrupt && (
            <Notice tone="warning" title="Profil tersimpan rusak" className="mb-4">
              Nilai document_profile bukan JSON valid — form menampilkan default. Simpan untuk menimpanya.
            </Notice>
          )}
          {error && <Notice tone="danger" title={error} className="mb-4" />}
          <fieldset disabled={locked} className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <FormField label="Nama header" htmlFor="doc-header-name">
                <TextInput
                  id="doc-header-name"
                  value={draft.headerName}
                  onChange={(e) => patch({ headerName: e.target.value })}
                  placeholder="cth: TOKO MAJU JAYA"
                />
              </FormField>
              <FormField label="Baris kota (tanda tangan)" htmlFor="doc-city-line">
                <TextInput
                  id="doc-city-line"
                  value={draft.cityLine}
                  onChange={(e) => patch({ cityLine: e.target.value })}
                  placeholder="cth: MALANG"
                />
              </FormField>
            </div>
            <FormField label="Baris alamat" htmlFor="doc-address-line">
              <TextInput
                id="doc-address-line"
                value={draft.addressLine}
                onChange={(e) => patch({ addressLine: e.target.value })}
              />
            </FormField>
            <div className="grid gap-4 md:grid-cols-2">
              <FormField label="Tagline struk" htmlFor="doc-tagline">
                <TextInput
                  id="doc-tagline"
                  value={draft.tagline}
                  onChange={(e) => patch({ tagline: e.target.value })}
                  placeholder="cth: PEREMPATAN KAMOLAN"
                />
              </FormField>
              <FormField label="Ucapan terima kasih" htmlFor="doc-thanks">
                <TextInput
                  id="doc-thanks"
                  value={draft.thanksNote}
                  onChange={(e) => patch({ thanksNote: e.target.value })}
                  placeholder="cth: TERIMA KASIH"
                />
              </FormField>
            </div>
            <FormField label="Catatan retur" htmlFor="doc-return-note">
              <TextArea
                id="doc-return-note"
                rows={2}
                value={draft.returnNote}
                onChange={(e) => patch({ returnNote: e.target.value })}
              />
            </FormField>
            <FormField label="Titik jual (satu per baris)" htmlFor="doc-selling">
              <TextArea
                id="doc-selling"
                rows={2}
                value={draft.sellingPoints}
                onChange={(e) => patch({ sellingPoints: e.target.value })}
              />
            </FormField>
          </fieldset>
        </FormSection>

        <FormSection title="Kontak dan rekening" description="Nomor telepon berlabel dan rekening bank yang tampil di dokumen.">
          <fieldset disabled={locked} className="space-y-6">
            <div>
              <p className="text-sm font-semibold text-foreground">Telepon berlabel</p>
              <div className="mt-2 space-y-2">
                {draft.phones.map((p, i) => (
                  <div key={i} className="flex gap-2">
                    <TextInput
                      aria-label={`Label telepon ${i + 1}`}
                      value={p.label}
                      onChange={(e) =>
                        patch({ phones: draft.phones.map((r, j) => (j === i ? { ...r, label: e.target.value } : r)) })
                      }
                      placeholder="Label (TOKO)"
                    />
                    <TextInput
                      aria-label={`Nomor telepon ${i + 1}`}
                      value={p.number}
                      onChange={(e) =>
                        patch({ phones: draft.phones.map((r, j) => (j === i ? { ...r, number: e.target.value } : r)) })
                      }
                      placeholder="Nomor"
                    />
                    <Button
                      variant="secondary"
                      size="sm"
                      aria-label={`Hapus telepon ${i + 1}`}
                      onClick={() => patch({ phones: draft.phones.filter((_, j) => j !== i) })}
                    >
                      Hapus
                    </Button>
                  </div>
                ))}
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => patch({ phones: [...draft.phones, { label: "", number: "" }] })}
                >
                  Tambah telepon
                </Button>
              </div>
            </div>

            <div>
              <p className="text-sm font-semibold text-foreground">Rekening bank</p>
              <div className="mt-2 space-y-2">
                {draft.banks.map((b, i) => (
                  <div key={i} className="grid grid-cols-1 gap-2 sm:grid-cols-4">
                    <TextInput
                      aria-label={`Bank ${i + 1}`}
                      value={b.bank}
                      onChange={(e) =>
                        patch({ banks: draft.banks.map((r, j) => (j === i ? { ...r, bank: e.target.value } : r)) })
                      }
                      placeholder="Bank"
                    />
                    <TextInput
                      aria-label={`Pemilik rekening ${i + 1}`}
                      value={b.holder}
                      onChange={(e) =>
                        patch({ banks: draft.banks.map((r, j) => (j === i ? { ...r, holder: e.target.value } : r)) })
                      }
                      placeholder="Pemilik"
                    />
                    <TextInput
                      aria-label={`Nomor rekening ${i + 1}`}
                      value={b.number}
                      onChange={(e) =>
                        patch({ banks: draft.banks.map((r, j) => (j === i ? { ...r, number: e.target.value } : r)) })
                      }
                      placeholder="Nomor"
                    />
                    <Button
                      variant="secondary"
                      size="sm"
                      aria-label={`Hapus rekening ${i + 1}`}
                      onClick={() => patch({ banks: draft.banks.filter((_, j) => j !== i) })}
                    >
                      Hapus
                    </Button>
                  </div>
                ))}
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => patch({ banks: [...draft.banks, { bank: "", holder: "", number: "" }] })}
                >
                  Tambah rekening
                </Button>
              </div>
            </div>
          </fieldset>
        </FormSection>

        <FormSection
          title="Kalibrasi kertas kontinu"
          description="Ukuran dalam mm. Harus identik dengan ukuran kertas custom di driver — margin kanan 37,91 mengkompensasi jangkauan print head, bukan margin visual."
        >
          <fieldset disabled={locked} className="space-y-4">
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              <FormField label="Lebar" htmlFor="doc-paper-width">
                <TextInput
                  id="doc-paper-width"
                  inputMode="decimal"
                  value={String(paper.widthMm ?? "")}
                  onChange={(e) => setPaperNum("widthMm", e.target.value)}
                />
              </FormField>
              <FormField label="Tinggi" htmlFor="doc-paper-height">
                <TextInput
                  id="doc-paper-height"
                  inputMode="decimal"
                  value={String(paper.heightMm ?? "")}
                  onChange={(e) => setPaperNum("heightMm", e.target.value)}
                />
              </FormField>
              <span className="hidden sm:block" />
              <FormField label="Margin atas" htmlFor="doc-paper-mt">
                <TextInput
                  id="doc-paper-mt"
                  inputMode="decimal"
                  value={String(paper.marginTopMm ?? "")}
                  onChange={(e) => setPaperNum("marginTopMm", e.target.value)}
                />
              </FormField>
              <FormField label="Margin kanan" htmlFor="doc-paper-mr">
                <TextInput
                  id="doc-paper-mr"
                  inputMode="decimal"
                  value={String(paper.marginRightMm ?? "")}
                  onChange={(e) => setPaperNum("marginRightMm", e.target.value)}
                />
              </FormField>
              <FormField label="Margin bawah" htmlFor="doc-paper-mb">
                <TextInput
                  id="doc-paper-mb"
                  inputMode="decimal"
                  value={String(paper.marginBottomMm ?? "")}
                  onChange={(e) => setPaperNum("marginBottomMm", e.target.value)}
                />
              </FormField>
              <FormField label="Margin kiri" htmlFor="doc-paper-ml">
                <TextInput
                  id="doc-paper-ml"
                  inputMode="decimal"
                  value={String(paper.marginLeftMm ?? "")}
                  onChange={(e) => setPaperNum("marginLeftMm", e.target.value)}
                />
              </FormField>
            </div>
            <label htmlFor="doc-show-letterhead" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
              <Checkbox
                id="doc-show-letterhead"
                checked={paper.showLetterhead ?? true}
                onCheckedChange={(v) => patch({ paper: { ...paper, showLetterhead: v === true } })}
              />
              Tampilkan kop (matikan untuk form preprinted)
            </label>
            <label htmlFor="doc-paper-default" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
              <Checkbox
                id="doc-paper-default"
                checked={paper.isDefault ?? false}
                onCheckedChange={(v) => patch({ paper: { ...paper, isDefault: v === true } })}
              />
              Jadikan mode cetak nota bawaan
            </label>
          </fieldset>

          <Item variant="outline" asChild className="mt-6">
            <Link to={ROUTE_PATHS.printCalibration}>
              <ItemMedia variant="icon">
                <Printer aria-hidden />
              </ItemMedia>
              <ItemContent>
                <ItemTitle>Kalibrasi kertas</ItemTitle>
                <ItemDescription>Cetak halaman uji dan sesuaikan margin untuk kertas kontinu.</ItemDescription>
              </ItemContent>
              <ItemActions>
                <ChevronRight className="size-4 text-muted-foreground" aria-hidden />
              </ItemActions>
            </Link>
          </Item>
        </FormSection>
      </div>
      {canManage && (
        <SettingsFooter
          count={changed}
          saving={saving}
          onReset={() => {
            setDraft(baseline);
            setError(null);
          }}
          onSave={() => void handleSave()}
          saveLabel="Simpan profil dokumen"
        />
      )}
    </div>
  );
}
