import { CircleDashed, Info, OctagonAlert, TriangleAlert, type LucideIcon } from "lucide-react";
import type { Tone } from "@/shared/lib/format";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { userKey } from "@/shared/stores/ui-prefs.store";
import type { AlertArea, AlertSeverity, HealthAlert } from "@/modules/data-health/types";

// Tingkat dibawa oleh BENTUK ikon + kata (brief §2 #3–4, WCAG 1.4.1): segi delapan = kritis,
// segitiga = perhatian, lingkaran = info. Warna hanya penguat.
export const SEVERITY: Record<AlertSeverity, { label: string; tone: Tone; Icon: LucideIcon; text: string }> = {
  critical: { label: "Kritis", tone: "danger", Icon: OctagonAlert, text: "text-destructive" },
  warning: { label: "Perlu perhatian", tone: "warning", Icon: TriangleAlert, text: "text-warning" },
  info: { label: "Info", tone: "info", Icon: Info, text: "text-primary" },
};

/** Ikon pemeriksaan yang gagal dijalankan — tidak pernah hijau. */
export const FAILED_ICON = CircleDashed;

/** id elemen judul baris alert (fokus & gulir). */
export function alertHeadingId(id: string): string {
  return `alert-${id.replace(/[^a-z0-9]+/gi, "-")}`;
}

/** Urutan tetap area di halaman (brief §5d). */
export const AREA_ORDER: AlertArea[] = ["keuangan", "persediaan", "master", "dokumen", "keamanan"];

export const AREA_LABEL: Record<AlertArea, string> = {
  keuangan: "Keuangan",
  persediaan: "Persediaan",
  master: "Master",
  dokumen: "Dokumen",
  keamanan: "Keamanan",
};

const RANK: Record<AlertSeverity, number> = { critical: 0, warning: 1, info: 2 };

/** Tingkat tertinggi dari sekumpulan alert (null bila kosong). */
export function topSeverity(list: HealthAlert[]): AlertSeverity | null {
  let best: AlertSeverity | null = null;
  for (const a of list) if (best === null || RANK[a.severity] < RANK[best]) best = a.severity;
  return best;
}

/** "64 produk" */
export function countLabel(a: Pick<HealthAlert, "count" | "countUnit">): string {
  return `${formatNumber(a.count)} ${a.countUnit}`;
}

/** Baris 2 kartu dashboard: jumlah · Rp, atau jumlah · umur bila tanpa Rp. */
export function cardDetail(a: HealthAlert, today: string): string {
  const parts = [countLabel(a)];
  if (a.amountRp !== null) parts.push(formatIDR(a.amountRp));
  else {
    const age = oldestDays(a.oldestAt, today);
    if (age !== null && age > 0) parts.push(`tertua ${formatNumber(age)} hari`);
  }
  return parts.join(" · ");
}

/** Nama aksesibel tautan baris kartu (brief §8). */
export function cardAccessibleName(a: HealthAlert, today: string): string {
  return `${SEVERITY[a.severity].label}: ${a.title}, ${cardDetail(a, today)}`;
}

/** Umur masalah dalam hari kalender dari `oldestAt` (YYYY-MM-DD) ke hari ini WIB. */
export function oldestDays(oldestAt: string | null, today: string): number | null {
  if (!oldestAt || oldestAt.length < 10) return null;
  const from = Date.parse(`${oldestAt.slice(0, 10)}T00:00:00Z`);
  const to = Date.parse(`${today}T00:00:00Z`);
  if (Number.isNaN(from) || Number.isNaN(to)) return null;
  return Math.max(0, Math.round((to - from) / 86_400_000));
}

// Zona waktu terkunci Asia/Jakarta (ADR-0006): cap waktu server (UTC, RFC 3339) selalu
// ditampilkan dalam WIB, tidak bergantung pada zona perangkat.
const WIB = "Asia/Jakarta";

/** "10.42" */
export function wibClock(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleTimeString("id-ID", { timeZone: WIB, hour: "2-digit", minute: "2-digit" });
}

/** "23 Okt" */
export function wibDay(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso.length <= 10 ? `${iso}T00:00:00+07:00` : iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleDateString("id-ID", { timeZone: WIB, day: "numeric", month: "short" });
}

/** "Sab, 24 Okt" — akhir durasi tunda yang dipilih, dihitung dari sekarang. */
export function snoozeEndLabel(days: number, now = Date.now()): string {
  const d = new Date(now + days * 86_400_000);
  return d.toLocaleDateString("id-ID", { timeZone: WIB, weekday: "short", day: "numeric", month: "short" });
}

/** Menit sejak cap waktu (untuk "data basi", brief §7). */
export function minutesSince(iso: string | null | undefined, now = Date.now()): number {
  if (!iso) return 0;
  const t = Date.parse(iso);
  return Number.isNaN(t) ? 0 : Math.max(0, Math.floor((now - t) / 60_000));
}

/** Basi bila hasil lebih tua dari 15 menit. */
export const STALE_MINUTES = 15;

// --- Jejak penyelesaian (brief §2, elemen khas) -----------------------------------------
// Pembanding PER PENGGUNA di localStorage: {[alertId]: {count, at}}. Hanya diperbarui bila
// alertnya baru atau jumlahnya naik, agar kemajuan tidak hilang saat halaman dibuka ulang.
// Kenyamanan per penonton: penyimpanan bisa kosong/ditolak, maka semua akses dibungkus try/catch.

export interface SeenMark {
  count: number;
  at: string;
}

const SEEN_BASE = "data-health-seen";

export function readSeen(): Record<string, SeenMark> {
  try {
    const raw = localStorage.getItem(userKey(SEEN_BASE));
    const parsed = raw ? (JSON.parse(raw) as unknown) : {};
    return parsed && typeof parsed === "object" ? (parsed as Record<string, SeenMark>) : {};
  } catch {
    return {};
  }
}

/** Gabungkan pembanding lama dengan alert aktif sekarang; alert yang hilang dibuang. */
export function mergeSeen(prev: Record<string, SeenMark>, active: HealthAlert[], nowIso: string): Record<string, SeenMark> {
  const next: Record<string, SeenMark> = {};
  for (const a of active) {
    const old = prev[a.id];
    next[a.id] = old && old.count >= a.count ? old : { count: a.count, at: nowIso };
  }
  return next;
}

export function writeSeen(next: Record<string, SeenMark>): void {
  try {
    localStorage.setItem(userKey(SEEN_BASE), JSON.stringify(next));
  } catch {
    // Penyimpanan ditolak (mode privat): bilah jejak cukup tidak tampil.
  }
}

export interface Progress {
  remaining: number;
  from: number;
  since: string;
  /** "sejak ditunda" (server) atau "sejak Anda lihat" (lokal). */
  basis: "snooze" | "seen";
}

/** Kemajuan sebagian; null bila jumlah tidak berkurang dari pembanding. */
export function progressOf(a: HealthAlert, seen: SeenMark | undefined): Progress | null {
  if (a.snooze.countAtSnooze !== null && a.snooze.snoozedAt && a.count < a.snooze.countAtSnooze) {
    return { remaining: a.count, from: a.snooze.countAtSnooze, since: a.snooze.snoozedAt, basis: "snooze" };
  }
  if (seen && a.count < seen.count) return { remaining: a.count, from: seen.count, since: seen.at, basis: "seen" };
  return null;
}

/** Total rupiah terdampak dari alert aktif (yang bernilai uang). */
export function totalAmount(list: HealthAlert[]): number {
  return list.reduce((n, a) => n + (a.amountRp ?? 0), 0);
}
