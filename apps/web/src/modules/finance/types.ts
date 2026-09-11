// Tipe finance — kunci camelCase terverifikasi dari backend
// (apps/api/internal/modules/finance/presentation/handler.go):
// - accountView: {id, code, name, type, isCash, status}
// - entryView: {id, number, branchId, date, memo, sourceType, sourceId,
//   status, reversedBy, lines?[{accountId, accountCode, accountName,
//   debit, credit}]}. List (withLines=false) TANPA lines/total.
// - expenseView: {id, number, branchId, expenseAccountId, payAccountId,
//   amount, date, notes, journalEntryId, status}
// - periods/tax-periods: [{year, month, status}]
// - mappings: OBJEK {key: accountView}, bukan array.

export interface Account {
  id: number;
  code: string;
  name: string;
  type: string;
  isCash: boolean;
  status: string;
}

export interface JournalLine {
  accountId: number;
  accountCode: string;
  accountName: string;
  debit: number;
  credit: number;
}

export interface JournalEntry {
  id: number;
  number: string;
  branchId: number;
  date: string;
  memo: string;
  sourceType: string;
  sourceId: number;
  status: string;
  reversedBy: number;
  lines?: JournalLine[];
}

export interface Expense {
  id: number;
  number: string;
  branchId: number;
  expenseAccountId: number;
  payAccountId: number;
  amount: number;
  date: string;
  notes: string;
  journalEntryId: number;
  status: string;
}

export interface FiscalPeriod {
  year: number;
  month: number;
  status: string;
}

export type AccountMappings = Record<string, Account>;

// Tipe akun valid (contracts/client.go): aset | kewajiban | modal |
// pendapatan | beban.
export const ACCOUNT_TYPES = [
  "aset",
  "kewajiban",
  "modal",
  "pendapatan",
  "beban",
] as const;

export type AccountType = (typeof ACCOUNT_TYPES)[number];

export function accountTypeLabel(t: string): string {
  switch (t) {
    case "aset":
      return "Aset";
    case "kewajiban":
      return "Kewajiban";
    case "modal":
      return "Modal";
    case "pendapatan":
      return "Pendapatan";
    case "beban":
      return "Beban";
    default:
      return t || "-";
  }
}

// Kunci pemetaan valid (application/service.go: mappingKeys).
export const MAPPING_KEYS = [
  "cash",
  "bank",
  "receivable",
  "inventory",
  "ppn_in",
  "payable",
  "ppn_out",
  "revenue",
  "sales_return",
  "cogs",
  "expense",
  "writeoff",
  "adjustment",
] as const;

export function mappingKeyLabel(key: string): string {
  switch (key) {
    case "cash":
      return "Kas Tunai";
    case "bank":
      return "Bank";
    case "receivable":
      return "Piutang Usaha";
    case "inventory":
      return "Persediaan";
    case "ppn_in":
      return "PPN Masukan";
    case "payable":
      return "Hutang Usaha";
    case "ppn_out":
      return "PPN Keluaran";
    case "revenue":
      return "Penjualan";
    case "sales_return":
      return "Retur Penjualan";
    case "cogs":
      return "HPP";
    case "expense":
      return "Beban Operasional";
    case "writeoff":
      return "Beban Hapus Stok";
    case "adjustment":
      return "Penyesuaian Persediaan";
    default:
      return key;
  }
}

// docType yang diterima buildLines (application/builders.go). business_expense,
// manual, dan reversal DITOLAK ("tipe dokumen tidak dikenal").
export const POSTING_DOC_TYPES = [
  "goods_receipt",
  "delivery",
  "payment",
  "sales_return",
  "purchase_return",
] as const;

export function docTypeLabel(t: string): string {
  switch (t) {
    case "goods_receipt":
      return "Penerimaan Barang";
    case "delivery":
      return "Pengiriman";
    case "payment":
      return "Pembayaran";
    case "sales_return":
      return "Retur Jual";
    case "purchase_return":
      return "Retur Beli";
    case "business_expense":
      return "Biaya";
    case "manual":
      return "Manual";
    case "reversal":
      return "Pembalik";
    default:
      return t || "-";
  }
}

export function journalSourceLabel(e: Pick<JournalEntry, "sourceType" | "sourceId">): string {
  if (!e.sourceType) return "Manual";
  const base = docTypeLabel(e.sourceType);
  return e.sourceId > 0 ? `${base} #${e.sourceId}` : base;
}

const MONTH_SHORT = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "Mei",
  "Jun",
  "Jul",
  "Agu",
  "Sep",
  "Okt",
  "Nov",
  "Des",
];

export function periodLabel(year: number, month: number): string {
  const m = MONTH_SHORT[month - 1] ?? String(month);
  return `${m} ${year}`;
}

export function entryTotals(lines: JournalLine[] | undefined): {
  debit: number;
  credit: number;
} {
  let debit = 0;
  let credit = 0;
  for (const l of lines ?? []) {
    debit += l.debit;
    credit += l.credit;
  }
  return { debit, credit };
}

