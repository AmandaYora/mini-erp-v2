import { useState } from "react";
import type { FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Button,
  DateInput,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
  SelectInput,
  TableSkeleton,
  TextArea,
  TextInput,
  ToggleGroup,
  ToggleGroupItem,
  INPUT,
} from "@/shared/components/ui";
import { LineItemsEditor, Money, Page, Qty, Stepper, StickyFooter } from "@/shared/components/patterns";
import type { LineColumn } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { ReturnLinesTable } from "@/shared/components/ui/return-lines-table";
import { formatIDR, todayWIB } from "@/shared/lib/format";
import { parseQty, validateReason, validateReturnDate } from "@/shared/lib/return-lines";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { salesReturnSchema } from "@/modules/salesreturn/schemas/sales-return.schema";
import { salesReturnLookupService, salesReturnService } from "@/modules/salesreturn/services/sales-return.service";
import {
  RETURN_CONDITIONS,
  returnConditionLabel,
  returnDifferenceLabel,
  salesSettlementOutlook,
} from "@/modules/salesreturn/types";
import type {
  LocationOption,
  ReplacementProductOption,
  ReturnCondition,
  SalesReturnPreview,
} from "@/modules/salesreturn/types";

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
  unitPrice: number;
  qty: string;
  locationId: number | null;
  condition: ReturnCondition;
}

interface ReplDraft {
  key: string;
  productId: number | null;
  productLabel: string;
  variants: { id: number; code: string; name: string }[];
  variantId: number | null;
  uoms: string[];
  uom: string;
  qty: string;
  /** Kosong = harga katalog (server). */
  unitPrice: string;
  locationId: number | null;
  loadingDetail: boolean;
}

type ReturnMode = "return_only" | "exchange";

const STEPS = [
  { key: "source", label: "Pilih dokumen" },
  { key: "items", label: "Pilih barang & mode" },
  { key: "review", label: "Tinjau" },
];

let replKey = 1;

