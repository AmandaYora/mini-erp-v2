import { useMemo, useState } from "react";
import { Button, FormField, SectionCard, TextArea, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
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

function num(raw: string): number | undefined {
  const t = raw.trim().replace(",", ".");
  if (t === "") return undefined;
  const v = Number(t);
  return Number.isFinite(v) ? v : NaN;
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

  const [headerName, setHeaderName] = useState(() => parsed.profile.headerName ?? "");
  const [addressLine, setAddressLine] = useState(() => parsed.profile.addressLine ?? "");
  const [tagline, setTagline] = useState(() => parsed.profile.tagline ?? "");
  const [cityLine, setCityLine] = useState(() => parsed.profile.cityLine ?? "");
  const [phones, setPhones] = useState<DocumentPhone[]>(() => parsed.profile.phones ?? []);
  const [banks, setBanks] = useState<DocumentBankAccount[]>(() => parsed.profile.bankAccounts ?? []);
  const [sellingPoints, setSellingPoints] = useState(() =>
    (parsed.profile.sellingPoints ?? []).join("\n"),
  );
  const [returnNote, setReturnNote] = useState(() => parsed.profile.returnNote ?? "");
  const [thanksNote, setThanksNote] = useState(() => parsed.profile.thanksNote ?? "");
  const [paper, setPaper] = useState<ContinuousPaperProfile>(() => ({
    ...CONTINUOUS_PAPER_DEFAULTS,
    ...(parsed.profile.continuousNota ?? {}),
  }));
  const [corrupt, setCorrupt] = useState(() => parsed.corrupt);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Sinkron ulang saat identitas raw berubah (pengganti effect parse).
  const [prevRaw, setPrevRaw] = useState<string | undefined>(raw);
  if (raw !== prevRaw) {
    setPrevRaw(raw);
    setCorrupt(parsed.corrupt);
    setHeaderName(parsed.profile.headerName ?? "");
    setAddressLine(parsed.profile.addressLine ?? "");
    setTagline(parsed.profile.tagline ?? "");
    setCityLine(parsed.profile.cityLine ?? "");
    setPhones(parsed.profile.phones ?? []);
    setBanks(parsed.profile.bankAccounts ?? []);
    setSellingPoints((parsed.profile.sellingPoints ?? []).join("\n"));
    setReturnNote(parsed.profile.returnNote ?? "");
    setThanksNote(parsed.profile.thanksNote ?? "");
    setPaper({ ...CONTINUOUS_PAPER_DEFAULTS, ...(parsed.profile.continuousNota ?? {}) });
    setError(null);
  }

  function setPaperNum(key: keyof ContinuousPaperProfile, value: string) {
    const v = num(value);
    setPaper((p) => ({ ...p, [key]: Number.isNaN(v) ? value : v }) as ContinuousPaperProfile);
  }

  async function handleSave() {
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
      headerName: headerName.trim(),
      addressLine: addressLine.trim(),
      tagline: tagline.trim(),
      cityLine: cityLine.trim(),
      phones: phones
        .filter((p) => p.label.trim() !== "" || p.number.trim() !== "")
        .map((p) => ({ label: p.label.trim(), number: p.number.trim() })),
      bankAccounts: banks
        .filter((b) => b.bank.trim() !== "" || b.number.trim() !== "")
        .map((b) => ({ bank: b.bank.trim(), holder: b.holder.trim(), number: b.number.trim() })),
      sellingPoints: sellingPoints.split("\n").map((s) => s.trim()).filter(Boolean),
      returnNote: returnNote.trim(),
      thanksNote: thanksNote.trim(),
      continuousNota: { ...paper, widthMm: width, heightMm: height } as ContinuousPaperProfile,
    };
    setError(null);
    setSaving(true);
    try {
      const res = await companyService.saveSettings({ document_profile: JSON.stringify(profile) });
      toast.fromServer(res.message, "Profil dokumen disimpan");
      onSaved();
    } catch (err) {
      const msg = toApiError(err).message;
      setError(msg);
      toast.danger("Gagal menyimpan profil dokumen", msg);
    } finally {
      setSaving(false);
    }
  }

  return (
    <SectionCard
      title="Profil Dokumen"
      description="Kop, rekening, dan kalibrasi kertas untuk halaman cetak. Satu kunci JSON document_profile."
      actions={
        canManage ? (
          <Button size="sm" onClick={() => void handleSave()} disabled={saving}>
            {saving ? "Menyimpan…" : "Simpan Profil Dokumen"}
          </Button>
        ) : undefined
      }
    >
      {corrupt && (
        <div className="mb-4">
          <Notice tone="warning" title="Profil tersimpan rusak">
            Nilai document_profile bukan JSON valid — form menampilkan default. Simpan untuk menimpanya.
          </Notice>
        </div>
      )}
      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error} />
        </div>
      )}
      <fieldset disabled={!canManage || saving}>
        <div className="grid gap-4 md:grid-cols-2">
          <FormField label="Nama header">
            <TextInput value={headerName} onChange={(e) => setHeaderName(e.target.value)} placeholder="cth: TOKO MAJU JAYA" />
          </FormField>
          <FormField label="Baris kota (tanda tangan)">
            <TextInput value={cityLine} onChange={(e) => setCityLine(e.target.value)} placeholder="cth: MALANG" />
          </FormField>
        </div>
        <div className="mt-4">
          <FormField label="Baris alamat">
            <TextInput value={addressLine} onChange={(e) => setAddressLine(e.target.value)} />
          </FormField>
        </div>
        <div className="mt-4 grid gap-4 md:grid-cols-2">
          <FormField label="Tagline struk">
            <TextInput value={tagline} onChange={(e) => setTagline(e.target.value)} placeholder="cth: PEREMPATAN KAMOLAN" />
          </FormField>
          <FormField label="Ucapan terima kasih">
            <TextInput value={thanksNote} onChange={(e) => setThanksNote(e.target.value)} placeholder="cth: TERIMA KASIH" />
          </FormField>
        </div>
        <div className="mt-4">
          <FormField label="Catatan retur">
            <TextArea rows={2} value={returnNote} onChange={(e) => setReturnNote(e.target.value)} />
          </FormField>
        </div>
        <div className="mt-4">
          <FormField label="Titik jual (satu per baris)">
            <TextArea rows={2} value={sellingPoints} onChange={(e) => setSellingPoints(e.target.value)} />
          </FormField>
        </div>

        <div className="mt-6 border-t border-hairline pt-4">
          <p className="text-sm font-semibold text-ink">Telepon berlabel</p>
          <div className="mt-2 space-y-2">
            {phones.map((p, i) => (
              <div key={i} className="flex gap-2">
                <TextInput value={p.label} onChange={(e) => setPhones((rows) => rows.map((r, j) => (j === i ? { ...r, label: e.target.value } : r)))} placeholder="Label (TOKO)" />
                <TextInput value={p.number} onChange={(e) => setPhones((rows) => rows.map((r, j) => (j === i ? { ...r, number: e.target.value } : r)))} placeholder="Nomor" />
                <Button variant="secondary" size="sm" onClick={() => setPhones((rows) => rows.filter((_, j) => j !== i))}>
                  Hapus
                </Button>
              </div>
            ))}
            <Button variant="secondary" size="sm" onClick={() => setPhones((rows) => [...rows, { label: "", number: "" }])}>
              Tambah telepon
            </Button>
          </div>
        </div>

        <div className="mt-6 border-t border-hairline pt-4">
          <p className="text-sm font-semibold text-ink">Rekening bank</p>
          <div className="mt-2 space-y-2">
            {banks.map((b, i) => (
              <div key={i} className="grid grid-cols-1 gap-2 sm:grid-cols-4">
                <TextInput value={b.bank} onChange={(e) => setBanks((rows) => rows.map((r, j) => (j === i ? { ...r, bank: e.target.value } : r)))} placeholder="Bank" />
                <TextInput value={b.holder} onChange={(e) => setBanks((rows) => rows.map((r, j) => (j === i ? { ...r, holder: e.target.value } : r)))} placeholder="Pemilik" />
                <TextInput value={b.number} onChange={(e) => setBanks((rows) => rows.map((r, j) => (j === i ? { ...r, number: e.target.value } : r)))} placeholder="Nomor" />
                <Button variant="secondary" size="sm" onClick={() => setBanks((rows) => rows.filter((_, j) => j !== i))}>
                  Hapus
                </Button>
              </div>
            ))}
            <Button variant="secondary" size="sm" onClick={() => setBanks((rows) => [...rows, { bank: "", holder: "", number: "" }])}>
              Tambah rekening
            </Button>
          </div>
        </div>

        <div className="mt-6 border-t border-hairline pt-4">
          <p className="text-sm font-semibold text-ink">Kalibrasi kertas kontinu (mm)</p>
          <p className="text-muted mt-1 text-[0.82rem]">
            Ukuran harus identik dengan ukuran kertas custom di driver — margin kanan 37,91 mengkompensasi
            jangkauan print head, bukan margin visual.
          </p>
          <div className="mt-3 grid grid-cols-2 gap-4 sm:grid-cols-4">
            <FormField label="Lebar">
              <TextInput inputMode="decimal" value={String(paper.widthMm ?? "")} onChange={(e) => setPaperNum("widthMm", e.target.value)} />
            </FormField>
            <FormField label="Tinggi">
              <TextInput inputMode="decimal" value={String(paper.heightMm ?? "")} onChange={(e) => setPaperNum("heightMm", e.target.value)} />
            </FormField>
            <FormField label="Margin atas">
              <TextInput inputMode="decimal" value={String(paper.marginTopMm ?? "")} onChange={(e) => setPaperNum("marginTopMm", e.target.value)} />
            </FormField>
            <FormField label="Margin kanan">
              <TextInput inputMode="decimal" value={String(paper.marginRightMm ?? "")} onChange={(e) => setPaperNum("marginRightMm", e.target.value)} />
            </FormField>
            <FormField label="Margin bawah">
              <TextInput inputMode="decimal" value={String(paper.marginBottomMm ?? "")} onChange={(e) => setPaperNum("marginBottomMm", e.target.value)} />
            </FormField>
            <FormField label="Margin kiri">
              <TextInput inputMode="decimal" value={String(paper.marginLeftMm ?? "")} onChange={(e) => setPaperNum("marginLeftMm", e.target.value)} />
            </FormField>
          </div>
          <div className="mt-3 flex items-center gap-2">
            <input
              id="doc-show-letterhead"
              type="checkbox"
              checked={paper.showLetterhead ?? true}
              onChange={(e) => setPaper((p) => ({ ...p, showLetterhead: e.target.checked }))}
            />
            <label htmlFor="doc-show-letterhead" className="text-sm text-ink">
              Tampilkan kop (matikan untuk form preprinted)
            </label>
          </div>
          <div className="mt-3 flex items-center gap-2">
            <input
              id="doc-paper-default"
              type="checkbox"
              checked={paper.isDefault ?? false}
              onChange={(e) => setPaper((p) => ({ ...p, isDefault: e.target.checked }))}
            />
            <label htmlFor="doc-paper-default" className="text-sm text-ink">
              Jadikan mode cetak nota bawaan
            </label>
          </div>
        </div>
      </fieldset>
    </SectionCard>
  );
}
