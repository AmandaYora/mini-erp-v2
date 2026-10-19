// Pajak order jual: cermin money.LineAmounts server (apps/api/internal/shared/money/lines.go).
// Hanya BANTUAN tampilan — server menghitung ulang dan menjadi sumber kebenaran.

export type SalesTaxType = "none" | "include" | "exclude";

/** Tarif PPN bawaan bila order ditandai kena pajak (belum ada pengaturan perusahaan). */
export const DEFAULT_VAT_RATE = 11;

export interface TaxLineInput {
  /** Net baris setelah diskon, sebelum pajak (rupiah). */
  net: number;
  /** false = baris dikecualikan dari pajak order. */
  taxable: boolean;
}

export interface TaxTotals {
  /** Dasar pengenaan pajak (DPP). */
  base: number;
  tax: number;
  total: number;
}

export function lineTax(net: number, taxType: SalesTaxType, taxRate: number): TaxTotals {
  switch (taxType) {
    case "exclude": {
      const tax = Math.round((net * taxRate) / 100);
      return { base: net, tax, total: net + tax };
    }
    case "include": {
      const base = taxRate > 0 ? Math.round(net / (1 + taxRate / 100)) : net;
      return { base, tax: net - base, total: net };
    }
    default:
      return { base: net, tax: 0, total: net };
  }
}

export function estimateTax(lines: TaxLineInput[], taxType: SalesTaxType, taxRate: number): TaxTotals {
  return lines.reduce<TaxTotals>(
    (acc, l) => {
      const t = l.taxable ? lineTax(l.net, taxType, taxRate) : lineTax(l.net, "none", 0);
      return { base: acc.base + t.base, tax: acc.tax + t.tax, total: acc.total + t.total };
    },
    { base: 0, tax: 0, total: 0 },
  );
}

export function normalizeTaxType(v: string | undefined): SalesTaxType {
  return v === "include" || v === "exclude" ? v : "none";
}
