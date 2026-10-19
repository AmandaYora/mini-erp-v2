import type { WhatsAppChannel } from "@/modules/dashboard/services/dashboard.service";

export type WhatsAppState = "connected" | "connecting" | "disconnected";

/** Status kartu WA: terhubung, menghubungkan (menunggu pindai QR), atau belum. */
export function whatsappState(ch: WhatsAppChannel | null | undefined): WhatsAppState {
  if (!ch) return "disconnected";
  if (ch.connected) return "connected";
  return ch.qrDataUrl ? "connecting" : "disconnected";
}

export const WHATSAPP_LABEL: Record<WhatsAppState, string> = {
  connected: "Terhubung",
  connecting: "Menghubungkan",
  disconnected: "Belum terhubung",
};
