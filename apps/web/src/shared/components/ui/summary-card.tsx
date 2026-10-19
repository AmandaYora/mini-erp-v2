import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

export type SummaryTone = "brand" | "success" | "warning" | "danger";

export interface SummaryCardProps {
  label: string;
  value: string;
  hint?: string;
  tone?: SummaryTone;
  onClick?: () => void;
  /** Perubahan vs pembanding, mis. "+12% vs kemarin" — tampilkan hanya bila datanya ada. */
  delta?: { text: string; tone: "up" | "down" | "flat" };
  /** Sisipan di kanan label (mis. ikon info). */
  aside?: ReactNode;
}

const DOT: Record<SummaryTone, string> = {
  brand: "bg-primary",
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-destructive",
};

const DELTA: Record<"up" | "down" | "flat", string> = {
  up: "text-success",
  down: "text-destructive",
  flat: "text-muted-foreground",
};

/** Kartu KPI: label kecil, angka besar tabular (Jakarta), petunjuk/delta. Bila `onClick`, ia tombol sungguhan. */
export function SummaryCard({ label, value, hint, tone = "brand", onClick, delta, aside }: SummaryCardProps) {
  const body = (
    <>
      <span className="flex items-center gap-2">
        <span aria-hidden className={cn("size-2 flex-none rounded-full", DOT[tone])} />
        <span className="truncate text-sm font-medium text-muted-foreground">{label}</span>
        {aside && <span className="ml-auto">{aside}</span>}
      </span>
      <span className="tabular mt-2 block truncate font-display text-3xl leading-tight font-semibold tracking-[-0.02em] text-foreground">
        {value}
      </span>
      {(hint || delta) && (
        <span className="mt-1 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
          {delta && <span className={cn("font-medium", DELTA[delta.tone])}>{delta.text}</span>}
          {hint && <span>{hint}</span>}
        </span>
      )}
    </>
  );
  const cls = "block w-full rounded-xl border border-border bg-card p-4 text-left";
  if (!onClick) return <div className={cls}>{body}</div>;
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(cls, "transition-colors duration-150 hover:border-input hover:bg-background")}
    >
      {body}
    </button>
  );
}
