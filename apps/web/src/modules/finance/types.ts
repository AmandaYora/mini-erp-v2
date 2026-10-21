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
  // Dimensi baris (F2b): produk/varian/pihak + keterangan; 0/"" bila tidak ada.
  productId?: number;
  variantId?: number;
  partyId?: number;
  description?: string;
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
  // Alasan pembatalan (wajib saat batal); kosong bila belum dibatalkan.
  cancelReason?: string;
  /** Pihak lawan opsional (FN-10). */
  partyId?: number;
  partyName?: string;
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

// Kunci pemetaan valid (application/service.go: mappingKeys) — 18 kunci.
export const MAPPING_KEYS = [
  "cash",
  "bank",
  "receivable",
  "inventory",
  "ppn_in",
  "payable",
  "ppn_out",
  "customer_advance",
  "supplier_advance",
  "opening_equity",
  "revenue",
  "sales_return",
  "cogs",
  "expense",
  "writeoff",
  "adjustment",
  "rounding_difference",
  "purchase_return_difference",
] as const;

const MAPPING_LABELS: Record<string, string> = {
  cash: "Kas Tunai",
  bank: "Bank",
  receivable: "Piutang Usaha",
  inventory: "Persediaan",
  ppn_in: "PPN Masukan",
  payable: "Hutang Usaha",
  ppn_out: "PPN Keluaran",
  customer_advance: "Uang Muka Pelanggan",
  supplier_advance: "Uang Muka Supplier",
  opening_equity: "Modal Awal / Saldo Awal",
  revenue: "Penjualan",
  sales_return: "Retur Penjualan",
  cogs: "HPP",
  expense: "Beban Operasional",
  writeoff: "Beban Hapus Stok",
  adjustment: "Penyesuaian Persediaan",
  rounding_difference: "Pembulatan / Selisih Posting",
  purchase_return_difference: "Selisih Retur Pembelian",
};

export function mappingKeyLabel(key: string): string {
  return MAPPING_LABELS[key] ?? key;
}

// Tipe akun yang diizinkan per kunci (service.go: mappingTypes) — dipakai
// untuk menyaring pilihan akun di MappingModal; backend tetap memvalidasi.
export const MAPPING_ACCOUNT_TYPES: Record<string, readonly AccountType[]> = {
  cash: ["aset"],
  bank: ["aset"],
  receivable: ["aset"],
  inventory: ["aset"],
  ppn_in: ["aset"],
  payable: ["kewajiban"],
  ppn_out: ["kewajiban"],
  revenue: ["pendapatan"],
  sales_return: ["pendapatan"],
  cogs: ["beban"],
  expense: ["beban"],
  writeoff: ["beban"],
  adjustment: ["beban"],
  customer_advance: ["kewajiban"],
  supplier_advance: ["aset"],
  opening_equity: ["modal"],
  rounding_difference: ["beban"],
  purchase_return_difference: ["beban"],
};

/** Kunci pemetaan yang belum menunjuk akun aktif. */
export function missingMappingKeys(maps: AccountMappings): string[] {
  return MAPPING_KEYS.filter((k) => {
    const a = maps[k];
    return !a || a.status !== "active";
  });
}

