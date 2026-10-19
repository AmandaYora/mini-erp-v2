import { useEffect } from "react";
import type { ReactNode } from "react";
import { useAuthStore, wireAuthExpiry } from "@/modules/auth/stores/auth.store";
import { tokenStore } from "@/shared/services/http-client";
import { loginRedirectTarget } from "@/modules/auth/lib/session-redirect";
import { router } from "@/app/routes";

export function AppProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    // Sekali saat boot: sesi mati total → tendang ke login.
    wireAuthExpiry(() => {
      // Navigasi dalam SPA (bukan reload penuh): state di memori/draf tetap
      // hidup, dan ?next membawa pengguna kembali ke halaman ini setelah masuk.
      const target = loginRedirectTarget(window.location.pathname, window.location.search);
      if (target !== null) void router.navigate(target, { replace: true });
    });

    // Pulihkan sesi saat boot bila ada token tersimpan.
    if (tokenStore.access || tokenStore.refresh) {
      useAuthStore.getState().loadMe();
    } else {
      useAuthStore.setState({ ready: true });
    }
  }, []);

  return <>{children}</>;
}
