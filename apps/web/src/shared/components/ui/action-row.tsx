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
      className={`mt-6 flex gap-3 border-t border-hairline pt-5 ${JUSTIFY[align]}`}
    >
      {children}
    </div>
  );
}
