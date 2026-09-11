import { useState } from "react";
import type { FormEvent } from "react";
import {
  ActionRow,
  Button,
  DateInput,
  FormField,
  Modal,
  SearchSelect,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, todayWIB } from "@/shared/lib/format";
import { manualJournalSchema } from "@/modules/finance/schemas/finance.schema";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import type { Account } from "@/modules/finance/types";

interface ManualRow {
  key: number;
  accountCode: string;
  debit: string;
  credit: string;
}

let nextRowKey = 1;

interface ManualJournalModalProps {
  open: boolean;
  accounts: Account[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: ManualJournalValues) => void;
  /** Override judul + deskripsi (default: Jurnal Manual) — dipakai Saldo
   * Awal & Koreksi Fiskal yang memakai editor baris yang sama. */
  title?: string;
  description?: string;
  submitLabel?: string;
}

// Jurnal manual: akun dipilih by CODE (backend mencari by code, huruf besar
// otomatis — posting.go: Manual). Balance dicek di klien sebelum submit.
export function ManualJournalModal({
  open,
  accounts,
  submitting,
  serverError,
  onClose,
  onSubmit,
  title = "Jurnal Manual",
  description = "Untuk koreksi, saldo awal, dan penyesuaian. Harus seimbang (debit = kredit).",
  submitLabel,
}: ManualJournalModalProps) {
  const [date, setDate] = useState(todayWIB());
  const [memo, setMemo] = useState("");
  const [rows, setRows] = useState<ManualRow[]>([
    { key: nextRowKey++, accountCode: "", debit: "", credit: "" },
    { key: nextRowKey++, accountCode: "", debit: "", credit: "" },
  ]);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const accountOptions = accounts
    .filter((a) => a.status === "active")
    .map((a) => ({ value: a.code, label: `${a.code} — ${a.name}` }));

  const totals = rows.reduce(
    (s, r) => ({
      debit: s.debit + (Number(r.debit) > 0 ? Number(r.debit) : 0),
      credit: s.credit + (Number(r.credit) > 0 ? Number(r.credit) : 0),
    }),
    { debit: 0, credit: 0 },
  );
  const balanced = totals.debit > 0 && totals.debit === totals.credit;

  function updateRow(key: number, patch: Partial<ManualRow>) {
    setRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  function removeRow(key: number) {
    setRows((prev) => prev.filter((r) => r.key !== key));
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = manualJournalSchema.safeParse({
      date: date.trim() ? date.trim() : undefined,
      memo: memo.trim(),
      lines: rows.map((r) => ({
        accountCode: r.accountCode,
        debit: r.debit.trim() === "" ? 0 : r.debit,
        credit: r.credit.trim() === "" ? 0 : r.credit,
      })),
    });
    if (!parsed.success) {
      const mapped: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        const k = issue.path.join(".");
        if (!mapped[k]) mapped[k] = issue.message;
        if (issue.path[0] === "lines" && !mapped.lines) {
          mapped.lines = issue.message;
        }
      }
      if (parsed.error.flatten().fieldErrors.memo?.[0]) {
        mapped.memo = parsed.error.flatten().fieldErrors.memo![0]!;
      }
      setErrors(mapped);
      return;
    }
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <Modal
      open={open}
      title={title}
      description={description}
      size="xl"
      onClose={onClose}
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <FormField label="Tanggal" helperText="Kosongkan = hari ini">
            <DateInput value={date} onChange={(e) => setDate(e.target.value)} />
          </FormField>
          <div className="sm:col-span-2">
            <FormField label="Memo" required errorText={errors.memo}>
              <TextArea
                value={memo}
                onChange={(e) => setMemo(e.target.value)}
                placeholder="cth. Koreksi saldo kas"
                rows={1}
              />
            </FormField>
          </div>
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <p className="text-heading text-[0.85rem] font-medium">Baris Jurnal</p>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() =>
                setRows((prev) => [
                  ...prev,
                  { key: nextRowKey++, accountCode: "", debit: "", credit: "" },
                ])
              }
            >
              Tambah Baris
            </Button>
          </div>
          {errors.lines && (
            <p className="text-bad mb-2 text-[0.82rem]">{errors.lines}</p>
          )}
          <div className="flex flex-col gap-2">
            {rows.map((r, i) => (
              <div
                key={r.key}
                className="grid grid-cols-[1fr_1fr_1fr_auto] items-end gap-2"
              >
                <FormField
                  label={i === 0 ? "Akun" : ""}
                  errorText={errors[`lines.${i}.accountCode`]}
                >
                  <SearchSelect
                    options={accountOptions}
                    value={r.accountCode || null}
                    onChange={(v) =>
                      updateRow(r.key, {
                        accountCode: typeof v === "string" ? v : "",
                      })
                    }
                    placeholder="Pilih akun…"
                  />
                </FormField>
                <FormField
                  label={i === 0 ? "Debit (Rp)" : ""}
                  errorText={errors[`lines.${i}.debit`]}
                >
                  <TextInput
                    value={r.debit}
                    onChange={(e) => updateRow(r.key, { debit: e.target.value })}
                    placeholder="0"
                    inputMode="numeric"
                  />
                </FormField>
                <FormField label={i === 0 ? "Kredit (Rp)" : ""}>
                  <TextInput
                    value={r.credit}
                    onChange={(e) =>
                      updateRow(r.key, { credit: e.target.value })
                    }
                    placeholder="0"
                    inputMode="numeric"
                  />
                </FormField>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  disabled={rows.length <= 2}
                  onClick={() => removeRow(r.key)}
                >
                  Hapus
                </Button>
              </div>
            ))}
          </div>
          <p className="text-muted mt-2 text-[0.82rem]">
            Total debit {formatIDR(totals.debit)} · total kredit{" "}
            {formatIDR(totals.credit)}
            {!balanced && " — harus sama dan lebih dari 0"}
          </p>
        </div>

        <ActionRow>
          <Button
            type="button"
            variant="secondary"
            onClick={onClose}
            disabled={submitting}
          >
            Batal
          </Button>
          <Button type="submit" disabled={submitting || !balanced}>
            {submitting ? "Menyimpan…" : (submitLabel ?? "Simpan Jurnal")}
          </Button>
        </ActionRow>
      </form>
    </Modal>
  );
}
