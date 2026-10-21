import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, FormSheet, TextArea, TextInput } from "@/shared/components/ui";
import { DescriptionList, Money, Qty } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { RupiahInput } from "@/modules/payment/components/RupiahInput";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import {
  correctionReasonSchema,
  issueMap,
  priceCorrectionLineSchema,
} from "@/modules/purchasing/schemas/purchase-correction.schema";
import type { PriceCorrectionLineValues } from "@/modules/purchasing/schemas/purchase-correction.schema";
import { BalanceAfter } from "@/modules/purchasing/components/BalanceAfter";
import type { PurchaseCorrectionPreview, PurchaseOrder } from "@/modules/purchasing/types";

interface PriceCorrectionSheetProps {
  open: boolean;
  order: PurchaseOrder;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onConfirm: (lines: PriceCorrectionLineValues[], reason: string) => void;
}

interface RowState {
  lineId: number;
  unitPrice: string;
  discountPct: string;
  discountNominal: string;
}

const signed = (v: number) => (v > 0 ? `+${formatIDR(v)}` : v < 0 ? `−${formatIDR(-v)}` : formatIDR(0));

/**
 * Koreksi harga beli setelah barang diterima. Order mengambil harga baru
 * (tagihan pemasok ikut berubah); selisih nilai barang yang SUDAH diterima
 * dijurnal: bagian yang masih di gudang ke Persediaan, bagian yang sudah
 * keluar ke HPP, plus PPN Masukan dan Hutang. Pembagiannya dihitung server.
 */
