import { useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  DataTable,
  DateInput,
  FormField,
  Modal,
  SearchSelect,
  SectionCard,
  SelectInput,
  TextArea,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { purchaseReturnSchema } from "@/modules/purchasereturn/schemas/purchase-return.schema";
import {
  purchaseReturnLookupService,
  purchaseReturnService,
} from "@/modules/purchasereturn/services/purchase-return.service";
import type {
  LocationOption,
  PurchaseReturn,
  PurchaseReturnPreview,
} from "@/modules/purchasereturn/types";

interface LineDraft {
  key: string;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  orderedQty: number;
  received: number;
  returned: number;
  remaining: number;
  unitPrice: number;
  qty: string;
  locationId: number | null;
}

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (ret: PurchaseReturn) => void;
}

function clampQty(raw: string, max: number): string {
  if (raw.trim() === "") return "";
  const n = Number(raw);
  if (Number.isNaN(n)) return raw;
  if (n < 0) return "0";
  if (n > max) return String(max);
  return raw;
}

export function PurchaseReturnCreateModal({ open, onClose, onCreated }: Props) {
  const [sourceId, setSourceId] = useState<number | null>(null);
  const [lines, setLines] = useState<LineDraft[]>([]);
  const [preview, setPreview] = useState<PurchaseReturnPreview | null>(null);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [returnDate, setReturnDate] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const { data: lookupData, loading: loadingLookups, error: lookupError } = useAsyncData(
    () => {
      if (!open) return Promise.resolve(null);
      return Promise.all([
        purchaseReturnLookupService.sourceOrders(),
        purchaseReturnLookupService.locationOptions(),
      ]).then(([orders, locs]) => ({ orders, locations: locs }));
    },
    [open],
  );
  const candidates = lookupData?.orders ?? [];
  const lookupLocations = lookupData?.locations ?? [];

  // Konteks membawa sisa bisa-diretur per produk (diterima − diretur) +
  // lokasi aktif — pratinjau & clamp memakai angka ini, bukan qty PO.
  // Gagal (mis. 403) → fallback ke detail PO dengan clamp qty order.
  const { data: ctxBundle, loading: ctxLoading, error: ctxFetchError } = useAsyncData(
    () => {
      if (!open || sourceId === null) return Promise.resolve(null);
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
              orderedQty: it.orderedQty,
              received: it.received,
              returned: it.returned,
              remaining: it.remaining,
              unitPrice: 0,
              qty: it.remaining > 0 ? String(it.remaining) : "",
              locationId: null,
            }),
          ),
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
                orderedQty: it.qty,
                received: it.qty,
                returned: 0,
                remaining: it.qty,
                unitPrice: it.unitPrice,
                qty: String(it.qty),
                locationId: null,
              }),
            ),
            locations: [] as LocationOption[],
            contextError: firstMsg as string | null,
          }));
        },
      );
    },
    [open, sourceId],
  );
  const activeBundle = ctxBundle && ctxBundle.sourceId === sourceId ? ctxBundle : null;
  const contextError = activeBundle ? activeBundle.contextError : ctxFetchError;
  const loadingSource = sourceId === null ? false : ctxLoading;
  const locations =
    activeBundle && activeBundle.locations.length > 0
      ? activeBundle.locations
      : lookupLocations;

  // Reset saat modal dibuka.
  const [appliedCtx, setAppliedCtx] = useState<number | null>(null);
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setSourceId(null);
      setLines([]);
      setAppliedCtx(null);
      setPreview(null);
      setPreviewError(null);
      setReturnDate(todayWIB());
      setNotes("");
      setFormError(null);
    }
  }

  // Terapkan baris konteks sekali per order sumber terpilih.
  if (open && activeBundle && appliedCtx !== activeBundle.sourceId) {
    setAppliedCtx(activeBundle.sourceId);
    setLines(activeBundle.lines);
  }

  function handleSourceChange(v: string | number | null) {
    const id = typeof v === "number" ? v : v === null ? null : Number(v);
    if (id === null || Number.isNaN(id)) {
      setSourceId(null);
      setLines([]);
      setAppliedCtx(null);
      setPreview(null);
      return;
    }
    setSourceId(id);
    setAppliedCtx(null);
    setFormError(null);
    setPreview(null);
    setPreviewError(null);
  }

  function patchLine(key: string, patch: Partial<LineDraft>) {
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...patch } : l)));
    setPreview(null);
  }

  const estimateTotal = lines.reduce((sum, l) => {
    const q = Number(l.qty);
    if (Number.isNaN(q) || q <= 0) return sum;
    return sum + Math.round(q * l.unitPrice);
  }, 0);

  function collectPayload() {
    if (sourceId === null) {
      return { error: "Order beli wajib dipilih." };
    }
    const parsed = purchaseReturnSchema.safeParse({
      purchaseOrderId: sourceId,
      returnDate: returnDate.trim(),
      notes: notes.trim(),
      items: lines
        .filter((l) => Number(l.qty) > 0)
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
      .preview({
        purchaseOrderId: collected.data.purchaseOrderId,
        items: collected.data.items,
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
        items: collected.data.items,
      })
      .then((res) => {
        toast.fromServer(res.message, "Retur pembelian dibuat", res.data.number);
        onCreated(res.data);
        onClose();
      })
      .catch((err) => setFormError(toApiError(err).message))
      .finally(() => setBusy(false));
  }

  return (
    <Modal
      open={open}
      title="Buat Retur Pembelian"
      description="Retur dibuat sebagai draf terhadap order beli terkonfirmasi/selesai."
      size="xl"
      onClose={onClose}
    >
      {lookupError && (
        <div className="mb-4">
          <Notice tone="danger" title={lookupError} />
        </div>
      )}
      <form onSubmit={handleSubmit}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Order Beli Sumber" required>
            <SearchSelect
              options={candidates.map((o) => ({
                value: o.id,
                label: `${o.number} — ${o.partyName || "-"}`,
              }))}
              value={sourceId}
              placeholder={loadingLookups ? "Memuat order…" : "Pilih order…"}
              disabled={loadingLookups || busy}
              onChange={handleSourceChange}
            />
          </FormField>
          <FormField label="Tanggal Retur" helperText="Kosongkan untuk hari ini (server).">
            <DateInput
              value={returnDate}
              onChange={(e) => setReturnDate(e.target.value)}
              disabled={busy}
            />
          </FormField>
        </div>

        <div className="mt-4">
          <FormField label="Alasan / Catatan">
            <TextArea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Alasan retur…"
              rows={2}
              disabled={busy}
            />
          </FormField>
        </div>

        <div className="mt-4">
          <SectionCard
            title="Item Retur"
            description="Qty dibatasi sisa yang benar-benar bisa diretur (diterima − sudah diretur) dari konteks PO."
          >
            {contextError && (
              <div className="mb-4">
                <Notice tone="warning" title="Konteks sisa tidak tersedia — memakai qty order">
                  {contextError}
                </Notice>
              </div>
            )}
            {loadingSource ? (
              <p className="text-muted py-4 text-center text-sm">Memuat item order…</p>
            ) : (
              <DataTable<LineDraft>
                rows={lines}
                rowKey={(r) => r.key}
                emptyTitle="Belum ada item"
                emptyDescription="Pilih order beli sumber untuk memuat baris item."
                columns={[
                  {
                    header: "Produk",
                    render: (r) => (
                      <div>
                        <p className="font-medium">{r.productName}</p>
                        <p className="text-muted text-xs">
                          {r.productCode} · {r.uom}
                        </p>
                        <p className="text-muted text-xs">
                          Sisa {formatNumber(r.remaining)} dari {formatNumber(r.orderedQty)} dipesan
                          (diterima {formatNumber(r.received)}, diretur {formatNumber(r.returned)})
                        </p>
                      </div>
                    ),
                  },
                  {
                    header: "Qty Retur",
                    render: (r) => (
                      <input
                        type="number"
                        min={0}
                        max={r.remaining}
                        step="any"
                        value={r.qty}
                        disabled={busy}
                        onChange={(e) =>
                          patchLine(r.key, {
                            qty: clampQty(e.target.value, r.remaining),
                          })
                        }
                        className="w-28 rounded-md border border-hairline bg-surface px-2 py-1.5 text-right text-sm text-ink"
                      />
                    ),
                  },
                  {
                    header: "Lokasi Keluar",
                    render: (r) => (
                      <SelectInput
                        value={r.locationId ?? ""}
                        disabled={busy}
                        onChange={(e) =>
                          patchLine(r.key, {
                            locationId: e.target.value === "" ? null : Number(e.target.value),
                          })
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
            {lines.length > 0 && (
              <p className="mt-3 text-right text-sm text-ink">
                Estimasi total: <span className="font-bold">{formatIDR(estimateTotal)}</span>{" "}
                <span className="text-muted">(harga final dihitung server — gunakan Pratinjau)</span>
              </p>
            )}
          </SectionCard>
        </div>

        <div className="mt-4">
          <div className="flex flex-wrap items-center gap-3">
            <Button
              variant="secondary"
              onClick={handlePreview}
              disabled={busy || previewBusy || loadingSource || sourceId === null}
            >
              {previewBusy ? "Mempratinjau…" : "Pratinjau"}
            </Button>
            <span className="text-muted text-xs">
              Hitung subtotal, diskon, dan PPN di server tanpa menyimpan.
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
                  Subtotal {formatIDR(preview.subtotal)} · Diskon {formatIDR(preview.discountTotal)} ·
                  PPN {formatIDR(preview.taxTotal)} · Total{" "}
                  <span className="font-bold">{formatIDR(preview.total)}</span>
                </span>
              </Notice>
            </div>
          )}
        </div>

        {formError && (
          <div className="mt-4">
            <Notice tone="danger" title={formError} />
          </div>
        )}

        <div className="mt-6 flex justify-end gap-3">
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Batal
          </Button>
          <Button type="submit" disabled={busy || loadingSource || lines.length === 0}>
            {busy ? "Menyimpan…" : "Simpan Draf"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
