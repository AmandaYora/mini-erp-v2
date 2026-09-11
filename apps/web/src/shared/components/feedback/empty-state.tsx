import type { ReactNode } from "react";

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="rounded-lg border border-dashed border-hairline bg-surface-subtle px-6 py-16 text-center">
      <p className="font-semibold text-ink">{title}</p>
      {description && (
        <p className="text-muted mx-auto mt-1 max-w-[420px] text-sm">
          {description}
        </p>
      )}
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}
