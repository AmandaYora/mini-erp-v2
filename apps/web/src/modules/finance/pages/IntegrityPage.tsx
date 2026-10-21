import { useState } from "react";
import { RefreshCw } from "lucide-react";
import {
  Badge,
  Button,
  CardSkeleton,
  ConfirmDialog,
  DataTable,
  PageHeader,
  SectionCard,
  SummaryCard,
} from "@/shared/components/ui";
import { Money, Page } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatDate, formatIDR, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeService } from "@/modules/finance/services/finance.service";
import { integritySummary, inventoryGapHint } from "@/modules/finance/lib/reports";
import type { RollupDiff } from "@/modules/finance/types";

// Integritas pembukuan (T7, perm finance.manage): rollup harian vs baris jurnal.
// Periksa dulu (GET), hitung ulang rollup (POST rebuild) hanya bila ada selisih.
export default function IntegrityPage() {
  const { data, loading, error, reload } = useAsyncData(() => financeService.integrity(), []);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [rebuilding, setRebuilding] = useState(false);

  async function handleRebuild() {
    setRebuilding(true);
    try {
      const res = await financeService.rebuildIntegrity();
      toast.fromServer(res.message, "Rollup dihitung ulang");
      setConfirmOpen(false);
      reload();
    } catch (err) {
      toast.danger("Gagal menghitung ulang rollup", toApiError(err).message);
    } finally {
      setRebuilding(false);
    }
  }

  const diffs = data?.rollupDiffs ?? [];
  const findings = diffs.length + (data?.unbalanced ?? 0);

  return (
    <Page>
      <PageHeader
        title="Integritas Pembukuan"
        description="Membandingkan rollup harian dengan baris jurnal dan memeriksa jurnal yang tidak seimbang."
        actions={
          <Button variant="secondary" size="sm" loading={loading} onClick={reload}>
            <RefreshCw aria-hidden /> Periksa ulang
          </Button>
        }
      />

      {error ? (
        <ErrorState message={error} onRetry={reload} title="Gagal memeriksa integritas" />
      ) : loading && !data ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
        </div>
      ) : data ? (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <SummaryCard
              label="Status pembukuan"
              value={data.consistent ? "Semua seimbang" : `${formatNumber(findings)} temuan`}
              tone={data.consistent ? "success" : "danger"}
              hint={integritySummary(data)}
            />
            <SummaryCard label="Selisih rollup" value={formatNumber(diffs.length)} tone={diffs.length > 0 ? "danger" : "success"} />
            <SummaryCard
              label="Jurnal tidak seimbang"
              value={formatNumber(data.unbalanced)}
              tone={data.unbalanced > 0 ? "danger" : "success"}
              hint={data.unbalanced > 0 ? "Tidak bisa diperbaiki otomatis; telusuri jurnalnya." : undefined}
            />
          </div>
          {data.inventory && (
            <SectionCard
              title="Persediaan: buku vs ledger biaya"
              description="Saldo akun Persediaan di buku harus sama dengan nilai persediaan dari ledger biaya (Laporan Nilai Persediaan)."
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <SummaryCard label="Saldo akun Persediaan (buku)" value={formatIDR(data.inventory.book)} />
                <SummaryCard label="Nilai ledger biaya" value={formatIDR(data.inventory.ledger)} />
                <SummaryCard
                  label="Selisih tak terjelaskan"
                  value={formatIDR(data.inventory.unexplained)}
                  tone={data.inventory.consistent ? "success" : "danger"}
                  hint={inventoryGapHint(data.inventory)}
                />
              </div>
              {data.inventory.missingCost > 0 && (
                <p className="mt-3 text-sm text-warning">
                  {formatNumber(data.inventory.missingCost)} produk berstok belum punya harga modal — lengkapi di halaman Saldo Awal.
                </p>
              )}
            </SectionCard>
          )}
          <SectionCard
            flush
            title="Selisih rollup"
            description="Rollup harian per akun yang tidak sama dengan jumlah baris jurnal."
            actions={
              diffs.length > 0 ? (
                <Button variant="danger" size="sm" onClick={() => setConfirmOpen(true)}>
                  Hitung ulang rollup
                </Button>
              ) : undefined
            }
          >
            <DataTable<RollupDiff>
              caption="Selisih rollup harian"
              rows={diffs}
              rowKey={(r) => `${r.accountId}-${r.date}-${r.scope}`}
              emptyTitle="Tidak ada selisih"
              emptyDescription="Rollup harian cocok dengan seluruh baris jurnal."
              columns={[
                { header: "Tanggal", render: (r) => formatDate(r.date) },
                { header: "Akun", render: (r) => `#${r.accountId}` },
                { header: "Cakupan", render: (r) => <Badge tone="neutral">{r.scope === 0 ? "Komersial" : "Fiskal"}</Badge> },
                {
                  header: "Baris (D / K)",
                  align: "right",
                  render: (r) => (
                    <span className="inline-flex gap-1">
                      <Money value={r.linesDebit} /> / <Money value={r.linesCredit} />
                    </span>
                  ),
                },
                {
                  header: "Rollup (D / K)",
                  align: "right",
                  render: (r) => (
                    <span className="inline-flex gap-1">
                      <Money value={r.rollupDebit} /> / <Money value={r.rollupCredit} />
                    </span>
                  ),
                },
              ]}
            />
          </SectionCard>
        </>
      ) : null}

      <ConfirmDialog
        open={confirmOpen}
        title="Hitung ulang rollup?"
        message={`Rollup harian cabang ini dihitung ulang dari baris jurnal untuk ${diffs.length} selisih yang ditemukan. Baris jurnal tidak diubah.`}
        confirmLabel={rebuilding ? "Menghitung…" : "Hitung ulang"}
        tone="danger"
        onConfirm={() => void handleRebuild()}
        onCancel={() => (rebuilding ? undefined : setConfirmOpen(false))}
      />
    </Page>
  );
}
