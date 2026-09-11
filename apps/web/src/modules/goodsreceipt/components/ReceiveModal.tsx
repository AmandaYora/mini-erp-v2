import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  DataTable,
  DateInput,
  FormField,
  Modal,
  SearchSelect,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatNumber, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { goodsreceiptService } from "@/modules/goodsreceipt/services/goodsreceipt.service";
import { receiveSchema } from "@/modules/goodsreceipt/schemas/goodsreceipt.schema";
import type {
  PurchaseOrderOption,
  StockLocationOption,
} from "@/modules/goodsreceipt/types";

interface ReceiveLineRow {
  key: number;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  /** Batas atas klien = qty PO baris ini (sisa sejati dihitung server). */
  maxQty: number;
  qty: string;
  locationId: number | null;
}

let nextLineKey = 1;

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

interface ReceiveModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: (id: number) => void;
}

/** Modal Terima: pilih PO confirmed → baris prefill qty PO → catat penerimaan. */
export function ReceiveModal({ open, onClose, onCreated }: ReceiveModalProps) {
  const [poOptions, setPoOptions] = useState<PurchaseOrderOption[]>([]);
  const [locations, setLocations] = useState<StockLocationOption[]>([]);
  const [poId, setPoId] = useState<number | null>(null);
  const [loadingPo, setLoadingPo] = useState(false);
  const [lines, setLines] = useState<ReceiveLineRow[]>([]);
  const [receivedAt, setReceivedAt] = useState(todayWIB());
  const [notes, setNotes] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Reset formulir tiap modal dibuka — render-phase adjustment (bukan effect).
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setPoId(null);
      setLines([]);
      setReceivedAt(todayWIB());
      setNotes("");
      setFormError(null);
      setSaving(false);
    }
  }

  // Opsi PO + lokasi dimuat tiap modal dibuka. Hanya callback async
  // (tanpa setState sinkron di body) → bebas peringatan set-state-in-effect.
  useEffect(() => {
    if (!open) return;
    let alive = true;
    goodsreceiptService
      .purchaseOrderOptions()
      .then((res) => {
        if (alive) setPoOptions(res.items);
      })
      .catch((err) => {
        if (alive) setFormError(toApiError(err).message);
      });
    goodsreceiptService
      .locationOptions()
      .then((rows) => {
        if (alive) setLocations(rows);
      })
      .catch((err) => {
        if (alive)
          toast.danger("Gagal memuat lokasi", toApiError(err).message);
      });
    return () => {
      alive = false;
    };
  }, [open ]);

  async function handlePoChange(v: string | number | null) {
    const id = toId(v);
    setPoId(id);
    setLines([]);
    setFormError(null);
    if (!id) return;
    setLoadingPo(true);
    try {
      const po = await goodsreceiptService.purchaseOrderDetail(id);
      // Prefill tiap baris PO dengan qty penuh — orderView TIDAK membawa
      // receivedQty, jadi sisa parsial tak bisa dihitung klien; server yang
      // menolak kelebihan ("Melebihi sisa PO").
      setLines(
        po.items.map((it) => ({
          key: nextLineKey++,
          productId: it.productId,
          variantId: it.variantId,
          productCode: it.productCode,
          productName: it.productName,
          uom: it.uom,
          maxQty: it.qty,
          qty: String(it.qty),
          locationId: null,
        })),
      );
    } catch (err) {
      setFormError(toApiError(err).message);
    } finally {
      setLoadingPo(false);
    }
  }

  function patchLine(key: number, patch: Partial<ReceiveLineRow>) {
    setLines((prev) =>
      prev.map((r) => (r.key === key ? { ...r, ...patch } : r)),
    );
  }

  /** Jepit qty klien ke [0, maxQty] — backend menolak over-receipt. */
  function handleQtyChange(key: number, maxQty: number, raw: string) {
    if (raw.trim() === "") {
      patchLine(key, { qty: "" });
      return;
    }
    const n = Number(raw);
    if (Number.isNaN(n)) return;
    if (n > maxQty) {
      patchLine(key, { qty: String(maxQty) });
      return;
    }
    patchLine(key, { qty: raw });
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = receiveSchema.safeParse({
      purchaseOrderId: poId ?? 0,
      receivedAt: receivedAt.trim(),
      notes,
      items: lines.map((r) => ({
        productId: r.productId,
        variantId: r.variantId,
        locationId: r.locationId ?? 0,
        uom: r.uom,
        qty: r.qty,
      })),
    });
    if (!parsed.success) {
      const first = parsed.error.issues[0];
      setFormError(first?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setFormError(null);
    setSaving(true);
    try {
      const res = await goodsreceiptService.create({
        purchaseOrderId: parsed.data.purchaseOrderId,
        receivedAt:
          parsed.data.receivedAt.trim() !== ""
            ? parsed.data.receivedAt.trim()
            : undefined,
        notes: parsed.data.notes,
        items: parsed.data.items,
      });
      toast.fromServer(res.message, "Penerimaan dicatat", res.data.orderNumber);
      onCreated(res.data.id);
    } catch (err) {
      const apiErr = toApiError(err);
      setFormError(apiErr.message);
      toast.danger("Gagal mencatat penerimaan", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      title="Catat Penerimaan"
      description="Terima barang dari purchase order terkonfirmasi. Stok masuk dicatat server per baris."
      size="xl"
      onClose={() => {
        if (!saving) onClose();
      }}
      actions={
        <>
          <Button
            variant="secondary"
            disabled={saving}
            onClick={onClose}
          >
            Batal
          </Button>
          <Button type="submit" form="receive-form" disabled={saving}>
            {saving ? "Menyimpan…" : "Simpan Penerimaan"}
          </Button>
        </>
      }
    >
      <form id="receive-form" onSubmit={(e) => void handleSubmit(e)}>
        {formError && (
          <div className="mb-4">
            <Notice tone="danger" title="Penerimaan belum tersimpan">
              {formError}
            </Notice>
          </div>
        )}
        <div className="grid gap-4 md:grid-cols-2">
          <FormField label="Purchase Order" required>
            <SearchSelect
              options={poOptions.map((o) => ({
                value: o.id,
                label: `${o.number} — ${o.partyName || "-"}`,
              }))}
              value={poId}
              placeholder="Pilih PO terkonfirmasi…"
              onChange={(v) => void handlePoChange(v)}
            />
          </FormField>
          <FormField
            label="Tanggal Terima"
            helperText="Kosongkan untuk memakai tanggal hari ini."
          >
            <DateInput
              value={receivedAt}
              onChange={(e) => setReceivedAt(e.target.value)}
            />
          </FormField>
        </div>
        <div className="mt-4">
          <FormField label="Catatan">
            <TextArea
              rows={2}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Keterangan tambahan (opsional)"
            />
          </FormField>
        </div>

        <div className="mt-4">
          {loadingPo ? (
            <p className="text-muted py-4 text-center text-sm">
              Memuat baris PO…
            </p>
          ) : lines.length === 0 ? (
            <p className="text-muted py-4 text-center text-sm">
              Pilih purchase order untuk memuat baris barang.
            </p>
          ) : (
            <DataTable<ReceiveLineRow>
              rows={lines}
              rowKey={(r) => r.key}
              emptyTitle="Tidak ada baris"
              columns={[
                {
                  header: "Produk",
                  render: (r) => (
                    <span>
                      <span className="font-medium">{r.productName}</span>{" "}
                      <span className="text-muted text-xs">{r.productCode}</span>
                    </span>
                  ),
                },
                { header: "Satuan", render: (r) => r.uom },
                {
                  header: "Maks (Qty PO)",
                  align: "right",
                  render: (r) => formatNumber(r.maxQty),
                },
                {
                  header: "Qty Terima",
                  render: (r) => (
                    <TextInput
                      type="number"
                      min="0"
                      step="any"
                      value={r.qty}
                      onChange={(e) =>
                        handleQtyChange(r.key, r.maxQty, e.target.value)
                      }
                    />
                  ),
                },
                {
                  header: "Lokasi",
                  render: (r) => (
                    <SearchSelect
                      options={locations.map((l) => ({
                        value: l.id,
                        label: `${l.code} — ${l.name}`,
                      }))}
                      value={r.locationId}
                      placeholder="Pilih lokasi…"
                      onChange={(v) =>
                        patchLine(r.key, { locationId: toId(v) })
                      }
                    />
                  ),
                },
              ]}
            />
          )}
        </div>
      </form>
    </Modal>
  );
}
