import { useState } from "react";
import type { FormEvent, ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateInput,
  FilterBar,
  FormField,
  PageHeader,
  SectionCard,
  Segmented,
  SelectInput,
  SummaryCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  formatNumber,
  monthStartWIB,
  todayWIB,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import type {
  InventoryPosition,
  PartyBalance,
  ProductMarginRow,
  TrialRow,
} from "@/modules/finance/types";
import {
  accountTypeLabel,
  formatPercent,
  periodLabel,
} from "@/modules/finance/types";

type Tab =
  | "neraca-saldo"
  | "laba-rugi"
  | "neraca"
  | "pajak"
  | "kas"
  | "persediaan"
  | "marjin"
  | "piutang-hutang";

const TABS: { value: Tab; label: string }[] = [
  { value: "neraca-saldo", label: "Neraca Saldo" },
  { value: "laba-rugi", label: "Laba Rugi" },
  { value: "neraca", label: "Neraca" },
  { value: "pajak", label: "Pajak" },
  { value: "kas", label: "Kas" },
  { value: "persediaan", label: "Persediaan" },
  { value: "marjin", label: "Marjin" },
  { value: "piutang-hutang", label: "Piutang & Hutang" },
];

const MONTHS = [
  "Januari",
  "Februari",
  "Maret",
  "April",
  "Mei",
  "Juni",
  "Juli",
  "Agustus",
  "September",
  "Oktober",
  "November",
  "Desember",
];

function defaultYearMonth(): { year: number; month: number } {
  const today = todayWIB();
  return { year: Number(today.slice(0, 4)), month: Number(today.slice(5, 7)) };
}

function yearOptions(): number[] {
  const y = defaultYearMonth().year;
  return [y, y - 1, y - 2, y - 3, y - 4];
}

function YearMonthFields({
  year,
  month,
  onYear,
  onMonth,
}: {
  year: number;
  month: number;
  onYear: (v: number) => void;
  onMonth: (v: number) => void;
}) {
  return (
    <>
      <div className="min-w-36">
        <FormField label="Tahun">
          <SelectInput
            value={year}
            onChange={(e) => onYear(Number(e.target.value))}
          >
            {yearOptions().map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </SelectInput>
        </FormField>
      </div>
      <div className="min-w-44">
        <FormField label="Bulan">
          <SelectInput
            value={month}
            onChange={(e) => onMonth(Number(e.target.value))}
          >
            {MONTHS.map((label, i) => (
              <option key={label} value={i + 1}>
                {label}
              </option>
            ))}
          </SelectInput>
        </FormField>
      </div>
    </>
  );
}

function TotalsBar({ children }: { children: ReactNode }) {
  return (
    <div className="border-t border-hairline px-5 py-3.5">
      <div className="flex flex-wrap items-center justify-end gap-x-8 gap-y-1 text-sm">
        {children}
      </div>
    </div>
  );
}

// --- Neraca Saldo (?year=&month=) ---

function TrialBalanceSection() {
  const initial = defaultYearMonth();
  const [year, setYear] = useState(initial.year);
  const [month, setMonth] = useState(initial.month);
  const { data: report, loading, error, reload } = useAsyncData(
    () =>
      financeReportsService.trialBalance(year, month).catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat neraca saldo", apiErr.message);
        throw err;
      }),
    [year, month],
  );

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  return (
    <>
      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <YearMonthFields
            year={year}
            month={month}
            onYear={setYear}
            onMonth={setMonth}
          />
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title={`Neraca Saldo — ${periodLabel(year, month)}`}
        actions={
          report ? (
            <Badge tone={report.balanced ? "success" : "danger"}>
              {report.balanced ? "Seimbang" : "Tidak seimbang"}
            </Badge>
          ) : undefined
        }
      >
        {loading && !report ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<TrialRow>
              rows={report?.rows ?? []}
              rowKey={(r) => r.code}
              emptyTitle="Belum ada transaksi"
              emptyDescription="Tidak ada akun yang tersentuh pada periode ini."
              columns={[
                { header: "Kode", render: (r) => r.code },
                { header: "Akun", render: (r) => r.name },
                {
                  header: "Tipe",
                  render: (r) => (
                    <Badge tone="neutral">{accountTypeLabel(r.type)}</Badge>
                  ),
                },
                {
                  header: "Debit",
                  align: "right",
                  render: (r) => formatIDR(r.debit),
                },
                {
                  header: "Kredit",
                  align: "right",
                  render: (r) => formatIDR(r.credit),
                },
              ]}
            />
            {report && report.rows.length > 0 && (
              <TotalsBar>
                <span>
                  Total Debit{" "}
                  <strong>{formatIDR(report.totalDebit)}</strong>
                </span>
                <span>
                  Total Kredit{" "}
                  <strong>{formatIDR(report.totalCredit)}</strong>
                </span>
              </TotalsBar>
            )}
          </>
        )}
      </SectionCard>
    </>
  );
}

