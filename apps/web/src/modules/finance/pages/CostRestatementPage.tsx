import { useState } from "react";
import { Link } from "react-router-dom";
import { CheckCircle2, RefreshCw, XCircle } from "lucide-react";
import {
  Button,
  CardSkeleton,
  DataTable,
  DateInput,
  FormField,
  PageHeader,
  SectionCard,
  SummaryCard,
} from "@/shared/components/ui";
import { DocNumber, Money, Page } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { correctionsService } from "@/modules/finance/services/corrections.service";
import { CorrectionReasonModal } from "@/modules/finance/components/CorrectionReasonModal";
import { failedGates, gateLabel, offsetSummary, restateState } from "@/modules/finance/lib/corrections";
import type { InventoryRecon, RestateProduct, RestateReport, RestateSkipped } from "@/modules/finance/types";

// Hitung Ulang HPP (OP-04, sama dengan cmd/costrebuild): dry run otomatis saat halaman dibuka,
// gerbang ditampilkan, komit butuh finance.close.force + alasan. Satu jurnal koreksi.
export default function CostRestatementPage() {
  const can = useAuthStore((s) => s.can);
  const canCommit = can("finance.close.force");
  const [date, setDate] = useState(todayWIB());
  const { data, loading, error, reload } = useAsyncData(
    () => correctionsService.restatePreview(date).then((r) => r.data),
    [date],
  );
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [result, setResult] = useState<RestateReport | null>(null);

  async function handleCommit(reason: string) {
    setCommitting(true);
    try {
      const res = await correctionsService.restateCommit(reason, date);
      toast.fromServer(res.message, "HPP dihitung ulang", res.data.entryNumber ? `Jurnal ${res.data.entryNumber}` : undefined);
      setResult(res.data);
      setConfirmOpen(false);
      reload();
    } catch (err) {
      toast.danger("Gagal menghitung ulang HPP", formatApiError(err));
    } finally {
      setCommitting(false);
    }
  }

  const state = data ? restateState(data) : null;
  const products = data?.products ?? [];
  const skipped = data?.skipped ?? [];

  return (
    <Page>
      <PageHeader
        title="Hitung Ulang HPP"
        description="Menghitung ulang harga pokok setiap barang keluar (pengiriman, selisih stok, hapus stok, retur beli) dari rantai biaya, lalu membukukan selisihnya dalam satu jurnal koreksi. Barang masuk dan histori migrasi tidak diubah."
        actions={
          <>
            <Button variant="secondary" size="sm" loading={loading} onClick={reload}>
              <RefreshCw aria-hidden /> Periksa ulang
            </Button>
            {canCommit && (
              <Button variant="danger" size="sm" disabled={state !== "ready"} onClick={() => setConfirmOpen(true)}>
                Bukukan koreksi
              </Button>
            )}
          </>
        }
      />

      <div className="mb-4 max-w-xs">
        <FormField label="Tanggal jurnal koreksi" htmlFor="restate-date" helperText="Tidak boleh di masa depan atau di periode tertutup.">
          <DateInput id="restate-date" value={date} max={todayWIB()} onChange={(e) => setDate(e.target.value || todayWIB())} />
        </FormField>
      </div>

      {result?.applied && <CommitResult report={result} />}

      {error ? (
        <ErrorState message={error} onRetry={reload} title="Gagal menghitung pratinjau" />
      ) : loading && !data ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
        </div>
      ) : data ? (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryCard label="Posisi diperiksa" value={formatNumber(data.positions)} />
            <SummaryCard
              label="Baris keluar dikoreksi"
              value={formatNumber(data.rowsChanged)}
              tone={data.rowsChanged > 0 ? "warning" : "success"}
              hint={data.rowsChanged === 0 ? "Semua HPP sudah sesuai rantai biaya." : undefined}
            />
            <SummaryCard
              label="Selisih Persediaan"
              value={formatIDR(data.delta)}
              tone={data.delta === 0 ? "success" : "warning"}
              hint={data.delta < 0 ? "Persediaan turun, akun lawan naik" : data.delta > 0 ? "Persediaan naik, akun lawan turun" : undefined}
            />
            <SummaryCard
              label="Dilewati"
              value={formatNumber(skipped.length)}
              tone={skipped.length > 0 ? "warning" : "success"}
              hint={skipped.length > 0 ? "Posisi berakhir negatif; periksa terpisah." : undefined}
            />
          </div>

          <SectionCard
            title="Gerbang pembukuan"
            description={
              canCommit
                ? "Koreksi hanya bisa dibukukan bila semua gerbang lolos."
                : "Membukukan koreksi butuh izin Keuangan: Tutup Periode Tanpa Gerbang. Halaman ini hanya pratinjau."
            }
          >
            <ul className="divide-y divide-border">
              {(data.gates ?? []).map((g) => (
                <li key={g.key} className="flex items-start gap-3 py-2.5 first:pt-0 last:pb-0">
                  {g.ok ? (
                    <CheckCircle2 aria-hidden className="mt-0.5 size-4 shrink-0 text-success" />
                  ) : (
                    <XCircle aria-hidden className="mt-0.5 size-4 shrink-0 text-destructive" />
                  )}
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-foreground">
                      {gateLabel(g.key)} <span className="sr-only">{g.ok ? "lolos" : "gagal"}</span>
                    </p>
                    <p className="text-sm text-muted-foreground">{g.detail}</p>
                  </div>
                </li>
              ))}
            </ul>
            {state === "blocked" && (
              <p className="mt-3 text-sm text-destructive">
                {failedGates(data.gates).length} gerbang gagal. Selesaikan dulu, lalu periksa ulang.
              </p>
            )}
            <p className="mt-3 text-sm text-muted-foreground">
              {data.cutoverMovementId > 0
                ? `Batas migrasi: mutasi stok #${formatNumber(data.cutoverMovementId)} ke bawah adalah histori sistem lama dan tidak dihitung ulang.`
                : "Tidak ada batas migrasi: semua barang keluar ikut diperiksa."}
            </p>
          </SectionCard>

          <SectionCard flush title="Selisih per produk" description="Nilai posisi sebelum dan sesudah HPP barang keluarnya dihitung ulang.">
            <DataTable<RestateProduct>
              caption="Selisih hitung ulang HPP per produk"
              rows={products}
              rowKey={(r) => `${r.productId}-${r.variantId}`}
              emptyTitle="Tidak ada selisih"
              emptyDescription="HPP setiap barang keluar sudah sama dengan rantai biayanya."
              columns={[
                { header: "Kode", render: (r) => r.productCode || `#${r.productId}` },
                { header: "Produk", render: (r) => r.productName || "-" },
                { header: "Baris", align: "right", render: (r) => formatNumber(r.rows) },
                { header: "Sebelum", align: "right", render: (r) => <Money value={Math.round(r.valueBefore)} /> },
                { header: "Sesudah", align: "right", render: (r) => <Money value={Math.round(r.valueAfter)} /> },
                { header: "Selisih", align: "right", render: (r) => <Money value={r.delta} accounting /> },
                { header: "Akun lawan", render: (r) => offsetSummary(r.offsets, formatIDR) },
              ]}
            />
          </SectionCard>

          {skipped.length > 0 && (
            <SectionCard flush title="Dilewati" description="Posisi yang sengaja tidak disentuh.">
              <DataTable<RestateSkipped>
                caption="Posisi yang dilewati"
                rows={skipped}
                rowKey={(r) => `${r.productId}-${r.variantId}`}
                columns={[
                  { header: "Kode", render: (r) => r.productCode || `#${r.productId}` },
                  { header: "Produk", render: (r) => r.productName || "-" },
                  { header: "Alasan", render: (r) => r.reason },
                ]}
              />
            </SectionCard>
          )}
        </>
      ) : null}

      {confirmOpen && data && (
        <CorrectionReasonModal
          title="Bukukan hitung ulang HPP?"
          description={`${formatNumber(data.rowsChanged)} baris keluar pada ${formatNumber(products.length)} produk ditulis ulang, dan satu jurnal koreksi tertanggal ${date} dibukukan.`}
          confirmLabel="Bukukan koreksi"
          submitting={committing}
          onClose={() => setConfirmOpen(false)}
          onSubmit={(reason) => void handleCommit(reason)}
        >
          <p className="text-sm text-muted-foreground">
            Selisih Persediaan: <Money value={data.delta} accounting className="font-medium text-foreground" />
          </p>
        </CorrectionReasonModal>
      )}
    </Page>
  );
}

