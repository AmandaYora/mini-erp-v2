import type { ReactNode } from "react";

export type ActionRowAlign = "start" | "center" | "end";

const JUSTIFY: Record<ActionRowAlign, string> = {
  start: "justify-start",
  center: "justify-center",
  end: "justify-end",
};

export function ActionRow({
  children,
  align = "end",
}: {
  children: ReactNode;
  align?: ActionRowAlign;
}) {
  return (
    <div
      className={`mt-6 flex flex-wrap items-center gap-2 border-t border-border pt-5 ${JUSTIFY[align]}`}
    >
      {children}
    </div>
  );
}
