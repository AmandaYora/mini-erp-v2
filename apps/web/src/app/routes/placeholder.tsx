import { useLocation } from "react-router-dom";
import { getPageTitle } from "@/app/routes/registry";

// Halaman sementara untuk modul yang belum dibangun. Sengaja TANPA import
// shared/components (dibangun agen paralel) — hanya class tailwind + token DS.
export default function PlaceholderPage({ title }: { title?: string }) {
  const { pathname } = useLocation();
  const heading = title ?? getPageTitle(pathname);

  return (
    <div>
      <h1 className="text-xl font-semibold text-ink">{heading}</h1>
      <div className="mt-4 rounded-[10px] border border-hairline bg-surface p-4 text-sm text-muted">
        Modul segera hadir.
      </div>
    </div>
  );
}
