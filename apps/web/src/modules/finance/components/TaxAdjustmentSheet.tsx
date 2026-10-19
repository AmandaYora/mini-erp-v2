import { useState } from "react";
import type { FormEvent } from "react";
import { Button, DateInput, FormField, FormSheet, SelectInput, TextArea } from "@/shared/components/ui";
import { FileDropzone, Money } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { todayWIB } from "@/shared/lib/format";
import { useJournalDraft } from "@/modules/finance/hooks/use-journal-draft";
import { draftDifference, draftTotals, isDraftBalanced, parseJournalDraft } from "@/modules/finance/lib/journal-lines";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import { JournalLinesEditor } from "@/modules/finance/components/JournalLinesEditor";
import { TAX_CLASSES } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

/** Jenis, catatan, dan lampiran opsional koreksi (FN-07). */
export interface TaxAdjustmentExtra {
  classification: string;
  note: string;
  file: File | null;
}

interface TaxAdjustmentSheetProps {
  accounts: Account[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: ManualJournalValues, extra: TaxAdjustmentExtra) => void;
}

// Koreksi fiskal = jurnal bertipe; hanya memengaruhi laporan fiskal. Editor baris sama dengan jurnal manual.
export function TaxAdjustmentSheet({ accounts, submitting, serverError, onClose, onSubmit }: TaxAdjustmentSheetProps) {
  const draft = useJournalDraft();
  const [date, setDate] = useState(todayWIB());
  const [memo, setMemo] = useState("");
  const [classification, setClassification] = useState("");
  const [note, setNote] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const totals = draftTotals(draft.lines);
  const diff = draftDifference(draft.lines);
  const balanced = isDraftBalanced(draft.lines);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = parseJournalDraft({ date, memo, lines: draft.lines });
    const next: Record<string, string> = parsed.ok ? {} : { ...parsed.errors };
    if (classification === "") next.classification = "Pilih jenis koreksi";
    if (!parsed.ok || Object.keys(next).length > 0) {
      setErrors(next);
      return;
    }
    setErrors({});
    onSubmit(parsed.values, { classification, note: note.trim(), file });
  }

  return (
    <FormSheet
      open
      size="lg"
      title="Koreksi fiskal"
      description="Hanya memengaruhi laporan fiskal — laba komersial tidak berubah."
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="tax-adjustment-form" loading={submitting} disabled={!balanced}>
            Catat koreksi
          </Button>
        </>
      }
    >
      <form id="tax-adjustment-form" onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
        {serverError && <Notice tone="danger" title={serverError} />}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Tanggal" htmlFor="adjust-date" helperText="Kosongkan = hari ini">
            <DateInput id="adjust-date" value={date} onChange={(e) => setDate(e.target.value)} />
          </FormField>
          <FormField label="Memo" htmlFor="adjust-memo" required errorText={errors.memo}>
            <TextArea id="adjust-memo" value={memo} onChange={(e) => setMemo(e.target.value)} rows={1} placeholder="cth. Koreksi beban non-deductible" />
          </FormField>
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Jenis koreksi" htmlFor="adjust-class" required errorText={errors.classification}>
            <SelectInput id="adjust-class" value={classification} onChange={(e) => setClassification(e.target.value)}>
              <option value="">Pilih jenis…</option>
              {TAX_CLASSES.map((c) => (
                <option key={c.value} value={c.value}>
                  {c.label}
                </option>
              ))}
            </SelectInput>
          </FormField>
          <FormField label="Lampiran (gambar)" helperText="Opsional">
            <FileDropzone
              accept="image/jpeg,image/png,image/webp"
              fileName={file?.name ?? null}
              label="Pilih gambar bukti"
              hint="JPG, PNG, atau WebP, maks 5 MB"
              onFile={setFile}
            />
          </FormField>
        </div>
        <FormField label="Catatan" htmlFor="adjust-note" helperText="Dasar koreksi untuk konsultan pajak (maks 1000 karakter)">
          <TextArea id="adjust-note" value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={1000} />
        </FormField>
        {errors.lines && <p className="text-sm font-medium text-destructive">{errors.lines}</p>}
        <JournalLinesEditor draft={draft} accounts={accounts} errors={errors} compact />
        <p className="flex flex-wrap items-center gap-x-4 text-sm text-muted-foreground">
          <span>
            Debit <Money value={totals.debit} className="font-medium text-foreground" />
          </span>
          <span>
            Kredit <Money value={totals.credit} className="font-medium text-foreground" />
          </span>
          <span>
            Selisih D−K <Money value={diff} className={diff === 0 ? "font-medium text-foreground" : "font-semibold text-destructive"} />
          </span>
        </p>
      </form>
    </FormSheet>
  );
}
