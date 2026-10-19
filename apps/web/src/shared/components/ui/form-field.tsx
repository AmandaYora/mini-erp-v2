import { cloneElement, isValidElement, useId, type ReactElement, type ReactNode } from "react";

export interface FormFieldProps {
  label: string;
  htmlFor?: string;
  required?: boolean;
  helperText?: string;
  errorText?: string;
  children: ReactNode;
}

/** Label + kontrol + bantuan/galat. Wajib diisi ditandai `*` (tenang), dibacakan "wajib diisi". */
export function FormField({ label, htmlFor, required, helperText, errorText, children }: FormFieldProps) {
  // Tanpa htmlFor eksplisit, hubungkan label ke satu kontrol anak (id otomatis) agar tiap isian bernama aksesibel.
  const autoId = useId();
  const single = isValidElement(children) ? (children as ReactElement<{ id?: string }>) : null;
  const controlId = htmlFor ?? single?.props.id ?? (single ? autoId : undefined);
  const control = single && !htmlFor && !single.props.id ? cloneElement(single, { id: autoId }) : children;
  return (
    <div>
      <label htmlFor={controlId} className="mb-1.5 flex items-center gap-1 text-sm font-medium text-foreground">
        {label}
        {required && (
          <>
            <span aria-hidden className="text-destructive">
              *
            </span>
            <span className="sr-only">(wajib diisi)</span>
          </>
        )}
      </label>
      {control}
      {errorText ? (
        <p role="alert" className="mt-1.5 text-xs font-medium text-destructive">
          {errorText}
        </p>
      ) : helperText ? (
        <p className="mt-1.5 text-xs text-muted-foreground">{helperText}</p>
      ) : null}
    </div>
  );
}
