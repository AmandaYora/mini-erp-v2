import type { ButtonSize, ButtonVariant } from "./button";

/**
 * Kelas tombol bersama. File sendiri (tanpa komponen) agar button.tsx hanya
 * mengekspor komponen — nol peringatan react-refresh/only-export-components.
 * Di-re-export lewat shared/components/ui agar import lama tetap jalan.
 */
const BASE =
  "inline-flex cursor-pointer items-center justify-center rounded-md font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50";

const VARIANTS: Record<ButtonVariant, string> = {
  primary: "bg-brand text-white hover:bg-brand-hover",
  secondary: "border border-hairline bg-surface text-ink hover:bg-surface-subtle",
  ghost: "text-brand hover:bg-brand/10",
  danger: "bg-bad text-white hover:opacity-90",
};

const SIZES: Record<ButtonSize, string> = {
  sm: "px-3 py-1.5 text-sm",
  md: "px-4 py-2 text-[0.95rem]",
};

export function buttonClass(
  variant: ButtonVariant = "primary",
  size: ButtonSize = "md",
): string {
  return `${BASE} ${VARIANTS[variant]} ${SIZES[size]}`;
}
