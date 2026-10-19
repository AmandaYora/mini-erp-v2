import { ACCOUNT_TYPES, accountTypeLabel } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

export type AccountTreeRow =
  | { kind: "group"; id: string; type: string; label: string; count: number; expanded: boolean }
  | { kind: "account"; id: string; account: Account };

/**
 * Bagan akun sebagai pohon dua tingkat: golongan (tipe akun) → akun terurut kode.
 * API akun tidak punya kolom induk, jadi hierarki mengikuti tipe — bukan tebakan dari awalan kode.
 * Mengembalikan baris datar siap dirender; golongan yang dilipat menyembunyikan akunnya.
 * Saat `forceOpen` (sedang mencari) semua golongan terbuka.
 */
export function buildAccountTree(
  accounts: Account[] | null | undefined,
  collapsed: ReadonlySet<string>,
  forceOpen = false,
): AccountTreeRow[] {
  const list = accounts ?? [];
  const known = new Set<string>(ACCOUNT_TYPES);
  const types = [...ACCOUNT_TYPES, ...[...new Set(list.map((a) => a.type))].filter((t) => !known.has(t))];
  const rows: AccountTreeRow[] = [];
  for (const type of types) {
    const members = list
      .filter((a) => a.type === type)
      .sort((a, b) => a.code.localeCompare(b.code, "id", { numeric: true }));
    if (members.length === 0) continue;
    const expanded = forceOpen || !collapsed.has(type);
    rows.push({ kind: "group", id: `group:${type}`, type, label: accountTypeLabel(type), count: members.length, expanded });
    if (expanded) for (const account of members) rows.push({ kind: "account", id: `account:${account.id}`, account });
  }
  return rows;
}

/** Saring akun menurut kata kunci (kode atau nama) dan status; kata kunci kosong = semua. */
export function filterAccounts(
  accounts: Account[] | null | undefined,
  query: string,
  status: "" | "active" | "archived",
): Account[] {
  const q = query.trim().toLowerCase();
  return (accounts ?? []).filter((a) => {
    if (status && a.status !== status) return false;
    if (!q) return true;
    return a.code.toLowerCase().includes(q) || a.name.toLowerCase().includes(q);
  });
}
