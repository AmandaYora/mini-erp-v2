import type { SalesLineFormValues } from "@/modules/sales/schemas/sales-order.schema";
import type { SalesOrderLinePayload } from "@/modules/sales/types";

/** Parse input harga manual: "" = null (server memberi harga), selain itu angka. */
export function parseManualPrice(raw: string): number | null {
  const t = raw.trim();
  if (t === "") return null;
  const n = Number(t);
  return Number.isFinite(n) ? n : Number.NaN;
}

/** Baris form tervalidasi → payload API. unitPrice dihilangkan bila null. */
export function toLinePayload(it: SalesLineFormValues): SalesOrderLinePayload {
  const out: SalesOrderLinePayload = {
    productId: it.productId,
    variantId: it.variantId,
    uom: it.uom.trim(),
    qty: it.qty,
    discountPct: it.discountPct,
    discountNominal: it.discountNominal,
    taxable: it.taxable,
  };
  if (it.unitPrice !== null) out.unitPrice = it.unitPrice;
  if (it.discountReason.trim() !== "") out.discountReason = it.discountReason.trim();
  if (it.note.trim() !== "") out.note = it.note.trim();
  return out;
}
