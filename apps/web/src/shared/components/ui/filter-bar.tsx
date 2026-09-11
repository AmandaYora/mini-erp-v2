import type { ReactNode } from "react";

export function FilterBar({ children }: { children: ReactNode }) {
  return (
    <div className="mb-4 flex flex-wrap items-end gap-4 rounded-lg border border-hairline bg-surface px-6 py-4">
      {children}
    </div>
  );
}
