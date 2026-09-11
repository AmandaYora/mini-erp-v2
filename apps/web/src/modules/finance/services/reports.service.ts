// Service laporan keuangan — HANYA via helper http-client (apiGet; blob
// CSV via httpClient.get). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/finance/reports/trial-balance?year=&month= →
//   {rows[{code,name,type,debit,credit}],totalDebit,totalCredit,balanced}
//   (presentation/reports.go: TrialBalance).
// - GET /api/v1/finance/reports/profit-loss?from=&to= (YYYY-MM-DD) →
//   {revenue,cogs,gross,expense,profit,from,to} (ProfitLoss).
// - GET /api/v1/finance/reports/balance-sheet?date= (YYYY-MM-DD) →
//   {assets,liabilities,equity (TrialRow[], debit=saldo),totalAssets,
//   totalLiabilitiesEquity,balanced,date} (BalanceSheet).
// - GET /api/v1/finance/reports/general-ledger?account=&from=&to= →
//   ARRAY [{entryId,number,date,memo,debit,credit,balance}] langsung,
//   termasuk baris {memo:"Saldo awal"} bila saldo awal ≠ 0 (GeneralLedger).
// - GET /api/v1/finance/reports/tax-summary?year=&month= →
//   {year,month,ppnOut,ppnIn,payable} (TaxSummary).
// - GET /api/v1/finance/reports/tax-detail?year=&month= → ARRAY
//   [{date,number,memo,revenue,ppn}]; +&format=csv → file CSV mentah
//   (kolom tanggal,nomor,keterangan,omzet,ppn) untuk kertas kerja SPT.
// - GET /api/v1/finance/reports/inventory-value (tanpa param) →
//   {positions[{productId,variantId,productCode,productName,qty,avgCost,
//   value}],total} (InventoryValue).
// - GET /api/v1/finance/reports/margin?from=&to= →
//   {revenue,cogs,gross,percent} (Margin) — agregat satu angka.
// - GET /api/v1/finance/reports/margin-by-product?from=&to=[&limit=] → ARRAY
//   [{productId,variantId,productCode,productName,revenue,cogs,gross,percent}]
//   terurut marjin terbesar (analytics.go: MarginByProduct).
// - GET /api/v1/finance/reports/receivables[?asOf=] dan .../payables[?asOf=] →
//   {parties[{partyId,partyCode,partyName,debit,credit,balance}],total}.
//   asOf default hari ini (WIB); saldo bersifat kumulatif, bukan rentang.
// - GET /api/v1/finance/accounts[?status=] → ARRAY
//   [{id,code,name,type,isCash,status}] (handler.go: ListAccounts).

import { apiGet, httpClient } from "@/shared/services/http-client";
import type {
  Account,
  BalanceSheetReport,
  CashSummaryReport,
  FiscalSummaryReport,
  PartyBalanceReport,
  ProductMarginRow,
  InventoryValueReport,
  LedgerLine,
  MarginReport,
  ProfitLossReport,
  TaxDetailRow,
  TaxSummary,
  TrialBalanceReport,
} from "@/modules/finance/types";

/** Ambil nama berkas dari Content-Disposition supaya server tetap pemilik
 * penamaan — klien tidak menebak-nebak varian paket. */
function fileNameFrom(header: unknown): string | null {
  if (typeof header !== "string") return null;
  const m = header.match(/filename="([^"]+)"/);
  return m ? m[1] : null;
}

export const financeReportsService = {
  trialBalance: (year: number, month: number) =>
    apiGet<TrialBalanceReport>("/api/v1/finance/reports/trial-balance", {
      year,
      month,
    }),

  profitLoss: (from: string, to: string) =>
    apiGet<ProfitLossReport>("/api/v1/finance/reports/profit-loss", {
      from,
      to,
    }),

  balanceSheet: (date: string) =>
    apiGet<BalanceSheetReport>("/api/v1/finance/reports/balance-sheet", {
      date,
    }),

  generalLedger: (account: string, from: string, to: string) =>
    apiGet<LedgerLine[]>("/api/v1/finance/reports/general-ledger", {
      account,
      from,
      to,
    }),

  taxSummary: (year: number, month: number) =>
    apiGet<TaxSummary>("/api/v1/finance/reports/tax-summary", { year, month }),

  taxDetail: (year: number, month: number) =>
    apiGet<TaxDetailRow[]>("/api/v1/finance/reports/tax-detail", {
      year,
      month,
    }),

  /** Unduh kertas kerja SPT — CSV mentah (?format=csv), bukan envelope JSON. */
  downloadTaxDetailCsv: async (year: number, month: number): Promise<void> => {
    const res = await httpClient.get("/api/v1/finance/reports/tax-detail", {
      params: { year, month, format: "csv" },
      responseType: "blob",
    });
    const blob = new Blob([res.data], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `ppn-detail-${year}-${String(month).padStart(2, "0")}.csv`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },

  inventoryValue: () =>
    apiGet<InventoryValueReport>("/api/v1/finance/reports/inventory-value"),

  margin: (from: string, to: string) =>
    apiGet<MarginReport>("/api/v1/finance/reports/margin", { from, to }),

  marginByProduct: (from: string, to: string, limit?: number) =>
    apiGet<ProductMarginRow[]>("/api/v1/finance/reports/margin-by-product", {
      from,
      to,
      limit: limit && limit > 0 ? limit : undefined,
    }),

  receivables: (asOf?: string) =>
    apiGet<PartyBalanceReport>("/api/v1/finance/reports/receivables", {
      asOf: asOf ? asOf : undefined,
    }),

  payables: (asOf?: string) =>
    apiGet<PartyBalanceReport>("/api/v1/finance/reports/payables", {
      asOf: asOf ? asOf : undefined,
    }),

  /** Ringkasan kas — posisi per rekening is_cash (tanpa tabel kas). */
  cashSummary: (year: number, month: number) =>
    apiGet<CashSummaryReport>("/api/v1/finance/reports/cash-summary", {
      year,
      month,
    }),

  /** Rekonsiliasi fiskal — laba komersil → laba fiskal per rentang. */
  fiscalSummary: (from: string, to: string) =>
    apiGet<FiscalSummaryReport>("/api/v1/finance/reports/fiscal-summary", {
      from,
      to,
    }),

  /**
   * Unduh paket kertas kerja pajak (.xlsx).
   *
   * `variant` adalah saklar tingkat APLIKASI, bukan label dokumen — nama
   * berkas datang dari server dan menggambarkan isinya sendiri, sehingga
   * istilah internal tidak pernah sampai ke tangan konsultan pajak.
   *   "actual" → pembukuan komersial apa adanya
   *   "capped" → peredaran bruto dibatasi plafon tahunan
   */
  downloadTaxPackage: async (
    year: number,
    month: number,
    variant: "actual" | "capped" = "actual",
  ): Promise<void> => {
    const res = await httpClient.get("/api/v1/finance/export/tax-package", {
      params: { year, month, variant },
      responseType: "blob",
    });
    const blob = new Blob([res.data], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileNameFrom(res.headers["content-disposition"])
      ?? `paket-pajak-${year}-${String(month).padStart(2, "0")}.xlsx`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },

  /** Daftar akun untuk SearchSelect buku besar (dipilih berdasarkan kode). */
  accounts: () => apiGet<Account[]>("/api/v1/finance/accounts"),
};
