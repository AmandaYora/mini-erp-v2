// Fungsi murni dasbor (mudah diuji): tanggal WIB, format ringkas, sapaan, dan
// penyusunan daftar "Perlu tindakan" dari data yang sudah di-fetch.
import { ROUTE_PATHS, stockItemPath } from "@/app/routes/route-paths";
import type { CriticalStockItem, PriorityOrder } from "@/modules/dashboard/types";

/** YYYY-MM-DD digeser N hari (kalender UTC agar stabil di sekitar tengah malam). */
export function addDays(ymd: string, delta: number): string {
  const d = new Date(`${ymd}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + delta);
  return d.toISOString().slice(0, 10);
}

/** Sumbu Y padat: 1500000 → "2 jt", 250000 → "250 rb". */
export function compactIDR(v: number): string {
  const abs = Math.abs(v);
  if (abs >= 1_000_000_000) return `${Math.round((v / 1_000_000_000) * 10) / 10} M`;
  if (abs >= 1_000_000) return `${Math.round(v / 1_000_000)} jt`;
  if (abs >= 1_000) return `${Math.round(v / 1_000)} rb`;
  return String(Math.round(v));
}

/** Selisih hari antara `today` (YYYY-MM-DD, WIB) dan tanggal tempo (positif = sudah lewat tempo). */
export function overdueDays(dueDate: string, today: string): number {
  const ms = new Date(`${today}T00:00:00Z`).getTime() - new Date(`${dueDate.slice(0, 10)}T00:00:00Z`).getTime();
  return Math.round(ms / 86_400_000);
}

/** Sapaan menurut jam WIB (0–23). */
export function greetingForHour(hour: number): string {
  if (hour >= 4 && hour < 11) return "Selamat pagi";
  if (hour >= 11 && hour < 15) return "Selamat siang";
  if (hour >= 15 && hour < 18) return "Selamat sore";
  return "Selamat malam";
}

/** Jam WIB (0–23) dari waktu epoch ms. */
export function hourWIB(now = Date.now()): number {
  return new Date(now + 7 * 3600 * 1000).getUTCHours();
}

/** Nama panggilan: kata pertama nama lengkap; kosong bila tak ada. */
export function firstName(fullName: string | null | undefined): string {
  return (fullName ?? "").trim().split(/\s+/)[0] ?? "";
}

export function greetingTitle(fullName: string | null | undefined, now = Date.now()): string {
  const name = firstName(fullName);
  const greet = greetingForHour(hourWIB(now));
  return name ? `${greet}, ${name}` : greet;
}

/** "Jumat, 16 Okt 2026" untuk subjudul. */
export function longDateWIB(ymd: string): string {
  const d = new Date(`${ymd}T00:00:00Z`);
  return d.toLocaleDateString("id-ID", { weekday: "long", day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
}

export const STATUS_ORDER = ["draft", "confirmed", "completed", "cancelled"] as const;
export type OrderStatusKey = (typeof STATUS_ORDER)[number];

export interface StatusSegment {
  status: OrderStatusKey;
  count: number;
  /** Persentase dari total (0–100), 0 bila total 0. */
  percent: number;
}

/** Hitungan per status → segmen bar 100% (urutan tetap). */
export function statusSegments(counts: Record<string, number> | null | undefined): { total: number; segments: StatusSegment[] } {
  const total = STATUS_ORDER.reduce((n, s) => n + (counts?.[s] ?? 0), 0);
  return {
    total,
    segments: STATUS_ORDER.map((s) => {
      const count = counts?.[s] ?? 0;
      return { status: s, count, percent: total > 0 ? (count / total) * 100 : 0 };
    }),
  };
}

export type ActionKind = "order" | "stock";

export interface ActionItem {
  key: string;
  kind: ActionKind;
  /** danger = lewat tempo/habis, warning = menjelang tempo/kritis. */
  tone: "danger" | "warning";
  title: string;
  detail: string;
  to: string;
}

/**
 * Keterangan satu pesanan prioritas. Dengan alasan dari server (DB-01) tampil
 * "Lewat jatuh tempo · terlambat 3 hari · umur 12 hari"; tanpa alasan tetap
 * teks lama (data lama / klien lama).
 */
export function orderDetail(o: PriorityOrder, late: number): string {
  if (!o.reason) return late > 0 ? `Terlambat ${late} hari` : "Jatuh tempo dalam 7 hari";
  const parts = [o.kind === "purchase" ? `Pembelian: ${o.reason}` : o.reason];
  if (late > 0) parts.push(`terlambat ${late} hari`);
  if (o.ageDays && o.ageDays > 0) parts.push(`umur ${o.ageDays} hari`);
  return parts.join(" · ");
}

/** Berapa baris stok kritis & pesanan prioritas yang ditampilkan di kartu. */
export const ACTION_STOCK_LIMIT = 3;
export const ACTION_ORDER_LIMIT = 5;

/**
 * Gabungan priorityOrders + criticalStock menjadi satu daftar "Perlu tindakan".
 * Urutan: yang lewat tempo/habis dulu, lalu yang menjelang/kritis. Tiap baris punya tautan
 * ke daftar yang sudah terfilter (SO dicari per nomor; stok ke kartu stok produk).
 */
export function buildActionItems(
  orders: PriorityOrder[] | null | undefined,
  stock: CriticalStockItem[] | null | undefined,
  today: string,
): ActionItem[] {
  const items: ActionItem[] = [];
  for (const o of (orders ?? []).slice(0, ACTION_ORDER_LIMIT)) {
    const late = o.dueDate ? overdueDays(o.dueDate, today) : 0;
    const purchase = o.kind === "purchase";
    items.push({
      key: `${purchase ? "po" : "so"}-${o.id}`,
      kind: "order",
      tone: late > 0 ? "danger" : "warning",
      title: `${o.number} · ${o.partyName || (purchase ? "Supplier" : "Tunai")}`,
      detail: orderDetail(o, late),
      to: `${purchase ? ROUTE_PATHS.purchaseOrders : ROUTE_PATHS.salesOrders}?search=${encodeURIComponent(o.number)}`,
    });
  }
  for (const s of (stock ?? []).slice(0, ACTION_STOCK_LIMIT)) {
    items.push({
      key: `stk-${s.productId}`,
      kind: "stock",
      tone: s.available <= 0 ? "danger" : "warning",
      title: `${s.productCode} · ${s.productName}`,
      detail: s.available <= 0 ? `Stok habis (minimum ${s.minStock})` : `Sisa ${s.available} dari minimum ${s.minStock}`,
      to: stockItemPath(s.productId),
    });
  }
  // sort stabil: urutan asal dipertahankan dalam nada yang sama
  return items.sort((a, b) => (a.tone === b.tone ? 0 : a.tone === "danger" ? -1 : 1));
}
