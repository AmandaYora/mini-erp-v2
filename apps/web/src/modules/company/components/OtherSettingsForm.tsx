import { useState } from "react";
import { Button, FormField, TextInput } from "@/shared/components/ui";
import { FormSection } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { companyService } from "@/modules/company/services/company.service";
import SettingsFooter from "@/modules/company/components/SettingsFooter";
import ReadOnlyHint from "@/modules/company/components/ReadOnlyHint";
import type { CompanySettings } from "@/modules/company/types";

interface SettingRow {
  localId: number;
  key: string;
  value: string;
}

let nextRowId = 1;

function toRows(settings: CompanySettings, registeredKeys: string[]): SettingRow[] {
  // Kunci terdaftar punya kontrol sendiri; baris bebas hanya untuk sisanya.
  return Object.entries(settings)
    .filter(([key]) => !registeredKeys.includes(key))
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => ({ localId: nextRowId++, key, value }));
}

/** Pasangan kunci-nilai di luar pengaturan terdaftar. Induk memberi `key` baru setelah simpan. */
export default function OtherSettingsForm({
  settings,
  registeredKeys,
  canManage,
  onSaved,
}: {
  settings: CompanySettings;
  registeredKeys: string[];
  canManage: boolean;
  onSaved: (settings: CompanySettings) => void;
}) {
  const [baseline] = useState(() => toRows(settings, registeredKeys));
  const [rows, setRows] = useState<SettingRow[]>(baseline);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const changed = diffCount(rows, baseline);

  function updateRow(localId: number, patch: Partial<SettingRow>) {
    setRows((prev) => prev.map((r) => (r.localId === localId ? { ...r, ...patch } : r)));
  }

  async function handleSave() {
    if (!canManage) return;
    const payload: CompanySettings = {};
    for (const row of rows) {
      const key = row.key.trim();
      if (key === "") {
        setError("Semua baris pengaturan wajib memiliki kunci.");
        return;
      }
      if (key in payload) {
        setError(`Kunci ganda: "${key}". Gabungkan menjadi satu baris.`);
        return;
      }
      payload[key] = row.value;
    }
    if (Object.keys(payload).length === 0) {
      setError("Tidak ada pengaturan untuk disimpan.");
      return;
    }
    setError(null);
    setSaving(true);
    try {
      // Server melakukan merge (tidak menghapus kunci lain).
      const saved = await companyService.saveSettings(payload);
      toast.fromServer(saved.message, "Pengaturan perusahaan disimpan");
      onSaved(saved.data ?? {});
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal menyimpan pengaturan", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="py-6 first:pt-0">
      <div>
        <FormSection
          title="Pengaturan lain"
          description="Pasangan kunci-nilai di luar pengaturan terdaftar. Penyimpanan menggabungkan (merge) dengan yang sudah ada."
        >
          <ReadOnlyHint show={!canManage} />
          {error && (
            <Notice tone="danger" title="Pengaturan belum tersimpan" className="mb-4">
              {error}
            </Notice>
          )}
          {rows.length === 0 ? (
            <p className="text-sm text-muted-foreground">Belum ada pengaturan. Tambah baris baru untuk mulai.</p>
          ) : (
            <div className="space-y-3">
              {rows.map((row, i) => (
                <div key={row.localId} className="grid items-end gap-3 md:grid-cols-[1fr_1fr_auto]">
                  <FormField label="Kunci" htmlFor={`other-key-${row.localId}`}>
                    <TextInput
                      id={`other-key-${row.localId}`}
                      value={row.key}
                      onChange={(e) => updateRow(row.localId, { key: e.target.value })}
                      placeholder="cth: pos.receipt_footer"
                      disabled={!canManage || saving}
                    />
                  </FormField>
                  <FormField label="Nilai" htmlFor={`other-value-${row.localId}`}>
                    <TextInput
                      id={`other-value-${row.localId}`}
                      value={row.value}
                      onChange={(e) => updateRow(row.localId, { value: e.target.value })}
                      placeholder="Nilai pengaturan"
                      disabled={!canManage || saving}
                    />
                  </FormField>
                  {canManage && (
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`Hapus baris ${i + 1}`}
                      onClick={() => setRows((prev) => prev.filter((r) => r.localId !== row.localId))}
                      disabled={saving}
                    >
                      Hapus
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}
          {canManage && (
            <Button
              variant="secondary"
              size="sm"
              className="mt-4"
              onClick={() => setRows((prev) => [...prev, { localId: nextRowId++, key: "", value: "" }])}
              disabled={saving}
            >
              Tambah baris
            </Button>
          )}
        </FormSection>
      </div>
      {canManage && (
        <SettingsFooter
          count={changed}
          saving={saving}
          onReset={() => {
            setRows(baseline);
            setError(null);
          }}
          onSave={() => void handleSave()}
          saveLabel="Simpan pengaturan"
        />
      )}
    </div>
  );
}

/** Jumlah baris yang ditambah, dihapus, atau diubah terhadap kondisi tersimpan. */
function diffCount(rows: SettingRow[], baseline: SettingRow[]): number {
  const base = new Map(baseline.map((r) => [r.key, r.value]));
  const cur = new Map(rows.map((r) => [r.key, r.value]));
  let n = 0;
  for (const [k, v] of cur) if (base.get(k) !== v) n++;
  for (const k of base.keys()) if (!cur.has(k)) n++;
  return n;
}
