import { useState } from "react";
import type { FormEvent } from "react";
import { Button, Checkbox, FormField, FormSheet, SelectInput, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import type { BankAccountPayload } from "@/modules/finance/services/finance.service";
import type { Account, BankAccount } from "@/modules/finance/types";
import { nextBankAccountCode } from "@/modules/finance/lib/bank-accounts";

interface BankAccountFormSheetProps {
  mode: "create" | "edit";
  initial?: BankAccount | null;
  /** Bagan akun: sumber pilihan akun kas/bank yang sudah ada. */
  accounts: Account[];
  /** Rekening yang ada: akun yang sudah dipakai tidak ditawarkan lagi. */
  existing: BankAccount[];
  /** Akun kas tunai terpetakan — bukan rekening. */
  cashAccountId: number | null;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: BankAccountPayload) => void;
}

const NEW = "new";

// Rekening perusahaan (ADR-0026): bank + nomor + atas nama di atas satu akun kas/bank.
// Akun buku besar terkunci setelah dibuat (pembayaran dan jurnal menunjuknya).
export function BankAccountFormSheet({
  mode,
  initial,
  accounts,
  existing,
  cashAccountId,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: BankAccountFormSheetProps) {
  const usedAccounts = new Set(existing.map((b) => b.accountId));
  const eligible = accounts.filter(
    (a) => a.status === "active" && a.isCash && a.type === "aset" && a.id !== cashAccountId && !usedAccounts.has(a.id),
  );
  const [bankName, setBankName] = useState(initial?.bankName ?? "");
  const [accountNumber, setAccountNumber] = useState(initial?.accountNumber ?? "");
  const [accountHolder, setAccountHolder] = useState(initial?.accountHolder ?? "");
  const [ledger, setLedger] = useState<string>(NEW);
  const [accountCode, setAccountCode] = useState(nextBankAccountCode(accounts));
  const [isDefault, setIsDefault] = useState(initial?.isDefault ?? existing.filter((b) => b.status === "active").length === 0);
  const [showOnDocuments, setShowOnDocuments] = useState(initial?.showOnDocuments ?? true);
  const [errors, setErrors] = useState<Record<string, string>>({});

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const next: Record<string, string> = {};
    if (bankName.trim() === "") next.bankName = "Wajib diisi.";
    if (accountNumber.trim() === "") next.accountNumber = "Wajib diisi.";
    if (accountHolder.trim() === "") next.accountHolder = "Wajib diisi.";
    if (mode === "create" && ledger === NEW && accountCode.trim() === "") next.accountCode = "Isi kode akun baru.";
    setErrors(next);
    if (Object.keys(next).length > 0) return;
    const body: BankAccountPayload = {
      bankName: bankName.trim(),
      accountNumber: accountNumber.trim(),
      accountHolder: accountHolder.trim(),
      isDefault,
      showOnDocuments,
      sortOrder: initial?.sortOrder ?? 0,
    };
    if (mode === "create") {
      if (ledger === NEW) body.accountCode = accountCode.trim().toUpperCase();
      else body.accountId = Number(ledger);
    }
    onSubmit(body);
  }

  return (
    <FormSheet
      open
      title={mode === "create" ? "Tambah rekening" : `Ubah rekening ${initial?.bankName ?? ""}`}
      description="Rekening tampil di nota dan dipilih kasir untuk pembayaran transfer. Setiap rekening punya akun kas/bank sendiri di buku besar."
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="bank-account-form" loading={submitting}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="bank-account-form" onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
        {serverError && <Notice tone="danger" title={serverError} />}
        <FormField label="Bank" htmlFor="bank-name" required errorText={errors.bankName}>
          <TextInput id="bank-name" value={bankName} onChange={(e) => setBankName(e.target.value)} placeholder="cth. BCA" />
        </FormField>
        <FormField label="Nomor rekening" htmlFor="bank-number" required errorText={errors.accountNumber}>
          <TextInput id="bank-number" value={accountNumber} onChange={(e) => setAccountNumber(e.target.value)} inputMode="numeric" />
        </FormField>
        <FormField label="Atas nama" htmlFor="bank-holder" required errorText={errors.accountHolder}>
          <TextInput id="bank-holder" value={accountHolder} onChange={(e) => setAccountHolder(e.target.value)} />
        </FormField>
        {mode === "create" ? (
          <>
            <FormField
              label="Akun buku besar"
              htmlFor="bank-ledger"
              helperText="Saldo rekening dicatat di akun ini. Tidak bisa diganti setelah disimpan."
            >
              <SelectInput id="bank-ledger" value={ledger} onChange={(e) => setLedger(e.target.value)}>
                <option value={NEW}>Buat akun kas/bank baru</option>
                {eligible.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.code} — {a.name}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            {ledger === NEW && (
              <FormField label="Kode akun baru" htmlFor="bank-code" required errorText={errors.accountCode}>
                <TextInput id="bank-code" value={accountCode} onChange={(e) => setAccountCode(e.target.value)} placeholder="cth. 1011" />
              </FormField>
            )}
          </>
        ) : (
          <FormField label="Akun buku besar" htmlFor="bank-ledger">
            <TextInput id="bank-ledger" value={`${initial?.accountCode ?? ""} — ${initial?.accountName ?? ""}`} disabled readOnly />
          </FormField>
        )}
        <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
          <Checkbox checked={isDefault} onCheckedChange={(v) => setIsDefault(v === true)} aria-label="Jadikan rekening default" />
          Jadikan default (terisi otomatis di order dan pembayaran transfer)
        </label>
        <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
          <Checkbox checked={showOnDocuments} onCheckedChange={(v) => setShowOnDocuments(v === true)} aria-label="Tampil di dokumen" />
          Tampil di daftar rekening pada struk/nota
        </label>
      </form>
    </FormSheet>
  );
}
