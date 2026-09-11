import type { ReactNode } from "react";

export interface FormFieldProps {
  label: string;
  htmlFor?: string;
  required?: boolean;
  helperText?: string;
  errorText?: string;
  children: ReactNode;
}

export function FormField({
  label,
  htmlFor,
  required,
  helperText,
  errorText,
  children,
}: FormFieldProps) {
  return (
    <div>
      <label
        htmlFor={htmlFor}
        className="text-heading mb-1.5 block text-[0.85rem] font-medium"
      >
        {label}
        {required && (
          <span className="text-bad ml-1.5 text-[0.78rem] font-semibold">
            Wajib
          </span>
        )}
      </label>
      {children}
      {errorText ? (
        <p className="text-bad mt-1 text-[0.82rem]">{errorText}</p>
      ) : helperText ? (
        <p className="text-muted mt-1 text-[0.8rem]">{helperText}</p>
      ) : null}
    </div>
  );
}
