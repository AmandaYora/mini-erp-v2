import type { ReactNode } from "react";
import { Popover } from "radix-ui";
import { cn } from "@/shared/lib/cn";

/**
 * Cangkang popover bersama trio select (sync / hierarchical / async), dibangun di atas
 * Popover Radix: menempel ke trigger, membalik arah sesuai ruang, menutup dengan Esc /
 * klik luar, dan — yang terpenting — bisa hidup di dalam Dialog Radix (Modal) tanpa
 * pointer-events diblokir. Menggantikan posisi manual + createPortal lama.
 */
export const SelectPopover = Popover.Root;
export const SelectPopoverTrigger = Popover.Trigger;

/** Tinggi daftar mengikuti ruang tersedia (dikurangi tinggi kotak cari), minimal 120px. */
export const LIST_MAX_HEIGHT =
  "max(120px, min(20rem, calc(var(--radix-popover-content-available-height) - 56px)))";

export function SelectPopoverContent({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <Popover.Portal>
      <Popover.Content
        align="start"
        sideOffset={4}
        collisionPadding={8}
        className={cn("z-[60] w-[var(--radix-popover-trigger-width)] min-w-0", className)}
      >
        {children}
      </Popover.Content>
    </Popover.Portal>
  );
}
