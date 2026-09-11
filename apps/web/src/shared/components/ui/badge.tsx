import type { ReactNode } from "react";
import type { Tone } from "@/shared/lib/format";

const TONES: Record<Tone, string> = {
  success: "bg-ok-soft text-ok",
  warning: "bg-warn-soft text-warn",
  danger: "bg-bad-soft text-bad",
  info: "bg-brand/10 text-brand",
  neutral: "bg-surface-sunken text-muted",
};

export function Badge({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold ${TONES[tone]}`}
    >
      {children}
    </span>
  );
}
