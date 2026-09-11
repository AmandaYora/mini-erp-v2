import { Link } from "react-router-dom";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

export default function ForbiddenPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-surface-subtle p-4">
      <div className="w-full max-w-sm rounded-[10px] border border-hairline bg-surface p-8 text-center shadow-md">
        <p className="text-4xl font-bold text-ink">403</p>
        <h1 className="mt-2 font-semibold text-ink">Anda tidak memiliki akses</h1>
        <p className="mt-1 text-sm text-muted">
          Hubungi administrator jika Anda merasa ini keliru.
        </p>
        <Link
          to={ROUTE_PATHS.dashboard}
          className="mt-6 inline-block rounded-lg bg-brand px-4 py-2 font-semibold text-white hover:bg-brand-hover"
        >
          Kembali ke Dashboard
        </Link>
      </div>
    </div>
  );
}