export function PriceCorrectionSheet({ open, order, submitting, serverError, onClose, onConfirm }: PriceCorrectionSheetProps) {
  const formId = useId();
  const [rows, setRows] = useState<RowState[]>(() =>
    order.items.map((it) => ({
      lineId: it.id,
      unitPrice: String(it.unitPrice),
      discountPct: String(it.discountPct),
      discountNominal: String(it.discountNominal),
    })),
  );
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [preview, setPreview] = useState<PurchaseCorrectionPreview | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);

  function update(lineId: number, patch: Partial<RowState>) {
    setRows((prev) => prev.map((r) => (r.lineId === lineId ? { ...r, ...patch } : r)));
    setPreview(null); // angka berubah: pratinjau lama tidak berlaku
  }

  /** Baris yang berubah dari harga sekarang, tervalidasi; null bila ada galat. */
  function changedLines(): PriceCorrectionLineValues[] | null {
    const next: Record<string, string> = {};
    const out: PriceCorrectionLineValues[] = [];
    rows.forEach((r, i) => {
      const it = order.items[i];
      const parsed = priceCorrectionLineSchema.safeParse({
        lineId: r.lineId,
        unitPrice: Number(r.unitPrice),
        discountPct: Number(r.discountPct),
        discountNominal: Number(r.discountNominal),
      });
      if (!parsed.success) {
        for (const [k, msg] of Object.entries(issueMap(parsed.error))) next[`${r.lineId}.${k}`] = msg;
        return;
      }
      const v = parsed.data;
      if (v.unitPrice !== it.unitPrice || v.discountPct !== it.discountPct || v.discountNominal !== it.discountNominal) {
        out.push(v);
      }
    });
    if (out.length === 0 && Object.keys(next).length === 0) next.lines = "Ubah minimal satu harga";
    setErrors(next);
    return Object.keys(next).length > 0 ? null : out;
  }

  async function runPreview() {
    const lines = changedLines();
    if (!lines) return;
    setPreviewing(true);
    setPreviewError(null);
    try {
      setPreview(await purchaseOrderService.previewPriceCorrection(order.id, { lines }));
    } catch (err) {
      setPreview(null);
      setPreviewError(formatApiError(err));
    } finally {
      setPreviewing(false);
    }
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const lines = changedLines();
    const parsedReason = correctionReasonSchema.safeParse(reason);
    if (!parsedReason.success) {
      setErrors((prev) => ({ ...prev, reason: parsedReason.error.issues[0]?.message ?? "" }));
      return;
    }
    if (!lines || !preview) return;
    onConfirm(lines, parsedReason.data);
  }

  const byItem = new Map((preview?.lines ?? []).map((l) => [l.itemId, l]));

  return (
    <FormSheet
      open={open}
      title={`Koreksi harga ${order.number}`}
      description="Untuk harga atau diskon yang salah setelah barang diterima. Tagihan pemasok dan nilai persediaan ikut dikoreksi."
      size="lg"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="button" variant="secondary" onClick={() => void runPreview()} loading={previewing}>
            Hitung dampak
          </Button>
          <Button type="submit" form={formId} loading={submitting} disabled={!preview}>
            Simpan koreksi
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-5">
        {serverError && <Notice tone="danger" title={serverError} />}
        {previewError && <Notice tone="danger" title="Koreksi tidak bisa dihitung">{previewError}</Notice>}
        {errors.lines && <Notice tone="warning" title={errors.lines} />}

        <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
          {order.items.map((it, i) => {
            const r = rows[i];
            const pl = byItem.get(it.id);
            const err = (k: string) => errors[`${it.id}.${k}`];
            return (
              <li key={it.id} className="flex flex-col gap-3 p-4">
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <p className="font-medium">
                    {it.productName}{" "}
                    <span className="text-xs font-normal text-muted-foreground">
                      <Qty value={it.qty} unit={it.uom} />
                    </span>
                  </p>
                  <p className="text-xs text-muted-foreground">
                    Sekarang {formatIDR(it.unitPrice)}
                    {it.discountPct > 0 ? `, diskon ${it.discountPct}%` : ""}
                    {it.discountNominal > 0 ? `, potongan ${formatIDR(it.discountNominal)}` : ""}
                  </p>
                </div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                  <FormField label="Harga satuan baru" errorText={err("unitPrice")}>
                    <RupiahInput value={r.unitPrice} onValueChange={(v) => update(it.id, { unitPrice: v })} />
                  </FormField>
                  <FormField label="Diskon %" errorText={err("discountPct")}>
                    <TextInput
                      type="number"
                      min={0}
                      max={100}
                      step="any"
                      value={r.discountPct}
                      onChange={(e) => update(it.id, { discountPct: e.target.value })}
                    />
                  </FormField>
                  <FormField label="Potongan nominal" errorText={err("discountNominal")}>
                    <RupiahInput value={r.discountNominal} onValueChange={(v) => update(it.id, { discountNominal: v })} />
                  </FormField>
                </div>
                {pl && (
                  <p className="text-sm text-muted-foreground">
                    Subtotal <Money value={pl.oldLineTotal} /> menjadi{" "}
                    <Money value={pl.newLineTotal} className="font-medium text-foreground" />. Sudah diterima{" "}
                    <Qty value={pl.receivedQty} unit={pl.uom} />, selisih nilainya {signed(pl.valueDelta)}
                    {pl.taxDelta !== 0 ? ` + PPN ${signed(pl.taxDelta)}` : ""}.
                  </p>
                )}
              </li>
            );
          })}
        </ul>

        {preview && (
          <section aria-label="Dampak koreksi" className="flex flex-col gap-3">
            <h3 className="text-sm font-semibold text-foreground">Dampak koreksi</h3>
            <DescriptionList
              columns={2}
              items={[
                { label: "Total order sekarang", value: <Money value={preview.oldGrandTotal} /> },
                { label: "Total order baru", value: <Money value={preview.newGrandTotal} className="font-semibold" /> },
                { label: "Persediaan (barang masih di gudang)", value: signed(preview.stockDelta) },
                { label: "HPP (barang sudah keluar)", value: signed(preview.cogsDelta) },
                ...(preview.expenseDelta !== 0 ? [{ label: "Beban (jasa)", value: signed(preview.expenseDelta) }] : []),
                { label: "PPN Masukan", value: signed(preview.taxDelta) },
                { label: "Hutang ke pemasok", value: signed(preview.payableDelta) },
                { label: "Saldo pemasok", value: <BalanceAfter outstanding={preview.outstanding} /> },
              ]}
            />
            {preview.payableDelta === 0 && (
              <p className="text-xs text-muted-foreground">
                Belum ada barang yang diterima pada baris yang berubah: hanya harga order yang berubah, tanpa jurnal.
              </p>
            )}
            {preview.payableDelta !== 0 && (
              <p className="text-xs text-muted-foreground">
                Jurnal koreksi masuk antrean posting keuangan dengan tanggal hari ini.
              </p>
            )}
          </section>
        )}

        <FormField label="Alasan" required errorText={errors.reason}>
          <TextArea
            rows={2}
            maxLength={500}
            value={reason}
            placeholder="Mis. harga di faktur pemasok berbeda dari PO"
            onChange={(e) => setReason(e.target.value)}
          />
        </FormField>
      </form>
    </FormSheet>
  );
}
