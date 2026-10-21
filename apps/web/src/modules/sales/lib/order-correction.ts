import type { HeaderEditValues } from "@/modules/sales/schemas/order-correction.schema";
import type { SalesOrder, SalesOrderLinePayload, SalesOrderPayload, ShortClosePlan } from "@/modules/sales/types";

// Koreksi order jual — fungsi murni untuk halaman detail. Server tetap penentu:
// editMode berasal dari GET detail (penjaga dependen yang sama dengan PUT), dan
// PUT menolak perubahan ekonomi bila sudah ada dokumen turunan.

/** Ke mana tombol "Ubah order" membawa pengguna. */
export type EditAction = "form" | "header" | null;

export function editAction(o: Pick<SalesOrder, "status" | "editMode">): EditAction {
  if (o.status === "draft") return "form";
  if (o.status !== "confirmed") return null;
  if (o.editMode === "full") return "form";
  if (o.editMode === "header") return "header";
  return null;
}

/** Nilai awal dialog ubah info order dari order yang dimuat. */
export function headerEditInitial(o: SalesOrder): HeaderEditValues {
  return {
    paymentTerms: o.paymentTerms,
    dueDate: (o.dueDate ?? "").slice(0, 10),
    notes: o.notes ?? "",
    shipToLabel: o.shipToLabel ?? "",
    shipToRecipient: o.shipToRecipient ?? "",
    shipToPhone: o.shipToPhone ?? "",
    shipToAddress: o.shipToAddress ?? "",
    taxInvoiceNumber: o.taxInvoiceNumber ?? "",
    taxInvoiceDate: (o.taxInvoiceDate ?? "").slice(0, 10),
    assignedUserId: (o.assignedUserId ?? 0) > 0 ? (o.assignedUserId ?? null) : null,
    reason: "",
  };
}

function shipToChanged(a: HeaderEditValues, b: HeaderEditValues): boolean {
  return (
    a.shipToLabel !== b.shipToLabel ||
    a.shipToRecipient !== b.shipToRecipient ||
    a.shipToPhone !== b.shipToPhone ||
    a.shipToAddress !== b.shipToAddress
  );
}

/** Apakah ada yang berubah selain alasan (tombol simpan nonaktif bila tidak). */
export function headerEditDirty(initial: HeaderEditValues, v: HeaderEditValues): boolean {
  return (
    shipToChanged(initial, v) ||
    initial.dueDate !== v.dueDate ||
    initial.notes !== v.notes ||
    initial.taxInvoiceNumber !== v.taxInvoiceNumber ||
    initial.taxInvoiceDate !== v.taxInvoiceDate ||
    initial.assignedUserId !== v.assignedUserId
  );
}

/**
 * Body PUT untuk koreksi kolom ringan: semua kolom ekonomi (pihak, kanal,
 * termin, pajak, baris) dikirim persis seperti tersimpan — server memeriksa
 * bahwa tidak ada yang berubah (lightOnly). Alamat kirim dari buku alamat
 * tetap ditautkan selama tidak disunting; begitu disunting, ia menjadi alamat
 * sekali pakai.
 */
export function headerEditPayload(o: SalesOrder, initial: HeaderEditValues, v: HeaderEditValues): SalesOrderPayload {
  const keepBookAddress = o.shipToAddressId > 0 && !shipToChanged(initial, v);
  const items: SalesOrderLinePayload[] = o.items.map((it) => {
    const line: SalesOrderLinePayload = {
      productId: it.productId,
      variantId: it.variantId,
      uom: it.uom,
      qty: it.qty,
      discountPct: it.discountPct,
      discountNominal: it.discountNominal,
      taxable: it.taxable,
    };
    if (it.pricingSource === "manual") line.unitPrice = it.unitPrice;
    if (it.discountReason) line.discountReason = it.discountReason;
    if (it.note) line.note = it.note;
    return line;
  });
  return {
    partyId: o.partyId,
    channel: o.channel,
    orderDate: o.orderDate.slice(0, 10),
    dueDate: v.dueDate,
    paymentTerms: o.paymentTerms,
    taxType: o.taxType,
    taxRate: o.taxRate,
    notes: v.notes,
    assignedUserId: v.assignedUserId ?? 0,
    shipToAddressId: keepBookAddress ? o.shipToAddressId : 0,
    ...(keepBookAddress
      ? {}
      : {
          shipToLabel: v.shipToLabel,
          shipToRecipient: v.shipToRecipient,
          shipToPhone: v.shipToPhone,
          shipToAddress: v.shipToAddress,
        }),
    taxInvoiceNumber: v.taxInvoiceNumber,
    taxInvoiceDate: v.taxInvoiceDate,
    ...(v.reason !== "" ? { reason: v.reason } : {}),
    headerOnly: true,
    items,
  };
}

/** Ringkasan pratinjau tutup sisa order untuk teks dialog. */
export function shortCloseSummary(plan: ShortClosePlan): { cut: number; removed: number; reduction: number } {
  let cut = 0;
  let removed = 0;
  for (const l of plan.lines) {
    if (l.removed) removed += 1;
    else if (l.deliveredQty < l.orderedQty) cut += 1;
  }
  return { cut, removed, reduction: plan.oldGrandTotal - plan.newGrandTotal };
}
