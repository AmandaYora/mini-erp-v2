import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateInput,
  FormField,
  PageHeader,
  SectionCard,
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
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { ManualJournalModal } from "@/modules/finance/components/ManualJournalModal";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import type {
  Account,
  JournalEntry,
} from "@/modules/finance/types";

// Penyesuaian pajak (H-revisi): koreksi fiskal sebagai jurnal bertipe
// (tanpa tabel worksheet). Buku komersil tidak pernah melihatnya;
// rekonsiliasi komersil → fiskal dihitung eksplisit di bawah.
export default function TaxAdjustmentsPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { data: adjData, loading, error, reload: reloadList } = useAsyncData(
    () =>
      Promise.all([
        financeService.taxAdjustments(),
        financeService.accounts("active").catch(() => [] as Account[]),
      ]).then(([list, acc]) => ({ rows: list ?? [], accounts: acc ?? [] })),
    [],
  );
  const rows = adjData?.rows ?? [];
  const accounts = adjData?.accounts ?? [];
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const {
    data: fiscal,
    loading: fiscalLoading,
    error: fiscalError,
    reload: reloadFiscal,
  } = useAsyncData(
    () => {
      if (from.trim() === "" || to.trim() === "") {
        return Promise.resolve(null);
      }
      return financeReportsService.fiscalSummary(from.trim(), to.trim());
    },
    [from, to],
  );

  async function handleSubmit(values: ManualJournalValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.createTaxAdjustment({
        date: values.date,
        memo: values.memo,
        lines: values.lines,
      });
      toast.fromServer(res.message, "Koreksi fiskal dicatat", res.data.number);
      setModalOpen(false);
      reloadList();
      reloadFiscal();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Penyesuaian Pajak"
        description="Koreksi fiskal komersil → fiskal. Buku komersil tidak tersentuh — pembalik lewat jurnal."
        actions={
          canManage ? (
            <Button onClick={() => setModalOpen(true)}>Koreksi Baru</Button>
          ) : undefined
        }
      />

      <SectionCard
        title="Rekonsiliasi Fiskal"
        description="Laba komersil + koreksi = laba fiskal per rentang."
        actions={
          <Button
            variant="secondary"
            size="sm"
            onClick={() => reloadFiscal()}
            disabled={fiscalLoading}
          >
            Hitung Ulang
          </Button>
        }
      >
        <div className="mb-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Dari" htmlFor="fisc-from">
            <DateInput
              id="fisc-from"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
            />
          </FormField>
          <FormField label="Sampai" htmlFor="fisc-to">
            <DateInput
              id="fisc-to"
              value={to}
              onChange={(e) => setTo(e.target.value)}
            />
          </FormField>
        </div>
        {fiscalError ? (
          <Notice tone="danger" title="Gagal menghitung rekonsiliasi">
            {fiscalError}
          </Notice>
        ) : fiscalLoading ? (
          <p className="text-muted px-1 py-4 text-sm">Menghitung…</p>
        ) : fiscal ? (
          <>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="rounded-md border border-hairline p-4">
                <p className="text-muted text-xs font-semibold uppercase">
                  Laba Komersil
                </p>
                <p className="text-lg font-bold text-ink">
                  {formatIDR(fiscal.commercial.profit)}
                </p>
              </div>
              <div className="rounded-md border border-hairline p-4">
                <p className="text-muted text-xs font-semibold uppercase">
                  Laba Fiskal
                </p>
                <p className="text-lg font-bold text-ink">
                  {formatIDR(fiscal.fiscalProfit)}
                </p>
              </div>
            </div>
            <div className="mt-4">
              <DataTable
                rows={fiscal.lines}
                rowKey={(l) => l.code}
                emptyTitle="Tidak ada koreksi"
                emptyDescription="Belum ada jurnal koreksi fiskal pada rentang ini."
                columns={[
                  {
                    header: "Akun",
                    render: (l) => (
                      <span>
                        <span className="font-semibold">{l.code}</span>{" "}
                        <span className="text-muted">{l.name}</span>
                      </span>
                    ),
                  },
                  {
                    header: "Nominal",
                    align: "right",
                    render: (l) => formatIDR(l.debit + l.credit),
                  },
                  {
                    header: "Efek Laba",
                    align: "right",
                    render: (l) => (
                      <span className="inline-flex items-center gap-2">
                        {formatIDR(l.effect)}
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
          </>
        ) : null}
      </SectionCard>

      <div className="mt-4">
        <SectionCard
          title="Riwayat Koreksi"
          description="Append-only: salah catat dibalik lewat jurnal, tidak diubah."
        >
          {error ? (
            <Notice tone="danger" title="Gagal memuat koreksi">
              {error}{" "}
              <button
                type="button"
                onClick={() => reloadList()}
                className="cursor-pointer font-semibold text-brand hover:underline"
              >
                Coba lagi
              </button>
            </Notice>
          ) : loading ? (
            <p className="text-muted px-1 py-6 text-sm">Memuat koreksi…</p>
          ) : (
            <DataTable<JournalEntry>
              rows={rows}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada koreksi"
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.financeJournals}/${r.id}`}
                      className="font-semibold text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                {
                  header: "Tanggal",
                  render: (r) => formatDate(r.date.slice(0, 10)),
                },
                {
                  header: "Memo",
                  render: (r) => r.memo || "-",
                },
                {
                  header: "Status",
                  render: (r) => (
                    <Badge tone={r.status === "reversed" ? "neutral" : "info"}>
                      {r.status === "reversed" ? "Dibalik" : "Tercatat"}
                    </Badge>
                  ),
                },
              ]}
            />
          )}
        </SectionCard>
      </div>

      {modalOpen && (
        <ManualJournalModal
          open={modalOpen}
          accounts={accounts}
          submitting={submitting}
          serverError={serverError}
          onClose={() => {
            if (!submitting) setModalOpen(false);
          }}
          onSubmit={(v) => void handleSubmit(v)}
          title="Koreksi Fiskal"
          description="Hanya memengaruhi laporan fiskal — laba komersil tidak berubah."
          submitLabel="Catat Koreksi"
        />
      )}
    </div>
  );
}
