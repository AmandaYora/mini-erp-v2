import type { ReactNode } from "react";

export interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
}

export function PageHeader({ eyebrow, title, description, actions }: PageHeaderProps) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        {eyebrow && (
          <p className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">
            {eyebrow}
          </p>
        )}
        <h1 className="text-[1.75rem] font-bold text-ink">{title}</h1>
        {description && (
          <p className="text-muted mt-1 text-[0.95rem]">{description}</p>
        )}
      </div>
      {actions && <div className="flex gap-3">{actions}</div>}
    </div>
  );
}
