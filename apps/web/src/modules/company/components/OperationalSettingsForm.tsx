import { useState } from "react";
import { Checkbox, FormField, SelectInput, TextInput } from "@/shared/components/ui";
import { FormSection } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { companyService } from "@/modules/company/services/company.service";
import {
  editableDefinitions,
  initialSettingValues,
  parseOperationalSettings,
  serverSettingErrors,
} from "@/modules/company/schemas/settings.schema";
import { countChanged } from "@/modules/company/lib/dirty";
import SettingsFooter from "@/modules/company/components/SettingsFooter";
import ReadOnlyHint from "@/modules/company/components/ReadOnlyHint";
import type { CompanySettings, SettingDefinition } from "@/modules/company/types";

const THRESHOLD_KEY = "stock_adjust_approval_threshold";
const ALWAYS_APPROVE_KEY = "stock_adjust_always_approve";

/**
 * Form pengaturan operasional terdaftar (F3b-7): kontrol bertipe per kunci
 * dari GET /company/settings/definitions, galat server tampil per kolom.
 * Induk memberi `key` baru agar form dimuat ulang setelah simpan.
 */
export default function OperationalSettingsForm({
  definitions,
  settings,
  canManage,
  onSaved,
}: {
  definitions: SettingDefinition[];
  settings: CompanySettings;
  canManage: boolean;
  onSaved: (settings: CompanySettings) => void;
}) {
  const defs = editableDefinitions(definitions);
  const [baseline] = useState(() => initialSettingValues(defs, settings));
  const [values, setValues] = useState(baseline);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  if (defs.length === 0) return null;

  function setValue(key: string, value: string) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    // Lapis ke-3 mode baca-saja: simpan diabaikan.
    if (!canManage) return;
    const parsed = parseOperationalSettings(defs, values);
    if (!parsed.ok) {
      setErrors(parsed.errors);
      setFormError("Periksa kembali isian yang ditandai.");
      return;
    }
    setErrors({});
    setFormError(null);
    setSaving(true);
    try {
      const res = await companyService.saveSettings(parsed.payload);
      toast.fromServer(res.message, "Pengaturan operasional disimpan");
      onSaved(res.data ?? {});
    } catch (err) {
      const apiErr = toApiError(err);
      setErrors(serverSettingErrors(apiErr.errors));
      setFormError(apiErr.message);
      toast.danger("Gagal menyimpan pengaturan", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  const disabled = !canManage || saving;
  const changed = countChanged(values, baseline);

  return (
    <div className="py-6 first:pt-0">
      <div>
        <FormSection
          title="Pengaturan operasional"
          description="Aturan yang dibaca modul stok dan pengiriman. Nilai kosong memakai bawaan sistem."
        >
          <ReadOnlyHint show={!canManage} />
          {formError && (
            <Notice tone="danger" title="Pengaturan belum tersimpan" className="mb-4">
              {formError}
            </Notice>
          )}
          <div className="space-y-4">
            {defs.map((def) => {
              const value = values[def.key] ?? def.default;
              const err = errors[def.key];
              const id = `setting-${def.key}`;
              if (def.kind === "boolean") {
                return (
                  <div key={def.key}>
                    <label htmlFor={id} className="flex cursor-pointer items-start gap-2.5 text-sm text-foreground">
                      <Checkbox
                        id={id}
                        className="mt-0.5"
                        checked={value === "true"}
                        disabled={disabled}
                        onCheckedChange={(v) => setValue(def.key, v === true ? "true" : "false")}
                      />
                      <span>{def.label}</span>
                    </label>
                    {err && <p className="mt-1 text-xs text-destructive">{err}</p>}
                  </div>
                );
              }
              if (def.kind === "enum") {
                return (
                  <FormField key={def.key} label={def.label} htmlFor={id} errorText={err}>
                    <SelectInput id={id} value={value} disabled={disabled} onChange={(e) => setValue(def.key, e.target.value)}>
                      {(def.enum ?? []).map((opt) => (
                        <option key={opt} value={opt}>
                          {opt}
                        </option>
                      ))}
                    </SelectInput>
                  </FormField>
                );
              }
              const thresholdIrrelevant = def.key === THRESHOLD_KEY && values[ALWAYS_APPROVE_KEY] === "true";
              return (
                <FormField
                  key={def.key}
                  label={def.label}
                  htmlFor={id}
                  errorText={err}
                  helperText={thresholdIrrelevant ? "Tidak berlaku selama setiap koreksi wajib persetujuan." : undefined}
                >
                  <TextInput
                    id={id}
                    inputMode="decimal"
                    value={value}
                    disabled={disabled}
                    onChange={(e) => setValue(def.key, e.target.value)}
                  />
                </FormField>
              );
            })}
          </div>
        </FormSection>
      </div>
      {canManage && (
        <SettingsFooter
          count={changed}
          saving={saving}
          onReset={() => {
            setValues(baseline);
            setErrors({});
            setFormError(null);
          }}
          onSave={() => void handleSave()}
          saveLabel="Simpan pengaturan"
        />
      )}
    </div>
  );
}
