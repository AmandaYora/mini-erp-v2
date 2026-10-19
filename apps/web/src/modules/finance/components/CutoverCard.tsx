import { useState } from "react";
import type { FormEvent } from "react";
import { Button, DataTable, FormField, FormSheet, SectionCard, TextInput } from "@/shared/components/ui";
import { Qty } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeService } from "@/modules/finance/services/finance.service";
import { cutoverSchema } from "@/modules/finance/schemas/finance.schema";
import { CUTOVER_DOC_TYPES, docTypeLabel } from "@/modules/finance/types";

interface CutoverCardProps {
  canManage: boolean;
  /** Dipanggil setelah batas berubah agar antrean dimuat ulang. */
  onChanged: () => void;
}

// Batas cutover per jenis dokumen (GET/PUT /finance/cutover): dokumen dengan id <= batas dianggap sudah
// dijurnal di sistem lama dan tidak masuk antrean. Backend menolak batas yang menutupi dokumen yang
// sudah dijurnal sistem ini. Ubah batas lewat panel samping.
export function CutoverCard({ canManage, onChanged }: CutoverCardProps) {
  const { data, loading, error, reload } = useAsyncData(() => financeService.cutover().then((m) => m ?? {}), []);
  const [editing, setEditing] = useState<string | null>(null);
  const [value, setValue] = useState("");
  const [fieldError, setFieldError] = useState<string | undefined>(undefined);
  const [serverError, setServerError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  function startEdit(docType: string) {
    setEditing(docType);
    setValue(String(data?.[docType] ?? 0));
    setFieldError(undefined);
    setServerError(null);
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    const parsed = cutoverSchema.safeParse({ docType: editing, coveredThroughId: value });
    if (!parsed.success) {
      setFieldError(parsed.error.flatten().fieldErrors.coveredThroughId?.[0] ?? "ID tidak valid");
      return;
    }
    setFieldError(undefined);
    setSaving(true);
    setServerError(null);
    try {
      const res = await financeService.setCutover(parsed.data.docType, parsed.data.coveredThroughId);
      toast.fromServer(res.message, "Batas cutover disimpan", docTypeLabel(parsed.data.docType));
      setEditing(null);
      reload();
      onChanged();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <SectionCard
      flush
      title="Batas cutover"
      description="Dokumen dengan ID sampai batas ini dianggap sudah dijurnal (saldo awal atau jurnal migrasi) dan tidak masuk antrean. 0 = tidak ada batas."
    >
      <DataTable<string>
        caption="Batas cutover per jenis dokumen"
        rows={[...CUTOVER_DOC_TYPES]}
        rowKey={(t) => t}
        loading={loading}
        error={error}
        onRetry={reload}
        emptyTitle="Tidak ada jenis dokumen"
        columns={[
          { header: "Jenis dokumen", render: (t) => docTypeLabel(t) },
          { header: "Dijurnal s/d ID", align: "right", render: (t) => <Qty value={data?.[t] ?? 0} /> },
          {
            header: "",
            align: "right",
            render: (t) =>
              canManage ? (
                <Button variant="ghost" size="sm" aria-label={`Ubah batas ${docTypeLabel(t)}`} onClick={() => startEdit(t)}>
                  Ubah
                </Button>
              ) : null,
          },
        ]}
      />
      {editing && (
        <FormSheet
          open
          title={`Batas ${docTypeLabel(editing)}`}
          description="Dokumen dengan ID sampai batas ini tidak masuk antrean posting."
          onClose={() => (saving ? undefined : setEditing(null))}
          actions={
            <>
              <Button variant="secondary" disabled={saving} onClick={() => setEditing(null)}>
                Batal
              </Button>
              <Button type="submit" form="cutover-form" loading={saving}>
                Simpan batas
              </Button>
            </>
          }
        >
          <form id="cutover-form" onSubmit={(e) => void handleSubmit(e)} className="flex flex-col gap-4" noValidate>
            {serverError && <Notice tone="danger" title={serverError} />}
            <FormField label={`Batas ${docTypeLabel(editing)} (ID)`} htmlFor="cutover-value" errorText={fieldError}>
              <TextInput id="cutover-value" value={value} onChange={(e) => setValue(e.target.value)} inputMode="numeric" disabled={saving} />
            </FormField>
          </form>
        </FormSheet>
      )}
    </SectionCard>
  );
}
