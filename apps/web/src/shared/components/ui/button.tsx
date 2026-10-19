import type { ButtonHTMLAttributes } from "react";
import { Slot } from "radix-ui";
import { Spinner } from "@/shared/components/primitives/spinner";
import { cn } from "@/shared/lib/cn";
import { buttonClass } from "./button-class";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg" | "icon" | "icon-sm";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Tampilkan spinner dan nonaktifkan tombol selama aksi berjalan. */
  loading?: boolean;
  /** Render sebagai anak (mis. <Link>) tanpa membungkus <button> di dalam <a>. */
  asChild?: boolean;
}

export function Button({
  variant = "primary",
  size = "md",
  className,
  type = "button",
  loading = false,
  asChild = false,
  disabled,
  children,
  ...rest
}: ButtonProps) {
  const cls = cn(buttonClass(variant, size), className);
  if (asChild) {
    return (
      <Slot.Root className={cls} {...rest}>
        {children}
      </Slot.Root>
    );
  }
  return (
    <button type={type} className={cls} disabled={disabled || loading} aria-busy={loading || undefined} {...rest}>
      {loading && <Spinner aria-hidden className="size-4" />}
      {children}
    </button>
  );
}