// --- Laba Rugi (?from=&to= YYYY-MM-DD) ---

interface PLRow {
  label: string;
  amount: number;
  bold?: boolean;
}

function ProfitLossSection() {
  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const { data: report, loading, error, reload } = useAsyncData(
    () =>
      financeReportsService.profitLoss(from, to).catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat laba rugi", apiErr.message);
        throw err;
      }),
    [from, to],
  );

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  const rows: PLRow[] = report
    ? [
        { label: "Pendapatan", amount: report.revenue },
        { label: "Harga Pokok Penjualan", amount: report.cogs },
        { label: "Laba Kotor", amount: report.gross, bold: true },
        { label: "Beban Operasional", amount: report.expense },
        { label: "Laba Bersih", amount: report.profit, bold: true },
      ]
    : [];

  return (
    <>
      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <div className="min-w-44">
            <FormField label="Dari">
              <DateInput
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
            </FormField>
          </div>
          <div className="min-w-44">
            <FormField label="Sampai">
              <DateInput value={to} onChange={(e) => setTo(e.target.value)} />
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title="Laba Rugi"
        description={
          report
            ? `${formatDate(report.from)} – ${formatDate(report.to)}`
            : undefined
        }
      >
        {loading && !report ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <DataTable<PLRow>
            rows={rows}
            rowKey={(r) => r.label}
            emptyTitle="Belum ada data"
            emptyDescription="Tidak ada pendapatan atau beban pada rentang ini."
            columns={[
              {
                header: "Keterangan",
                render: (r) =>
                  r.bold ? <strong>{r.label}</strong> : r.label,
              },
              {
                header: "Jumlah",
                align: "right",
                render: (r) =>
                  r.bold ? (
                    <strong>{formatIDR(r.amount)}</strong>
                  ) : (
                    formatIDR(r.amount)
                  ),
              },
            ]}
          />
        )}
      </SectionCard>
    </>
  );
}

// --- Neraca (?date= YYYY-MM-DD) ---

function BalanceGroup({
  title,
  rows,
  total,
}: {
  title: string;
  rows: TrialRow[];
  total: number;
}) {
  return (
    <div className="mb-6 last:mb-0">
      <h3 className="mb-2 text-sm font-semibold text-heading">{title}</h3>
      <DataTable<TrialRow>
        rows={rows}
        rowKey={(r) => `${r.code}-${r.name}`}
        emptyTitle={`Belum ada ${title.toLowerCase()}`}
        columns={[
          {
            header: "Kode",
            render: (r) => (r.code ? r.code : "-"),
          },
          { header: "Akun", render: (r) => r.name },
          {
            header: "Saldo",
            align: "right",
            render: (r) => formatIDR(r.debit),
          },
        ]}
      />
      {rows.length > 0 && (
        <TotalsBar>
          <span>
            Total {title} <strong>{formatIDR(total)}</strong>
          </span>
        </TotalsBar>
      )}
    </div>
  );
}

function BalanceSheetSection() {
  const [date, setDate] = useState(todayWIB());
  const { data: report, loading, error, reload } = useAsyncData(
    () =>
      financeReportsService.balanceSheet(date).catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat neraca", apiErr.message);
        throw err;
      }),
    [date],
  );

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  return (
    <>
      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <div className="min-w-44">
            <FormField label="Per tanggal">
              <DateInput
                value={date}
                onChange={(e) => setDate(e.target.value)}
              />
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title="Neraca"
        description={
          report ? `Posisi per ${formatDate(report.date)}` : undefined
        }
        actions={
          report ? (
            <Badge tone={report.balanced ? "success" : "danger"}>
              {report.balanced ? "Seimbang" : "Tidak seimbang"}
            </Badge>
          ) : undefined
        }
      >
        {loading && !report ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : report ? (
          <>
            <BalanceGroup
              title="Aset"
              rows={report.assets}
              total={report.totalAssets}
            />
            <BalanceGroup
              title="Kewajiban"
              rows={report.liabilities}
              total={report.liabilities.reduce((s, r) => s + r.debit, 0)}
            />
            <BalanceGroup
              title="Modal"
              rows={report.equity}
              total={report.equity.reduce((s, r) => s + r.debit, 0)}
            />
            <TotalsBar>
              <span>
                Total Aset <strong>{formatIDR(report.totalAssets)}</strong>
              </span>
              <span>
                Total Kewajiban + Modal{" "}
                <strong>{formatIDR(report.totalLiabilitiesEquity)}</strong>
              </span>
            </TotalsBar>
          </>
        ) : null}
      </SectionCard>
    </>
  );
}

