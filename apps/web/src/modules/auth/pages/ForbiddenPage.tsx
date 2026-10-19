import { Link } from "react-router-dom";
import { ShieldOff } from "lucide-react";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { buttonClass } from "@/shared/components/ui";

export default function ForbiddenPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="flex w-full max-w-sm flex-col items-center rounded-xl border border-border bg-card p-8 text-center">
        <span className="mb-3 flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <ShieldOff size={20} aria-hidden />
        </span>
        <p className="font-display text-3xl font-semibold text-foreground">403</p>
        <h1 className="mt-1 text-lg font-semibold text-foreground">Anda tidak memiliki akses</h1>
        <p className="mt-1 text-sm text-muted-foreground">Hubungi administrator jika Anda merasa ini keliru.</p>
        <Link to={ROUTE_PATHS.dashboard} className={`${buttonClass("primary", "md")} mt-6 hover:no-underline`}>
          Kembali ke Dashboard
        </Link>
      </div>
    </div>
  );
}
