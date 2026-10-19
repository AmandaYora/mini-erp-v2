import { Link } from "react-router-dom";
import { Badge } from "@/shared/components/ui/badge";
import type { Tone } from "@/shared/lib/format";
import { Stepper, type StepperStep } from "./stepper";

export interface RelatedDocument {
  /** Nomor/label dokumen, mis. "SJ-0042". */
  label: string;
  /** Jenis, mis. "Surat jalan". */
  kind: string;
  to: string;
  status?: { label: string; tone: Tone };
}

/**
 * Rel Dokumen — elemen khas DS v2: tahapan siklus dokumen + chip dokumen terkait yang bisa diklik.
 * Menjawab "sudah dikirim? sudah dibayar?" dalam satu pandangan. Generik: definisi langkah dan
 * dokumen terkait datang dari halaman (data yang sudah dimuat), tanpa endpoint baru.
 */
export function DocumentRail({
  steps,
  current,
  stopped = false,
  related = [],
}: {
  steps: StepperStep[];
  current: string;
  stopped?: boolean;
  related?: RelatedDocument[];
}) {
  return (
    <div className="flex flex-col gap-4 rounded-xl border border-border bg-card px-5 py-4 lg:flex-row lg:items-center lg:gap-8">
      <Stepper steps={steps} current={current} stopped={stopped} className="min-w-0 lg:max-w-xl lg:flex-1" />
      {related.length > 0 && (
        <div className="flex min-w-0 flex-wrap items-center gap-2 lg:ml-auto lg:justify-end">
          <span className="text-xs font-medium text-muted-foreground">Terkait</span>
          {related.map((r) => (
            <Link
              key={r.to}
              to={r.to}
              title={r.kind}
              className="inline-flex items-center gap-1.5 rounded-md border border-border bg-background px-2 py-1 text-sm font-medium text-foreground hover:border-input hover:no-underline"
            >
              <span className="font-mono text-xs">{r.label}</span>
              {r.status && <Badge tone={r.status.tone}>{r.status.label}</Badge>}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
