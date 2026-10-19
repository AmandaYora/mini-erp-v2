// Service finance — HANYA via helper http-client. Bentuk endpoint terverifikasi
// dari backend (apps/api/internal/modules/finance/presentation/handler.go +
// RegisterRoutes):
// - GET /api/v1/finance/posting/preview?docType&docId → entryView+lines
//   (perm finance.posting.post; docType valid: goods_receipt|delivery|payment|
//   sales_return|purchase_return — builders.go).
// - POST /api/v1/finance/posting/post {docType, docId} → entryView+lines.
// - GET /api/v1/finance/journals?from&to&accountId&page&limit → paginasi
//   entryView TANPA lines (perm finance.view).
// - POST /api/v1/finance/journals/manual {date?, memo, lines:[{accountCode,
//   debit, credit}]} → entryView+lines (perm finance.manage).
// - GET /api/v1/finance/journals/:id → entryView+lines.
// - POST /api/v1/finance/journals/:id/reverse {reason} → entryView+lines.
// - GET /api/v1/finance/accounts?status → accountView[].
// - POST /api/v1/finance/accounts {code, name, type, isCash} → accountView.
// - PUT /api/v1/finance/accounts/:id {name, type, isCash} (code terkunci).
// - POST /api/v1/finance/accounts/:id/archive (tanpa body).
// - GET /api/v1/finance/account-mappings → OBJEK {key: accountView}.
// - PUT /api/v1/finance/account-mappings {key, accountId}.
// - GET /api/v1/finance/expenses?page&limit → paginasi expenseView.
// - POST /api/v1/finance/expenses {expenseAccountId, payAccountId, amount,
//   date?, notes?} → expenseView.
// - GET /api/v1/finance/expenses/:id → expenseView.
// - POST /api/v1/finance/expenses/:id/cancel (tanpa body) → expenseView.
// - GET /api/v1/finance/periods → [{year, month, status}].
// - POST /api/v1/finance/periods/close|/reopen {year, month}.
// - GET /api/v1/finance/tax-periods → [{year, month, status}].
// - POST /api/v1/finance/tax-periods/close|/reopen {year, month}.

import {
  apiGet,
  apiPage,
  apiPostFull,
  apiPutFull,
  apiUploadFull,
} from "@/shared/services/http-client";
import type {
  Account,
  AccountMappings,
  BatchItem,
  BatchPostResult,
  CloseDayResult,
  CutoverMap,
  DailyOverview,
  Expense,
  FiscalPeriod,
  IntegrityReport,
  JournalEntry,
  OpeningInventory,
  OpeningPreview,
  PeriodReadiness,
  PostingSource,
  TaxAdjustment,
  TaxAdjustmentFile,
  UncostedItem,
} from "@/modules/finance/types";

export interface JournalListParams {
  from?: string;
  to?: string;
  accountId?: number;
  q?: string;
  sourceType?: string;
  status?: string;
  page: number;
  limit: number;
}

export interface ExpenseListParams {
  /** Pihak lawan (FN-10). */
  partyId?: number;
  q?: string;
  from?: string;
  to?: string;
  status?: string;
  expenseAccountId?: number;
  payAccountId?: number;
  page: number;
  limit: number;
}

function journalListParams(p: JournalListParams): Record<string, unknown> {
  return {
    from: p.from ? p.from : undefined,
    to: p.to ? p.to : undefined,
    accountId: p.accountId && p.accountId > 0 ? p.accountId : undefined,
    q: p.q?.trim() ? p.q.trim() : undefined,
    sourceType: p.sourceType ? p.sourceType : undefined,
    status: p.status ? p.status : undefined,
    page: p.page,
    limit: p.limit,
  };
}

export interface ManualLinePayload {
  accountCode: string;
  debit: number;
  credit: number;
}

export interface ExpenseCreatePayload {
  expenseAccountId: number;
  payAccountId: number;
  amount: number;
  date?: string;
  notes?: string;
  partyId?: number;
}

export interface ReopenPeriodPayload extends PeriodPayload {
  reason: string;
}

export interface PeriodPayload {
  year: number;
  month: number;
}

