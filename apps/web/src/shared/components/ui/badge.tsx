import type { ReactNode } from "react";
import type { Tone } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";

// Arti nada tetap di seluruh aplikasi (PLAN §4.2):
// neutral = belum berjalan · info = berjalan · warning = perlu perhatian
// success = selesai · muted = berhenti (batal/arsip) · danger = masalah
const TONES: Record<Tone, { pill: string; dot: string }> = {
  success: { pill: "bg-success-soft text-success", dot: "bg-success" },
  warning: { pill: "bg-warning-soft text-warning", dot: "bg-warning" },
  danger: { pill: "bg-destructive-soft text-destructive", dot: "bg-destructive" },
  info: { pill: "bg-accent text-accent-foreground", dot: "bg-primary" },
  neutral: { pill: "bg-muted text-foreground", dot: "bg-muted-foreground" },
  muted: { pill: "border border-dashed border-input bg-transparent text-muted-foreground", dot: "bg-input" },
};

export function Badge({ tone, children, className }: { tone: Tone; children: ReactNode; className?: string }) {
  const t = TONES[tone];
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-[3px] text-xs font-medium whitespace-nowrap",
        t.pill,
        className,
      )}
    >
      <span aria-hidden className={cn("size-1.5 flex-none rounded-full", t.dot)} />
      {children}
    </span>
  );
}
