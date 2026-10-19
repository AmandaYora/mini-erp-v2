import type { OrderReport, OrderReportRow, OrderReportType } from "@/modules/reporting/types";

type Raw = Record<string, unknown>;

function pick(raw: Raw, camel: string, pascal: string): unknown {
  return raw[camel] ?? raw[pascal];
}

function num(v: unknown): number {
  return typeof v === "number" ? v : Number(v ?? 0) || 0;
}

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

/** Terima baris camelCase maupun PascalCase (backend belum bertag json). */
export function normalizeOrderRow(raw: Raw): OrderReportRow {
  return {
    id: num(pick(raw, "id", "ID")),
    number: str(pick(raw, "number", "Number")),
    grandTotal: num(pick(raw, "grandTotal", "GrandTotal")),
    status: str(pick(raw, "status", "Status")),
    partyId: num(pick(raw, "partyId", "PartyID")),
    partyName: str(pick(raw, "partyName", "PartyName")),
    orderDate: str(pick(raw, "orderDate", "OrderDate")),
    dueDate: str(pick(raw, "dueDate", "DueDate")),
  };
}

export function normalizeOrderReport(raw: {
  type?: string;
  rows?: Raw[] | null;
  total?: number;
  grandTotal?: number;
}): OrderReport {
  const type: OrderReportType = raw.type === "purchasing" ? "purchasing" : "sales";
  return {
    type,
    rows: (raw.rows ?? []).map(normalizeOrderRow),
    total: raw.total ?? 0,
    grandTotal: raw.grandTotal ?? 0,
  };
}

/** Rentang tanggal valid bila kosong atau from <= to. */
export function validRange(from: string, to: string): boolean {
  return !from || !to || from <= to;
}

/** Path halaman dokumen sumber per tipe laporan order. */
export function orderDetailPath(type: OrderReportType, id: number): string {
  return type === "sales" ? `/sales-orders/${id}` : `/purchase-orders/${id}`;
}
