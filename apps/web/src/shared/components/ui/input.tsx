import type {
  InputHTMLAttributes,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
} from "react";
import { INPUT } from "./field-classes";

function ErrorText({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="text-bad mt-1 text-[0.82rem]">{message}</p>;
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
        className={className ? `${INPUT} ${className}` : INPUT}
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
        className={className ? `${INPUT} ${className}` : INPUT}
        {...rest}
      />
      <ErrorText message={error} />
    </div>
  );
}
