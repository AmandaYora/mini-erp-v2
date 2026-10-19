import type { ReactNode } from "react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/shared/components/primitives/sheet";
import { cn } from "@/shared/lib/cn";

export type FormSheetSize = "md" | "lg";

const WIDTH: Record<FormSheetSize, string> = {
  md: "sm:max-w-[480px]",
  lg: "sm:max-w-[640px]",
};

export interface FormSheetProps {
  open: boolean;
  title: string;
  description?: string;
  badge?: ReactNode;
  /** Tombol footer (Batal · Simpan). Menempel di dasar panel. */
  actions?: ReactNode;
  size?: FormSheetSize;
  onClose: () => void;
  children: ReactNode;
}

/**
 * Panel samping kanan untuk CRUD master data dan aksi pada dokumen (PLAN §7.0).
 * API sengaja sama dengan Modal agar konversi modal → sheet hanya mengganti nama komponen.
 */
export function FormSheet({ open, title, description, badge, actions, size = "md", onClose, children }: FormSheetProps) {
  return (
    <Sheet open={open} onOpenChange={(next) => !next && onClose()}>
      <SheetContent side="right" className={cn("flex w-full flex-col gap-0 bg-card p-0", WIDTH[size])}>
        <SheetHeader className="border-b border-border p-5 text-left">
          <div className="flex flex-wrap items-center gap-2 pr-8">
            <SheetTitle className="text-xl font-semibold text-foreground">{title}</SheetTitle>
            {badge}
          </div>
          <SheetDescription className={description ? "text-sm text-muted-foreground" : "sr-only"}>
            {description ?? title}
          </SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto p-5">{children}</div>
        {actions && (
          <SheetFooter className="flex-row justify-end gap-2 border-t border-border bg-card p-4">{actions}</SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  );
}
