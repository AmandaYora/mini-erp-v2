import { useState } from "react";
import { Link } from "react-router-dom";
import { ArrowRight } from "lucide-react";
import {
  Button,
  DataTable,
  DateInput,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { DescriptionList, DocNumber, Money, Page, Qty } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatDate, formatDateTime, formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { ProductSearchSelect } from "@/modules/products/components/ProductSearchSelect";
import { correctionsService } from "@/modules/finance/services/corrections.service";
import { CorrectionReasonModal } from "@/modules/finance/components/CorrectionReasonModal";
import { newCorrectionRef } from "@/modules/finance/lib/corrections";
import { revaluationCostSchema } from "@/modules/finance/schemas/corrections.schema";
import type { RevaluationPosition, RevaluationResult } from "@/modules/finance/types";

const HISTORY_LIMIT = 20;

// Revaluasi Harga Modal: koreksi harga modal non-nol yang salah (saldo awal / barang masuk salah harga).
// Satu baris nilai di ledger biaya + satu jurnal Persediaan vs Selisih Stok; komit butuh
// finance.close.force dan alasan. Barang keluar berikutnya memakai rata-rata baru.
export default function InventoryRevaluationPage() {
  const can = useAuthStore((s) => s.can);
  const canCommit = can("finance.close.force");

  const [productId, setProductId] = useState<number | null>(null);
  const [variantId, setVariantId] = useState<number | null>(null);
  const [cost, setCost] = useState("");
  const [costError, setCostError] = useState<string | undefined>(undefined);
  const [date, setDate] = useState(todayWIB());
  const [preview, setPreview] = useState<{ key: string; result: RevaluationResult } | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [clientRef, setClientRef] = useState(newCorrectionRef);
  const [last, setLast] = useState<RevaluationResult | null>(null);
  const [page, setPage] = useState(1);

  const positions = useAsyncData(
    () => (productId ? correctionsService.revaluationPositions(productId).then((r) => r ?? []) : Promise.resolve([])),
    [productId],
  );
  const history = useAsyncData(() => correctionsService.listRevaluations(page, HISTORY_LIMIT), [page]);

  const held = (positions.data ?? []).filter((p) => p.qty > 0);
  const position: RevaluationPosition | undefined =
    held.find((p) => p.variantId === variantId) ?? (held.length === 1 ? held[0] : undefined);
  const inputKey = position ? `${position.productId}:${position.variantId}:${cost}:${date}` : "";
  const current = preview && preview.key === inputKey ? preview.result : null;

  function selectProduct(id: number | null) {
    setProductId(id);
    setVariantId(null);
    setCost("");
    setCostError(undefined);
    setPreview(null);
  }

  async function handlePreview() {
    if (!position) return;
    const parsed = revaluationCostSchema.safeParse({ newUnitCost: cost });
    if (!parsed.success || cost.trim() === "") {
      setCostError(parsed.success ? "Harga modal baru wajib diisi" : parsed.error.flatten().fieldErrors.newUnitCost?.[0]);
      return;
    }
    setCostError(undefined);
    setPreviewing(true);
    try {
      const res = await correctionsService.revaluationPreview({
        productId: position.productId,
        variantId: position.variantId,
        newUnitCost: parsed.data.newUnitCost,
        date,
      });
      setPreview({ key: inputKey, result: res.data });
    } catch (err) {
      setPreview(null);
      toast.danger("Pratinjau revaluasi gagal", formatApiError(err));
    } finally {
      setPreviewing(false);
    }
  }

  async function handleCommit(reason: string) {
    if (!position || !current) return;
    setCommitting(true);
    try {
      const res = await correctionsService.revaluationCommit({
        productId: position.productId,
        variantId: position.variantId,
        newUnitCost: Number(cost),
        date,
        reason,
        clientRef,
      });
      toast.fromServer(res.message, "Harga modal direvaluasi", res.data.entryNumber ? `Jurnal ${res.data.entryNumber}` : undefined);
      setLast(res.data);
      setConfirmOpen(false);
      setClientRef(newCorrectionRef());
      setPreview(null);
      setCost("");
      positions.reload();
      if (page === 1) history.reload();
      else setPage(1);
    } catch (err) {
      toast.danger("Gagal merevaluasi harga modal", formatApiError(err));
    } finally {
      setCommitting(false);
    }
  }

  const line = current?.lines?.[0];

  return (
    <Page>
      <PageHeader
        title="Revaluasi Harga Modal"
        description="Untuk harga modal yang terlanjur salah (saldo awal atau barang masuk salah harga). Nilai persediaan produk disetel ke stok × harga baru; selisihnya dibukukan ke Selisih Stok, dan barang keluar berikutnya memakai harga baru."
      />

      {last?.applied && <LastResult result={last} />}

      <SectionCard title="Koreksi harga modal" description="Pilih produk, isi harga modal yang benar per satuan dasar, lalu lihat pratinjaunya.">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <FormField label="Produk" required>
            <ProductSearchSelect value={productId} trackedOnly onChange={(id) => selectProduct(id)} />
          </FormField>
          {held.length > 1 && (
            <FormField label="Varian" htmlFor="reval-variant" required>
              <SelectInput
                id="reval-variant"
                value={variantId ?? ""}
                onChange={(e) => {
                  setVariantId(e.target.value ? Number(e.target.value) : null);
                  setPreview(null);
                }}
              >
                <option value="">Pilih varian…</option>
                {held.map((p) => (
                  <option key={p.variantId} value={p.variantId}>
                    {p.variantName || `Varian #${p.variantId}`}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          )}
        </div>

        {productId && positions.error ? (
          <div className="mt-4">
            <ErrorState message={positions.error} onRetry={positions.reload} title="Gagal membaca posisi persediaan" />
          </div>
        ) : productId && !positions.loading && held.length === 0 ? (
          <p className="mt-4 text-sm text-warning">Produk ini tidak punya stok di cabang aktif. Revaluasi hanya untuk produk berstok.</p>
        ) : null}

        {position && (
          <>
            <DescriptionList
              className="mt-5"
              columns={4}
              items={[
                { label: "Stok", value: <Qty value={position.qty} unit={position.baseUom} /> },
                { label: "Nilai persediaan", value: <Money value={position.value} /> },
                { label: `Harga modal rata-rata / ${position.baseUom || "satuan"}`, value: <Money value={Math.round(position.averageCost)} /> },
                {
                  label: "Harga referensi",
                  value: (
                    <span className="text-sm">
                      Beli terakhir {position.lastReceiptCost > 0 ? formatIDR(Math.round(position.lastReceiptCost)) : "belum ada"}
                      <br />
                      Master {position.catalogCost > 0 ? formatIDR(Math.round(position.catalogCost)) : "belum diisi"}
                    </span>
                  ),
                },
              ]}
            />
            <div className="mt-5 grid grid-cols-1 items-start gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
              <FormField
                label={`Harga modal baru / ${position.baseUom || "satuan"}`}
                htmlFor="reval-cost"
                required
                errorText={costError}
              >
                <TextInput
                  id="reval-cost"
                  inputMode="numeric"
                  value={cost}
                  onChange={(e) => setCost(e.target.value.replace(/[^\d]/g, ""))}
                  placeholder="Rupiah bulat, mis. 9000"
                />
              </FormField>
              <FormField label="Tanggal jurnal" htmlFor="reval-date">
                <DateInput id="reval-date" value={date} max={todayWIB()} onChange={(e) => setDate(e.target.value || todayWIB())} />
              </FormField>
              <div className="md:pt-6">
                <Button variant="secondary" loading={previewing} onClick={() => void handlePreview()}>
                  Pratinjau
                </Button>
              </div>
            </div>
          </>
        )}

        {line && current && (
          <div className="mt-5 rounded-lg border border-border bg-muted/40 p-4">
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
              <div>
                <p className="text-sm text-muted-foreground">Nilai sekarang</p>
                <p className="font-display text-xl tabular">{formatIDR(line.valueBefore)}</p>
              </div>
              <ArrowRight aria-hidden className="size-5 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">Nilai baru</p>
                <p className="font-display text-xl tabular">{formatIDR(line.valueAfter)}</p>
              </div>
              <div className="ml-auto text-right">
                <p className="text-sm text-muted-foreground">Selisih dibukukan</p>
                <p className={`font-display text-xl tabular ${line.delta < 0 ? "text-destructive" : "text-success"}`}>
                  {line.delta > 0 ? "+" : ""}
                  {formatIDR(line.delta)}
                </p>
              </div>
            </div>
            <p className="mt-3 text-sm text-muted-foreground">
              {line.delta > 0
                ? "Persediaan didebit, Selisih Stok dikredit."
                : "Persediaan dikredit, Selisih Stok didebit."}{" "}
              Harga modal {formatIDR(Math.round(line.unitCostBefore))} menjadi {formatIDR(line.unitCostAfter)} per {position?.baseUom || "satuan"} untuk{" "}
              {formatNumber(line.qty)} {position?.baseUom}.
            </p>
            <div className="mt-4 flex flex-wrap items-center gap-3">
              {canCommit ? (
                <Button variant="danger" onClick={() => setConfirmOpen(true)}>
                  Bukukan revaluasi
                </Button>
              ) : (
                <p className="text-sm text-muted-foreground">
                  Membukukan revaluasi butuh izin Keuangan: Tutup Periode Tanpa Gerbang.
                </p>
              )}
            </div>
          </div>
        )}
      </SectionCard>

      <SectionCard flush title="Riwayat revaluasi" description="Revaluasi yang sudah dibukukan di cabang ini, terbaru dulu.">
        {history.error ? (
          <ErrorState message={history.error} onRetry={history.reload} title="Gagal memuat riwayat" />
        ) : (
          <>
            <DataTable<RevaluationResult>
              caption="Riwayat revaluasi harga modal"
              rows={history.data?.items}
              loading={history.loading}
              rowKey={(r) => String(r.id)}
              emptyTitle="Belum ada revaluasi"
              emptyDescription="Revaluasi yang dibukukan akan tercatat di sini bersama jurnal dan alasannya."
              columns={[
                { header: "Tanggal", render: (r) => formatDate(r.date) },
                {
                  header: "Produk",
                  render: (r) =>
                    (r.lines ?? [])
                      .map((l) => `${l.productCode || `#${l.productId}`}: ${formatIDR(Math.round(l.unitCostBefore))} → ${formatIDR(l.unitCostAfter)}`)
                      .join("; ") || "-",
                },
                { header: "Selisih", align: "right", render: (r) => <Money value={r.totalDelta} accounting /> },
                {
                  header: "Jurnal",
                  render: (r) =>
                    r.entryId ? (
                      <Link to={`${ROUTE_PATHS.financeJournals}/${r.entryId}`} className="font-medium text-primary hover:underline">
                        <DocNumber value={r.entryNumber || `#${r.entryId}`} />
                      </Link>
                    ) : (
                      "-"
                    ),
                },
                { header: "Alasan", render: (r) => r.reason },
                { header: "Dicatat", render: (r) => formatDateTime(r.createdAt) },
              ]}
            />
            {history.data && history.data.meta.total > HISTORY_LIMIT && (
              <Pagination page={page} limit={HISTORY_LIMIT} total={history.data.meta.total} onPageChange={setPage} />
            )}
          </>
        )}
      </SectionCard>

      {confirmOpen && line && (
        <CorrectionReasonModal
          title="Bukukan revaluasi harga modal?"
          description={`${line.productCode} ${line.productName}: nilai persediaan ${formatIDR(line.valueBefore)} menjadi ${formatIDR(line.valueAfter)}. Satu jurnal tertanggal ${date} dibukukan.`}
          confirmLabel="Bukukan revaluasi"
          submitting={committing}
          onClose={() => setConfirmOpen(false)}
          onSubmit={(reason) => void handleCommit(reason)}
        />
      )}
    </Page>
  );
}

function LastResult({ result }: { result: RevaluationResult }) {
  return (
    <SectionCard title="Revaluasi terakhir dibukukan" className="mb-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div>
          <p className="text-sm text-muted-foreground">Jurnal</p>
          {result.entryId ? (
            <Link to={`${ROUTE_PATHS.financeJournals}/${result.entryId}`} className="font-medium text-primary hover:underline">
              <DocNumber value={result.entryNumber || `#${result.entryId}`} />
            </Link>
          ) : (
            "-"
          )}
          <p className="mt-1 text-sm text-muted-foreground">Selisih {formatIDR(result.totalDelta)}</p>
        </div>
        {result.reconBefore && (
          <div>
            <p className="text-sm text-muted-foreground">Selisih tak terjelaskan sebelum</p>
            <p className="tabular">{formatIDR(result.reconBefore.unexplained)}</p>
          </div>
        )}
        {result.reconAfter && (
          <div>
            <p className="text-sm text-muted-foreground">Selisih tak terjelaskan sesudah</p>
            <p className={`tabular ${result.reconAfter.consistent ? "text-success" : "text-destructive"}`}>
              {formatIDR(result.reconAfter.unexplained)}
              {result.reconAfter.consistent ? " (buku = ledger)" : " (periksa Integritas)"}
            </p>
          </div>
        )}
      </div>
    </SectionCard>
  );
}
