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
import { todayWIB } from "@/shared/lib/format";
import { expenseFormSchema } from "@/modules/finance/schemas/finance.schema";
import type { ExpenseFormValues } from "@/modules/finance/schemas/finance.schema";
import type { Account } from "@/modules/finance/types";

interface ExpenseFormModalProps {
  open: boolean;
  accounts: Account[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: ExpenseFormValues) => void;
}

// Catat biaya → POST /expenses {expenseAccountId, payAccountId, amount,
// date?, notes?} (handler.go: CreateExpense). Akun beban difilter type
// beban; akun bayar harus berbeda (aturan backend).
export function ExpenseFormModal({
  open,
  accounts,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: ExpenseFormModalProps) {
  const [expenseAccountId, setExpenseAccountId] = useState<number | null>(null);
  const [payAccountId, setPayAccountId] = useState<number | null>(null);
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const active = accounts.filter((a) => a.status === "active");
  const expenseOptions = active
    .filter((a) => a.type === "beban")
    .map((a) => ({ value: a.id, label: `${a.code} — ${a.name}` }));
  const payOptions = active.map((a) => ({
    value: a.id,
    label: `${a.code} — ${a.name}`,
  }));

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = expenseFormSchema.safeParse({
      expenseAccountId,
      payAccountId,
      amount,
      date: date.trim() ? date.trim() : undefined,
      notes: notes.trim() ? notes.trim() : undefined,
    });
    if (!parsed.success) {
      const mapped: Record<string, string> = {};
      for (const [k, v] of Object.entries(parsed.error.flatten().fieldErrors)) {
        if (v?.[0]) mapped[k] = v[0];
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
      title="Catat Biaya"
      description="Otomatis memposting jurnal (Dr beban / Cr kas). Pembatalan membalik jurnalnya."
      onClose={onClose}
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        <FormField
          label="Akun Beban"
          required
          errorText={errors.expenseAccountId}
        >
          <SearchSelect
            options={expenseOptions}
            value={expenseAccountId}
            onChange={(v) =>
              setExpenseAccountId(
                typeof v === "number" ? v : v === null ? null : Number(v),
              )
            }
            placeholder="Pilih akun beban…"
          />
        </FormField>

        <FormField label="Dibayar Dari" required errorText={errors.payAccountId}>
          <SearchSelect
            options={payOptions}
            value={payAccountId}
            onChange={(v) =>
              setPayAccountId(
                typeof v === "number" ? v : v === null ? null : Number(v),
              )
            }
            placeholder="Pilih akun kas/bank…"
          />
        </FormField>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Nominal (Rp)" required errorText={errors.amount}>
            <TextInput
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="cth. 250000"
              inputMode="numeric"
            />
          </FormField>
          <FormField label="Tanggal" helperText="Kosongkan = hari ini">
            <DateInput value={date} onChange={(e) => setDate(e.target.value)} />
          </FormField>
        </div>

        <FormField label="Catatan" errorText={errors.notes}>
          <TextArea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Keterangan (opsional)"
            rows={2}
          />
        </FormField>

        <ActionRow>
          <Button
            type="button"
            variant="secondary"
            onClick={onClose}
            disabled={submitting}
          >
            Batal
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? "Menyimpan…" : "Simpan Biaya"}
          </Button>
        </ActionRow>
      </form>
    </Modal>
  );
}
