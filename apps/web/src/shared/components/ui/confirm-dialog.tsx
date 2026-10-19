import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/shared/components/primitives/alert-dialog";
import { Button } from "./button";

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  tone?: "danger" | "primary";
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Konfirmasi (AlertDialog Radix, role=alertdialog). Tombol dikelola manual — Esc/overlay
 * memanggil onCancel; "konfirmasi" tidak ikut memicu onCancel.
 */
export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = "Konfirmasi",
  tone = "danger",
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && onCancel()}>
      <AlertDialogContent className="gap-0 rounded-xl border-border bg-card p-6 shadow-[var(--shadow-pop)] sm:max-w-[440px]">
        <AlertDialogHeader className="text-left">
          <AlertDialogTitle className="text-xl font-semibold text-foreground">{title}</AlertDialogTitle>
          <AlertDialogDescription className="mt-2 text-base text-foreground">{message}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter className="mt-6 flex-row justify-end gap-2 border-t border-border pt-4">
          <Button variant="secondary" size="sm" onClick={onCancel}>
            Batal
          </Button>
          <Button variant={tone === "danger" ? "danger" : "primary"} size="sm" onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
