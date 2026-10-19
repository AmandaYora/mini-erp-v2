import { toast as sonner } from "sonner";

/** Durasi tampil (ms) — sama dengan toast lama. */
export const TOAST_DURATION_MS = 4500;

const opts = (description?: string) => ({ description, duration: TOAST_DURATION_MS });

/**
 * Toast global (pojok kanan bawah). Dirender oleh <ToastViewport /> (Sonner, DS v2);
 * API pemanggil tidak berubah sejak toast kustom lama.
 */
export const toast = {
  success: (title: string, description?: string) => sonner.success(title, opts(description)),
  warning: (title: string, description?: string) => sonner.warning(title, opts(description)),
  danger: (title: string, description?: string) => sonner.error(title, opts(description)),
  info: (title: string, description?: string) => sonner.info(title, opts(description)),
  /** Tampilkan pesan server bila ada, fallback ke teks klien bila kosong.
   * Kontrak B1: aksi tulis memakai message backend sebagai sumber kebenaran. */
  fromServer: (serverMessage: string | undefined | null, fallback: string, description?: string) => {
    const msg = (serverMessage ?? "").trim();
    sonner.success(msg !== "" ? msg : fallback, opts(description));
  },
};
