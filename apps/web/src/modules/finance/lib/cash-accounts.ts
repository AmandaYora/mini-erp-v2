import type { CashAccountOption } from "@/modules/finance/types";

// Akun kas untuk pembayaran tunai (Kas Tunai, Kas Besar, …) — util murni. Kas
// terakhir yang dipakai diingat per arah uang di perangkat ini, supaya mis. semua
// bayar pemasok dari Kas Besar tidak perlu dipilih ulang setiap kali.

export type CashDirection = "in" | "out";

const KEY = "payment.lastKas.";

/** Kas yang boleh dipilih untuk pembayaran baru. */
export function activeCashAccounts(list: CashAccountOption[]): CashAccountOption[] {
  return list.filter((a) => a.status === "active");
}

/** Pemilih kas hanya berguna bila ada lebih dari satu kas aktif. */
export function cashAccountChoiceShown(list: CashAccountOption[]): boolean {
  return activeCashAccounts(list).length >= 2;
}

/** "1001 Kas Besar". */
export function cashAccountLabel(a: Pick<CashAccountOption, "code" | "name">): string {
  return `${a.code} ${a.name}`;
}

export function cashAccountByAccountId(list: CashAccountOption[], accountId: number | null | undefined): CashAccountOption | undefined {
  if (!accountId) return undefined;
  return list.find((a) => a.accountId === accountId);
}

export function rememberedCashAccount(direction: CashDirection): number | null {
  try {
    const v = Number(window.localStorage.getItem(KEY + direction));
    return Number.isInteger(v) && v > 0 ? v : null;
  } catch {
    return null;
  }
}

export function rememberCashAccount(direction: CashDirection, accountId: number | null) {
  try {
    if (accountId) window.localStorage.setItem(KEY + direction, String(accountId));
  } catch {
    // tanpa storage: pilihan tidak diingat
  }
}

/** Nilai awal pemilih: kas yang diminta (mis. kas pembayaran asli saat koreksi) bila
 * masih aktif, lalu kas terakhir untuk arah ini, lalu kas default (Kas Tunai). */
export function initialCashAccount(list: CashAccountOption[], direction: CashDirection, preferred?: number | null): number | null {
  const active = activeCashAccounts(list);
  for (const id of [preferred ?? null, rememberedCashAccount(direction)]) {
    if (id && active.some((a) => a.accountId === id)) return id;
  }
  return active.find((a) => a.isDefault)?.accountId ?? active[0]?.accountId ?? null;
}

/** cashAccountId yang dikirim untuk pembayaran tunai: hanya bila pemilih tampil. */
export function cashAccountForPayload(list: CashAccountOption[], value: number | null): number | undefined {
  return cashAccountChoiceShown(list) && value ? value : undefined;
}
