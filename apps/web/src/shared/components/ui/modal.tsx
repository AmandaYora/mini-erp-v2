import { useEffect } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

export type ModalSize = "sm" | "md" | "lg" | "xl";

const MAX_WIDTH: Record<ModalSize, string> = {
  sm: "max-w-[440px]",
  md: "max-w-[560px]",
  lg: "max-w-[760px]",
  xl: "max-w-[960px]",
};

export interface ModalProps {
  open: boolean;
  title: string;
  description?: string;
  badge?: ReactNode;
  actions?: ReactNode;
  size?: ModalSize;
  onClose: () => void;
  children: ReactNode;
}

export function Modal({
  open,
  title,
  description,
  badge,
  actions,
  size = "md",
  onClose,
  children,
}: ModalProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [open, onClose]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div
      className="fixed inset-0 z-[999] flex items-center justify-center bg-ink/50 p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`max-h-[90vh] w-full overflow-y-auto rounded-[10px] bg-surface p-6 ${MAX_WIDTH[size]}`}
      >
        <div className="flex items-start justify-between gap-4">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-bold text-ink">{title}</h2>
            {badge}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Tutup"
            className="text-muted cursor-pointer rounded p-1 hover:text-ink"
          >
            <X size={18} />
          </button>
        </div>
        {description && (
          <p className="text-muted mt-1 text-sm">{description}</p>
        )}
        <div className="mt-4">{children}</div>
        {actions && (
          <div className="mt-6 flex justify-end gap-3">{actions}</div>
        )}
      </div>
    </div>,
    document.body,
  );
}