// --- Pajak (?year=&month=): ringkasan + tautan ke rincian ---

function TaxSummarySection() {
  const initial = defaultYearMonth();
  const [year, setYear] = useState(initial.year);
  const [month, setMonth] = useState(initial.month);
  const { data: summary, loading, error, reload } = useAsyncData(
    () =>
      financeReportsService.taxSummary(year, month).catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat ringkasan pajak", apiErr.message);
        throw err;
      }),
    [year, month],
  );

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  const payable = summary?.payable ?? 0;

  return (
    <>
      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <YearMonthFields
            year={year}
            month={month}
            onYear={setYear}
            onMonth={setMonth}
          />
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title={`Pajak — ${periodLabel(year, month)}`}
        description="PPN Keluaran dikurangi PPN Masukan."
        actions={
          <Link
            to="/finance/reports/tax-detail"
            className="font-medium text-brand hover:underline"
          >
            Rincian per faktur
          </Link>
        }
      >
        {loading && !summary ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : summary ? (
          <div className="grid gap-4 sm:grid-cols-3">
            <SummaryCard
              label="PPN Keluaran"
              value={formatIDR(summary.ppnOut)}
              tone="brand"
            />
            <SummaryCard
              label="PPN Masukan"
              value={formatIDR(summary.ppnIn)}
              tone="brand"
            />
            <SummaryCard
              label={payable >= 0 ? "Kurang Bayar" : "Lebih Bayar"}
              value={formatIDR(Math.abs(payable))}
              tone={payable > 0 ? "danger" : "success"}
            />
          </div>
        ) : null}
      </SectionCard>
    </>
  );
}

// --- Kas (?year=&month=): posisi per rekening is_cash ---

function CashSection() {
  const initial = defaultYearMonth();
  const [year, setYear] = useState(initial.year);
  const [month, setMonth] = useState(initial.month);
  const { data: report, loading, error, reload } = useAsyncData(
    () =>
      financeReportsService.cashSummary(year, month).catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat ringkas kas", apiErr.message);
        throw err;
      }),
    [year, month],
  );

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  return (
    <>
      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <YearMonthFields
            year={year}
            month={month}
            onYear={setYear}
            onMonth={setMonth}
          />
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title={`Kas — ${periodLabel(year, month)}`}
        description="Arus per rekening kas/bank (flag is_cash di COA — tanpa tabel kas terpisah)."
      >
        {loading && !report ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : report ? (
          <>
            <DataTable
              rows={report.rows}
              rowKey={(r) => r.code}
              emptyTitle="Tidak ada mutasi kas"
              emptyDescription="Belum ada jurnal menyentuh rekening kas bulan ini."
              columns={[
                {
                  header: "Rekening",
                  render: (r) => (
                    <span>
                      <span className="font-semibold">{r.code}</span>{" "}
                      <span className="text-muted">{r.name}</span>
                    </span>
                  ),
                },
                {
                  header: "Masuk",
                  align: "right",
                  render: (r) => formatIDR(r.debit),
                },
                {
                  header: "Keluar",
                  align: "right",
                  render: (r) => formatIDR(r.credit),
                },
                {
                  header: "Saldo",
                  align: "right",
                  render: (r) => formatIDR(r.balance),
                },
              ]}
            />
            <TotalsBar>
              <span className="text-muted">Total saldo kas</span>
              <span className="font-bold text-ink">
                {formatIDR(report.total)}
              </span>
            </TotalsBar>
          </>
        ) : null}
      </SectionCard>
    </>
  );
}

