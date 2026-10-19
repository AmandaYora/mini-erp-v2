import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

/**
 * Seksi form: judul + deskripsi di kiri, isian di kanan (≥ 1024px); menumpuk di layar sempit.
 * Mengganti SectionCard bertumpuk di form panjang.
 */
export function FormSection({
  id,
  title,
  description,
  children,
  className,
}: {
  id?: string;
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      id={id}
      className={cn(
        "grid gap-x-10 gap-y-4 border-b border-border py-6 first:pt-0 last:border-b-0 lg:grid-cols-[240px_minmax(0,1fr)]",
        className,
      )}
    >
      <div>
        <h2 className="text-lg font-semibold text-foreground">{title}</h2>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </div>
      <div className="min-w-0">{children}</div>
    </section>
  );
}
