import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { z } from "zod";
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
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { deliverSchema } from "@/modules/delivery/schemas/delivery.schema";
import type {
  SalesOrderOption,
  StockLocationOption,
} from "@/modules/delivery/types";

interface DeliverLineRow {
  key: number;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  /** Batas atas klien = qty SO baris ini (sisa sejati dihitung server). */
  maxQty: number;
  qty: string;
  locationId: number | null;
}

let nextLineKey = 1;

// Dokumen pengiriman opsional — batas disalin dari
// delivery/application/service.go: validateDoc (driver/staff maks 100,
// plat maks 20).
const docSchema = z.object({
  driverName: z.string().max(100, "Nama supir maksimal 100 karakter").default(""),
  vehiclePlate: z.string().max(20, "Plat kendaraan maksimal 20 karakter").default(""),
  warehouseStaffName: z.string().max(100, "Nama petugas gudang maksimal 100 karakter").default(""),
  dropLocationNote: z.string().default(""),
});

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

interface CreateDeliveryModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: (id: number) => void;
}

/** Modal Buat: pilih SO confirmed → baris prefill qty SO → buat surat jalan (draf). */
export function CreateDeliveryModal({
  open,
  onClose,
  onCreated,
}: CreateDeliveryModalProps) {
  const [soOptions, setSoOptions] = useState<SalesOrderOption[]>([]);
  const [locations, setLocations] = useState<StockLocationOption[]>([]);
  const [soId, setSoId] = useState<number | null>(null);
  const [loadingSo, setLoadingSo] = useState(false);
  const [lines, setLines] = useState<DeliverLineRow[]>([]);
  const [deliveryDate, setDeliveryDate] = useState(todayWIB());
  const [driverName, setDriverName] = useState("");
  const [vehiclePlate, setVehiclePlate] = useState("");
  const [warehouseStaffName, setWarehouseStaffName] = useState("");
  const [dropLocationNote, setDropLocationNote] = useState("");
  const [notes, setNotes] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Reset formulir tiap modal dibuka — render-phase adjustment (bukan effect).
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setSoId(null);
      setLines([]);
      setDeliveryDate(todayWIB());
      setDriverName("");
      setVehiclePlate("");
      setWarehouseStaffName("");
      setDropLocationNote("");
      setNotes("");
      setFormError(null);
      setSaving(false);
    }
  }

  // Opsi SO + lokasi dimuat tiap modal dibuka. Hanya callback async
  // (tanpa setState sinkron di body) → bebas peringatan set-state-in-effect.
  useEffect(() => {
    if (!open) return;
    let alive = true;
    deliveryService
      .salesOrderOptions()
      .then((res) => {
        if (alive) setSoOptions(res.items);
      })
      .catch((err) => {
        if (alive) setFormError(toApiError(err).message);
      });
    deliveryService
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

  async function handleSoChange(v: string | number | null) {
    const id = toId(v);
    setSoId(id);
    setLines([]);
    setFormError(null);
    if (!id) return;
    setLoadingSo(true);
    try {
      const so = await deliveryService.salesOrderDetail(id);
      // Prefill tiap baris SO dengan qty penuh — orderView TIDAK membawa
      // deliveredQty, jadi sisa parsial tak bisa dihitung klien; server yang
      // menolak kelebihan ("Melebihi sisa SO").
      setLines(
        so.items.map((it) => ({
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
      setLoadingSo(false);
    }
  }

  function patchLine(key: number, patch: Partial<DeliverLineRow>) {
    setLines((prev) =>
      prev.map((r) => (r.key === key ? { ...r, ...patch } : r)),
    );
  }

  /** Jepit qty klien ke [0, maxQty] — backend menolak over-SO. */
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
    const parsed = deliverSchema.safeParse({
      salesOrderId: soId ?? 0,
      deliveryDate: deliveryDate.trim(),
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
    const doc = docSchema.safeParse({
      driverName,
      vehiclePlate,
      warehouseStaffName,
      dropLocationNote,
    });
    if (!doc.success) {
      const first = doc.error.issues[0];
      setFormError(first?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setFormError(null);
    setSaving(true);
    try {
      const trimOrUndefined = (v: string) =>
        v.trim() !== "" ? v.trim() : undefined;
      const res = await deliveryService.create({
        salesOrderId: parsed.data.salesOrderId,
        deliveryDate:
          parsed.data.deliveryDate.trim() !== ""
            ? parsed.data.deliveryDate.trim()
            : undefined,
        driverName: trimOrUndefined(doc.data.driverName),
        vehiclePlate: trimOrUndefined(doc.data.vehiclePlate),
        warehouseStaffName: trimOrUndefined(doc.data.warehouseStaffName),
        dropLocationNote: trimOrUndefined(doc.data.dropLocationNote),
        notes: parsed.data.notes,
        items: parsed.data.items,
      });
      toast.fromServer(res.message, "Surat jalan dibuat", res.data.number);
      onCreated(res.data.id);
    } catch (err) {
      const apiErr = toApiError(err);
      setFormError(apiErr.message);
      toast.danger("Gagal membuat surat jalan", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      title="Buat Surat Jalan"
      description="Draf tidak menggerakkan stok — stok keluar dicatat saat konfirmasi."
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
          <Button type="submit" form="delivery-form" disabled={saving}>
            {saving ? "Menyimpan…" : "Simpan Surat Jalan"}
          </Button>
        </>
      }
    >
      <form id="delivery-form" onSubmit={(e) => void handleSubmit(e)}>
        {formError && (
          <div className="mb-4">
            <Notice tone="danger" title="Surat jalan belum tersimpan">
              {formError}
            </Notice>
          </div>
        )}
        <div className="grid gap-4 md:grid-cols-2">
          <FormField label="Sales Order" required>
            <SearchSelect
              options={soOptions.map((o) => ({
                value: o.id,
                label: `${o.number} — ${o.partyName || "Walk-in"}`,
              }))}
              value={soId}
              placeholder="Pilih SO terkonfirmasi…"
              onChange={(v) => void handleSoChange(v)}
            />
          </FormField>
          <FormField
            label="Tanggal Kirim"
            helperText="Kosongkan untuk memakai tanggal hari ini."
          >
            <DateInput
              value={deliveryDate}
              onChange={(e) => setDeliveryDate(e.target.value)}
            />
          </FormField>
        </div>
        <div className="mt-4 grid gap-4 md:grid-cols-3">
          <FormField label="Supir">
            <TextInput
              value={driverName}
              maxLength={100}
              onChange={(e) => setDriverName(e.target.value)}
              placeholder="Nama supir (opsional)"
            />
          </FormField>
          <FormField label="Plat Kendaraan">
            <TextInput
              value={vehiclePlate}
              maxLength={20}
              onChange={(e) => setVehiclePlate(e.target.value)}
              placeholder="cth. B 1234 CD (opsional)"
            />
          </FormField>
          <FormField label="Petugas Gudang">
            <TextInput
              value={warehouseStaffName}
              maxLength={100}
              onChange={(e) => setWarehouseStaffName(e.target.value)}
              placeholder="Nama petugas (opsional)"
            />
          </FormField>
        </div>
        <div className="mt-4">
          <FormField label="Catatan Turun Barang">
            <TextArea
              rows={2}
              value={dropLocationNote}
              onChange={(e) => setDropLocationNote(e.target.value)}
              placeholder="Titik/lokasi turun barang (opsional)"
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
          {loadingSo ? (
            <p className="text-muted py-4 text-center text-sm">
              Memuat baris SO…
            </p>
          ) : lines.length === 0 ? (
            <p className="text-muted py-4 text-center text-sm">
              Pilih sales order untuk memuat baris barang.
            </p>
          ) : (
            <DataTable<DeliverLineRow>
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
                  header: "Maks (Qty SO)",
                  align: "right",
                  render: (r) => formatNumber(r.maxQty),
                },
                {
                  header: "Qty Kirim",
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
                  header: "Lokasi Asal",
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
