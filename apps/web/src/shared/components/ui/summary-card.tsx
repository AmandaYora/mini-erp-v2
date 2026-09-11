export type SummaryTone = "brand" | "success" | "warning" | "danger";

export interface SummaryCardProps {
  label: string;
  value: string;
  hint?: string;
  tone?: SummaryTone;
  onClick?: () => void;
}

const ACCENT: Record<SummaryTone, string> = {
  brand: "border-t-brand",
  success: "border-t-ok",
  warning: "border-t-warn",
  danger: "border-t-bad",
};

export function SummaryCard({
  label,
  value,
  hint,
  tone = "brand",
  onClick,
}: SummaryCardProps) {
  return (
    <div
      onClick={onClick}
      className={`rounded-lg border border-hairline border-t-4 bg-surface p-5 ${ACCENT[tone]}${onClick ? " cursor-pointer transition-colors hover:bg-surface-subtle" : ""}`}
    >
      <p className="text-muted text-sm">{label}</p>
      <p className="mt-1 text-2xl font-bold text-ink">{value}</p>
      {hint && <p className="text-muted mt-1 text-[0.82rem]">{hint}</p>}
    </div>
  );
}
