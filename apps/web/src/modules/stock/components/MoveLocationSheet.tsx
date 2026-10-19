import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, FormSheet, TextArea, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { stockService } from "@/modules/stock/services/stock.service";
import { moveLocationSchema } from "@/modules/stock/schemas/stock.schema";
import { LocationSelect } from "@/modules/stock/components/LocationSelect";
import { ProductPicker } from "@/modules/stock/components/ProductPicker";
import type { StockTransfer } from "@/modules/stock/types";

interface ItemRow {
  key: number;
  productId: number | null;
  variantId: number | null;
  qty: string;
  /** Tujuan khusus baris (null = tujuan dokumen). */
  toLocationId: number | null;
}

let nextRowKey = 1;
const newRow = (): ItemRow => ({ key: nextRowKey++, productId: null, variantId: null, qty: "", toLocationId: null });

/**
 * Pindah lokasi satu langkah (J4): dalam cabang aktif, stok langsung berpindah dan tercatat
 * sebagai surat transfer berstatus diterima. Sheet kanan (PLAN §7.0) — bukan dokumen bertahap.
 */
export function MoveLocationSheet({
  open,
  onClose,
  onMoved,
}: {
  open: boolean;
  onClose: () => void;
  onMoved: (transfer: StockTransfer) => void;
}) {
  const [from, setFrom] = useState<number | null>(null);
  const [to, setTo] = useState<number | null>(null);
  const [notes, setNotes] = useState("");
  const [rows, setRows] = useState<ItemRow[]>(() => [newRow()]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  function reset() {
    setFrom(null);
    setTo(null);
    setNotes("");
    setRows([newRow()]);
    setError(null);
  }

  function patchRow(key: number, patch: Partial<ItemRow>) {
    setRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  function close() {
    if (saving) return;
    onClose();
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = moveLocationSchema.safeParse({
      fromLocationId: from ?? 0,
      toLocationId: to ?? 0,
      notes,
      items: rows.map((r) => ({
        productId: r.productId ?? 0,
        variantId: r.variantId ?? 0,
        qty: r.qty,
        toLocationId: r.toLocationId ?? undefined,
      })),
    });
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setError(null);
    setSaving(true);
    try {
      const res = await stockService.moveLocation({
        fromLocationId: parsed.data.fromLocationId,
        toLocationId: parsed.data.toLocationId,
        notes: parsed.data.notes,
        items: parsed.data.items.map((it) => ({ ...it, notes: "" })),
      });
      toast.fromServer(res.message, "Stok dipindahkan", res.data.number);
      reset();
      onMoved(res.data);
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal memindahkan stok", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <FormSheet
      open={open}
      title="Pindah lokasi"
      description="Antar lokasi dalam cabang aktif. Stok langsung berpindah dan tercatat sebagai surat transfer yang sudah diterima."
      size="lg"
      onClose={close}
      actions={
        <>
          <Button variant="secondary" disabled={saving} onClick={close}>
            Batal
          </Button>
          <Button type="submit" form="move-form" loading={saving}>
            Pindahkan
          </Button>
        </>
      }
    >
      <form id="move-form" className="flex flex-col gap-4" onSubmit={(e) => void handleSubmit(e)}>
        {error && (
          <Notice tone="danger" title="Stok belum dipindahkan">
            {error}
          </Notice>
        )}
        <div className="grid gap-4 sm:grid-cols-2">
          <LocationSelect label="Lokasi asal" value={from} onChange={setFrom} required />
          <LocationSelect label="Lokasi tujuan" value={to} onChange={setTo} required />
        </div>
        <FormField label="Catatan" htmlFor="move-notes">
          <TextArea
            id="move-notes"
            rows={2}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Keterangan tambahan (opsional)"
          />
        </FormField>
        <div className="flex flex-col gap-3">
          {rows.map((row, idx) => (
            <div key={row.key} className="rounded-lg border border-border p-4">
              <div className="mb-3 flex items-center justify-between">
                <p className="text-sm font-semibold text-foreground">Barang {idx + 1}</p>
                {rows.length > 1 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setRows((prev) => prev.filter((r) => r.key !== row.key))}
                  >
                    Hapus
                  </Button>
                )}
              </div>
              <ProductPicker
                productId={row.productId}
                variantId={row.variantId}
                onProductChange={(id) => patchRow(row.key, { productId: id })}
                onVariantChange={(id) => patchRow(row.key, { variantId: id })}
              />
              <div className="mt-3 grid gap-4 sm:grid-cols-2">
                <FormField label="Jumlah" htmlFor={`move-qty-${row.key}`} required>
                  <TextInput
                    id={`move-qty-${row.key}`}
                    type="number"
                    min="0"
                    step="any"
                    inputMode="decimal"
                    placeholder="cth: 5"
                    value={row.qty}
                    onChange={(e) => patchRow(row.key, { qty: e.target.value })}
                  />
                </FormField>
                <LocationSelect
                  label="Tujuan barang ini (opsional)"
                  value={row.toLocationId}
                  onChange={(id) => patchRow(row.key, { toLocationId: id })}
                  allowClear
                  placeholder="Sama dengan lokasi tujuan"
                />
              </div>
            </div>
          ))}
          <div>
            <Button variant="secondary" size="sm" onClick={() => setRows((prev) => [...prev, newRow()])}>
              Tambah barang
            </Button>
          </div>
        </div>
      </form>
    </FormSheet>
  );
}
