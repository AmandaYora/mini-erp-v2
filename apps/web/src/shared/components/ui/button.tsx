import type { ButtonHTMLAttributes } from "react";
import { buttonClass } from "./button-class";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

export function Button({
  variant = "primary",
  size = "md",
  className,
  type = "button",
  ...rest
}: ButtonProps) {
  const cls = buttonClass(variant, size);
  return (
    <button
      type={type}
      className={className ? `${cls} ${className}` : cls}
      {...rest}
    />
  );
}
