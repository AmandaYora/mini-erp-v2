import { useState } from "react";
import type { FormEvent } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { loginSchema } from "@/modules/auth/schemas/login.schema";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { toApiError } from "@/shared/services/http-client";

const INPUT_CLASS =
  "w-full rounded-lg border border-hairline bg-surface px-3 py-2 text-ink placeholder:text-muted focus:border-brand";

export default function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const ready = useAuthStore((s) => s.ready);
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);
  const login = useAuthStore((s) => s.login);

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{ username?: string; password?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const next = searchParams.get("next") ?? ROUTE_PATHS.dashboard;

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-surface-subtle text-muted">
        Memuat…
      </div>
    );
  }
  if (user && branch) return <Navigate to={next} replace />;
  if (user && !branch) return <Navigate to={ROUTE_PATHS.selectBranch} replace />;

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = loginSchema.safeParse({ username, password });
    if (!parsed.success) {
      const errs: { username?: string; password?: string } = {};
      for (const issue of parsed.error.issues) {
        const field = issue.path[0];
        if (field === "username" || field === "password") {
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
      const result = await login(parsed.data.username, parsed.data.password);
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
    <div className="flex min-h-screen items-center justify-center bg-surface-subtle p-4">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm rounded-[10px] border border-hairline bg-surface p-8 shadow-md"
      >
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-md bg-brand text-base font-bold text-white">
            M
          </div>
          <span className="text-lg font-bold text-ink">mini-erp</span>
        </div>
        <p className="mt-2 text-sm text-muted">Masuk untuk melanjutkan</p>

        {formError && (
          <div
            role="alert"
            className="mt-4 rounded-lg border border-bad/30 bg-bad-soft px-3 py-2 text-sm font-medium text-bad"
          >
            {formError}
          </div>
        )}

        <div className="mt-6 space-y-4">
          <div>
            <label htmlFor="username" className="mb-1 block text-sm font-medium text-ink">
              Username
            </label>
            <input
              id="username"
              name="username"
              type="text"
              autoComplete="username"
              placeholder="Nama pengguna"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className={INPUT_CLASS}
            />
            {fieldErrors.username && (
              <p className="mt-1 text-xs text-bad">{fieldErrors.username}</p>
            )}
          </div>
          <div>
            <label htmlFor="password" className="mb-1 block text-sm font-medium text-ink">
              Password
            </label>
            <input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              placeholder="Kata sandi"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={INPUT_CLASS}
            />
            {fieldErrors.password && (
              <p className="mt-1 text-xs text-bad">{fieldErrors.password}</p>
            )}
          </div>
          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-lg bg-brand px-3 py-2 font-semibold text-white hover:bg-brand-hover disabled:opacity-60"
          >
            {submitting ? "Memasuk…" : "Masuk"}
          </button>
        </div>
      </form>
    </div>
  );
}