// --- Tipe laporan (presentation/reports.go + application/reports.go).
// Semua kunci camelCase, uang integer rupiah. ---

export interface TrialRow {
  code: string;
  name: string;
  type: string;
  debit: number;
  credit: number;
}

/** GET reports/trial-balance?year=&month= → {rows,totalDebit,totalCredit,balanced}. */
export interface TrialBalanceReport {
  rows: TrialRow[];
  totalDebit: number;
  totalCredit: number;
  balanced: boolean;
}

/** GET reports/profit-loss?from=&to= (YYYY-MM-DD). */
export interface ProfitLossReport {
  revenue: number;
  cogs: number;
  gross: number;
  expense: number;
  profit: number;
  from: string;
  to: string;
}

/** GET reports/balance-sheet?date= (YYYY-MM-DD). Baris memakai debit=saldo. */
export interface BalanceSheetReport {
  assets: TrialRow[];
  liabilities: TrialRow[];
  equity: TrialRow[];
  totalAssets: number;
  totalLiabilitiesEquity: number;
  balanced: boolean;
  date: string;
}

/** GET reports/general-ledger?account=&from=&to= → array langsung. */
export interface LedgerLine {
  entryId: number;
  number: string;
  date: string;
  memo: string;
  debit: number;
  credit: number;
  balance: number;
}

/** GET reports/tax-summary?year=&month=. */
export interface TaxSummary {
  year: number;
  month: number;
  ppnOut: number;
  ppnIn: number;
  payable: number;
}

/** GET reports/tax-detail?year=&month= → array langsung. */
export interface TaxDetailRow {
  date: string;
  number: string;
  memo: string;
  revenue: number;
  ppn: number;
  // Hasil verifikasi Tahap C (JANGAN ditebak ulang): baris membawa
  // taxInvoiceNumber + taxInvoiceDate (finance/presentation/reports.go,
  // JSON + CSV sudah menyajikan keduanya).
  taxInvoiceNumber: string;
  taxInvoiceDate: string;
}

/** GET reports/inventory-value (tanpa param) → {positions,total}. */
export interface InventoryPosition {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  qty: number;
  avgCost: number;
  value: number;
}

export interface InventoryValueReport {
  positions: InventoryPosition[];
  total: number;
}

/** GET reports/margin?from=&to= (YYYY-MM-DD). */
export interface MarginReport {
  revenue: number;
  cogs: number;
  gross: number;
  percent: number;
}

/**
 * GET reports/margin-by-product?from=&to=[&limit=] — satu baris per posisi
 * produk/varian, terurut marjin kotor terbesar. Angka berasal dari BUKU
 * (dimensi produk pada baris jurnal), bukan dari tabel order.
 */
export interface ProductMarginRow {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  revenue: number;
  cogs: number;
  gross: number;
  percent: number;
}

/** Satu lawan transaksi dengan saldo terbuka pada akun kontrol. */
export interface PartyBalance {
  partyId: number;
  partyCode: string;
  partyName: string;
  debit: number;
  credit: number;
  /** Selalu positif = masih terutang; yang lunas tidak ikut terkirim. */
  balance: number;
}

/** GET reports/receivables|payables[?asOf=]. */
export interface PartyBalanceReport {
  parties: PartyBalance[];
  total: number;
}

/** "12,50%" — persen gaya Indonesia. */
export function formatPercent(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return "-";
  return `${n.toFixed(2).replace(".", ",")}%`;
}

/** GET /finance/posting-sources — dokumen terkonfirmasi yang belum dijurnal. */
export interface PostingSource {
  docType: string;
  docId: number;
  number: string;
  date: string;
}

/** POST /finance/posting/post-batch & /close-day — hasil per dokumen. */
export interface BatchPostResult {
  docType: string;
  docId: number;
  entryId?: number;
  entryNumber?: string;
  error?: string;
}

/** GET /finance/close/readiness — daftar periksa pra-tutup periode. */
export interface PeriodReadiness {
  year: number;
  month: number;
  closed: boolean;
  unposted: number;
  unbalanced: number;
  missingTaxInvoice: number;
  missingSupplierInvoice: number;
  ready: boolean;
  blockers: string[];
}

/** Satu rekening kas/bank pada ringkasan kas. */
export interface CashRow {
  code: string;
  name: string;
  debit: number;
  credit: number;
  balance: number;
}

/** GET /finance/reports/cash-summary. */
export interface CashSummaryReport {
  rows: CashRow[];
  total: number;
}

/** Satu baris koreksi pada rekonsiliasi fiskal. */
export interface FiscalLine {
  code: string;
  name: string;
  type: string;
  debit: number;
  credit: number;
  effect: number;
}

/** GET /finance/reports/fiscal-summary — komersil → fiskal. */
export interface FiscalSummaryReport {
  commercial: ProfitLossReport;
  lines: FiscalLine[];
  fiscalProfit: number;
  from: string;
  to: string;
}
