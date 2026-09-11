import { useId, useState } from "react";
import type { InputHTMLAttributes } from "react";
import { INPUT } from "./field-classes";

export type PasswordInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  inputClassName?: string;
};

function EyeIcon({ visible }: { visible: boolean }) {
  return (
    <svg aria-hidden="true" fill="none" height="18" viewBox="0 0 24 24" width="18">
      <path
        d="M2.25 12s3.5-6.25 9.75-6.25S21.75 12 21.75 12 18.25 18.25 12 18.25 2.25 12 2.25 12Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
      <path
        d="M12 14.75A2.75 2.75 0 1 0 12 9.25a2.75 2.75 0 0 0 0 5.5Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
      {!visible ? (
        <path
          d="M4.5 19.5 19.5 4.5"
          stroke="currentColor"
          strokeLinecap="round"
          strokeWidth="1.8"
        />
      ) : null}
    </svg>
  );
}

/**
 * PasswordInput — input kata sandi dengan tombol intip. Diport dari aplikasi
 * lama; memakai token INPUT yang sama dengan TextInput.
 */
export function PasswordInput({ inputClassName = INPUT, id, style, ...props }: PasswordInputProps) {
  const generatedId = useId();
  const inputId = id ?? generatedId;
  const [visible, setVisible] = useState(false);
  const label = visible ? "Sembunyikan password" : "Tampilkan password";

  return (
    <div className="relative">
      <input
        {...props}
        className={[inputClassName, "pr-[46px]"].filter(Boolean).join(" ")}
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
        className="absolute inset-y-0 right-0 flex cursor-pointer items-center px-3 text-muted transition-colors hover:text-ink"
      >
        <EyeIcon visible={visible} />
      </button>
    </div>
  );
}
