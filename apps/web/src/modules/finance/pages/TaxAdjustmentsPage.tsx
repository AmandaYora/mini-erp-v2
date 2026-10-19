import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Badge, Button, DataTable, DateInput, FormField, PageHeader, Pagination, SectionCard, SummaryCard } from "@/shared/components/ui";
import { DocNumber, Money, Page } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatDate, formatIDR, formatNumber, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { TaxAdjustmentSheet } from "@/modules/finance/components/TaxAdjustmentSheet";
import type { TaxAdjustmentExtra } from "@/modules/finance/components/TaxAdjustmentSheet";
import { TaxAdjustmentFilesModal } from "@/modules/finance/components/TaxAdjustmentFilesModal";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import type { Account, FiscalClass, FiscalLine, TaxAdjustment } from "@/modules/finance/types";

const LIMIT = 10;

// Penyesuaian pajak (T1 + sheet): koreksi fiskal sebagai jurnal bertipe (tanpa tabel worksheet).
// Buku komersial tidak pernah melihatnya; rekonsiliasi komersial → fiskal dihitung eksplisit.
export default function TaxAdjustmentsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.tax_adjustment.manage");

  const { data: adjData, loading, error, reload: reloadList } = useAsyncData(
    () =>
      Promise.all([financeService.taxAdjustments(), financeService.accounts("active").catch(() => [] as Account[])]).then(
        ([list, acc]) => ({ rows: list ?? [], accounts: acc ?? [] }),
      ),
    [],
  );
  const rows = adjData?.rows ?? [];
  const accounts = adjData?.accounts ?? [];
  const [page, setPage] = useState(1);
  const pageRows = rows.slice((page - 1) * LIMIT, page * LIMIT);

  const [filesFor, setFilesFor] = useState<TaxAdjustment | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const { data: fiscal, loading: fiscalLoading, error: fiscalError, reload: reloadFiscal } = useAsyncData(() => {
    if (from.trim() === "" || to.trim() === "") return Promise.resolve(null);
    return financeReportsService.fiscalSummary(from.trim(), to.trim());
  }, [from, to]);
  // Backend dapat mengirim null untuk slice kosong — jangan pernah meneruskannya ke tabel.
  const fiscalLines: FiscalLine[] = fiscal?.lines ?? [];
  const fiscalClasses: FiscalClass[] = fiscal?.classes ?? [];

  async function handleSubmit(values: ManualJournalValues, extra: TaxAdjustmentExtra) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.createTaxAdjustment({
        date: values.date,
        memo: values.memo,
        lines: values.lines,
        classification: extra.classification,
        note: extra.note || undefined,
      });
      if (extra.file) {
        try {
          await financeService.attachTaxAdjustment(res.data.id, extra.file);
        } catch (err) {
          toast.danger("Koreksi tercatat, lampiran gagal diunggah", toApiError(err).message);
        }
      }
      toast.fromServer(res.message, "Koreksi fiskal dicatat", res.data.number);
      setSheetOpen(false);
      setPage(1);
      reloadList();
      reloadFiscal();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  const newButton = canManage ? (
    <Button
      onClick={() => {
        setServerError(null);
        setSheetOpen(true);
      }}
    >
      Koreksi baru
    </Button>
  ) : undefined;

  return (
    <Page>
      <PageHeader
        title="Penyesuaian Pajak"
        description="Koreksi fiskal komersial → fiskal. Buku komersial tidak tersentuh — pembalik lewat jurnal."
        actions={newButton}
      />

      <SectionCard
        title="Rekonsiliasi fiskal"
        description="Laba komersial + koreksi = laba fiskal per rentang."
        actions={
          <Button variant="secondary" size="sm" loading={fiscalLoading} onClick={reloadFiscal}>
            Hitung ulang
          </Button>
        }
      >
        <div className="mb-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Dari" htmlFor="fisc-from">
            <DateInput id="fisc-from" value={from} onChange={(e) => setFrom(e.target.value)} />
          </FormField>
          <FormField label="Sampai" htmlFor="fisc-to">
            <DateInput id="fisc-to" value={to} onChange={(e) => setTo(e.target.value)} />
          </FormField>
        </div>
        {fiscalError ? (
          <ErrorState message={fiscalError} onRetry={reloadFiscal} title="Gagal menghitung rekonsiliasi" />
        ) : fiscal ? (
          <div className="flex flex-col gap-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <SummaryCard label="Laba komersial" value={formatIDR(fiscal.commercial.profit)} tone="brand" />
              <SummaryCard label="Laba fiskal" value={formatIDR(fiscal.fiscalProfit)} tone="success" />
            </div>
            {fiscalClasses.length > 0 && (
              <div className="-mx-5 border-t border-border">
                <DataTable<FiscalClass>
                  caption="Koreksi fiskal per jenis"
                  rows={fiscalClasses}
                  rowKey={(c) => c.classification || "none"}
                  emptyTitle="Tidak ada koreksi"
                  columns={[
                    { header: "Jenis koreksi", render: (c) => c.label },
                    { header: "Jumlah", align: "right", render: (c) => formatNumber(c.entries) },
                    { header: "Efek laba", align: "right", render: (c) => <Money accounting value={c.effect} /> },
                  ]}
                />
              </div>
            )}
            <div className="-mx-5 -mb-5 border-t border-border">
              <DataTable<FiscalLine>
                caption="Koreksi fiskal per akun"
                rows={fiscalLines}
                rowKey={(l) => l.code}
                loading={fiscalLoading}
                emptyTitle="Tidak ada koreksi"
                emptyDescription="Belum ada jurnal koreksi fiskal pada rentang ini."
                columns={[
                  {
                    header: "Akun",
                    render: (l) => (
                      <span>
                        <span className="font-mono text-xs">{l.code}</span> <span className="text-muted-foreground">{l.name}</span>
                      </span>
                    ),
                  },
                  { header: "Nominal", align: "right", render: (l) => <Money value={l.debit + l.credit} /> },
                  {
                    header: "Efek laba",
                    align: "right",
                    render: (l) => (
                      <span className="inline-flex items-center gap-2">
                        <Money accounting value={l.effect} />
                        <Badge tone={l.effect !== 0 ? "warning" : "neutral"}>
                          {l.effect === 0
                            ? "netral"
                            : `${formatNumber(Math.round((l.effect / Math.max(1, Math.abs(fiscal.commercial.profit))) * 1000) / 10)}%`}
                        </Badge>
                      </span>
                    ),
                  },
                ]}
              />
            </div>
          </div>
        ) : null}
      </SectionCard>

      <SectionCard flush title="Riwayat koreksi" description="Append-only: salah catat dibalik lewat jurnal, tidak diubah.">
        <DataTable<TaxAdjustment>
          caption="Riwayat koreksi fiskal"
          rows={pageRows}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reloadList}
          onRowClick={(r) => navigate(`${ROUTE_PATHS.financeJournals}/${r.id}`)}
          emptyTitle="Belum ada koreksi"
          emptyDescription="Koreksi fiskal yang dicatat akan muncul di sini."
          emptyAction={newButton}
          columns={[
            {
              header: "Nomor",
              render: (r) => (
                <Link to={`${ROUTE_PATHS.financeJournals}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", render: (r) => formatDate(r.date.slice(0, 10)) },
            { header: "Memo", render: (r) => r.memo || "-" },
            {
              header: "Jenis",
              render: (r) => <Badge tone={r.classification ? "info" : "neutral"}>{r.classLabel || "Belum diklasifikasi"}</Badge>,
            },
            { header: "Catatan", render: (r) => (r.note ? <span className="line-clamp-2 max-w-xs">{r.note}</span> : "-") },
            {
              header: "Lampiran",
              render: (r) => (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    setFilesFor(r);
                  }}
                >
                  Lihat
                </Button>
              ),
            },
            {
              header: "Status",
              render: (r) => <Badge tone={r.status === "reversed" ? "muted" : "info"}>{r.status === "reversed" ? "Dibalik" : "Tercatat"}</Badge>,
            },
          ]}
        />
        <Pagination page={page} limit={LIMIT} total={rows.length} onPageChange={setPage} />
      </SectionCard>

      {sheetOpen && (
        <TaxAdjustmentSheet
          accounts={accounts}
          submitting={submitting}
          serverError={serverError}
          onClose={() => (submitting ? undefined : setSheetOpen(false))}
          onSubmit={(v, extra) => void handleSubmit(v, extra)}
        />
      )}
      {filesFor && <TaxAdjustmentFilesModal adjustment={filesFor} canManage={canManage} onClose={() => setFilesFor(null)} />}
    </Page>
  );
}
