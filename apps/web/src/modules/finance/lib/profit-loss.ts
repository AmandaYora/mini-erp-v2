import type { AccountingRowKind } from "@/modules/finance/components/AccountingTable";
import type { PLLine, ProfitLossReport } from "@/modules/finance/types";

export interface PLRow {
  key: string;
  label: string;
  amount: number;
  kind: AccountingRowKind;
  indent?: boolean;
}

function section(key: string, title: string, lines: PLLine[] | undefined, total: number, sign: 1 | -1): PLRow[] {
  // Biaya tampil negatif (mengurangi laba); rincian dan golongan memakai tanda yang sama.
  const out: PLRow[] = [{ key, label: title, amount: sign * total, kind: "group" }];
  for (const l of lines ?? []) {
    out.push({ key: `${key}-${l.code}`, label: `${l.code} ${l.name}`, amount: sign * l.amount, kind: "line", indent: true });
  }
  return out;
}

/** Susun baris laporan laba rugi: golongan + rincian per akun + subtotal (FN-03). */
export function buildProfitLossRows(r: ProfitLossReport): PLRow[] {
  return [
    ...section("revenue", "Pendapatan", r.revenueLines, r.revenue, 1),
    ...section("cogs", "Harga Pokok Penjualan", r.cogsLines, r.cogs, -1),
    { key: "gross", label: "Laba Kotor", amount: r.gross, kind: "subtotal" },
    ...section("expense", "Beban Operasional", r.expenseLines, r.expense, -1),
    { key: "profit", label: "Laba Bersih", amount: r.profit, kind: "total" },
  ];
}

/** Jumlah rincian sebuah golongan (untuk uji: harus sama dengan angka ringkasan). */
export function sumLines(lines: PLLine[] | undefined): number {
  return (lines ?? []).reduce((a, l) => a + l.amount, 0);
}