export const financeService = {
  preview: (docType: string, docId: number) =>
    apiGet<JournalEntry>("/api/v1/finance/posting/preview", {
      docType,
      docId,
    }),

  post: (docType: string, docId: number) =>
    apiPostFull<JournalEntry>("/api/v1/finance/posting/post", { docType, docId }),

  journals: (p: JournalListParams) =>
    apiPage<JournalEntry>("/api/v1/finance/journals", journalListParams(p)),

  journal: (id: number) =>
    apiGet<JournalEntry>(`/api/v1/finance/journals/${id}`),

  reverse: (id: number, reason: string) =>
    apiPostFull<JournalEntry>(`/api/v1/finance/journals/${id}/reverse`, {
      reason,
    }),

  manual: (body: { date?: string; memo: string; lines: ManualLinePayload[] }) =>
    apiPostFull<JournalEntry>("/api/v1/finance/journals/manual", body),

  accounts: (status?: string) =>
    apiGet<Account[]>("/api/v1/finance/accounts", {
      status: status ? status : undefined,
    }),

  createAccount: (body: {
    code: string;
    name: string;
    type: string;
    isCash: boolean;
  }) => apiPostFull<Account>("/api/v1/finance/accounts", body),

  updateAccount: (
    id: number,
    body: { name: string; type: string; isCash: boolean },
  ) => apiPutFull<Account>(`/api/v1/finance/accounts/${id}`, body),

  // Tanpa body — backend hanya butuh :id (handler.go: ArchiveAccount).
  archiveAccount: (id: number) =>
    apiPostFull<unknown>(`/api/v1/finance/accounts/${id}/archive`),

  mappings: () =>
    apiGet<AccountMappings>("/api/v1/finance/account-mappings"),

  setMapping: (key: string, accountId: number) =>
    apiPutFull<unknown>("/api/v1/finance/account-mappings", { key, accountId }),

  expenses: (p: ExpenseListParams) =>
    apiPage<Expense>("/api/v1/finance/expenses", {
      q: p.q?.trim() ? p.q.trim() : undefined,
      from: p.from ? p.from : undefined,
      to: p.to ? p.to : undefined,
      status: p.status ? p.status : undefined,
      expenseAccountId:
        p.expenseAccountId && p.expenseAccountId > 0 ? p.expenseAccountId : undefined,
      payAccountId: p.payAccountId && p.payAccountId > 0 ? p.payAccountId : undefined,
      partyId: p.partyId && p.partyId > 0 ? p.partyId : undefined,
      page: p.page,
      limit: p.limit,
    }),

  expense: (id: number) =>
    apiGet<Expense>(`/api/v1/finance/expenses/${id}`),

  createExpense: (body: ExpenseCreatePayload) =>
    apiPostFull<Expense>("/api/v1/finance/expenses", body),

  // Alasan wajib (maks 255) — handler.go: CancelExpense {reason}.
  cancelExpense: (id: number, reason: string) =>
    apiPostFull<Expense>(`/api/v1/finance/expenses/${id}/cancel`, { reason }),

  periods: () => apiGet<FiscalPeriod[]>("/api/v1/finance/periods"),

  closePeriod: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/periods/close", body),

  // Buka kembali wajib beralasan (FN-05): alasan masuk diff audit.
  reopenPeriod: (body: ReopenPeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/periods/reopen", body),

  taxPeriods: () => apiGet<FiscalPeriod[]>("/api/v1/finance/tax-periods"),

  closeTaxPeriod: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/tax-periods/close", body),

  reopenTaxPeriod: (body: ReopenPeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/tax-periods/reopen", body),

  // GET /api/v1/finance/posting-sources — antrian turunan: dokumen
  // terkonfirmasi yang belum dijurnal (tanpa tabel antrean).
  postingSources: () =>
    apiGet<PostingSource[]>("/api/v1/finance/posting-sources"),

  // GET /finance/posting/overview?date= — ikhtisar tutup hari (FN-02).
  dailyOverview: (date?: string) =>
    apiGet<DailyOverview>("/api/v1/finance/posting/overview", { date: date || undefined }),

  // FN-01: abaikan dokumen macet (alasan wajib) / pulihkan ke antrian.
  ignorePosting: (docType: string, docId: number, reason: string) =>
    apiPostFull<PostingSource>("/api/v1/finance/posting/ignore", { docType, docId, reason }),

  restorePosting: (docType: string, docId: number) =>
    apiPostFull<unknown>("/api/v1/finance/posting/restore", { docType, docId }),

  // Item membawa action "post" (default) atau "reverse" (jurnal yatim);
  // maksimal 50 per panggilan, kegagalan satu item tidak menghentikan lainnya.
  postBatch: (items: BatchItem[]) =>
    apiPostFull<BatchPostResult[]>("/api/v1/finance/posting/post-batch", {
      items,
    }),

  // POST /api/v1/finance/posting/close-day {date} — posting semua antrean
  // tertanggal ≤ date (tutup harian).
  closeDay: (date: string) =>
    apiPostFull<CloseDayResult>("/api/v1/finance/posting/close-day", {
      date,
    }),

  // GET /api/v1/finance/posting/orphans — jurnal hidup milik dokumen yang
  // sudah dibatalkan (perlu dibalik via postBatch action "reverse").
  orphans: () => apiGet<PostingSource[]>("/api/v1/finance/posting/orphans"),

  // GET/PUT /api/v1/finance/cutover — batas per jenis dokumen: dokumen dengan
  // id <= coveredThroughId dianggap sudah dijurnal di tempat lain.
  cutover: () => apiGet<CutoverMap>("/api/v1/finance/cutover"),

  setCutover: (docType: string, coveredThroughId: number) =>
    apiPutFull<unknown>("/api/v1/finance/cutover", {
      docType,
      coveredThroughId,
    }),

  // Biaya (harga modal): produk yang belum berbiaya + sinkronisasi ledger.
  uncosted: () =>
    apiGet<UncostedItem[]>("/api/v1/finance/costing/uncosted"),

  costingSync: () => apiPostFull<unknown>("/api/v1/finance/costing/sync"),

  // Saldo awal = satu jurnal cutover per cabang (tanpa tabel saldo awal).
  openingStatus: () =>
    apiGet<JournalEntry | null>("/api/v1/finance/opening"),

  previewOpening: (body: {
    date?: string;
    memo?: string;
    lines: ManualLinePayload[];
  }) => apiPostFull<OpeningPreview>("/api/v1/finance/opening/preview", body),

  // Persediaan saldo awal dihitung server dari stok awal x harga modal.
  openingInventory: () =>
    apiGet<OpeningInventory>("/api/v1/finance/opening/inventory"),

  // Lengkapi harga modal produk berstok setelah saldo awal dikunci.
  completeOpening: (body: {
    items: { productId: number; variantId: number; unitCost?: number }[];
  }) => apiPostFull<JournalEntry & { total: number }>("/api/v1/finance/opening/complete", body),

  postOpening: (body: {
    date?: string;
    memo?: string;
    lines: ManualLinePayload[];
  }) => apiPostFull<JournalEntry>("/api/v1/finance/opening/post", body),

  // Koreksi fiskal = jurnal bertipe (tanpa tabel worksheet).
  taxAdjustments: () =>
    apiGet<TaxAdjustment[]>("/api/v1/finance/tax-adjustments"),

  createTaxAdjustment: (body: {
    date?: string;
    memo: string;
    lines: ManualLinePayload[];
    classification: string;
    note?: string;
  }) => apiPostFull<TaxAdjustment>("/api/v1/finance/tax-adjustments", body),

  // FN-07: lampiran (gambar) koreksi fiskal.
  taxAdjustmentFiles: (id: number) =>
    apiGet<TaxAdjustmentFile[]>(`/api/v1/finance/tax-adjustments/${id}/attachments`),

  attachTaxAdjustment: (id: number, file: File) => {
    const form = new FormData();
    form.append("file", file);
    return apiUploadFull<TaxAdjustmentFile>(`/api/v1/finance/tax-adjustments/${id}/attachments`, form);
  },

  // GET /finance/integrity — rollup vs baris jurnal (perm finance.manage).
  integrity: () => apiGet<IntegrityReport>("/api/v1/finance/integrity"),

  // POST /finance/integrity/rebuild — hitung ulang rollup harian.
  rebuildIntegrity: () =>
    apiPostFull<unknown>("/api/v1/finance/integrity/rebuild"),

  readiness: (year: number, month: number) =>
    apiGet<PeriodReadiness>("/api/v1/finance/close/readiness", {
      year,
      month,
    }),

  safeClose: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/close/safe-close", body),
};
