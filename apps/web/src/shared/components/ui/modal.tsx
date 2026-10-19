import type { ReactNode } from "react";
import { X } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/primitives/dialog";
import { cn } from "@/shared/lib/cn";

export type ModalSize = "sm" | "md" | "lg" | "xl";

const MAX_WIDTH: Record<ModalSize, string> = {
  sm: "sm:max-w-[440px]",
  md: "sm:max-w-[560px]",
  lg: "sm:max-w-[760px]",
  xl: "sm:max-w-[960px]",
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

/**
 * Dialog (Radix): fokus terkunci + dipulihkan, Esc, klik overlay, `aria-labelledby`.
 * Dropdown di dalamnya memakai Popover Radix (select-popover), jadi bisa dibuka di sini.
 */
export function Modal({ open, title, description, badge, actions, size = "md", onClose, children }: ModalProps) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent
        showCloseButton={false}
        className={cn(
          "block max-h-[90vh] w-full gap-0 overflow-y-auto rounded-xl border-border bg-card p-6 shadow-[var(--shadow-pop)]",
          MAX_WIDTH[size],
        )}
      >
        <DialogHeader className="flex-row items-start justify-between gap-4 text-left">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <DialogTitle className="text-xl font-semibold text-foreground">{title}</DialogTitle>
            {badge}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Tutup"
            className="-mt-1 -mr-1 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          >
            <X size={18} />
          </button>
        </DialogHeader>
        <DialogDescription className={description ? "mt-1 text-sm text-muted-foreground" : "sr-only"}>
          {description ?? title}
        </DialogDescription>
        <div className="mt-4">{children}</div>
        {actions && (
          <DialogFooter className="mt-6 flex-row justify-end gap-2 border-t border-border pt-4">
            {actions}
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  );
}
