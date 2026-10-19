import type { DayStatus } from "@/modules/finance/types";

/** Label status hari tutup buku (kosakata legacy). */
export function dayStatusLabel(s: DayStatus): string {
  switch (s) {
    case "kosong":
      return "Belum ada transaksi";
    case "belum":
      return "Belum selesai";
    case "selesai_catatan":
      return "Selesai dengan catatan";
    default:
      return "Lengkap";
  }
}

export function dayStatusTone(s: DayStatus): "neutral" | "warning" | "info" | "success" {
  switch (s) {
    case "kosong":
      return "neutral";
    case "belum":
      return "warning";
    case "selesai_catatan":
      return "info";
    default:
      return "success";
  }
}