// docType yang diterima buildLines (application/builders.go). business_expense,
// manual, dan reversal DITOLAK ("tipe dokumen tidak dikenal").
export const POSTING_DOC_TYPES = [
  "goods_receipt",
  "delivery",
  "payment",
  "sales_return",
  "purchase_return",
  "sales_return_settlement",
  "purchase_return_settlement",
  "stock_movement",
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
    case "sales_return_settlement":
      return "Saldo Customer (Retur Jual)";
    case "purchase_return_settlement":
      return "Saldo Supplier (Retur Beli)";
    case "stock_movement":
      return "Koreksi/Hapus Stok";
    case "business_expense":
      return "Biaya";
    case "opening":
      return "Saldo Awal";
    case "tax_adjustment":
      return "Koreksi Fiskal";
    case "opening_supplement":
      return "Lengkapi Saldo Awal";
    case "cost_restatement":
      return "Hitung Ulang HPP";
    case "inventory_revaluation":
      return "Revaluasi Harga Modal";
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

/** GET reports/trial-balance?year=&month= atau ?asOf=YYYY-MM-DD (kumulatif) →
 * {rows,totalDebit,totalCredit,balanced}. */
export interface TrialBalanceReport {
  rows: TrialRow[];
  totalDebit: number;
  totalCredit: number;
  balanced: boolean;
}

/** GET reports/profit-loss?from=&to= (YYYY-MM-DD). */
/** Satu akun pada rincian laba rugi (FN-03); jumlah satu golongan = angka ringkasannya. */
export interface PLLine {
  code: string;
  name: string;
  amount: number;
}

export interface ProfitLossReport {
  revenue: number;
  cogs: number;
  gross: number;
  expense: number;
  profit: number;
  revenueLines?: PLLine[];
  cogsLines?: PLLine[];
  expenseLines?: PLLine[];
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

/** GET reports/general-ledger?account=&from=&to=&page=&limit= → {data, meta};
 * halaman 1 diawali baris {entryId:0, memo:"Saldo awal"} bila saldo awal tidak nol. */
export interface LedgerLine {
  entryId: number;
  number: string;
  date: string;
  memo: string;
  debit: number;
  credit: number;
  balance: number;
  /** Dimensi baris (FN-08). */
  partyName?: string;
  productName?: string;
  description?: string;
}

/** Satu akun pada buku besar semua akun (FN-08); saldo menurut sisi normal akun. */
export interface LedgerAccountRow {
  code: string;
  name: string;
  type: string;
  normal: "debit" | "credit";
  opening: number;
  debit: number;
  credit: number;
  closing: number;
}

/** GET reports/general-ledger tanpa ?account= → {rows,totalDebit,totalCredit} + meta. */
export interface LedgerOverview {
  rows: LedgerAccountRow[];
  totalDebit: number;
  totalCredit: number;
}

/** Transaksi kena pajak tanpa nomor faktur (FN-06). */
export interface TaxException {
  orderId: number;
  orderNumber: string;
  kind: "sales" | "purchase" | string;
  date: string;
  partyName: string;
  dpp: number;
  ppn: number;
}

/** GET reports/tax-summary?year=&month=[&page=&limit=]. */
export interface TaxSummary {
  year: number;
  month: number;
  ppnOut: number;
  ppnIn: number;
  payable: number;
  salesDpp?: number;
  purchaseDpp?: number;
  exceptions?: TaxException[];
  exceptionsMeta?: { page: number; limit: number; total: number; totalPages: number };
}

/** Jenis baris rincian PPN: keluaran jual, retur jual, masukan beli, retur beli. */
export type TaxDetailKind = "sales" | "sales_return" | "purchase" | "purchase_return";

/** GET reports/tax-detail?year=&month=&page=&limit= → {data, meta} (berpaginasi). */
export interface TaxDetailRow {
  entryId: number;
  kind: TaxDetailKind | string;
  /** PPN masukan (pembelian) — terpisah dari ppn (keluaran). */
  ppnIn: number;
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
  /** FN-06: DPP baris kena pajak, pesanan induk, pihak, dan penanda tanpa faktur. */
  dpp?: number;
  orderId?: number;
  orderNumber?: string;
  partyName?: string;
  missingInvoice?: boolean;
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

/** Pecahan per cabang (FN-09). */
export interface BranchValue {
  branchId: number;
  branchName: string;
  total: number;
  positions: number;
}

export interface InventoryValueReport {
  positions: InventoryPosition[];
  total: number;
  byBranch?: BranchValue[];
}

/** GET reports/margin?from=&to= (YYYY-MM-DD). */
export interface BranchMargin {
  branchId: number;
  branchName: string;
  revenue: number;
  cogs: number;
  gross: number;
  percent: number;
}

export interface MarginReport {
  revenue: number;
  cogs: number;
  gross: number;
  percent: number;
  /** Diisi untuk cakupan semua cabang (FN-09). */
  byBranch?: BranchMargin[];
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

/**
 * GET /finance/posting-sources & /finance/posting/orphans — satu item antrean.
 * periodClosed: dokumen jatuh di bulan yang sudah ditutup; tetap tampil
 * (peringatan) tetapi TIDAK ikut posting massal / tutup harian.
 */
export interface PostingSource {
  docType: string;
  docId: number;
  number: string;
  date: string;
  periodClosed: boolean;
  /** FN-01: sengaja diabaikan dengan alasan; tidak memblokir tutup periode. */
  ignored?: boolean;
  ignoreReason?: string;
  ignoredAt?: string;
  /** FN-10: nominal, pihak, ringkasan. amountKind "order" = total pesanan induk. */
  amount?: number;
  amountKind?: "document" | "order" | string;
  partyId?: number;
  partyName?: string;
  summary?: string;
}

/** Status hari pada ikhtisar tutup hari (FN-02). */
export type DayStatus = "kosong" | "belum" | "selesai_catatan" | "lengkap";

export interface OverviewTaskItem {
  docType: string;
  docId: number;
  number: string;
  date: string;
  amount: number;
  partyName?: string;
  message: string;
}

export interface OverviewTask {
  code: string;
  label: string;
  hint: string;
  count: number;
  items: OverviewTaskItem[];
}

/** GET /finance/posting/overview?date= (FN-02). */
export interface DailyOverview {
  date: string;
  status: DayStatus;
  activity: number;
  totals: {
    sales: { amount: number; count: number };
    purchases: { amount: number; count: number };
    returns: { amount: number; count: number };
    cashIn: number;
    cashOut: number;
  };
  ready: { today: number; backlog: number };
  tasks: OverviewTask[];
  backlog: { count: number; ready: number; needsInput: number; tasks: OverviewTask[] };
  ignored: number;
  dataWarning: { invalidDateCount: number };
  probed: number;
  probeCapped: boolean;
}

/**
 * Aksi satu item batch: post (default), reverse (jurnal yatim) atau catchup
 * (jurnalkan dokumen bulan tertutup ke periode terbuka, ditandai Susulan;
 * butuh izin finance.close.force).
 */
export type BatchAction = "post" | "reverse" | "catchup";

export interface BatchItem {
  docType: string;
  docId: number;
  action?: BatchAction;
}

/** POST /finance/posting/post-batch & /close-day — hasil per dokumen. */
export interface BatchPostResult {
  docType: string;
  docId: number;
  entryId?: number;
  entryNumber?: string;
  error?: string;
}

/** POST /finance/posting/close-day — data: hasil per dokumen + sisa antrean (N-52). */
export interface CloseDayResult {
  results: BatchPostResult[];
  posted: number;
  failed: number;
  /** Dokumen layak-tutup yang MASIH antre (gagal atau melewati batas 500 per panggilan). */
  remaining: number;
}

/** Batas satu panggilan batch (application.MaxBatch). */
export const MAX_BATCH = 50;

/** GET /finance/cutover → {docType: coveredThroughId}. */
export type CutoverMap = Record<string, number>;

/** Jenis dokumen yang punya batas cutover (SetCutover.valid). */
export const CUTOVER_DOC_TYPES = [
  "delivery",
  "goods_receipt",
  "payment",
  "sales_return",
  "purchase_return",
  "sales_return_settlement",
  "purchase_return_settlement",
  "stock_movement",
] as const;

export type CheckStatus = "ok" | "warning" | "failed";

/** Satu gerbang pada daftar periksa tutup periode. */
export interface ReadinessCheck {
  key: string;
  label: string;
  status: CheckStatus;
  detail: string;
  count: number;
}

export function checkLabel(s: CheckStatus): string {
  return s === "ok" ? "Lolos" : s === "warning" ? "Peringatan" : "Gagal";
}

export function checkTone(s: CheckStatus): "success" | "warning" | "danger" {
  return s === "ok" ? "success" : s === "warning" ? "warning" : "danger";
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
  latePeriodDocs: number;
  orphans: number;
  checks: ReadinessCheck[];
  ready: boolean;
  blockers: string[];
  warnings: string[];
}

/** GET /finance/costing/uncosted — produk yang belum punya harga modal. */
export interface UncostedItem {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
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
export interface FiscalClass {
  classification: string;
  label: string;
  entries: number;
  effect: number;
}

export interface FiscalSummaryReport {
  commercial: ProfitLossReport;
  lines: FiscalLine[];
  classes?: FiscalClass[];
  fiscalProfit: number;
  from: string;
  to: string;
}

/** Satu rekening kas pada laporan kas rentang (?from&to). */
export interface CashFlowRow {
  code: string;
  name: string;
  opening: number;
  in: number;
  out: number;
  closing: number;
}

/** GET /finance/reports/cash-summary?from=&to= → saldo awal + masuk - keluar = akhir. */
export interface CashFlowReport {
  rows: CashFlowRow[];
  openingTotal: number;
  closingTotal: number;
}

/** Satu dokumen terbuka pada umur piutang/hutang (?view=documents). */
export interface AgingDoc {
  orderId: number;
  number: string;
  partyId: number;
  partyName: string;
  orderDate: string;
  dueDate: string;
  grandTotal: number;
  paid: number;
  outstanding: number;
  ageDays: number;
  overdueDays: number;
}

export interface AgingBuckets {
  d0_30: number;
  d31_60: number;
  d61_90: number;
  over90: number;
}

/**
 * GET reports/receivables|payables?view=documents[&asOf=]. Dokumen bertanggal ≤ asOf dikurangi
 * pembayaran bertanggal ≤ asOf. `glBalance` = saldo akun kendali (Piutang/Hutang Usaha) per asOf,
 * semua baris (dengan atau tanpa pihak); `difference` = glBalance − total.
 */
export interface AgingReport {
  asOf: string;
  documents: AgingDoc[];
  buckets: AgingBuckets;
  total: number;
  accountCode: string;
  accountName: string;
  glBalance: number;
  difference: number;
}

/** Selisih rollup harian vs baris jurnal. */
export interface RollupDiff {
  accountId: number;
  date: string;
  scope: number;
  linesDebit: number;
  linesCredit: number;
  rollupDebit: number;
  rollupCredit: number;
}

/** GET /finance/integrity (perm finance.manage). */
export interface IntegrityReport {
  rollupDiffs: RollupDiff[];
  unbalanced: number;
  /** Rekonsiliasi akun Persediaan buku vs nilai ledger biaya (K-05). */
  inventory?: InventoryRecon;
  consistent: boolean;
}

/** Akun Persediaan buku dibanding nilai ledger biaya; diff = buku − ledger. */
export interface InventoryRecon {
  book: number;
  ledger: number;
  diff: number;
  tolerance: number;
  /** Dokumen belum dijurnal dan nilai ledger yang sudah mereka gerakkan. */
  pending: number;
  pendingValue: number;
  /** Nilai ledger dokumen yang sengaja diabaikan dari antrian posting. */
  ignoredValue: number;
  /** Selisih setelah dokumen belum dijurnal dan diabaikan; harus ≈ 0. */
  unexplained: number;
  missingCost: number;
  consistent: boolean;
}

/** Posisi berstok yang bernilai nol di ledger biaya. */
export interface MissingCostItem {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  qty: number;
  /** Harga beli katalog per satuan dasar (0 = belum ada). */
  suggestedCost: number;
}

/** GET /finance/opening/inventory — persediaan yang dijurnal server saat saldo awal. */
export interface OpeningInventory {
  total: number;
  positions: number;
  estimated: number;
  missingCost: MissingCostItem[];
  openingPosted: boolean;
}

/** Pratinjau saldo awal = jurnal + ringkasan persediaan terhitung. */
export type OpeningPreview = JournalEntry & { inventory?: OpeningInventory };

export const JOURNAL_STATUSES = ["posted", "reversed"] as const;
export const JOURNAL_SOURCE_TYPES = [
  "goods_receipt",
  "delivery",
  "payment",
  "sales_return",
  "purchase_return",
  "sales_return_settlement",
  "purchase_return_settlement",
  "stock_movement",
  "business_expense",
  "opening",
  "tax_adjustment",
  "manual",
  "reversal",
] as const;

/** Jenis koreksi fiskal (FN-07). */
export const TAX_CLASSES = [
  { value: "positive", label: "Koreksi positif" },
  { value: "negative", label: "Koreksi negatif" },
  { value: "final_income", label: "Penghasilan final" },
  { value: "non_object", label: "Bukan objek pajak" },
] as const;

/** Koreksi fiskal = jurnal bertipe + jenis + catatan (FN-07). */
export interface TaxAdjustment extends JournalEntry {
  classification: string;
  classLabel: string;
  note: string;
}

/** Lampiran (gambar) koreksi fiskal. */
export interface TaxAdjustmentFile {
  id: number;
  name: string;
  mime: string;
  sizeBytes: number;
  url: string;
}

// --- Koreksi harga modal (Hitung Ulang HPP, Revaluasi) ----------------------

/** Satu gerbang hitung ulang HPP (trial_balance | inventory_recon | pending_documents | period_open). */
export interface RestateGate {
  key: string;
  ok: boolean;
  detail: string;
}

/** Selisih satu posisi (produk/varian) hasil hitung ulang HPP. */
export interface RestateProduct {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  rows: number;
  valueBefore: number;
  valueAfter: number;
  /** Sesudah − sebelum (rupiah bulat): positif = Persediaan naik. */
  delta: number;
  /** Delta per kunci pemetaan akun lawan (cogs, adjustment, writeoff, purchase_return_difference). */
  offsets: Record<string, number> | null;
}

export interface RestateSkipped {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  reason: string;
}

/** POST /finance/cost-restatement/preview|commit. */
export interface RestateReport {
  commit: boolean;
  branchId: number;
  positions: number;
  rowsChanged: number;
  products: RestateProduct[] | null;
  skipped: RestateSkipped[] | null;
  delta: number;
  gates: RestateGate[] | null;
  entryId?: number;
  entryNumber?: string;
  reconBefore?: InventoryRecon;
  reconAfter?: InventoryRecon;
  applied: boolean;
  date: string;
  /** Mutasi stok legacy terakhir; baris keluar ≤ batas ini tidak pernah dihitung ulang. 0 = tanpa histori migrasi. */
  cutoverMovementId: number;
}

/** GET /finance/inventory-revaluations/positions?productId= — satu posisi per varian. */
export interface RevaluationPosition {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  variantName: string;
  baseUom: string;
  qty: number;
  value: number;
  averageCost: number;
  /** Harga satuan penerimaan barang terakhir di ledger (0 = belum ada). */
  lastReceiptCost: number;
  /** Harga beli master per satuan dasar (0 = belum diisi). */
  catalogCost: number;
}

export interface RevaluationLine {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  qty: number;
  unitCostBefore: number;
  unitCostAfter: number;
  valueBefore: number;
  valueAfter: number;
  delta: number;
}

/** Pratinjau / hasil komit / baris riwayat revaluasi harga modal. */
export interface RevaluationResult {
  id?: number;
  branchId: number;
  date: string;
  reason: string;
  totalDelta: number;
  lines: RevaluationLine[] | null;
  applied: boolean;
  replayed?: boolean;
  entryId?: number;
  entryNumber?: string;
  createdBy?: number;
  createdAt?: string;
  reconBefore?: InventoryRecon;
  reconAfter?: InventoryRecon;
}
