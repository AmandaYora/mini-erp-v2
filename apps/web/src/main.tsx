import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/inter";
import "@fontsource-variable/plus-jakarta-sans";
import "@fontsource-variable/geist-mono";
import App from "@/app/App";
import { ErrorBoundary } from "@/shared/components/feedback/error-boundary";
import { installPreloadErrorHandler } from "@/shared/lib/chunk-recovery";
import { applyDensity, useUiPrefs } from "@/shared/stores/ui-prefs.store";
import "@/styles/globals.css";

applyDensity(useUiPrefs.getState().density);
// OP-01: chunk basi pasca-deploy → muat ulang sekali (lihat shared/lib/chunk-recovery.ts).
installPreloadErrorHandler();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary fullScreen>
      <App />
    </ErrorBoundary>
  </StrictMode>
);
