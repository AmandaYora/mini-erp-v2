import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

export interface SectionCardProps {
  title?: string;
  description?: string;
  actions?: ReactNode;
  /** "plain" = tanpa bingkai kartu (seksi di dalam halaman). */
  variant?: "card" | "plain";
  /** Tanpa padding isi — untuk tabel/DataTable yang mengatur ruangnya sendiri. */
  flush?: boolean;
  className?: string;
  children: ReactNode;
}

export function SectionCard({ title, description, actions, variant = "card", flush = false, className, children }: SectionCardProps) {
  const card = variant === "card";
  return (
    <section className={cn(card && "overflow-hidden rounded-xl border border-border bg-card", className)}>
      {(title || actions) && (
        <div
          className={cn(
            "flex flex-wrap items-center justify-between gap-3",
            card ? "border-b border-border px-5 py-4" : "pb-3",
          )}
        >
          <div className="min-w-0">
            {title && <h2 className="text-lg font-semibold text-foreground">{title}</h2>}
            {description && <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className={card && !flush ? "p-5" : undefined}>{children}</div>
    </section>
  );
}
