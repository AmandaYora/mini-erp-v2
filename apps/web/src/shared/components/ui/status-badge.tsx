import { statusLabel, statusTone, type Tone } from "@/shared/lib/format";
import { Badge } from "./badge";

/**
 * Badge status dokumen dengan peta nada tunggal (PLAN §4.2). `label`/`tone` menimpa bila
 * perlu (mis. label khusus domain); status tak dikenal tampil netral dengan teks aslinya.
 */
export function StatusBadge({ status, label, tone }: { status: string | null | undefined; label?: string; tone?: Tone }) {
  return <Badge tone={tone ?? statusTone(status)}>{label ?? statusLabel(status)}</Badge>;
}