function uniqueUoms(p: { baseUom: string; purchaseUom: string; salesUom: string }): string[] {
  return [p.salesUom, p.purchaseUom, p.baseUom].filter((u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i);
}

/**
 * Buat retur jual (T3): Stepper 3 langkah. Sumber dari `?from=<salesOrderId>` terkunci dan
 * langsung memuat baris. Perilaku mode return_only/exchange dan pratinjau server sama dengan modal lama.
 */
export default function SalesReturnCreatePage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const fromParam = Number(params.get("from"));
  const lockedSourceId = Number.isInteger(fromParam) && fromParam > 0 ? fromParam : null;

  usePageCrumb("Baru");

  const [step, setStep] = useState(lockedSourceId ? 1 : 0);
  const [sourceId, setSourceId] = useState<number | null>(lockedSourceId);
  const [lines, setLines] = useState<LineDraft[]>([]);
  const [returnMode, setReturnMode] = useState<ReturnMode>("return_only");
  const [replacements, setReplacements] = useState<ReplDraft[]>([]);
  const [preview, setPreview] = useState<SalesReturnPreview | null>(null);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [returnDate, setReturnDate] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [reason, setReason] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [appliedCtx, setAppliedCtx] = useState<number | null>(null);

  const { data: lookupData, error: lookupError } = useAsyncData(
    () =>
      Promise.all([
        salesReturnLookupService.locationOptions(),
        // Best-effort: butuh perm products.view; editor pengganti tetap bisa
        // dibuka dan menampilkan error saat produk dipilih bila gagal.
        salesReturnLookupService.productOptions().catch(() => [] as ReplacementProductOption[]),
      ]).then(([locs, prods]) => ({ locations: locs, products: prods })),
    [],
  );
  const lookupLocations = lookupData?.locations ?? [];
  const productOptions = lookupData?.products ?? [];

  // Label order sumber terkunci (nomor + pelanggan).
  const { data: lockedOrder } = useAsyncData(
    () => (lockedSourceId ? salesReturnLookupService.sourceOrderDetail(lockedSourceId) : Promise.resolve(null)),
    [lockedSourceId],
  );

  // Konteks membawa sisa bisa-diretur per produk (terkirim − diretur) +
  // lokasi aktif — pratinjau & clamp memakai angka ini, bukan qty order.
  // Gagal (mis. 403) → fallback ke detail SO dengan clamp qty order.
  const {
    data: ctxBundle,
    loading: ctxLoading,
    error: ctxFetchError,
  } = useAsyncData(() => {
    if (sourceId === null) return Promise.resolve(null);
    const sid = sourceId;
    return salesReturnService.context(sid).then(
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
            orderedQty: it.orderedQty,
            fulfilled: it.delivered,
            returned: it.returned,
            remaining: it.remaining,
            unitPrice: 0,
            qty: it.remaining > 0 ? String(it.remaining) : "",
            locationId: null,
            condition: "normal",
          }),
        ),
        locations: ctx.locations.map(
          (l): LocationOption => ({ id: l.id, code: l.code, name: l.name, status: "active" }),
        ),
        contextError: null as string | null,
      }),
      (err: unknown) => {
        // Context butuh salesreturn.view; bila gagal (mis. 403) tampilkan
        // alasan dan fallback ke detail SO dengan clamp qty order.
        const firstMsg = toApiError(err).message;
        return salesReturnLookupService.sourceOrderDetail(sid).then((order) => ({
          sourceId: sid,
          lines: (order.items ?? []).map(
            (it, i): LineDraft => ({
              key: `${it.productId}/${it.variantId}/${i}`,
              productId: it.productId,
              variantId: it.variantId,
              productCode: it.productCode,
              productName: it.productName,
              uom: it.uom,
              orderedQty: it.qty,
              fulfilled: it.qty,
              returned: 0,
              remaining: it.qty,
              unitPrice: it.unitPrice,
              qty: String(it.qty),
              locationId: null,
              condition: "normal",
            }),
          ),
          locations: [] as LocationOption[],
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

  function handleModeChange(v: string) {
    if (v === "") return; // ToggleGroup tunggal: abaikan klik ulang yang membatalkan pilihan.
    setReturnMode(v === "exchange" ? "exchange" : "return_only");
    if (v !== "exchange") setReplacements([]);
    setPreview(null);
    setPreviewError(null);
    setFormError(null);
  }

  function addReplacementRow() {
    setReplacements((prev) => [
      ...prev,
      {
        key: `repl-${replKey++}`,
        productId: null,
        productLabel: "",
        variants: [],
        variantId: null,
        uoms: [],
        uom: "",
        qty: "1",
        unitPrice: "",
        locationId: null,
        loadingDetail: false,
      },
    ]);
    setPreview(null);
  }

  function patchRepl(key: string, patch: Partial<ReplDraft>) {
    setReplacements((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
    setPreview(null);
  }

  function removeReplAt(index: number) {
    setReplacements((prev) => prev.filter((_, i) => i !== index));
    setPreview(null);
  }

  function handleReplProduct(key: string, v: string | number | null) {
    const pid = typeof v === "number" ? v : v === null ? null : Number(v);
    if (pid === null || Number.isNaN(pid)) {
      patchRepl(key, { productId: null, productLabel: "", variants: [], variantId: null, uoms: [], uom: "" });
      return;
    }
    const opt = productOptions.find((p) => p.id === pid);
    patchRepl(key, {
      productId: pid,
      productLabel: opt ? `${opt.code} — ${opt.name}` : `#${pid}`,
      loadingDetail: true,
    });
    salesReturnLookupService
      .productDetail(pid)
      .then((d) => {
        const uoms = uniqueUoms(d);
        const def = d.variants.find((x) => x.id > 0);
        patchRepl(key, {
          variants: d.variants.map((x) => ({ id: x.id, code: x.code, name: x.name })),
          variantId: def ? def.id : null,
          uoms,
          uom: d.salesUom || uoms[0] || "",
          loadingDetail: false,
        });
      })
      .catch((err) => {
        patchRepl(key, { productId: null, productLabel: "", loadingDetail: false });
        toast.danger("Gagal memuat varian produk", toApiError(err).message);
      });
  }

  const activeLines = lines.filter((l) => parseQty(l.qty) > 0);
  const estimateTotal = lines.reduce((sum, l) => {
    const q = parseQty(l.qty);
    if (q <= 0) return sum;
    return sum + Math.round(q * l.unitPrice);
  }, 0);

  function collectPayload() {
    if (sourceId === null) {
      return { error: "Order jual wajib dipilih." };
    }
    const reasonErr = validateReason(reason);
    if (reasonErr) return { error: reasonErr };
    const dateErr = validateReturnDate(returnDate, todayWIB());
    if (dateErr) return { error: dateErr };
    const parsed = salesReturnSchema.safeParse({
      salesOrderId: sourceId,
      returnDate: returnDate.trim(),
      notes: notes.trim(),
      reason: reason.trim(),
      returnMode,
      items: lines
        .filter((l) => parseQty(l.qty) > 0)
        .map((l) => ({
          productId: l.productId,
          variantId: l.variantId,
          // damaged/not_returned: lokasi ditentukan server / tidak ada.
          locationId: l.condition === "normal" ? l.locationId : null,
          uom: l.uom,
          qty: Number(l.qty),
          condition: l.condition,
        })),
      replacementItems:
        returnMode === "exchange"
          ? replacements
              .filter((r) => (r.productId ?? 0) > 0 && parseQty(r.qty) > 0)
              .map((r) => ({
                productId: r.productId as number,
                variantId: r.variantId ?? 0,
                locationId: r.locationId,
                uom: r.uom,
                qty: Number(r.qty),
                unitPrice: r.unitPrice.trim() === "" ? undefined : Number(r.unitPrice),
              }))
          : [],
    });
    if (!parsed.success) {
      return { error: parsed.error.issues[0]?.message ?? "Form belum valid." };
    }
    return {
      data: {
        salesOrderId: parsed.data.salesOrderId,
        returnDate: parsed.data.returnDate || undefined,
        notes: parsed.data.notes || undefined,
        reason: parsed.data.reason,
        returnMode: parsed.data.returnMode,
        items: parsed.data.items.map((it) => ({
          productId: it.productId,
          variantId: it.variantId,
          locationId: it.locationId ?? undefined,
          uom: it.uom,
          qty: it.qty,
          condition: it.condition,
        })),
        replacementItems: parsed.data.replacementItems.map((it) => ({
          productId: it.productId,
          variantId: it.variantId,
          locationId: it.locationId ?? undefined,
          uom: it.uom,
          qty: it.qty,
          unitPrice: it.unitPrice,
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
    salesReturnService
      .preview({
        salesOrderId: collected.data.salesOrderId,
        returnMode: collected.data.returnMode,
        items: collected.data.items,
        replacementItems: collected.data.replacementItems.length > 0 ? collected.data.replacementItems : undefined,
      })
      .then((res) => setPreview(res.data))
      .catch((err) => {
        setPreview(null);
        setPreviewError(toApiError(err).message);
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
    salesReturnService
      .create({
        salesOrderId: collected.data.salesOrderId,
        returnDate: collected.data.returnDate,
        notes: collected.data.notes,
        reason: collected.data.reason,
        returnMode: collected.data.returnMode,
        items: collected.data.items,
        replacementItems: collected.data.replacementItems.length > 0 ? collected.data.replacementItems : undefined,
      })
      .then((res) => {
        toast.fromServer(res.message, "Retur penjualan dibuat", res.data.number);
        void navigate(`${ROUTE_PATHS.salesReturns}/${res.data.id}`);
      })
      .catch((err) => setFormError(toApiError(err).message))
      .finally(() => setBusy(false));
  }

  const isExchange = returnMode === "exchange";
  const cancelTo = lockedSourceId ? `${ROUTE_PATHS.salesOrders}/${lockedSourceId}` : ROUTE_PATHS.salesReturns;

  const canNext = step === 0 ? sourceId !== null && !loadingSource : step === 1 ? activeLines.length > 0 : false;
  const nextHint =
    step === 0
      ? "Pilih order jual sumber."
      : step === 1 && activeLines.length === 0
        ? "Isi qty retur minimal satu baris."
        : "";

  const replColumns: LineColumn<ReplDraft>[] = [
    {
      id: "product",
      header: "Produk",
      width: "14rem",
      render: (r, i) => (
        <div>
          <SearchSelect
            aria-label={`Produk pengganti baris ${i + 1}`}
            options={productOptions.map((p) => ({ value: p.id, label: `${p.code} — ${p.name}` }))}
            value={r.productId}
            placeholder="Pilih produk…"
            disabled={busy || r.loadingDetail}
            onChange={(v) => handleReplProduct(r.key, v)}
          />
          {r.loadingDetail && <p className="mt-1 text-xs text-muted-foreground">Memuat varian…</p>}
        </div>
      ),
    },
    {
      id: "variant",
      header: "Varian",
      width: "10rem",
      render: (r) => (
        <SelectInput
          aria-label="Varian pengganti"
          value={r.variantId ?? ""}
          disabled={busy || r.productId === null}
          onChange={(e) => patchRepl(r.key, { variantId: e.target.value === "" ? null : Number(e.target.value) })}
        >
          <option value="">Pilih…</option>
          {r.variants.map((v) => (
            <option key={v.id} value={v.id}>
              {v.code} — {v.name}
            </option>
          ))}
        </SelectInput>
      ),
    },
    {
      id: "uom",
      header: "Satuan",
      width: "7rem",
      render: (r) =>
        r.uoms.length > 0 ? (
          <SelectInput
            aria-label="Satuan pengganti"
            value={r.uom}
            disabled={busy}
            onChange={(e) => patchRepl(r.key, { uom: e.target.value })}
          >
            <option value="">Pilih…</option>
            {r.uoms.map((u) => (
              <option key={u} value={u}>
                {u}
              </option>
            ))}
          </SelectInput>
        ) : (
          <TextInput
            aria-label="Satuan pengganti"
            value={r.uom}
            disabled={busy || r.productId === null}
            placeholder="Satuan"
            onChange={(e) => patchRepl(r.key, { uom: e.target.value })}
          />
        ),
    },
    {
      id: "qty",
      header: "Qty",
      width: "6rem",
      align: "right",
      render: (r) => (
        <input
          type="number"
          min={0}
          step="any"
          value={r.qty}
          disabled={busy}
          aria-label="Qty pengganti"
          onChange={(e) => patchRepl(r.key, { qty: e.target.value })}
          className={`${INPUT} tabular text-right`}
        />
      ),
    },
    {
      id: "price",
      header: "Harga (opsional)",
      width: "8rem",
      align: "right",
      render: (r) => (
        <input
          type="number"
          min={0}
          step={1}
          value={r.unitPrice}
          disabled={busy}
          placeholder="Katalog"
          aria-label="Harga pengganti"
          onChange={(e) => patchRepl(r.key, { unitPrice: e.target.value })}
          className={`${INPUT} tabular text-right`}
        />
      ),
    },
    {
      id: "location",
      header: "Lokasi keluar",
      width: "10rem",
      render: (r) => (
        <SelectInput
          aria-label="Lokasi keluar pengganti"
          value={r.locationId ?? ""}
          disabled={busy}
          onChange={(e) => patchRepl(r.key, { locationId: e.target.value === "" ? null : Number(e.target.value) })}
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
  ];

  const itemsReview: LineColumn<LineDraft>[] = [
    {
      id: "product",
      header: "Produk",
      render: (r) => (
        <div className="py-1">
          <p className="font-medium text-foreground">{r.productName}</p>
          <p className="text-xs text-muted-foreground">
            {r.productCode} · {returnConditionLabel(r.condition)}
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
        title="Buat retur jual"
        description="Retur dibuat sebagai draf terhadap order jual terkonfirmasi atau selesai."
      />

      <div className="rounded-xl border border-border bg-card px-5 py-4">
        <Stepper steps={STEPS} current={STEPS[step]?.key ?? "source"} />
      </div>

      {lookupError && <Notice tone="danger" title={lookupError} />}

      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        {step === 0 && (
          <SectionCard title="Pilih dokumen" description="Order jual sumber yang barangnya akan diretur.">
            <div className="max-w-xl">
              <FormField label="Order jual sumber" required>
                {lockedSourceId ? (
                  <TextInput
                    aria-label="Order jual sumber"
                    value={
                      lockedOrder
                        ? `${lockedOrder.number}${lockedOrder.partyName ? ` — ${lockedOrder.partyName}` : ""}`
                        : `#${lockedSourceId}`
                    }
                    readOnly
                    disabled
                  />
                ) : (
                  <SalesOrderSearchSelect
                    value={sourceId}
                    status="confirmed,completed"
                    placeholder="Cari order (nomor atau pelanggan)…"
                    disabled={busy}
                    onChange={handleSourceChange}
                  />
                )}
              </FormField>
            </div>
          </SectionCard>
        )}

        {step === 1 && (
          <>
            <SectionCard title="Mode retur" description="Tukar barang mewajibkan minimal satu baris barang pengganti.">
              <ToggleGroup
                type="single"
                variant="outline"
                aria-label="Mode retur"
                value={returnMode}
                onValueChange={handleModeChange}
              >
                <ToggleGroupItem value="return_only">Retur saja</ToggleGroupItem>
                <ToggleGroupItem value="exchange">Tukar barang</ToggleGroupItem>
              </ToggleGroup>
            </SectionCard>

            <SectionCard
              title="Barang yang diretur"
              description="Qty dibatasi sisa yang benar-benar bisa diretur (terkirim − sudah diretur) dari konteks order."
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
                  fulfilledLabel="terkirim"
                  emptyDescription="Pilih order jual sumber untuk memuat baris item."
                  onQtyChange={(key, qty) => patchLine(key, { qty })}
                  extraColumns={[
                    {
                      header: "Kondisi",
                      render: (r) => (
                        <SelectInput
                          aria-label={`Kondisi ${r.productName}`}
                          value={r.condition}
                          disabled={busy}
                          onChange={(e) =>
                            patchLine(r.key, { condition: e.target.value as ReturnCondition, locationId: null })
                          }
                        >
                          {RETURN_CONDITIONS.map((c) => (
                            <option key={c} value={c}>
                              {returnConditionLabel(c)}
                            </option>
                          ))}
                        </SelectInput>
                      ),
                    },
                    {
                      header: "Lokasi masuk",
                      render: (r) =>
                        r.condition === "normal" ? (
                          <SelectInput
                            aria-label={`Lokasi masuk ${r.productName}`}
                            value={r.locationId ?? ""}
                            disabled={busy}
                            onChange={(e) =>
                              patchLine(r.key, { locationId: e.target.value === "" ? null : Number(e.target.value) })
                            }
                          >
                            <option value="">Pilih (produk berstok)</option>
                            {locations.map((l) => (
                              <option key={l.id} value={l.id}>
                                {l.code} — {l.name}
                              </option>
                            ))}
                          </SelectInput>
                        ) : (
                          <span className="text-xs text-muted-foreground">
                            {r.condition === "damaged" ? "Otomatis: lokasi rusak" : "Tanpa gerak stok"}
                          </span>
                        ),
                    },
                  ]}
                />
              )}
            </SectionCard>

            {isExchange && (
              <SectionCard
                title="Barang pengganti"
                description="Barang yang dikirim ke pelanggan sebagai tukar. Stok dicek server saat pengiriman."
              >
                <LineItemsEditor<ReplDraft>
                  lines={replacements}
                  getKey={(r) => r.key}
                  columns={replColumns}
                  onAdd={addReplacementRow}
                  onRemove={removeReplAt}
                  addLabel="Tambah barang pengganti"
                  emptyText="Retur tukar wajib membawa minimal satu baris barang pengganti."
                />
              </SectionCard>
            )}
          </>
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
                    placeholder="Mis. barang cacat / salah kirim"
                    disabled={busy}
                  />
                </FormField>
                <FormField label="Tanggal retur" helperText="Kosongkan untuk hari ini (server).">
                  <DateInput
                    value={returnDate}
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

            <SectionCard
              title="Barang diretur"
              description={isExchange ? "Mode tukar barang." : "Mode retur saja."}
            >
              <LineItemsEditor<LineDraft>
                lines={activeLines}
                getKey={(r) => r.key}
                columns={itemsReview}
                readOnly
                emptyText="Tidak ada baris dengan qty retur."
              />
              {isExchange && (
                <p className="mt-3 text-sm text-muted-foreground">
                  {replacements.filter((r) => (r.productId ?? 0) > 0 && parseQty(r.qty) > 0).length} barang pengganti
                  akan dikirim setelah retur dikonfirmasi.
                </p>
              )}
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
                  Hitung subtotal, diskon, PPN, selisih tukar, dan cara nilai retur akan diselesaikan di server tanpa menyimpan.
                </span>
              </div>
              {previewError && (
                <div className="mt-3">
                  <Notice tone="danger" title={previewError} />
                </div>
              )}
              {preview && (
                <div className="mt-3">
                  <Notice
                    tone="info"
                    title={`Pratinjau (${preview.mode === "exchange" ? "Tukar barang" : "Retur saja"}) — belum disimpan`}
                  >
                    <span className="mt-1 block">
                      Subtotal {formatIDR(preview.subtotal)} · Diskon {formatIDR(preview.discountTotal)} · PPN{" "}
                      {formatIDR(preview.taxTotal)} · Total{" "}
                      <span className="font-semibold">{formatIDR(preview.total)}</span>
                      {preview.mode === "exchange" && (
                        <>
                          {" "}
                          · Total pengganti{" "}
                          <span className="font-semibold">{formatIDR(preview.replacementTotal)}</span> · Selisih{" "}
                          <span className="font-semibold">
                            {formatIDR(Math.abs(preview.replacementTotal - preview.total))}
                          </span>{" "}
                          ({returnDifferenceLabel(preview.replacementTotal - preview.total)})
                        </>
                      )}
                    </span>
                    <ul className="mt-2 list-disc space-y-0.5 pl-5" aria-label="Penyelesaian nilai retur">
                      {salesSettlementOutlook(preview, formatIDR).map((line) => (
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
