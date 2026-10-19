import type { TaxDetailRow } from "@/modules/finance/types";

/** Label jenis baris rincian PPN. */
export function taxKindLabel(kind: string): string {
  switch (kind) {
    case "sales":
      return "Penjualan";
    case "sales_return":
      return "Retur Jual";
    case "purchase":
      return "Pembelian";
    case "purchase_return":
      return "Retur Beli";
    default:
      return kind || "-";
  }
}

/** Nada badge: keluaran netral, retur peringatan, masukan info. */
export function taxKindTone(kind: string): "success" | "warning" | "info" | "neutral" {
  switch (kind) {
    case "sales":
      return "success";
    case "sales_return":
    case "purchase_return":
      return "warning";
    case "purchase":
      return "info";
    default:
      return "neutral";
  }
}

/** Banyaknya baris kena pajak tanpa nomor faktur (FN-06) pada daftar yang tampil. */
export function missingInvoiceCount(rows: TaxDetailRow[]): number {
  return rows.filter((r) => r.missingInvoice).length;
}

/** Jumlah halaman yang sedang tampil (bukan total periode). */
export function pageTaxTotals(rows: TaxDetailRow[]): {
  revenue: number;
  ppnOut: number;
  ppnIn: number;
  dpp: number;
} {
  return rows.reduce(
    (acc, r) => ({
      revenue: acc.revenue + r.revenue,
      ppnOut: acc.ppnOut + r.ppn,
      ppnIn: acc.ppnIn + (r.ppnIn ?? 0),
      dpp: acc.dpp + (r.dpp ?? 0),
    }),
    { revenue: 0, ppnOut: 0, ppnIn: 0, dpp: 0 },
  );
}