function ReconLine({ label, recon }: { label: string; recon?: InventoryRecon }) {
  if (!recon) return null;
  return (
    <SummaryCard
      label={label}
      value={formatIDR(recon.unexplained)}
      tone={recon.consistent ? "success" : "danger"}
      hint={`Buku ${formatIDR(recon.book)}, ledger ${formatIDR(recon.ledger)}`}
    />
  );
}

function CommitResult({ report }: { report: RestateReport }) {
  return (
    <SectionCard title="Koreksi terakhir dibukukan" className="mb-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div>
          <p className="text-sm text-muted-foreground">Jurnal koreksi</p>
          {report.entryId ? (
            <Link to={`${ROUTE_PATHS.financeJournals}/${report.entryId}`} className="font-medium text-primary hover:underline">
              <DocNumber value={report.entryNumber ?? `#${report.entryId}`} />
            </Link>
          ) : (
            <p className="text-sm">Selisih di bawah Rp 1, tanpa jurnal.</p>
          )}
          <p className="mt-1 text-sm text-muted-foreground">
            {formatNumber(report.rowsChanged)} baris, selisih {formatIDR(report.delta)}
          </p>
        </div>
        <ReconLine label="Selisih tak terjelaskan sebelum" recon={report.reconBefore} />
        <ReconLine label="Selisih tak terjelaskan sesudah" recon={report.reconAfter} />
      </div>
    </SectionCard>
  );
}
