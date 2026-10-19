import { useState } from "react";
import { Link } from "react-router-dom";
import { DataTable, FormField, PageHeader, Pagination, SearchSelect, SectionCard } from "@/shared/components/ui";
import { Money, Page } from "@/shared/components/patterns";
import { formatDate, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ExportCsvButton, ReportDateField, ReportToolbar } from "@/modules/finance/components/ReportToolbar";
import type { Account, LedgerAccountRow, LedgerLine } from "@/modules/finance/types";

const LIMIT = 200;
const OVERVIEW_LIMIT = 100;

function isOpening(row: LedgerLine): boolean {
  return !row.entryId && row.memo === "Saldo awal";
}

// Buku besar (T5): mutasi satu akun + saldo berjalan; 200 baris per halaman, header menempel.
export default function GeneralLedgerPage() {
  const { draft, patch, applied, apply, version } = useDraftFilters({
    account: null as string | number | null,
    from: monthStartWIB(),
    to: todayWIB(),
    branch: "",
  });
  const [committed, setCommitted] = useState(false);
  const [page, setPage] = useState(1);

  const { data: accounts } = useAsyncData(() => financeReportsService.accounts().then((a) => a ?? ([] as Account[])), []);

  // Tanpa akun terpilih = ikhtisar semua akun (FN-08); dengan akun = mutasi akun itu.
  const overviewMode = applied.account === null || applied.account === "";
  const { data, loading, error, reload } = useAsyncData(() => {
    if (!committed) return Promise.resolve(null);
    if (overviewMode) {
      return financeReportsService.generalLedgerOverview(applied.from, applied.to, page, OVERVIEW_LIMIT, applied.branch).then((res) => ({
        mode: "overview" as string,
        lines: [] as LedgerLine[],
        accounts: res.rows ?? [],
        total: res.meta.total,
        totalDebit: res.totalDebit,
        totalCredit: res.totalCredit,
      }));
    }
    return financeReportsService
      .generalLedger(String(applied.account), applied.from, applied.to, page, LIMIT, applied.branch)
      .then((res) => ({
        mode: "account" as string,
        lines: res.items ?? [],
        accounts: [] as LedgerAccountRow[],
        total: res.meta.total,
        totalDebit: 0,
        totalCredit: 0,
      }));
  }, [applied, version, page, committed]);
  const lines = data?.lines ?? [];
  const accountRows = data?.accounts ?? [];
  const total = data?.total ?? 0;
  const loaded = data !== null;

  function submit() {
    setPage(1);
    setCommitted(true);
    apply();
  }

  const accountLabel = (accounts ?? []).find((a) => a.code === String(applied.account));

  return (
    <Page>
      <PageHeader title="Buku Besar" description="Mutasi satu akun beserta saldo berjalan; tanpa memilih akun tampil ikhtisar semua akun. Hanya baca." />

      <ReportToolbar
        onApply={submit}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`buku-besar-${String(applied.account ?? "akun")}-${applied.from}_${applied.to}.csv`}
            getRows={() =>
              loaded && overviewMode
                ? [
                    ["Kode", "Akun", "Saldo awal", "Debit", "Kredit", "Saldo akhir"],
                    ...accountRows.map((a) => [a.code, a.name, a.opening, a.debit, a.credit, a.closing]),
                  ]
                : loaded
                ? [
                    ["Tanggal", "Nomor", "Keterangan", "Pihak", "Produk", "Debit", "Kredit", "Saldo"],
                    ...lines.map((l) => [l.date ? l.date.slice(0, 10) : "", l.number, l.memo, l.partyName ?? "", l.productName ?? "", l.debit, l.credit, l.balance]),
                  ]
                : null
            }
          />
        }
      >
        <FormField label="Akun" helperText="Kosongkan untuk semua akun">
          <SearchSelect
            options={(accounts ?? []).map((a) => ({ value: a.code, label: `${a.code} — ${a.name}` }))}
            value={draft.account}
            onChange={(v) => patch({ account: v })}
            placeholder="Semua akun"
          />
        </FormField>
        <ReportDateField label="Dari" value={draft.from} onChange={(from) => patch({ from })} />
        <ReportDateField label="Sampai" value={draft.to} onChange={(to) => patch({ to })} />
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>

      {loaded && overviewMode && (
        <SectionCard flush title="Semua akun" description={`${formatDate(applied.from)} – ${formatDate(applied.to)}. Saldo menurut sisi normal akun.`}>
          <DataTable<LedgerAccountRow>
            caption="Buku besar semua akun"
            rows={accountRows}
            rowKey={(r) => r.code}
            loading={loading}
            error={error}
            onRetry={reload}
            emptyTitle="Belum ada saldo atau mutasi"
            emptyDescription="Tidak ada akun bersaldo atau bermutasi pada rentang ini."
            columns={[
              { header: "Kode", render: (r) => <span className="font-mono text-xs">{r.code}</span> },
              { header: "Akun", render: (r) => r.name },
              { header: "Saldo awal", align: "right", render: (r) => <Money accounting value={r.opening} /> },
              { header: "Debit", align: "right", render: (r) => <Money accounting value={r.debit} /> },
              { header: "Kredit", align: "right", render: (r) => <Money accounting value={r.credit} /> },
              { header: "Saldo akhir", align: "right", render: (r) => <Money accounting value={r.closing} className="font-medium" /> },
            ]}
          />
          {total > 0 && <Pagination page={page} limit={OVERVIEW_LIMIT} total={total} onPageChange={setPage} />}
          <p className="px-4 py-3 text-sm text-muted-foreground">
            Total mutasi: debit <Money value={data?.totalDebit ?? 0} className="font-medium text-foreground" /> · kredit{" "}
            <Money value={data?.totalCredit ?? 0} className="font-medium text-foreground" />
          </p>
        </SectionCard>
      )}

      {!overviewMode && <SectionCard
        flush
        title={accountLabel ? `${accountLabel.code} — ${accountLabel.name}` : "Mutasi akun"}
        description={loaded ? `${formatDate(applied.from)} – ${formatDate(applied.to)}` : "Pilih akun lalu tekan Tampilkan."}
      >
        <DataTable<LedgerLine>
          caption="Mutasi akun buku besar"
          rows={loaded ? lines : []}
          rowKey={(r) => (r.entryId ? `line-${r.entryId}-${r.debit}-${r.credit}` : "saldo-awal")}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle={loaded ? "Belum ada mutasi" : "Belum ada data"}
          emptyDescription={loaded ? "Tidak ada jurnal untuk akun ini pada rentang tersebut." : "Pilih akun dan rentang tanggal terlebih dahulu."}
          columns={[
            { header: "Tanggal", render: (r) => (r.date ? formatDate(r.date) : "") },
            {
              header: "Nomor",
              render: (r) =>
                r.number ? (
                  r.entryId ? (
                    <Link to={`${ROUTE_PATHS.financeJournals}/${r.entryId}`} className="font-mono text-xs text-primary hover:underline">
                      {r.number}
                    </Link>
                  ) : (
                    r.number
                  )
                ) : (
                  "-"
                ),
            },
            { header: "Keterangan", render: (r) => (isOpening(r) ? <span className="font-semibold">{r.memo}</span> : r.memo || "-") },
            { header: "Pihak", render: (r) => r.partyName || "-" },
            { header: "Produk", render: (r) => r.productName || "-" },
            { header: "Debit", align: "right", render: (r) => <Money accounting value={r.debit} /> },
            { header: "Kredit", align: "right", render: (r) => <Money accounting value={r.credit} /> },
            { header: "Saldo", align: "right", render: (r) => <Money accounting value={r.balance} className="font-medium" /> },
          ]}
        />
        {loaded && total > 0 && <Pagination page={page} limit={LIMIT} total={total} onPageChange={setPage} />}
      </SectionCard>}
    </Page>
  );
}
