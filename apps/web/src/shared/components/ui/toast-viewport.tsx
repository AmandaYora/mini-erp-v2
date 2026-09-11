import { X } from "lucide-react";
import type { Tone } from "@/shared/lib/format";
import { useToastStore } from "@/shared/stores/toast.store";

const BORDER: Record<Tone, string> = {
  success: "border-l-ok",
  warning: "border-l-warn",
  danger: "border-l-bad",
  info: "border-l-brand",
  neutral: "border-l-hairline",
};

export function ToastViewport() {
  const toasts = useToastStore((s) => s.toasts);
  const dismiss = useToastStore((s) => s.dismiss);

  if (toasts.length === 0) return null;

  return (
    <div className="fixed right-6 bottom-6 z-[1000] flex flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`min-w-[280px] max-w-[360px] rounded-md border-l-4 bg-surface p-4 shadow-md ${BORDER[t.tone]}`}
        >
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-semibold text-ink">{t.title}</p>
              {t.description && (
                <p className="text-muted mt-0.5 text-sm">{t.description}</p>
              )}
            </div>
            <button
              type="button"
              onClick={() => dismiss(t.id)}
              aria-label="Tutup notifikasi"
              className="text-muted cursor-pointer rounded p-0.5 hover:text-ink"
            >
              <X size={16} />
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
