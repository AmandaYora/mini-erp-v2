import type { ReactNode } from "react";
import { Inbox, type LucideIcon } from "lucide-react";

/** Keadaan kosong = ajakan bertindak: judul jelas, satu aksi utama bila ada. */
export function EmptyState({
  title,
  description,
  action,
  icon: Icon = Inbox,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  icon?: LucideIcon;
}) {
  return (
    <div className="flex flex-col items-center px-6 py-14 text-center">
      <span className="mb-3 flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <Icon size={20} aria-hidden />
      </span>
      <p className="text-base font-semibold text-foreground">{title}</p>
      {description && <p className="mx-auto mt-1 max-w-[420px] text-sm text-muted-foreground">{description}</p>}
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}
