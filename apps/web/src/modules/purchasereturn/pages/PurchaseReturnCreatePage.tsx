import { useState } from "react";
import type { FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Button,
  DateInput,
  FormField,
  PageHeader,
  SectionCard,
  SelectInput,
  TableSkeleton,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { LineItemsEditor, Money, Page, Qty, Stepper, StickyFooter } from "@/shared/components/patterns";
import type { LineColumn } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { ReturnLinesTable } from "@/shared/components/ui/return-lines-table";
import { formatIDR, todayWIB } from "@/shared/lib/format";
import { parseQty, switchUom, validateReason, validateReturnDate } from "@/shared/lib/return-lines";
import type { ReturnUom } from "@/shared/lib/return-lines";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import { purchaseReturnSchema } from "@/modules/purchasereturn/schemas/purchase-return.schema";
import {
  purchaseReturnLookupService,
  purchaseReturnService,
} from "@/modules/purchasereturn/services/purchase-return.service";
import { purchaseSettlementOutlook } from "@/modules/purchasereturn/types";
import type { LocationOption, PurchaseReturnPreview } from "@/modules/purchasereturn/types";

interface LineDraft {
  key: string;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  orderedQty: number;
  fulfilled: number;
  returned: number;
  remaining: number;
  /** Satuan baris sumber; `uom` = satuan retur terpilih (temuan 1: bisa per PCS atas baris DUS). */
  orderUom: string;
  uomFactor?: number;
  remainingBase?: number;
  uoms?: ReturnUom[];
  unitPrice: number;
  qty: string;
  locationId: number | null;
}

const STEPS = [
  { key: "source", label: "Pilih dokumen" },
  { key: "items", label: "Pilih barang" },
  { key: "review", label: "Tinjau" },
];

/**
 * Buat retur beli (T3): Stepper 3 langkah. Sumber dari `?from=<purchaseOrderId>` terkunci dan
 * langsung memuat baris. Perilaku dan pratinjau server sama dengan modal lama.
 */
export default function PurchaseReturnCreatePage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const fromParam = Number(params.get("from"));
  const lockedSourceId = Number.isInteger(fromParam) && fromParam > 0 ? fromParam : null;

  usePageCrumb("Baru");

  const [step, setStep] = useState(lockedSourceId ? 1 : 0);
  const [sourceId, setSourceId] = useState<number | null>(lockedSourceId);
  const [lines, setLines] = useState<LineDraft[]>([]);
  const [preview, setPreview] = useState<PurchaseReturnPreview | null>(null);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [returnDate, setReturnDate] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [reason, setReason] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [appliedCtx, setAppliedCtx] = useState<number | null>(null);

  const { data: lookupData, error: lookupError } = useAsyncData(
    () => purchaseReturnLookupService.locationOptions().then((locs) => ({ locations: locs })),
    [],
  );
  const lookupLocations = lookupData?.locations ?? [];

  const { data: lockedOrder } = useAsyncData(
    () => (lockedSourceId ? purchaseReturnLookupService.sourceOrderDetail(lockedSourceId) : Promise.resolve(null)),
    [lockedSourceId],
  );

  // Konteks membawa sisa bisa-diretur per produk (diterima − diretur) +
  // lokasi aktif — pratinjau & clamp memakai angka ini, bukan qty PO.
  // Gagal (mis. 403) → fallback ke detail PO dengan clamp qty order.
  const {
    data: ctxBundle,
    loading: ctxLoading,
    error: ctxFetchError,
  } = useAsyncData(() => {
    if (sourceId === null) return Promise.resolve(null);
    const sid = sourceId;
    return purchaseReturnService.context(sid).then(
      (ctx) => ({
        sourceId: sid,
        lines: ctx.lines.map(
          (it, i): LineDraft => ({
            key: `${it.productId}/${it.variantId}/${i}`,
            productId: it.productId,
            variantId: it.variantId,
            productCode: it.productCode,
            productName: it.productName,
            uom: it.uom,
            orderUom: it.uom,
            uomFactor: it.uomFactor > 0 ? it.uomFactor : 1,
            remainingBase: it.remainingBase,
            uoms: it.uoms,
            orderedQty: it.orderedQty,
            fulfilled: it.received,
            returned: it.returned,
            remaining: it.remaining,
            unitPrice: 0,
            qty: it.remaining > 0 ? String(it.remaining) : "",
            // RT-04: lokasi asal bawaan = lokasi penerimaan terbaru (boleh diganti).
            locationId: it.defaultLocationId ?? null,
          }),
        ),
        firstReceivedAt: (ctx.firstReceivedAt ?? "").slice(0, 10),
        locations: ctx.locations.map(
          (l): LocationOption => ({ id: l.id, code: l.code, name: l.name, status: "active" }),
        ),
        contextError: null as string | null,
      }),
      (err: unknown) => {
        const firstMsg = toApiError(err).message;
        return purchaseReturnLookupService.sourceOrderDetail(sid).then((order) => ({
          sourceId: sid,
          lines: (order.items ?? []).map(
            (it, i): LineDraft => ({
              key: `${it.productId}/${it.variantId}/${i}`,
              productId: it.productId,
              variantId: it.variantId,
              productCode: it.productCode,
              productName: it.productName,
              uom: it.uom,
              orderUom: it.uom,
              orderedQty: it.qty,
              fulfilled: it.qty,
              returned: 0,
              remaining: it.qty,
              unitPrice: it.unitPrice,
              qty: String(it.qty),
              locationId: null,
            }),
          ),
          locations: [] as LocationOption[],
          firstReceivedAt: "",
          contextError: firstMsg as string | null,
        }));
      },
    );
  }, [sourceId]);
  const activeBundle = ctxBundle && ctxBundle.sourceId === sourceId ? ctxBundle : null;
  const contextError = activeBundle ? activeBundle.contextError : ctxFetchError;
  const loadingSource = sourceId === null ? false : ctxLoading;
  const locations = activeBundle && activeBundle.locations.length > 0 ? activeBundle.locations : lookupLocations;

  // Terapkan baris konteks sekali per order sumber terpilih.
  if (activeBundle && appliedCtx !== activeBundle.sourceId) {
    setAppliedCtx(activeBundle.sourceId);
    setLines(activeBundle.lines);
  }

  function handleSourceChange(id: number | null) {
    setSourceId(id);
    setAppliedCtx(null);
    setFormError(null);
    setPreview(null);
    setPreviewError(null);
    if (id === null) setLines([]);
  }

  function patchLine(key: string, patch: Partial<LineDraft>) {
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...patch } : l)));
    setPreview(null);
  }

  const activeLines = lines.filter((l) => parseQty(l.qty) > 0);
  const estimateTotal = lines.reduce((sum, l) => {
    const q = parseQty(l.qty);
    return q <= 0 ? sum : sum + Math.round(q * l.unitPrice);
  }, 0);

  function collectPayload() {
    if (sourceId === null) {
      return { error: "Order beli wajib dipilih." };
    }
    const reasonErr = validateReason(reason);
    if (reasonErr) return { error: reasonErr };
    const dateErr = validateReturnDate(returnDate, todayWIB(), activeBundle?.firstReceivedAt);
    if (dateErr) return { error: dateErr };
    const parsed = purchaseReturnSchema.safeParse({
      purchaseOrderId: sourceId,
      returnDate: returnDate.trim(),
      notes: notes.trim(),
      reason: reason.trim(),
      items: lines
        .filter((l) => parseQty(l.qty) > 0)
        .map((l) => ({
          productId: l.productId,
          variantId: l.variantId,
          locationId: l.locationId,
          uom: l.uom,
          qty: Number(l.qty),
        })),
    });
    if (!parsed.success) {
      return { error: parsed.error.issues[0]?.message ?? "Form belum valid." };
    }
    return {
      data: {
        purchaseOrderId: parsed.data.purchaseOrderId,
        returnDate: parsed.data.returnDate || undefined,
        notes: parsed.data.notes || undefined,
        reason: parsed.data.reason,
        items: parsed.data.items.map((it) => ({
          productId: it.productId,
          variantId: it.variantId,
          // Schema menjamin locationId positif.
          locationId: it.locationId as number,
          uom: it.uom,
          qty: it.qty,
        })),
      },
    };
  }

  function handlePreview() {
    const collected = collectPayload();
    if (collected.error || !collected.data) {
      setPreviewError(collected.error ?? "Form belum valid.");
      setPreview(null);
      return;
    }
    setPreviewError(null);
    setPreviewBusy(true);
    purchaseReturnService
      .preview({ purchaseOrderId: collected.data.purchaseOrderId, items: collected.data.items })
      .then((res) => setPreview(res.data))
      .catch((err) => {
        setPreview(null);
        setPreviewError(formatApiError(err));
      })
      .finally(() => setPreviewBusy(false));
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (step < 2) return;
    const collected = collectPayload();
    if (collected.error || !collected.data) {
      setFormError(collected.error ?? "Form belum valid.");
      return;
    }
    setFormError(null);
    setBusy(true);
    purchaseReturnService
      .create({
        purchaseOrderId: collected.data.purchaseOrderId,
        returnDate: collected.data.returnDate,
        notes: collected.data.notes,
        reason: collected.data.reason,
        items: collected.data.items,
      })
      .then((res) => {
        toast.fromServer(res.message, "Retur pembelian dibuat", res.data.number);
        void navigate(`${ROUTE_PATHS.purchaseReturns}/${res.data.id}`);
      })
      .catch((err) => setFormError(formatApiError(err)))
      .finally(() => setBusy(false));
  }

  const cancelTo = lockedSourceId ? `${ROUTE_PATHS.purchaseOrders}/${lockedSourceId}` : ROUTE_PATHS.purchaseReturns;
  const canNext = step === 0 ? sourceId !== null && !loadingSource : step === 1 ? activeLines.length > 0 : false;
  const nextHint =
    step === 0 ? "Pilih order beli sumber." : step === 1 && activeLines.length === 0 ? "Isi qty retur minimal satu baris." : "";

  const reviewColumns: LineColumn<LineDraft>[] = [
    {
      id: "product",
      header: "Produk",
      render: (r) => (
        <div className="py-1">
          <p className="font-medium text-foreground">{r.productName}</p>
          <p className="text-xs text-muted-foreground">
            {r.productCode} ·{" "}
            {locations.find((l) => l.id === r.locationId)
              ? `keluar dari ${locations.find((l) => l.id === r.locationId)?.code}`
              : "lokasi belum dipilih"}
          </p>
        </div>
      ),
    },
    {
      id: "qty",
      header: "Qty retur",
      align: "right",
      width: "8rem",
      render: (r) => (
        <span className="inline-block py-1.5">
          <Qty value={Number(r.qty)} unit={r.uom} />
        </span>
      ),
    },
  ];

  return (
    <Page>
      <PageHeader
        title="Buat retur beli"
        description="Retur dibuat sebagai draf terhadap order beli terkonfirmasi atau selesai."
      />

      <div className="rounded-xl border border-border bg-card px-5 py-4">
        <Stepper steps={STEPS} current={STEPS[step]?.key ?? "source"} />
      </div>

      {lookupError && <Notice tone="danger" title={lookupError} />}

      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        {step === 0 && (
          <SectionCard title="Pilih dokumen" description="Order beli sumber yang barangnya akan diretur.">
            <div className="max-w-xl">
              <FormField label="Order beli sumber" required>
                {lockedSourceId ? (
                  <TextInput
                    aria-label="Order beli sumber"
                    value={
                      lockedOrder
                        ? `${lockedOrder.number}${lockedOrder.partyName ? ` — ${lockedOrder.partyName}` : ""}`
                        : `#${lockedSourceId}`
                    }
                    readOnly
                    disabled
                  />
                ) : (
                  <PurchaseOrderSearchSelect
                    value={sourceId}
                    status="confirmed,completed"
                    placeholder="Cari order (nomor atau supplier)…"
                    disabled={busy}
                    onChange={handleSourceChange}
                  />
                )}
              </FormField>
            </div>
          </SectionCard>
        )}

        {step === 1 && (
          <SectionCard
            title="Barang yang diretur"
            description="Qty dibatasi sisa yang benar-benar bisa diretur (diterima − sudah diretur) dari konteks order."
          >
            {contextError && (
              <div className="mb-4">
                <Notice tone="warning" title="Konteks sisa tidak tersedia — memakai qty order">
                  {contextError}
                </Notice>
              </div>
            )}
            {loadingSource ? (
              <TableSkeleton rows={3} cols={4} />
            ) : (
              <ReturnLinesTable<LineDraft>
                rows={lines}
                disabled={busy}
                fulfilledLabel="diterima"
                emptyDescription="Pilih order beli sumber untuk memuat baris item."
                onQtyChange={(key, qty) => patchLine(key, { qty })}
                onUomChange={(key, uom) => {
                  const row = lines.find((l) => l.key === key);
                  if (row) patchLine(key, switchUom(row, uom));
                }}
                extraColumns={[
                  {
                    header: "Lokasi keluar",
                    render: (r) => (
                      <SelectInput
                        aria-label={`Lokasi keluar ${r.productName}`}
                        value={r.locationId ?? ""}
                        disabled={busy}
                        onChange={(e) =>
                          patchLine(r.key, { locationId: e.target.value === "" ? null : Number(e.target.value) })
                        }
                      >
                        <option value="">Pilih…</option>
                        {locations.map((l) => (
                          <option key={l.id} value={l.id}>
                            {l.code} — {l.name}
                          </option>
                        ))}
                      </SelectInput>
                    ),
                  },
                ]}
              />
            )}
          </SectionCard>
        )}

        {step === 2 && (
          <>
            <SectionCard title="Rincian retur">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField label="Alasan retur" required helperText="Wajib, maksimal 255 karakter.">
                  <TextInput
                    value={reason}
                    maxLength={255}
                    onChange={(e) => setReason(e.target.value)}
                    placeholder="Mis. barang cacat dari supplier"
                    disabled={busy}
                  />
                </FormField>
                <FormField
                  label="Tanggal retur"
                  helperText={
                    activeBundle?.firstReceivedAt
                      ? `Tidak lebih awal dari tanggal terima (${activeBundle.firstReceivedAt}). Kosongkan untuk hari ini.`
                      : "Kosongkan untuk hari ini (server)."
                  }
                >
                  <DateInput
                    value={returnDate}
                    min={activeBundle?.firstReceivedAt || undefined}
                    max={todayWIB()}
                    onChange={(e) => setReturnDate(e.target.value)}
                    disabled={busy}
                  />
                </FormField>
                <div className="sm:col-span-2">
                  <FormField label="Catatan">
                    <TextArea
                      value={notes}
                      onChange={(e) => setNotes(e.target.value)}
                      placeholder="Catatan tambahan (opsional)"
                      rows={2}
                      disabled={busy}
                    />
                  </FormField>
                </div>
              </div>
            </SectionCard>

            <SectionCard title="Barang diretur">
              <LineItemsEditor<LineDraft>
                lines={activeLines}
                getKey={(r) => r.key}
                columns={reviewColumns}
                readOnly
                emptyText="Tidak ada baris dengan qty retur."
              />
            </SectionCard>

            <div>
              <div className="flex flex-wrap items-center gap-3">
                <Button
                  variant="secondary"
                  onClick={handlePreview}
                  disabled={busy || previewBusy || loadingSource || sourceId === null}
                >
                  {previewBusy ? "Mempratinjau…" : "Pratinjau"}
                </Button>
                <span className="text-xs text-muted-foreground">
                  Hitung subtotal, diskon, PPN, dan cara nilai retur akan diselesaikan di server tanpa menyimpan.
                </span>
              </div>
              {previewError && (
                <div className="mt-3">
                  <Notice tone="danger" title={previewError} />
                </div>
              )}
              {preview && (
                <div className="mt-3">
                  <Notice tone="info" title="Pratinjau — belum disimpan">
                    <span className="mt-1 block">
                      Subtotal {formatIDR(preview.subtotal)} · Diskon {formatIDR(preview.discountTotal)} · PPN{" "}
                      {formatIDR(preview.taxTotal)} · Total{" "}
                      <span className="font-semibold">{formatIDR(preview.total)}</span>
                    </span>
                    <ul className="mt-2 list-disc space-y-0.5 pl-5" aria-label="Penyelesaian nilai retur">
                      {purchaseSettlementOutlook(preview, formatIDR).map((line) => (
                        <li key={line}>{line}</li>
                      ))}
                    </ul>
                  </Notice>
                </div>
              )}
            </div>
          </>
        )}

        {formError && <Notice tone="danger" title={formError} />}

        <StickyFooter
          summary={
            step > 0 && lines.length > 0 ? (
              <span>
                {activeLines.length} baris · Estimasi total <Money value={estimateTotal} className="font-semibold text-foreground" />
                <span className="hidden sm:inline"> (harga final dihitung server — gunakan Pratinjau)</span>
              </span>
            ) : (
              nextHint
            )
          }
        >
          <Button variant="secondary" onClick={() => void navigate(cancelTo)} disabled={busy}>
            Batal
          </Button>
          {step > (lockedSourceId ? 1 : 0) && (
            <Button variant="secondary" onClick={() => setStep(step - 1)} disabled={busy}>
              Kembali
            </Button>
          )}
          {step < 2 ? (
            <Button onClick={() => setStep(step + 1)} disabled={!canNext}>
              Lanjut
            </Button>
          ) : (
            <Button type="submit" loading={busy} disabled={loadingSource || lines.length === 0}>
              {busy ? "Menyimpan…" : "Simpan draf"}
            </Button>
          )}
        </StickyFooter>
      </form>
    </Page>
  );
}
