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

import { apiGet, apiPage, httpClient } from "@/shared/services/http-client";
import type {
  Account,
  AgingReport,
  CashFlowReport,
  BalanceSheetReport,
  CashSummaryReport,
  FiscalSummaryReport,
  PartyBalanceReport,
  ProductMarginRow,
  InventoryValueReport,
  LedgerLine,
  LedgerOverview,
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

/** ?branch= — kosong = cabang sesi; "all" atau id butuh finance.report.all_branches. */
function branchParam(branch?: string): string | undefined {
  return branch && branch !== "" ? branch : undefined;
}

export const financeReportsService = {
  trialBalance: (year: number, month: number, branch?: string) =>
    apiGet<TrialBalanceReport>("/api/v1/finance/reports/trial-balance", {
      year,
      month,
      branch: branchParam(branch),
    }),

  /** Neraca saldo kumulatif sampai tanggal (?asOf=YYYY-MM-DD). */
  trialBalanceAsOf: (asOf: string, branch?: string) =>
    apiGet<TrialBalanceReport>("/api/v1/finance/reports/trial-balance", {
      asOf,
      branch: branchParam(branch),
    }),

  profitLoss: (from: string, to: string, branch?: string) =>
    apiGet<ProfitLossReport>("/api/v1/finance/reports/profit-loss", {
      from,
      to,
      branch: branchParam(branch),
    }),

  balanceSheet: (date: string, branch?: string) =>
    apiGet<BalanceSheetReport>("/api/v1/finance/reports/balance-sheet", {
      date,
      branch: branchParam(branch),
    }),

  /** Buku besar berpaginasi {items, meta}; halaman 1 memuat baris Saldo awal. */
  generalLedger: (
    account: string,
    from: string,
    to: string,
    page = 1,
    limit = 200,
    branch?: string,
  ) =>
    apiPage<LedgerLine>("/api/v1/finance/reports/general-ledger", {
      account,
      from,
      to,
      page,
      limit,
      branch: branchParam(branch),
    }),

  taxSummary: (year: number, month: number, branch?: string, page = 1, limit = 20) =>
    apiGet<TaxSummary>("/api/v1/finance/reports/tax-summary", {
      year,
      month,
      page,
      limit,
      branch: branchParam(branch),
    }),

  /** Buku besar semua akun (tanpa ?account=): saldo awal, mutasi, saldo akhir per akun (FN-08). */
  generalLedgerOverview: async (from: string, to: string, page = 1, limit = 100, branch?: string) => {
    const res = await httpClient.get<{
      data: LedgerOverview;
      meta: { page: number; limit: number; total: number; totalPages: number };
    }>("/api/v1/finance/reports/general-ledger", { params: { from, to, page, limit, branch: branchParam(branch) } });
    return { ...res.data.data, meta: res.data.meta };
  },

  /** Rincian PPN berpaginasi (kind sales | sales_return | purchase). */
  taxDetail: (year: number, month: number, page = 1, limit = 200, branch?: string) =>
    apiPage<TaxDetailRow>("/api/v1/finance/reports/tax-detail", {
      year,
      month,
      page,
      limit,
      branch: branchParam(branch),
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

  inventoryValue: (branch?: string) =>
    apiGet<InventoryValueReport>("/api/v1/finance/reports/inventory-value", { branch: branchParam(branch) }),

  margin: (from: string, to: string, branch?: string) =>
    apiGet<MarginReport>("/api/v1/finance/reports/margin", {
      from,
      to,
      branch: branchParam(branch),
    }),

  marginByProduct: (from: string, to: string, limit?: number, branch?: string) =>
    apiGet<ProductMarginRow[]>("/api/v1/finance/reports/margin-by-product", {
      from,
      to,
      limit: limit && limit > 0 ? limit : undefined,
      branch: branchParam(branch),
    }),

  receivables: (asOf?: string, branch?: string) =>
    apiGet<PartyBalanceReport>("/api/v1/finance/reports/receivables", {
      asOf: asOf ? asOf : undefined,
      branch: branchParam(branch),
    }),

  payables: (asOf?: string, branch?: string) =>
    apiGet<PartyBalanceReport>("/api/v1/finance/reports/payables", {
      asOf: asOf ? asOf : undefined,
      branch: branchParam(branch),
    }),

  /** Umur per dokumen (?view=documents) — cabang sesi saja. */
  receivableDocuments: (asOf?: string) =>
    apiGet<AgingReport>("/api/v1/finance/reports/receivables", {
      view: "documents",
      asOf: asOf ? asOf : undefined,
    }),

  payableDocuments: (asOf?: string) =>
    apiGet<AgingReport>("/api/v1/finance/reports/payables", {
      view: "documents",
      asOf: asOf ? asOf : undefined,
    }),

  /** Laporan kas rentang: saldo awal, masuk, keluar, akhir per rekening. */
  cashFlow: (from: string, to: string, branch?: string) =>
    apiGet<CashFlowReport>("/api/v1/finance/reports/cash-summary", {
      from,
      to,
      branch: branchParam(branch),
    }),

  /** Ringkasan kas — posisi per rekening is_cash (tanpa tabel kas). */
  cashSummary: (year: number, month: number, branch?: string) =>
    apiGet<CashSummaryReport>("/api/v1/finance/reports/cash-summary", {
      year,
      month,
      branch: branchParam(branch),
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
    branch?: string,
  ): Promise<void> => {
    const res = await httpClient.get("/api/v1/finance/export/tax-package", {
      params: { year, month, variant, branch: branchParam(branch) },
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

  /** Paket pajak untuk rentang tanggal bebas (FN-04); varian "capped" harus dalam satu tahun pajak. */
  downloadTaxPackageRange: async (
    from: string,
    to: string,
    variant: "actual" | "capped" = "actual",
    branch?: string,
  ): Promise<void> => {
    const res = await httpClient.get("/api/v1/finance/export/tax-package", {
      params: { from, to, variant, branch: branchParam(branch) },
      responseType: "blob",
    });
    const blob = new Blob([res.data], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileNameFrom(res.headers["content-disposition"]) ?? `paket-pajak-${from}_${to}.xlsx`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },

  /** Daftar akun untuk SearchSelect buku besar (dipilih berdasarkan kode). */
  accounts: () => apiGet<Account[]>("/api/v1/finance/accounts"),
};
