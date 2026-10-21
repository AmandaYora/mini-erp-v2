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

export interface ReceiptOrderInfo {
  number: string;
  /** Tagihan order (total bersih setelah retur bila ada). */
  total: number;
}

export interface PaymentReceiptData {
  payment: Payment;
  /** Order yang dibayar (alokasi tunggal); null bila tanpa/multi alokasi atau tak terbaca. */
  order: ReceiptOrderInfo | null;
  /** Riwayat pembayaran order itu (untuk "Total Terbayar Setelah Ini"). */
  history: Payment[];
}

/**
 * Bukti pembayaran: pembayaran + order tunggal yang dibayarnya + riwayat
 * pembayaran order itu. Order & riwayat best-effort: tanpa izin, bukti tetap
 * tercetak dengan ringkasan yang tersedia.
 */
export async function fetchPaymentReceipt(id: number): Promise<PaymentReceiptData> {
  const payment = await paymentService.get(id);
  if (payment.allocations.length !== 1) {
    return { payment, order: null, history: [] };
  }
  const ref = payment.allocations[0];
  const [order, history] = await Promise.all([
    (async (): Promise<ReceiptOrderInfo | null> => {
      try {
        if (ref.orderType === "sales") {
          const o = await salesService.get(ref.orderId);
          return { number: o.number, total: o.netTotal ?? o.grandTotal };
        }
        if (ref.orderType === "purchase") {
          const o = await purchaseOrderService.get(ref.orderId);
          return { number: o.number, total: o.grandTotal };
        }
      } catch {
        // tanpa izin baca order: ringkasan tagihan dilewati
      }
      return null;
    })(),
    paymentService
      .list({ orderType: ref.orderType, orderId: ref.orderId, page: 1, limit: 100 })
      .then((page) => page.items)
      .catch(() => [] as Payment[]),
  ]);
  return { payment, order, history };
}

export interface PosReceiptData {
  order: SalesOrder;
  /** Riwayat pembayaran order (semua status; penyaringan di lib/pos-receipt). */
  payments: Payment[];
}

/**
 * Struk POS: order + riwayat pembayaran tersimpannya. Bila riwayat tak terbaca
 * (mis. tanpa izin daftar pembayaran), pembayaran ?paymentId= dipakai sebagai
 * cadangan agar struk checkout tetap lengkap.
 */
export async function fetchPosReceipt(orderId: number, paymentId: number): Promise<PosReceiptData> {
  const order = await salesService.get(orderId);
  let payments = await paymentService
    .list({ orderType: "sales", orderId, page: 1, limit: 100 })
    .then((page) => page.items)
    .catch(() => null);
  if (payments === null) {
    payments =
      Number.isInteger(paymentId) && paymentId > 0
        ? await paymentService
            .get(paymentId)
            .then((p) => [p])
            .catch(() => [])
        : [];
  }
  return { order, payments };
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
