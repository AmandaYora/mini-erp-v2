import type { ReactNode } from "react";
import type { Tone } from "@/shared/lib/format";

const WRAPPER: Record<Tone, string> = {
  success: "bg-ok-soft border-l-ok",
  warning: "bg-warn-soft border-l-warn",
  danger: "bg-bad-soft border-l-bad",
  info: "bg-brand/10 border-l-brand",
  neutral: "bg-surface-sunken border-l-hairline",
};

const TITLE: Record<Tone, string> = {
  success: "text-ok",
  warning: "text-warn",
  danger: "text-bad",
  info: "text-brand",
  neutral: "text-heading",
};

export function Notice({
  tone,
  title,
  children,
}: {
  tone: Tone;
  title: string;
  children?: ReactNode;
}) {
  return (
    <div className={`rounded-md border-l-4 p-4 ${WRAPPER[tone]}`}>
      <p className={`text-sm font-semibold ${TITLE[tone]}`}>{title}</p>
      {children && <div className="mt-1 text-sm text-ink">{children}</div>}
    </div>
  );
}
