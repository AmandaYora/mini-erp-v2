import { useState } from "react";
import type { FormEvent } from "react";
import { FormField, TextArea, TextInput } from "@/shared/components/ui";
import { FormSection } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { companyService } from "@/modules/company/services/company.service";
import { companyProfileSchema } from "@/modules/company/schemas/company.schema";
import { countChanged } from "@/modules/company/lib/dirty";
import SettingsFooter from "@/modules/company/components/SettingsFooter";
import ReadOnlyHint from "@/modules/company/components/ReadOnlyHint";
import type { CompanyProfile } from "@/modules/company/types";

interface ProfileValues {
  name: string;
  legalName: string;
  address: string;
  city: string;
  phone: string;
  email: string;
  taxId: string;
}

function toValues(p: CompanyProfile | null): ProfileValues {
  return {
    name: p?.name ?? "",
    legalName: p?.legalName ?? "",
    address: p?.address ?? "",
    city: p?.city ?? "",
    phone: p?.phone ?? "",
    email: p?.email ?? "",
    taxId: p?.taxId ?? "",
  };
}

/** Seksi "Profil": identitas perusahaan yang dicetak pada dokumen. */
export default function CompanyProfileForm({
  initial,
  canManage,
}: {
  initial: CompanyProfile | null;
  canManage: boolean;
}) {
  const [empty, setEmpty] = useState(initial === null);
  const [baseline, setBaseline] = useState<ProfileValues>(() => toValues(initial));
  const [values, setValues] = useState<ProfileValues>(() => toValues(initial));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  function set<K extends keyof ProfileValues>(key: K, value: string) {
    setValues((v) => ({ ...v, [key]: value }));
  }

  const changed = countChanged(values, baseline);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    // Lapis ke-3 mode baca-saja: submit diabaikan walau terpicu (mis. Enter).
    if (!canManage) return;
    const parsed = companyProfileSchema.safeParse(values);
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        errs[String(issue.path[0] ?? "")] ??= issue.message;
      }
      setErrors(errs);
      return;
    }
    setErrors({});
    setSaving(true);
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
      setEmpty(false);
      const next = toValues(saved.data);
      setValues(next);
      setBaseline(next);
      toast.fromServer(saved.message, "Profil perusahaan disimpan");
    } catch (err) {
      const apiErr = toApiError(err);
      const next: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) {
        if (fe.field) next[fe.field] ??= fe.message;
      }
      setErrors(next);
      toast.danger("Gagal menyimpan profil", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={(e) => void handleSubmit(e)} aria-label="Profil perusahaan" className="py-6 first:pt-0">
      <div>
      <FormSection
        id="profil"
        className="scroll-mt-20"
        title="Profil perusahaan"
        description="Identitas yang dicetak pada dokumen."
      >
        <ReadOnlyHint show={!canManage} />
        {empty && (
          <Notice tone="info" title="Profil belum diisi" className="mb-4">
            Lengkapi data di bawah lalu simpan untuk membuat profil perusahaan.
          </Notice>
        )}
        <fieldset disabled={!canManage || saving} className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <FormField label="Nama" htmlFor="company-name" required errorText={errors.name}>
              <TextInput
                id="company-name"
                value={values.name}
                onChange={(e) => set("name", e.target.value)}
                placeholder="cth: Toko Maju Jaya"
              />
            </FormField>
            <FormField label="Nama legal" htmlFor="company-legal" errorText={errors.legalName}>
              <TextInput
                id="company-legal"
                value={values.legalName}
                onChange={(e) => set("legalName", e.target.value)}
                placeholder="cth: PT Maju Jaya Abadi"
              />
            </FormField>
            <FormField label="Kota" htmlFor="company-city" errorText={errors.city}>
              <TextInput id="company-city" value={values.city} onChange={(e) => set("city", e.target.value)} />
            </FormField>
            <FormField label="Telepon" htmlFor="company-phone" errorText={errors.phone}>
              <TextInput id="company-phone" value={values.phone} onChange={(e) => set("phone", e.target.value)} />
            </FormField>
            <FormField label="Email" htmlFor="company-email" errorText={errors.email}>
              <TextInput
                id="company-email"
                type="email"
                value={values.email}
                onChange={(e) => set("email", e.target.value)}
              />
            </FormField>
            <FormField label="NPWP" htmlFor="company-taxid" errorText={errors.taxId}>
              <TextInput
                id="company-taxid"
                value={values.taxId}
                onChange={(e) => set("taxId", e.target.value)}
                placeholder="Nomor Pokok Wajib Pajak"
              />
            </FormField>
          </div>
          <FormField label="Alamat" htmlFor="company-address" errorText={errors.address}>
            <TextArea
              id="company-address"
              rows={2}
              value={values.address}
              onChange={(e) => set("address", e.target.value)}
            />
          </FormField>
        </fieldset>
      </FormSection>
      </div>
      {canManage && (
        <SettingsFooter
          submit
          count={changed}
          saving={saving}
          onReset={() => setValues(baseline)}
          saveLabel="Simpan profil"
        />
      )}
    </form>
  );
}