// --- Persediaan (tanpa param) ---
function InventorySection() {
  const { data: report, loading, error, reload } = useAsyncData(
    () =>
      financeReportsService.inventoryValue().catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat nilai persediaan", apiErr.message);
        throw err;
      }),
    [],
  );

  return (
    <>
      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title="Nilai Persediaan"
        description="Setiap posisi dinilai pada rata-rata bergerak."
      >
        {loading && !report ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<InventoryPosition>
              rows={report?.positions ?? []}
              rowKey={(r) =>
                `${r.productId}-${r.variantId}-${r.productCode}`
              }
              emptyTitle="Persediaan kosong"
              emptyDescription="Tidak ada posisi berstok saat ini."
              columns={[
                { header: "Kode", render: (r) => r.productCode || "-" },
                { header: "Produk", render: (r) => r.productName || "-" },
                {
                  header: "Qty",
                  align: "right",
                  render: (r) => formatNumber(r.qty),
                },
                {
                  header: "Rata-rata",
                  align: "right",
                  render: (r) => formatIDR(r.avgCost),
                },
                {
                  header: "Nilai",
                  align: "right",
                  render: (r) => (
                    <span className="font-medium">
                      {formatIDR(r.value)}
                    </span>
                  ),
                },
              ]}
            />
            {report && report.positions.length > 0 && (
              <TotalsBar>
                <span>
                  Total Nilai <strong>{formatIDR(report.total)}</strong>
                </span>
              </TotalsBar>
            )}
          </>
        )}
      </SectionCard>
    </>
  );
}

// --- Marjin (?from=&to= YYYY-MM-DD) ---

function MarginSection() {
  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([
        financeReportsService.margin(from, to),
        financeReportsService.marginByProduct(from, to),
      ])
        .then(([total, breakdown]) => ({ total, breakdown }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat marjin", apiErr.message);
          throw err;
        }),
    [from, to],
  );
  const report = data?.total ?? null;
  const rows = data?.breakdown ?? [];

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  return (
    <>
      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <div className="min-w-44">
            <FormField label="Dari">
              <DateInput
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
            </FormField>
          </div>
          <div className="min-w-44">
            <FormField label="Sampai">
              <DateInput value={to} onChange={(e) => setTo(e.target.value)} />
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard
        title="Marjin Kotor"
        description={
          report
            ? `${formatDate(from)} – ${formatDate(to)}`
            : undefined
        }
      >
        {loading && !report ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : report ? (
          <div className="grid gap-4 sm:grid-cols-4">
            <SummaryCard
              label="Pendapatan"
              value={formatIDR(report.revenue)}
              tone="brand"
            />
            <SummaryCard
              label="HPP"
              value={formatIDR(report.cogs)}
              tone="brand"
            />
            <SummaryCard
              label="Laba Kotor"
              value={formatIDR(report.gross)}
              tone="success"
            />
            <SummaryCard
              label="Marjin"
              value={formatPercent(report.percent)}
              tone="success"
            />
          </div>
        ) : null}
      </SectionCard>

      <SectionCard
        title="Marjin per Produk"
        description="Dari buku: pendapatan dikurangi retur, HPP dikurangi barang yang masuk kembali. Terurut laba kotor terbesar."
      >
        {loading && rows.length === 0 ? (
          <p className="text-muted py-8 text-center text-sm">Memuat...</p>
        ) : (
          <DataTable<ProductMarginRow>
            rows={rows}
            rowKey={(r) => `${r.productId}-${r.variantId}`}
            emptyTitle="Belum ada penjualan terbukukan"
            emptyDescription="Marjin per produk muncul setelah surat jalan di-posting ke jurnal."
            columns={[
              { header: "Kode", render: (r) => r.productCode || "-" },
              { header: "Produk", render: (r) => r.productName || "-" },
              {
                header: "Pendapatan",
                align: "right",
                render: (r) => formatIDR(r.revenue),
              },
              {
                header: "HPP",
                align: "right",
                render: (r) => formatIDR(r.cogs),
              },
              {
                header: "Laba Kotor",
                align: "right",
                render: (r) => (
                  <span className="font-medium">{formatIDR(r.gross)}</span>
                ),
              },
              {
                header: "Marjin",
                align: "right",
                render: (r) => (
                  <Badge tone={r.gross >= 0 ? "success" : "danger"}>
                    {formatPercent(r.percent)}
                  </Badge>
                ),
              },
            ]}
          />
        )}
      </SectionCard>
    </>
  );
}

// --- Piutang & Hutang (?asOf= YYYY-MM-DD) ---
//
// Saldo, bukan arus: yang ditanyakan "siapa masih berhutang per tanggal X",
// bukan "apa yang terjadi antara X dan Y". Karena itu hanya ada satu tanggal
// batas -- memakai rentang justru menyembunyikan faktur yang lebih tua.

