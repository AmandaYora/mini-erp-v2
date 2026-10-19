import { describe, expect, it } from "vitest";
import { buildProfitLossRows, sumLines } from "@/modules/finance/lib/profit-loss";
import type { ProfitLossReport } from "@/modules/finance/types";

const report: ProfitLossReport = {
  revenue: 1_000_000,
  cogs: 600_000,
  gross: 400_000,
  expense: 150_000,
  profit: 250_000,
  revenueLines: [
    { code: "4000", name: "Penjualan Barang", amount: 1_100_000 },
    { code: "4100", name: "Retur Penjualan", amount: -100_000 },
  ],
  cogsLines: [{ code: "5000", name: "HPP", amount: 600_000 }],
  expenseLines: [
    { code: "6000", name: "Beban Gaji", amount: 100_000 },
    { code: "6010", name: "Listrik", amount: 50_000 },
  ],
  from: "2026-10-01",
  to: "2026-10-31",
};

describe("laba rugi rinci (FN-03)", () => {
  it("rincian tiap golongan berjumlah sama dengan angka ringkasan", () => {
    expect(sumLines(report.revenueLines)).toBe(report.revenue);
    expect(sumLines(report.cogsLines)).toBe(report.cogs);
    expect(sumLines(report.expenseLines)).toBe(report.expense);
  });

  it("menyusun golongan, rincian per akun, subtotal, dan total", () => {
    const rows = buildProfitLossRows(report);
    expect(rows.map((r) => r.key)).toEqual([
      "revenue", "revenue-4000", "revenue-4100",
      "cogs", "cogs-5000",
      "gross",
      "expense", "expense-6000", "expense-6010",
      "profit",
    ]);
    expect(rows.find((r) => r.key === "revenue")?.amount).toBe(1_000_000);
    expect(rows.find((r) => r.key === "cogs")?.amount).toBe(-600_000);
    expect(rows.find((r) => r.key === "expense-6010")?.amount).toBe(-50_000);
    expect(rows.find((r) => r.key === "profit")?.kind).toBe("total");
    // Penjumlahan baris tersaji = laba bersih.
    const topLevel = rows.filter((r) => r.kind === "group").reduce((a, r) => a + r.amount, 0);
    expect(topLevel).toBe(report.profit);
  });

  it("laporan lama tanpa rincian tetap tampil sebagai ringkasan", () => {
    const rows = buildProfitLossRows({ ...report, revenueLines: undefined, cogsLines: undefined, expenseLines: undefined });
    expect(rows).toHaveLength(5);
  });
});
