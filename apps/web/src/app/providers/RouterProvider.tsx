import { Suspense } from "react";
import { RouterProvider as ReactRouterProvider } from "react-router-dom";
import { router } from "@/app/routes";

// Sengaja TANPA import shared/components (dibangun agen paralel) agar
// provider ini tetap kompilasi hijau apa pun yang terjadi di sana.
export function RouterProvider() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center text-muted">
          Memuat halaman…
        </div>
      }
    >
      <ReactRouterProvider router={router} />
    </Suspense>
  );
}
