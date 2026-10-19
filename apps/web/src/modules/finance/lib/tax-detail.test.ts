import { describe, expect, it } from "vitest";
import { pageTaxTotals, taxKindLabel, taxKindTone } from "@/modules/finance/lib/tax-detail";
import type { TaxDetailRow } from "@/modules/finance/types";

const row = (over: Partial<TaxDetailRow>): TaxDetailRow => ({
  entryId: 1,
  kind: "sales",
  ppnIn: 0,
  date: "2026-09-01",
  number: "J-1",
  memo: "",
  revenue: 1000,
  ppn: 110,
  taxInvoiceNumber: "",
  taxInvoiceDate: "",
  ...over,
});

describe("tax detail helpers", () => {
  it("labels kinds", () => {
    expect(taxKindLabel("sales")).toBe("Penjualan");
    expect(taxKindLabel("sales_return")).toBe("Retur Jual");
    expect(taxKindLabel("purchase")).toBe("Pembelian");
    expect(taxKindLabel("")).toBe("-");
    expect(taxKindTone("purchase")).toBe("info");
  });

  it("totals out and in separately", () => {
    const t = pageTaxTotals([
      row({}),
      row({ kind: "purchase", revenue: 0, ppn: 0, ppnIn: 55 }),
    ]);
    expect(t).toEqual({ revenue: 1000, ppnOut: 110, ppnIn: 55, dpp: 0 });
  });
});
