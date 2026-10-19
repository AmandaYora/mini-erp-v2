import { useState } from "react";
import type { FormEvent } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { loginSchema } from "@/modules/auth/schemas/login.schema";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { toApiError } from "@/shared/services/http-client";
import { Button, FormField, PasswordInput, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { LoadingState } from "@/shared/components/feedback/LoadingState";
import { AUTH_PANEL, BRAND_MARK } from "@/modules/auth/lib/auth-styles";
import { safeNextPath } from "@/modules/auth/lib/session-redirect";

export default function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const ready = useAuthStore((s) => s.ready);
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);
  const login = useAuthStore((s) => s.login);

  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{ identifier?: string; password?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const next = safeNextPath(searchParams.get("next"), ROUTE_PATHS.dashboard);

  if (!ready) return <LoadingState />;
  if (user && branch) return <Navigate to={next} replace />;
  if (user && !branch) return <Navigate to={ROUTE_PATHS.selectBranch} replace />;

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = loginSchema.safeParse({ identifier, password });
    if (!parsed.success) {
      const errs: { identifier?: string; password?: string } = {};
      for (const issue of parsed.error.issues) {
        const field = issue.path[0];
        if (field === "identifier" || field === "password") {
          errs[field] ??= issue.message;
        }
      }
      setFieldErrors(errs);
      return;
    }
    setFieldErrors({});
    setFormError(null);
    setSubmitting(true);
    try {
      const result = await login(parsed.data.identifier, parsed.data.password);
      if (result.requiresBranchSelection) {
        navigate(`${ROUTE_PATHS.selectBranch}?next=${encodeURIComponent(next)}`, {
          replace: true,
        });
      } else {
        navigate(next, { replace: true });
      }
    } catch (err) {
      setFormError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen bg-background">
      <aside className={AUTH_PANEL}>
        <div className="flex items-center gap-3">
          <span className={BRAND_MARK}>M</span>
          <span className="font-display text-lg font-semibold">Mini ERP</span>
        </div>
        <div className="max-w-md">
          <p className="font-display text-4xl leading-tight font-semibold tracking-[-0.02em]">
            Pembelian, stok, penjualan, dan pembukuan dalam satu tempat.
          </p>
          <p className="mt-4 text-base text-sidebar-foreground">
            Catat transaksi di setiap cabang, pantau stok dan piutang, dan tutup buku dengan angka yang bisa
            ditelusuri sampai ke dokumennya.
          </p>
        </div>
        <p className="text-xs text-sidebar-muted">Operasional multi-cabang</p>
      </aside>

      <main className="flex flex-1 flex-col items-center justify-center p-8">
        <div className="w-full max-w-sm">
          <span className={`${BRAND_MARK} mb-8 bg-primary text-primary-foreground lg:hidden`}>M</span>
          <h1 className="font-display text-3xl font-semibold tracking-[-0.02em] text-foreground">Masuk</h1>
          <p className="mt-1 mb-8 text-base text-muted-foreground">Gunakan akun yang diberikan administrator.</p>

          {formError && (
            <div className="mb-6">
              <Notice tone="danger" title="Gagal masuk">
                {formError}
              </Notice>
            </div>
          )}

          <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
            <FormField label="Nama pengguna atau email" htmlFor="identifier" errorText={fieldErrors.identifier}>
              <TextInput
                id="identifier"
                name="identifier"
                type="text"
                autoComplete="username"
                placeholder="mis. rina atau rina@toko.id"
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
                className="h-11"
              />
            </FormField>
            <FormField label="Kata sandi" htmlFor="password" errorText={fieldErrors.password}>
              <PasswordInput
                id="password"
                name="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                aria-invalid={fieldErrors.password ? true : undefined}
              />
            </FormField>
            <Button type="submit" size="lg" loading={submitting} className="mt-2 w-full">
              {submitting ? "Masuk…" : "Masuk"}
            </Button>
          </form>
        </div>
      </main>
    </div>
  );
}
