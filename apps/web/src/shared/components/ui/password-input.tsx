import { useId, useState } from "react";
import type { InputHTMLAttributes } from "react";
import { Eye, EyeOff } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import { INPUT } from "./field-classes";

export type PasswordInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  inputClassName?: string;
};

/** PasswordInput — input kata sandi dengan tombol intip; memakai kelas INPUT yang sama dengan TextInput. */
export function PasswordInput({ inputClassName = INPUT, id, style, ...props }: PasswordInputProps) {
  const generatedId = useId();
  const inputId = id ?? generatedId;
  const [visible, setVisible] = useState(false);
  const label = visible ? "Sembunyikan password" : "Tampilkan password";
  const Icon = visible ? EyeOff : Eye;

  return (
    <div className="relative">
      <input
        {...props}
        className={cn(inputClassName, "pr-11")}
        id={inputId}
        style={style}
        type={visible ? "text" : "password"}
      />
      <button
        type="button"
        aria-label={label}
        aria-pressed={visible}
        title={label}
        onClick={() => setVisible((v) => !v)}
        className="absolute inset-y-0 right-0 flex items-center rounded-r-md px-3 text-muted-foreground transition-colors hover:text-foreground"
      >
        <Icon size={18} aria-hidden />
      </button>
    </div>
  );
}
