import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  FormField,
  PageHeader,
  SectionCard,
  TextArea,
  TextInput,
  buttonClass,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { companyService } from "@/modules/company/services/company.service";
import { companyProfileSchema } from "@/modules/company/schemas/company.schema";
import DocumentProfileForm from "@/modules/company/components/DocumentProfileForm";
import type { CompanySettings } from "@/modules/company/types";

interface SettingRow {
  localId: number;
  key: string;
  value: string;
}

let nextRowId = 1;

function toRows(settings: CompanySettings): SettingRow[] {
  return Object.entries(settings)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => ({ localId: nextRowId++, key, value }));
}

export default function CompanyPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("company.manage");

  const [profileEmpty, setProfileEmpty] = useState(false);

  const [name, setName] = useState("");
  const [legalName, setLegalName] = useState("");
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [taxId, setTaxId] = useState("");
  const [profileErrors, setProfileErrors] = useState<Record<string, string>>({});
  const [savingProfile, setSavingProfile] = useState(false);

  const [rows, setRows] = useState<SettingRow[]>([]);
  const [settingsError, setSettingsError] = useState<string | null>(null);
  const [savingSettings, setSavingSettings] = useState(false);

  async function reloadSettings() {
    try {
      const settings = await companyService.getSettings();
      setRows(toRows(settings ?? {}));
    } catch (err) {
      toast.danger("Gagal memuat pengaturan", toApiError(err).message);
    }
  }

  const { data, loading } = useAsyncData(
    async () => {
      try {
        const [profile, settings] = await Promise.all([
          companyService.getProfile(),
          companyService.getSettings(),
        ]);
        return { profile, settings };
      } catch (err) {
        toast.danger("Gagal memuat data perusahaan", toApiError(err).message);
        throw err;
      }
    },
    [],
  );

  // Salin hasil fetch ke state form sekali saat tiba.
  const [applied, setApplied] = useState(false);
  if (data && !applied) {
    setApplied(true);
    if (data.profile) {
      setProfileEmpty(false);
      setName(data.profile.name ?? "");
      setLegalName(data.profile.legalName ?? "");
      setAddress(data.profile.address ?? "");
      setCity(data.profile.city ?? "");
      setPhone(data.profile.phone ?? "");
      setEmail(data.profile.email ?? "");
      setTaxId(data.profile.taxId ?? "");
    } else {
      setProfileEmpty(true);
    }
    setRows(toRows(data.settings));
  }

  async function handleProfileSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = companyProfileSchema.safeParse({
      name,
      legalName,
      address,
      city,
      phone,
      email,
      taxId,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        errs[String(issue.path[0] ?? "")] ??= issue.message;
      }
      setProfileErrors(errs);
      return;
    }
    setProfileErrors({});
    setSavingProfile(true);
    try {
      const saved = await companyService.saveProfile({
        name: parsed.data.name,
        legalName: parsed.data.legalName ?? "",
        address: parsed.data.address ?? "",
        city: parsed.data.city ?? "",
        phone: parsed.data.phone ?? "",
        email: parsed.data.email ?? "",
        taxId: parsed.data.taxId ?? "",
      });
      setProfileEmpty(false);
      const profile = saved.data;
      setName(profile.name ?? "");
      setLegalName(profile.legalName ?? "");
      setAddress(profile.address ?? "");
      setCity(profile.city ?? "");
      setPhone(profile.phone ?? "");
      setEmail(profile.email ?? "");
      setTaxId(profile.taxId ?? "");
      toast.fromServer(saved.message, "Profil perusahaan disimpan");
    } catch (err) {
      const apiErr = toApiError(err);
      const next: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) {
        if (fe.field) next[fe.field] ??= fe.message;
      }
      setProfileErrors(next);
      toast.danger("Gagal menyimpan profil", apiErr.message);
    } finally {
      setSavingProfile(false);
    }
  }

  function updateRow(localId: number, patch: Partial<SettingRow>) {
    setRows((prev) =>
      prev.map((r) => (r.localId === localId ? { ...r, ...patch } : r)),
    );
  }

  function removeRow(localId: number) {
    setRows((prev) => prev.filter((r) => r.localId !== localId));
  }

  function addRow() {
    setRows((prev) => [...prev, { localId: nextRowId++, key: "", value: "" }]);
  }

  async function handleSettingsSave() {
    const payload: CompanySettings = {};
    for (const row of rows) {
      const key = row.key.trim();
      if (key === "") {
        setSettingsError("Semua baris pengaturan wajib memiliki kunci.");
        return;
      }
      if (key in payload) {
        setSettingsError(`Kunci ganda: "${key}". Gabungkan menjadi satu baris.`);
        return;
      }
      payload[key] = row.value;
    }
    if (Object.keys(payload).length === 0) {
      setSettingsError("Tidak ada pengaturan untuk disimpan.");
      return;
    }
    setSettingsError(null);
    setSavingSettings(true);
    try {
      // Server melakukan merge (tidak menghapus kunci lain).
      const saved = await companyService.saveSettings(payload);
      setRows(toRows(saved.data ?? {}));
      toast.fromServer(saved.message, "Pengaturan perusahaan disimpan");
    } catch (err) {
      const apiErr = toApiError(err);
      setSettingsError(apiErr.message);
      toast.danger("Gagal menyimpan pengaturan", apiErr.message);
    } finally {
      setSavingSettings(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Pengaturan"
        title="Perusahaan"
        description="Profil identitas dan pengaturan operasional perusahaan."
        actions={
          <Link to="/print/calibration" className={buttonClass("secondary", "md")}>
            Kalibrasi Kertas
          </Link>
        }
      />

      {loading ? (
        <p className="text-muted py-6 text-sm">Memuat data perusahaan…</p>
      ) : (
        <div className="space-y-6">
          <SectionCard
            title="Profil Perusahaan"
            description="Identitas yang dicetak pada dokumen."
          >
            {profileEmpty && (
              <div className="mb-4">
                <Notice tone="info" title="Profil belum diisi">
                  Lengkapi data di bawah lalu simpan untuk membuat profil
                  perusahaan.
                </Notice>
              </div>
            )}
            <form
              id="company-profile-form"
              onSubmit={(e) => void handleProfileSubmit(e)}
            >
              <fieldset disabled={!canManage || savingProfile}>
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField
                    label="Nama"
                    required
                    errorText={profileErrors.name}
                  >
                    <TextInput
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="cth: Toko Maju Jaya"
                    />
                  </FormField>
                  <FormField
                    label="Nama Legal"
                    errorText={profileErrors.legalName}
                  >
                    <TextInput
                      value={legalName}
                      onChange={(e) => setLegalName(e.target.value)}
                      placeholder="cth: PT Maju Jaya Abadi"
                    />
                  </FormField>
                  <FormField label="Kota" errorText={profileErrors.city}>
                    <TextInput
                      value={city}
                      onChange={(e) => setCity(e.target.value)}
                    />
                  </FormField>
                  <FormField label="Telepon" errorText={profileErrors.phone}>
                    <TextInput
                      value={phone}
                      onChange={(e) => setPhone(e.target.value)}
                    />
                  </FormField>
                  <FormField label="Email" errorText={profileErrors.email}>
                    <TextInput
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </FormField>
                  <FormField label="NPWP" errorText={profileErrors.taxId}>
                    <TextInput
                      value={taxId}
                      onChange={(e) => setTaxId(e.target.value)}
                      placeholder="Nomor Pokok Wajib Pajak"
                    />
                  </FormField>
                </div>
                <div className="mt-4">
                  <FormField label="Alamat" errorText={profileErrors.address}>
                    <TextArea
                      rows={2}
                      value={address}
                      onChange={(e) => setAddress(e.target.value)}
                    />
                  </FormField>
                </div>
              </fieldset>
              {canManage && (
                <div className="mt-6 flex justify-end border-t border-hairline pt-5">
                  <Button
                    type="submit"
                    form="company-profile-form"
                    disabled={savingProfile}
                  >
                    {savingProfile ? "Menyimpan…" : "Simpan Profil"}
                  </Button>
                </div>
              )}
            </form>
          </SectionCard>

          <DocumentProfileForm
            raw={rows.find((r) => r.key === "document_profile")?.value}
            canManage={canManage}
            onSaved={() => void reloadSettings()}
          />

          <SectionCard
            title="Pengaturan"
            description="Pasangan kunci-nilai operasional. Penyimpanan menggabungkan (merge) dengan yang sudah ada."
            actions={
              canManage ? (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={addRow}
                  disabled={savingSettings}
                >
                  Tambah Baris
                </Button>
              ) : undefined
            }
          >
            {settingsError && (
              <div className="mb-4">
                <Notice tone="danger" title="Pengaturan belum tersimpan">
                  {settingsError}
                </Notice>
              </div>
            )}
            {rows.length === 0 ? (
              <p className="text-muted text-sm">
                Belum ada pengaturan. Tambah baris baru untuk mulai.
              </p>
            ) : (
              <div className="space-y-3">
                {rows.map((row) => (
                  <div
                    key={row.localId}
                    className="grid items-end gap-3 md:grid-cols-[1fr_1fr_auto]"
                  >
                    <FormField label="Kunci">
                      <TextInput
                        value={row.key}
                        onChange={(e) =>
                          updateRow(row.localId, { key: e.target.value })
                        }
                        placeholder="cth: pos.receipt_footer"
                        disabled={!canManage || savingSettings}
                      />
                    </FormField>
                    <FormField label="Nilai">
                      <TextInput
                        value={row.value}
                        onChange={(e) =>
                          updateRow(row.localId, { value: e.target.value })
                        }
                        placeholder="Nilai pengaturan"
                        disabled={!canManage || savingSettings}
                      />
                    </FormField>
                    {canManage && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => removeRow(row.localId)}
                        disabled={savingSettings}
                      >
                        Hapus
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            )}
            {canManage && rows.length > 0 && (
              <div className="mt-6 flex justify-end border-t border-hairline pt-5">
                <Button
                  onClick={() => void handleSettingsSave()}
                  disabled={savingSettings}
                >
                  {savingSettings ? "Menyimpan…" : "Simpan Pengaturan"}
                </Button>
              </div>
            )}
          </SectionCard>
        </div>
      )}
    </div>
  );
}
