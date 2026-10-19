// Fetch helper cetak — pembungkus tipis di atas service yang sudah ada.
// Modul lain TIDAK diubah; join best-effort (nama produk/pelanggan) gagal
// dengan anggun tanpa menggagalkan halaman cetak.

import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { salesService } from "@/modules/sales/services/sales.service";
import { paymentService } from "@/modules/payment/services/payment.service";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import type { DeliveryNote } from "@/modules/delivery/types";
import type { SalesOrder } from "@/modules/sales/types";
import type { Payment } from "@/modules/payment/types";

export interface DeliveryPrintData {
  /** Surat jalan (noteView backend — TIDAK membawa nama produk/pelanggan). */
  note: DeliveryNote;
  /**
   * Induk SO penuh untuk nama produk + pelanggan + ship-to (C2).
   * Best-effort: null bila gagal (mis. tanpa izin sales.view) — halaman
   * tetap mencetak ID mentah.
   */
  order: SalesOrder | null;
}

/**
 * Surat jalan + induk SO penuhnya. noteView tidak membawa productName /
 * partyName / ship-to, jadi SO diambil best-effort untuk label cetak.
 */
export async function fetchDeliveryPrint(id: number): Promise<DeliveryPrintData> {
  const note = await deliveryService.get(id);
  const order = await salesService.get(note.salesOrderId).catch(() => null);
  return { note, order };
}

/** Faktur/nota: orderView sales sudah membawa semuanya (C2 + C3). */
export function fetchSalesOrderForPrint(id: number): Promise<SalesOrder> {
  return salesService.get(id);
}

/** Kwitansi: paymentView + alokasinya. */
export function fetchPaymentForPrint(id: number): Promise<Payment> {
  return paymentService.get(id);
}

export interface AllocationBilling {
  total: number;
  paid: number;
  outstanding: number;
}

/**
 * Ringkasan tagihan tiap order yang dialokasi kwitansi (kunci "tipe:id").
 * Best-effort: tanpa izin/galat, order itu dilewati dan kwitansi tetap tercetak.
 */
export async function fetchAllocationBilling(
  allocs: { orderType: string; orderId: number }[],
): Promise<Record<string, AllocationBilling>> {
  const out: Record<string, AllocationBilling> = {};
  await Promise.all(
    allocs.slice(0, 10).map(async (a) => {
      try {
        if (a.orderType === "sales") {
          const o = await salesService.get(a.orderId);
          out[`sales:${a.orderId}`] = {
            total: o.grandTotal,
            paid: o.paidAmount ?? 0,
            outstanding: o.outstandingAmount ?? 0,
          };
        } else if (a.orderType === "purchase") {
          const o = await purchaseOrderService.get(a.orderId);
          out[`purchase:${a.orderId}`] = {
            total: o.grandTotal,
            paid: o.paidAmount ?? 0,
            outstanding: o.outstandingAmount ?? 0,
          };
        }
      } catch {
        // dilewati
      }
    }),
  );
  return out;
}

export type { DeliveryNote, Payment, SalesOrder };
