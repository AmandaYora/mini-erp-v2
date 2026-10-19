import { cva } from "class-variance-authority";
import type { ButtonSize, ButtonVariant } from "./button";

/**
 * Kelas tombol bersama (DS v2). File sendiri (tanpa komponen) agar button.tsx hanya
 * mengekspor komponen — nol peringatan react-refresh/only-export-components.
 * Dipakai juga untuk <Link> bergaya tombol: lebih baik `<Button asChild><Link/></Button>`.
 */
const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-md font-medium transition-[background-color,border-color,color,box-shadow] duration-150 select-none active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        primary: "bg-primary text-primary-foreground hover:bg-primary-hover",
        secondary:
          "border border-input bg-card text-foreground shadow-[var(--shadow-card)] hover:bg-accent hover:text-accent-foreground",
        ghost: "text-primary hover:bg-accent",
        danger: "bg-destructive text-white hover:brightness-95",
      },
      size: {
        sm: "h-8 px-3 text-sm",
        md: "h-9 px-4 text-sm",
        lg: "h-11 px-5 text-base",
        icon: "size-9",
        "icon-sm": "size-8",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

export function buttonClass(variant: ButtonVariant = "primary", size: ButtonSize = "md"): string {
  return buttonVariants({ variant, size });
}
