// Format id-ID terpusat: rupiah tanpa desimal, tanggal gaya Indonesia.
// Komponen JANGAN memformat manual — selalu lewat helper ini.

const idID = "id-ID";

export function formatIDR(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return "-";
  return new Intl.NumberFormat(idID, {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(n);
}

export function formatNumber(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return "-";
  return new Intl.NumberFormat(idID).format(n);
}

/** "10 Sep 2026" — tanggal saja. */
export function formatDate(v: string | null | undefined): string {
  if (!v) return "-";
  // Server mengirim jam dinding WIB "YYYY-MM-DD hh:mm:ss"; spasi diganti "T" agar terurai di semua mesin JS.
  const d = new Date(v.length <= 10 ? `${v}T00:00:00` : v.includes(" ") && !v.includes("T") ? v.replace(" ", "T") : v);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleDateString(idID, { day: "numeric", month: "short", year: "numeric" });
}

/** "10 Sep 2026 14:30" — tanggal + jam. */
export function formatDateTime(v: string | null | undefined): string {
  if (!v) return "-";
  const d = new Date(v.includes(" ") && !v.includes("T") ? v.replace(" ", "T") : v);
  if (Number.isNaN(d.getTime())) return "-";
  return (
    d.toLocaleDateString(idID, { day: "numeric", month: "short", year: "numeric" }) +
    " " +
    d.toLocaleTimeString(idID, { hour: "2-digit", minute: "2-digit" }).replace(".", ":")
  );
}

/** Hari ini dalam WIB (YYYY-MM-DD) — filter default dasbor/laporan. */
export function todayWIB(): string {
  return new Date(Date.now() + 7 * 3600 * 1000).toISOString().slice(0, 10);
}

/** Awal bulan berjalan dalam WIB (YYYY-MM-01). */
export function monthStartWIB(): string {
  return todayWIB().slice(0, 8) + "01";
}

/**
 * Arti nada (PLAN §4.2), satu peta untuk seluruh aplikasi:
 * neutral = belum berjalan · info = berjalan · warning = perlu perhatian ·
 * success = selesai · muted = berhenti (batal/arsip) · danger = masalah.
 */
export type Tone = "success" | "warning" | "danger" | "info" | "neutral" | "muted";

/** Status dokumen → tone badge (konsisten di semua modul). */
export function statusTone(status: string | null | undefined): Tone {
  switch ((status ?? "").toLowerCase()) {
    case "completed":
    case "active":
    case "paid":
    case "received":
    case "closed":
    case "posted":
      return "success";
    case "confirmed":
    case "dispatched":
    case "pending":
    case "open":
    case "partial":
      return "info";
    case "pending_approval":
    case "partially_paid":
      return "warning";
    case "draft":
      return "neutral";
    case "cancelled":
    case "archived":
    case "inactive":
    case "reversed":
    case "void":
      return "muted";
    case "expired":
    case "overdue":
      return "danger";
    default:
      return "neutral";
  }
}

/** Label Indonesia untuk status dokumen. */
export function statusLabel(status: string | null | undefined): string {
  switch ((status ?? "").toLowerCase()) {
    case "draft": return "Draf";
    case "confirmed": return "Terkonfirmasi";
    case "completed": return "Selesai";
    case "cancelled": return "Dibatalkan";
    case "dispatched": return "Dikirim";
    case "received": return "Diterima";
    case "active": return "Aktif";
    case "inactive": return "Nonaktif";
    case "archived": return "Diarsipkan";
    case "paid": return "Lunas";
    case "pending": return "Menunggu";
    case "open": return "Terbuka";
    case "closed": return "Ditutup";
    case "posted": return "Terposting";
    case "void": return "Dibatalkan";
    case "reversed": return "Dibalik";
    case "partial": return "Sebagian";
    case "partially_paid": return "Dibayar sebagian";
    case "pending_approval": return "Menunggu persetujuan";
    case "overdue": return "Jatuh tempo";
    case "expired": return "Kedaluwarsa";
    default: return status ?? "-";
  }
}
