import { useEffect } from "react";
import type { ReactNode } from "react";
import { useAuthStore, wireAuthExpiry } from "@/modules/auth/stores/auth.store";
import { tokenStore } from "@/shared/services/http-client";

export function AppProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    // Sekali saat boot: sesi mati total → tendang ke login.
    wireAuthExpiry(() => {
      const path = window.location.pathname;
      if (path !== "/login" && path !== "/select-branch" && path !== "/403") {
        window.location.href = "/login";
      }
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
