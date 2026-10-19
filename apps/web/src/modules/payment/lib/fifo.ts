import type { OrderBalance } from "@/modules/payment/types";

export interface FifoPreviewRow {
  orderType: string;
  orderId: number;
  number: string;
  outstanding: number;
  amount: number;
}

export interface FifoPreview {
  rows: FifoPreviewRow[];
  /** Nominal yang tidak tertampung tagihan terbuka (backend akan menolak bila > 0). */
  unallocated: number;
}

/**
 * Pratinjau alokasi FIFO sisi klien: tagihan terbuka diurut sesuai urutan
 * saldo dari server (tertua dulu), nominal dibagi sampai habis. Hanya
 * perkiraan — backend tetap penentu akhir (urut jatuh tempo, lalu tanggal).
 */
export function previewFifo(orders: OrderBalance[], amount: number): FifoPreview {
  const rows: FifoPreviewRow[] = [];
  let left = Number.isFinite(amount) && amount > 0 ? Math.floor(amount) : 0;
  for (const o of orders) {
    if (left <= 0) break;
    if (o.outstanding <= 0) continue;
    const take = Math.min(left, o.outstanding);
    rows.push({
      orderType: o.orderType,
      orderId: o.orderId,
      number: o.number,
      outstanding: o.outstanding,
      amount: take,
    });
    left -= take;
  }
  return { rows, unallocated: left };
}

/** Kembalian tunai: uang diterima - nominal, minimal 0. */
export function cashChange(tendered: number, amount: number): number {
  return tendered > amount ? tendered - amount : 0;
}
