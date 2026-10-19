import { useState } from "react";
import type { FormEvent } from "react";
import { Button, DateInput, FormField, FormSheet, SearchSelect, TextArea, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { todayWIB } from "@/shared/lib/format";
import { expenseFormSchema } from "@/modules/finance/schemas/finance.schema";
import type { ExpenseFormValues } from "@/modules/finance/schemas/finance.schema";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import type { Account } from "@/modules/finance/types";

interface ExpenseFormSheetProps {
  accounts: Account[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: ExpenseFormValues) => void;
}

// Catat biaya → POST /expenses {expenseAccountId, payAccountId, amount, date?, notes?}.
// Akun beban difilter tipe beban; pembayar harus akun kas/bank dan berbeda (aturan backend).
export function ExpenseFormSheet({ accounts, submitting, serverError, onClose, onSubmit }: ExpenseFormSheetProps) {
  const [expenseAccountId, setExpenseAccountId] = useState<number | null>(null);
  const [payAccountId, setPayAccountId] = useState<number | null>(null);
  const [amount, setAmount] = useState("");
  const [date, setDate] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [partyId, setPartyId] = useState<number | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const active = accounts.filter((a) => a.status === "active");
  const expenseOptions = active.filter((a) => a.type === "beban").map((a) => ({ value: a.id, label: `${a.code} — ${a.name}` }));
  const payOptions = active.filter((a) => a.type === "aset" && a.isCash).map((a) => ({ value: a.id, label: `${a.code} — ${a.name}` }));

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = expenseFormSchema.safeParse({
      expenseAccountId,
      payAccountId,
      amount,
      date: date.trim() ? date.trim() : undefined,
      notes: notes.trim() ? notes.trim() : undefined,
      partyId: partyId ?? undefined,
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

  const toId = (v: string | number | null) => (typeof v === "number" ? v : v === null ? null : Number(v));

  return (
    <FormSheet
      open
      title="Catat biaya"
      description="Otomatis memposting jurnal (debit beban, kredit kas). Pembatalan membalik jurnalnya."
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="expense-form" loading={submitting}>
            Simpan biaya
          </Button>
        </>
      }
    >
      <form id="expense-form" onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
        {serverError && <Notice tone="danger" title={serverError} />}
        <FormField label="Akun beban" required errorText={errors.expenseAccountId}>
          <SearchSelect options={expenseOptions} value={expenseAccountId} onChange={(v) => setExpenseAccountId(toId(v))} placeholder="Pilih akun beban…" />
        </FormField>
        <FormField label="Dibayar dari" required errorText={errors.payAccountId}>
          <SearchSelect options={payOptions} value={payAccountId} onChange={(v) => setPayAccountId(toId(v))} placeholder="Pilih akun kas atau bank…" />
        </FormField>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Nominal (Rp)" htmlFor="expense-amount" required errorText={errors.amount}>
            <TextInput id="expense-amount" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="cth. 250000" inputMode="numeric" />
          </FormField>
          <FormField label="Tanggal" htmlFor="expense-date" helperText="Kosongkan = hari ini">
            <DateInput id="expense-date" value={date} onChange={(e) => setDate(e.target.value)} />
          </FormField>
        </div>
        <FormField label="Pihak lawan" helperText="Opsional: pemasok jasa, pemilik sewa, dll.">
          <PartySearchSelect value={partyId} partyKind="any" onChange={(v) => setPartyId(v)} placeholder="Tanpa pihak" />
        </FormField>
        <FormField label="Catatan" htmlFor="expense-notes" errorText={errors.notes}>
          <TextArea id="expense-notes" value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Keterangan (opsional)" rows={2} />
        </FormField>
      </form>
    </FormSheet>
  );
}
