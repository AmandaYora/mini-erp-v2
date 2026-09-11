import { create } from "zustand";
import type { Tone } from "@/shared/lib/format";

export interface Toast {
  id: number;
  title: string;
  description?: string;
  tone: Tone;
}

let nextId = 1;

interface ToastState {
  toasts: Toast[];
  push: (t: Omit<Toast, "id">) => void;
  dismiss: (id: number) => void;
}

/** Toast global ala aplikasi lama (pojok kanan bawah, border-l-4). */
export const useToastStore = create<ToastState>((set) => ({
  toasts: [],
  push: (t) => {
    const id = nextId++;
    set((s) => ({ toasts: [...s.toasts.slice(-4), { ...t, id }] }));
    window.setTimeout(() => {
      set((s) => ({ toasts: s.toasts.filter((x) => x.id !== id) }));
    }, 4500);
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((x) => x.id !== id) })),
}));

export const toast = {
  success: (title: string, description?: string) => useToastStore.getState().push({ title, description, tone: "success" }),
  warning: (title: string, description?: string) => useToastStore.getState().push({ title, description, tone: "warning" }),
  danger: (title: string, description?: string) => useToastStore.getState().push({ title, description, tone: "danger" }),
  info: (title: string, description?: string) => useToastStore.getState().push({ title, description, tone: "info" }),
  /** Tampilkan pesan server bila ada, fallback ke teks klien bila kosong.
   * Kontrak B1: aksi tulis memakai message backend sebagai sumber kebenaran. */
  fromServer: (serverMessage: string | undefined | null, fallback: string, description?: string) => {
    const msg = (serverMessage ?? "").trim();
    useToastStore.getState().push({ title: msg !== "" ? msg : fallback, description, tone: "success" });
  },
};
