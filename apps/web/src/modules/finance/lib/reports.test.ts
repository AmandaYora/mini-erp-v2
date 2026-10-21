import { describe, expect, it } from "vitest";
import {
  agingBucketKey,
  agingBucketRows,
  cashFlowTotals,
  cashRowReconciles,
  integritySummary,
  inventoryGapHint,
  isOverdue,
} from "@/modules/finance/lib/reports";
import type { InventoryRecon } from "@/modules/finance/types";

describe("finance report helpers", () => {
  it("maps age to bucket", () => {
    expect(agingBucketKey(0)).toBe("d0_30");
    expect(agingBucketKey(30)).toBe("d0_30");
    expect(agingBucketKey(31)).toBe("d31_60");
    expect(agingBucketKey(90)).toBe("d61_90");
    expect(agingBucketKey(91)).toBe("over90");
  });

  it("lists buckets in order", () => {
    const rows = agingBucketRows({ d0_30: 1, d31_60: 2, d61_90: 3, over90: 4 });
    expect(rows.map((r) => r.amount)).toEqual([1, 2, 3, 4]);
  });

  it("detects overdue", () => {
    expect(isOverdue({ overdueDays: 1 })).toBe(true);
    expect(isOverdue({ overdueDays: 0 })).toBe(false);
  });

  it("reconciles cash rows and totals", () => {
    const rows = [
      { code: "1000", name: "Kas", opening: 100, in: 50, out: 30, closing: 120 },
      { code: "1010", name: "Bank", opening: 0, in: 10, out: 0, closing: 10 },
    ];
    expect(rows.every(cashRowReconciles)).toBe(true);
    expect(cashRowReconciles({ ...rows[0], closing: 1 })).toBe(false);
    expect(cashFlowTotals(rows)).toEqual({ in: 60, out: 30 });
  });

  it("summarises integrity", () => {
    expect(integritySummary({ rollupDiffs: [], unbalanced: 0, consistent: true })).toMatch(/konsisten/);
    const s = integritySummary({
      rollupDiffs: [
        { accountId: 1, date: "2026-01-01", scope: 0, linesDebit: 1, linesCredit: 0, rollupDebit: 2, rollupCredit: 0 },
      ],
      unbalanced: 2,
      consistent: false,
    });
    expect(s).toContain("1 selisih rollup");
    expect(s).toContain("2 jurnal tidak seimbang");
  });

  it("explains the inventory gap by pending documents before judging it", () => {
    const base: InventoryRecon = {
      book: 900_000, ledger: 1_000_000, diff: -100_000, tolerance: 101, pending: 1, pendingValue: 100_000,
      ignoredValue: 0, unexplained: 0, missingCost: 0, consistent: true,
    };
    expect(inventoryGapHint(base)).toMatch(/^Cocok/);
    expect(inventoryGapHint(base)).toContain("1 dokumen belum dijurnal");
    const broken = { ...base, pendingValue: 0, unexplained: -100_000, consistent: false };
    expect(inventoryGapHint(broken)).toMatch(/^Telusuri/);
    expect(inventoryGapHint(broken)).not.toContain("belum dijurnal");
  });
});
