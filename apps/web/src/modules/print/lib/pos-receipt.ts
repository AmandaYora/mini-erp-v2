// Logika murni struk POS (paritas pos-thermal-receipt-print-page.tsx sistem
// lama). Pembayaran dibaca dari riwayat TERSIMPAN order (bukan state klien),
// jadi cetak ulang memberi angka yang sama:
// - No Bayar / Tanggal / tunai diterima dari pembayaran terakhir;
// - Metode = gabungan metode semua pembayaran ("Tempo" bila belum dibayar & bertempo);
// - cap LUNAS / PIUTANG dari sisa tagihan riil;
// - baris barang bruto + baris "Diskon"; angka ringkasan dari lib/sales-document.

import { printMethodLabel } from "@/modules/print/document";
import { documentTotals, type DocOrder } from "@/modules/print/lib/sales-document";

export interface ReceiptPayment {
  id: number;
  number: string;
  method: string;
  paidAt: string;
  status: string;
  amount: number;
  /** Uang tunai diserahkan (hanya cash); 0 = tidak dicatat. */
  amountTendered: number;
}

/** Pembayaran sah order, urut waktu bayar lalu id. */
export function receiptPayments<P extends ReceiptPayment>(history: P[]): P[] {
  const t = (p: P) => {
    // Server mengirim "YYYY-MM-DD hh:mm:ss"; ganti spasi agar terurai di semua mesin JS.
    const raw = p.paidAt.includes(" ") && !p.paidAt.includes("T") ? p.paidAt.replace(" ", "T") : p.paidAt;
    const v = new Date(raw).getTime();
    return Number.isNaN(v) ? 0 : v;
  };
  return history.filter((p) => p.status !== "cancelled").sort((a, b) => t(a) - t(b) || a.id - b.id);
}

export interface ReceiptSummary {
  statusLabel: "LUNAS" | "PIUTANG" | "BATAL";
  paymentNumber: string;
  receiptDate: string;
  methods: string;
  subtotal: number;
  /** null = baris Diskon tidak dicetak. */
  discount: number | null;
  tax: number | null;
  total: number;
  paid: number;
  /** null = baris Tunai/Kembalian tidak dicetak. */
  tendered: number | null;
  change: number | null;
  /** null = baris Sisa tidak dicetak. */
  outstanding: number | null;
}

export function receiptSummary(
  order: DocOrder & { orderDate: string; paymentTerms: string },
  payments: ReceiptPayment[],
  hideDiscount: boolean,
): ReceiptSummary {
  const t = documentTotals(order, hideDiscount);
  const latest = payments.length > 0 ? payments[payments.length - 1] : null;
  const tendered = latest !== null && latest.method === "cash" && latest.amountTendered > 0 ? latest.amountTendered : null;
  const methods =
    payments.length > 0
      ? Array.from(new Set(payments.map((p) => printMethodLabel(p.method)))).join(", ")
      : order.paymentTerms === "net"
        ? "Tempo"
        : "-";
  return {
    statusLabel: order.status === "cancelled" ? "BATAL" : t.realOutstanding <= 0 ? "LUNAS" : "PIUTANG",
    paymentNumber: latest?.number ?? "-",
    receiptDate: latest?.paidAt ?? order.orderDate,
    methods,
    subtotal: t.grossSubtotal,
    discount: t.discount,
    tax: t.tax,
    total: t.total,
    paid: t.paid ?? 0,
    tendered,
    // Kembalian terhadap total yang tercetak (bruto saat diskon disembunyikan).
    change: tendered !== null ? Math.max(0, tendered - t.total) : null,
    outstanding: t.outstanding > 0 ? t.outstanding : null,
  };
}
