// Service finance — HANYA via helper http-client. Bentuk endpoint terverifikasi
// dari backend (apps/api/internal/modules/finance/presentation/handler.go +
// RegisterRoutes):
// - GET /api/v1/finance/posting/preview?docType&docId → entryView+lines
//   (perm finance.post; docType valid: goods_receipt|delivery|payment|
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
} from "@/shared/services/http-client";
import type {
  Account,
  AccountMappings,
  BatchPostResult,
  Expense,
  FiscalPeriod,
  JournalEntry,
  PeriodReadiness,
  PostingSource,
} from "@/modules/finance/types";

export interface JournalListParams {
  from?: string;
  to?: string;
  accountId?: number;
  page: number;
  limit: number;
}

function journalListParams(p: JournalListParams): Record<string, unknown> {
  return {
    from: p.from ? p.from : undefined,
    to: p.to ? p.to : undefined,
    accountId: p.accountId && p.accountId > 0 ? p.accountId : undefined,
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

  expenses: (page: number, limit: number) =>
    apiPage<Expense>("/api/v1/finance/expenses", { page, limit }),

  expense: (id: number) =>
    apiGet<Expense>(`/api/v1/finance/expenses/${id}`),

  createExpense: (body: ExpenseCreatePayload) =>
    apiPostFull<Expense>("/api/v1/finance/expenses", body),

  // Tanpa body — backend hanya butuh :id (handler.go: CancelExpense).
  cancelExpense: (id: number) =>
    apiPostFull<Expense>(`/api/v1/finance/expenses/${id}/cancel`),

  periods: () => apiGet<FiscalPeriod[]>("/api/v1/finance/periods"),

  closePeriod: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/periods/close", body),

  reopenPeriod: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/periods/reopen", body),

  taxPeriods: () => apiGet<FiscalPeriod[]>("/api/v1/finance/tax-periods"),

  closeTaxPeriod: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/tax-periods/close", body),

  reopenTaxPeriod: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/tax-periods/reopen", body),

  // GET /api/v1/finance/posting-sources — antrian turunan: dokumen
  // terkonfirmasi yang belum dijurnal (tanpa tabel antrean).
  postingSources: () =>
    apiGet<PostingSource[]>("/api/v1/finance/posting-sources"),

  postBatch: (items: { docType: string; docId: number }[]) =>
    apiPostFull<BatchPostResult[]>("/api/v1/finance/posting/post-batch", {
      items,
    }),

  // POST /api/v1/finance/posting/close-day {date} — posting semua antrean
  // tertanggal ≤ date (tutup harian).
  closeDay: (date: string) =>
    apiPostFull<BatchPostResult[]>("/api/v1/finance/posting/close-day", {
      date,
    }),

  // Saldo awal = satu jurnal cutover per cabang (tanpa tabel saldo awal).
  openingStatus: () =>
    apiGet<JournalEntry | null>("/api/v1/finance/opening"),

  previewOpening: (body: {
    date?: string;
    memo?: string;
    lines: ManualLinePayload[];
  }) => apiPostFull<JournalEntry>("/api/v1/finance/opening/preview", body),

  postOpening: (body: {
    date?: string;
    memo?: string;
    lines: ManualLinePayload[];
  }) => apiPostFull<JournalEntry>("/api/v1/finance/opening/post", body),

  // Koreksi fiskal = jurnal bertipe (tanpa tabel worksheet).
  taxAdjustments: () =>
    apiGet<JournalEntry[]>("/api/v1/finance/tax-adjustments"),

  createTaxAdjustment: (body: {
    date?: string;
    memo: string;
    lines: ManualLinePayload[];
  }) => apiPostFull<JournalEntry>("/api/v1/finance/tax-adjustments", body),

  readiness: (year: number, month: number) =>
    apiGet<PeriodReadiness>("/api/v1/finance/close/readiness", {
      year,
      month,
    }),

  safeClose: (body: PeriodPayload) =>
    apiPostFull<unknown>("/api/v1/finance/close/safe-close", body),
};
