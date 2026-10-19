import type { InputHTMLAttributes } from "react";
import { Button, INPUT } from "@/shared/components/ui";

interface RupiahInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "value" | "onChange" | "type"> {
  value: string;
  onValueChange: (value: string) => void;
  /** Bila diberikan, tampil tombol "Lunasi sisa" yang mengisi nominal ini. */
  fillAmount?: number;
  fillLabel?: string;
  invalid?: boolean;
}

/**
 * Input nominal rupiah: prefiks "Rp" + (opsional) tombol pengisi cepat. Nilai tetap string
 * agar validasi Zod di pemanggil tidak berubah; hanya tampilannya yang berbeda.
 */
export function RupiahInput({
  value,
  onValueChange,
  fillAmount,
  fillLabel = "Lunasi sisa",
  invalid,
  disabled,
  ...rest
}: RupiahInputProps) {
  return (
    <div>
      <div className="flex items-stretch gap-2">
        <div className="flex min-w-0 flex-1 items-stretch">
          <span
            aria-hidden
            className="inline-flex items-center rounded-l-md border border-r-0 border-input bg-muted px-3 text-sm text-muted-foreground"
          >
            Rp
          </span>
          <input
            {...rest}
            value={value}
            inputMode="numeric"
            disabled={disabled}
            aria-invalid={invalid ? true : undefined}
            onChange={(e) => onValueChange(e.target.value)}
            className={`${INPUT} tabular rounded-l-none text-right`}
          />
        </div>
        {fillAmount !== undefined && fillAmount > 0 && (
          <Button
            type="button"
            variant="secondary"
            disabled={disabled}
            onClick={() => onValueChange(String(fillAmount))}
          >
            {fillLabel}
          </Button>
        )}
      </div>
    </div>
  );
}
