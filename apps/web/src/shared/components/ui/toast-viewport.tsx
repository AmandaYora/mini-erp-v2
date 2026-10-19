import { Toaster } from "@/shared/components/primitives/sonner";

/** Wadah toast (Sonner). Dipasang sekali di kerangka aplikasi; dipicu lewat `toast.*`. */
export function ToastViewport() {
  return <Toaster position="bottom-right" closeButton visibleToasts={5} />;
}
