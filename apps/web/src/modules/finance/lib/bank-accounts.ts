import type { Account, BankAccount } from "@/modules/finance/types";

// Rekening perusahaan (ADR-0026) — util murni yang dipakai pemilih rekening,
// halaman pembayaran, POS, dan cetak.

/** "BCA 123 456 a.n. PT Maju" — label satu baris. */
export function bankAccountLabel(b: Pick<BankAccount, "bankName" | "accountNumber" | "accountHolder">): string {
  const holder = b.accountHolder.trim();
  return `${b.bankName} ${b.accountNumber}${holder ? ` a.n. ${holder}` : ""}`;
}

/** Rekening yang boleh dipilih untuk tagihan/pembayaran baru. */
export function activeBankAccounts(list: BankAccount[]): BankAccount[] {
  return list.filter((b) => b.status === "active");
}

/** Akun buku besar rekening default (null bila tidak ada). */
export function defaultBankAccountId(list: BankAccount[]): number | null {
  return activeBankAccounts(list).find((b) => b.isDefault)?.accountId ?? null;
}

/** Cari rekening dari id akun buku besarnya (termasuk yang diarsipkan). */
export function bankAccountByAccountId(list: BankAccount[], accountId: number | null | undefined): BankAccount | undefined {
  if (!accountId) return undefined;
  return list.find((b) => b.accountId === accountId);
}

/** Metode yang membawa rekening: transfer dan cek. Tunai tidak. */
export function methodUsesBankAccount(method: string | null | undefined): boolean {
  return method === "transfer" || method === "check";
}

/**
 * Nilai awal pemilih rekening pembayaran: rekening tujuan tagihan bila masih
 * aktif, lalu default perusahaan, lalu kosong.
 */
export function initialPaymentAccount(list: BankAccount[], billAccountId?: number | null): number | null {
  const bill = bankAccountByAccountId(activeBankAccounts(list), billAccountId ?? null);
  return bill?.accountId ?? defaultBankAccountId(list);
}

/** Pesan galat bila rekening wajib tetapi kosong (cermin aturan server); null = valid. */
export function paymentBankAccountError(bank: { required: boolean }, method: string, value: number | null): string | null {
  if (methodUsesBankAccount(method) && bank.required && value === null) return "Pilih rekening untuk transfer/cek.";
  return null;
}

/** Kode akun kas/bank berikutnya yang masih bebas (1011, 1012, …). */
export function nextBankAccountCode(accounts: Pick<Account, "code">[]): string {
  const used = new Set(accounts.map((a) => a.code));
  for (let n = 1011; n < 1100; n++) if (!used.has(String(n))) return String(n);
  return "";
}
