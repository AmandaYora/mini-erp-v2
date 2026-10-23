import { cn } from "@/shared/lib/cn";
import { SEVERITY } from "@/modules/data-health/lib/alerts";
import type { AlertSeverity } from "@/modules/data-health/types";

/** Ikon tingkat: bentuk berbeda per tingkat (segi delapan/segitiga/lingkaran), warna hanya penguat. */
export function SeverityIcon({ severity, size = 18, className }: { severity: AlertSeverity; size?: number; className?: string }) {
  const s = SEVERITY[severity];
  return <s.Icon size={size} aria-hidden className={cn("flex-none", s.text, className)} />;
}
