import type { ReactNode } from "react";

export interface SectionCardProps {
  title?: string;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
}

export function SectionCard({ title, description, actions, children }: SectionCardProps) {
  return (
    <div className="rounded-lg border border-hairline bg-surface">
      {title && (
        <div className="flex flex-wrap items-start justify-between gap-3 border-b border-hairline px-6 py-5">
          <div>
            <h2 className="text-base font-semibold text-ink">{title}</h2>
            {description && (
              <p className="text-muted mt-0.5 text-sm">{description}</p>
            )}
          </div>
          {actions && <div className="flex gap-2">{actions}</div>}
        </div>
      )}
      <div className="p-6">{children}</div>
    </div>
  );
}
