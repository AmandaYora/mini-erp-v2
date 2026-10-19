import type { ReactNode } from "react";

export interface PageHeaderProps {
  /** Diabaikan sejak DS v2: konteks "grup / menu" kini ada di breadcrumb kerangka. Dipertahankan agar pemanggil lama tetap valid. */
  eyebrow?: string;
  title: string;
  description?: string;
  /** Badge/status di samping judul (mis. status dokumen). */
  meta?: ReactNode;
  /** Info kecil di bawah judul (mis. "Toko Maju · 16 Okt 2026"). */
  subtitle?: ReactNode;
  /** Aksi halaman — aksi utama paling kanan. Maksimal satu tombol primary. */
  actions?: ReactNode;
  /** Pita di bawah header, mis. Rel Dokumen. */
  children?: ReactNode;
}

/** Satu-satunya <h1> di halaman (judul topbar lama sudah dihapus). */
export function PageHeader({ title, description, meta, subtitle, actions, children }: PageHeaderProps) {
  return (
    <div className="page-header mb-5">
      <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <h1 className="font-display text-2xl leading-tight font-semibold tracking-[-0.02em] text-foreground">
              {title}
            </h1>
            {meta}
          </div>
          {subtitle && <div className="mt-1 text-sm text-muted-foreground">{subtitle}</div>}
          {description && <p className="mt-1 max-w-[68ch] text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {children && <div className="mt-4">{children}</div>}
    </div>
  );
}
