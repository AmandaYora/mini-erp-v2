import type { IntegrityIssue } from "@/modules/ops/types";

/** Halaman dokumen untuk satu temuan; null bila jenis dokumen tak dikenal. */
export function issueDocPath(docType: string, docId: number): string | null {
  switch (docType) {
    case "delivery":
      return `/deliveries/${docId}`;
    case "goods_receipt":
      return `/goods-receipts/${docId}`;
    case "sales_return":
      return `/sales-returns/${docId}`;
    case "purchase_return":
      return `/purchase-returns/${docId}`;
    default:
      return null;
  }
}

export function issueDocLabel(docType: string): string {
  switch (docType) {
    case "delivery":
      return "Pengiriman";
    case "goods_receipt":
      return "Penerimaan Barang";
    case "sales_return":
      return "Retur Jual";
    case "purchase_return":
      return "Retur Beli";
    default:
      return docType || "-";
  }
}

/** Petunjuk tindakan singkat per jenis temuan (detail lengkap dari server). */
export function issueActionHint(kind: string): string {
  switch (kind) {
    case "draft_with_stock":
      return "Konfirmasi ulang untuk melanjutkan, atau batalkan untuk membalik stok.";
    case "pending_receipt":
      return "Posting ulang atau batalkan penerimaan.";
    case "confirmed_without_stock":
      return "Konfirmasi ulang retur agar stok bergerak.";
    case "cancelled_stock_not_reversed":
      return "Batalkan ulang retur untuk membalik gerakan stok.";
    default:
      return "Buka dokumen dan ulangi atau batalkan tindakannya.";
  }
}

/** Kelompokkan temuan per jenis dokumen (urutan kemunculan dipertahankan). */
export function groupByDocType(issues: IntegrityIssue[]): Map<string, IntegrityIssue[]> {
  const out = new Map<string, IntegrityIssue[]>();
  for (const i of issues) {
    const list = out.get(i.docType) ?? [];
    list.push(i);
    out.set(i.docType, list);
  }
  return out;
}