function PartyBalanceTable({
  rows,
  loading,
  emptyTitle,
}: {
  rows: PartyBalance[];
  loading: boolean;
  emptyTitle: string;
}) {
  if (loading && rows.length === 0) {
    return <p className="text-muted py-8 text-center text-sm">Memuat...</p>;
  }
  return (
    <DataTable<PartyBalance>
      rows={rows}
      rowKey={(r) => String(r.partyId)}
      emptyTitle={emptyTitle}
      emptyDescription="Saldo terbuka muncul setelah dokumen di-posting ke jurnal."
      columns={[
        { header: "Kode", render: (r) => r.partyCode || "-" },
        { header: "Nama", render: (r) => r.partyName || `#${r.partyId}` },
        {
          header: "Saldo",
          align: "right",
          render: (r) => (
            <span className="font-medium">{formatIDR(r.balance)}</span>
          ),
        },
      ]}
    />
  );
}

function PartyBalancesSection() {
  const [asOf, setAsOf] = useState(todayWIB());
  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([
        financeReportsService.receivables(asOf),
        financeReportsService.payables(asOf),
      ])
        .then(([ar, ap]) => ({ receivables: ar, payables: ap }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat piutang & hutang", apiErr.message);
          throw err;
        }),
    [asOf],
  );
  const receivables = data?.receivables ?? null;
  const payables = data?.payables ?? null;

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  return (
    <>
      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <div className="min-w-44">
            <FormField label="Per tanggal">
              <DateInput
                value={asOf}
                onChange={(e) => setAsOf(e.target.value)}
              />
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <div className="mb-4 grid gap-4 sm:grid-cols-2">
        <SummaryCard
          label="Total Piutang"
          value={formatIDR(receivables?.total ?? 0)}
          tone="brand"
        />
        <SummaryCard
          label="Total Hutang"
          value={formatIDR(payables?.total ?? 0)}
          tone="warning"
        />
      </div>

      <SectionCard
        title="Piutang Usaha"
        description={`Per ${formatDate(asOf)} - pelanggan yang masih punya saldo terbuka.`}
      >
        <PartyBalanceTable
          rows={receivables?.parties ?? []}
          loading={loading}
          emptyTitle="Tidak ada piutang terbuka"
        />
      </SectionCard>

      <SectionCard
        title="Hutang Usaha"
        description={`Per ${formatDate(asOf)} - pemasok yang masih harus dibayar.`}
      >
        <PartyBalanceTable
          rows={payables?.parties ?? []}
          loading={loading}
          emptyTitle="Tidak ada hutang terbuka"
        />
      </SectionCard>
    </>
  );
}

const PATH_TAB: Record<string, Tab> = {
  "/finance/reports/trial-balance": "neraca-saldo",
  "/finance/reports/profit-loss": "laba-rugi",
  "/finance/reports/balance-sheet": "neraca",
  "/finance/reports/tax-summary": "pajak",
  "/finance/reports/inventory-value": "persediaan",
  "/finance/reports/margin": "marjin",
  "/finance/reports/receivables-payables": "piutang-hutang",
};

export default function ReportsPage() {
  // Tab awal mengikuti path menu (deep-link antar-laporan tetap mendarat
  // di tab yang benar); pindah tab manual menimpa state lokal.
  const { pathname } = useLocation();
  const [tab, setTab] = useState<Tab>(PATH_TAB[pathname] ?? "neraca-saldo");
  // Komponen dipakai ulang antar-rute laporan — sinkronkan tab tiap pindah menu.
  const [prevPathname, setPrevPathname] = useState(pathname);
  if (pathname !== prevPathname) {
    setPrevPathname(pathname);
    setTab(PATH_TAB[pathname] ?? "neraca-saldo");
  }

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Laporan Keuangan"
        description="Neraca saldo, laba rugi, neraca, pajak, persediaan, marjin, serta piutang & hutang — hanya baca."
      />

      <div className="mb-4">
        <Segmented
          options={TABS}
          value={tab}
          onChange={(v) => setTab(v as Tab)}
        />
      </div>

      {tab === "neraca-saldo" && <TrialBalanceSection />}
      {tab === "laba-rugi" && <ProfitLossSection />}
      {tab === "neraca" && <BalanceSheetSection />}
      {tab === "pajak" && <TaxSummarySection />}
      {tab === "kas" && <CashSection />}
      {tab === "persediaan" && <InventorySection />}
      {tab === "marjin" && <MarginSection />}
      {tab === "piutang-hutang" && <PartyBalancesSection />}
    </div>
  );
}
