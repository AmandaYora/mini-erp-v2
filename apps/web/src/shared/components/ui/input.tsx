import type {
  InputHTMLAttributes,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
} from "react";
import { INPUT } from "./field-classes";

function ErrorText({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="mt-1 text-xs font-medium text-destructive">{message}</p>;
}

export interface TextInputProps
  extends InputHTMLAttributes<HTMLInputElement> {
  error?: string;
}

export function TextInput({ error, className, ...rest }: TextInputProps) {
  return (
    <div>
      <input
        className={className ? `${INPUT} ${className}` : INPUT}
        aria-invalid={error ? true : undefined}
        {...rest}
      />
      <ErrorText message={error} />
    </div>
  );
}

export interface DateInputProps extends InputHTMLAttributes<HTMLInputElement> {
  error?: string;
}

export function DateInput({ error, className, ...rest }: DateInputProps) {
  return (
    <div>
      <input
        type="date"
        className={className ? `${INPUT} ${className}` : INPUT}
        aria-invalid={error ? true : undefined}
        {...rest}
      />
      <ErrorText message={error} />
    </div>
  );
}

export interface SelectInputProps
  extends SelectHTMLAttributes<HTMLSelectElement> {
  error?: string;
}

export function SelectInput({
  error,
  className,
  children,
  ...rest
}: SelectInputProps) {
  return (
    <div>
      <select
        className={`${INPUT} select-chevron cursor-pointer appearance-none pr-9 ${className ?? ""}`}
        aria-invalid={error ? true : undefined}
        {...rest}
      >
        {children}
      </select>
      <ErrorText message={error} />
    </div>
  );
}

export interface TextAreaProps
  extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: string;
}

export function TextArea({ error, className, ...rest }: TextAreaProps) {
  return (
    <div>
      <textarea
        className={`${INPUT} h-auto min-h-[84px] py-2 ${className ?? ""}`}
        aria-invalid={error ? true : undefined}
        {...rest}
      />
      <ErrorText message={error} />
    </div>
  );
}
