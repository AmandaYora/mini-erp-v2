// Path halaman cetak (Tahap E). Didaftarkan pemilik rute lewat wiring
// snippet (lihat catatan di bawah) — modul ini TIDAK menyentuh registry.

export const PRINT_PATHS = {
  deliveryNote: "/print/deliveries/:id",
  salesInvoice: "/print/sales/:id",
  paymentReceipt: "/print/payments/:id",
  posReceipt: "/print/pos/:orderId",
  calibration: "/print/calibration",
} as const;

export function deliveryPrintPath(id: number | string): string {
  return `/print/deliveries/${id}`;
}

export function salesInvoicePrintPath(id: number | string): string {
  return `/print/sales/${id}`;
}

export function paymentReceiptPrintPath(id: number | string): string {
  return `/print/payments/${id}`;
}

export function posReceiptPrintPath(orderId: number | string): string {
  return `/print/pos/${orderId}`;
}
