import { Delete } from "lucide-react";
import { cn } from "@/shared/lib/cn";

export interface NumpadProps {
  /** Nilai sebagai string angka (desimal selalu memakai ".", mis. "12.5"). */
  value: string;
  onChange: (next: string) => void;
  /** Dipanggil saat tombol Enter ditekan. Tanpa ini tombol Enter tidak tampil aktif. */
  onEnter?: () => void;
  enterLabel?: string;
  /** Tampilkan tombol desimal. */
  allowDecimal?: boolean;
  /** Hanya mengubah label tombol desimal; nilai tetap memakai ".". */
  decimalSeparator?: "," | ".";
  /** Panjang maksimum string nilai. */
  maxLength?: number;
  disabled?: boolean;
  className?: string;
  "aria-label"?: string;
}

type Key =
  | { type: "digit"; digit: string }
  | { type: "double-zero" }
  | { type: "decimal" }
  | { type: "backspace" };

function apply(value: string, key: Key, opts: { allowDecimal: boolean; maxLength: number }): string {
  let next = value;
  switch (key.type) {
    case "digit":
      next = value === "0" ? key.digit : value + key.digit;
      break;
    case "double-zero":
      if (value === "" || value === "0") return value;
      next = `${value}00`;
      break;
    case "decimal":
      if (!opts.allowDecimal || value.includes(".")) return value;
      next = value === "" ? "0." : `${value}.`;
      break;
    case "backspace":
      return value.slice(0, -1);
  }
  return next.length > opts.maxLength ? value : next;
}

const KEY_CLASS =
  "flex h-14 min-w-12 touch-manipulation items-center justify-center rounded-lg border border-input bg-card font-display text-2xl font-medium text-foreground tabular select-none transition-[background-color,transform] duration-100 active:scale-[0.98] active:bg-accent disabled:pointer-events-none disabled:opacity-50";

/**
 * Papan angka layar sentuh (generik): 0–9, 00, hapus, desimal opsional, Enter.
 * Tanpa ketergantungan hover; setiap tombol ≥ 48px. Kontrol nilai ada di pemanggil,
 * sehingga bisa dipakai untuk qty, nominal uang, maupun persen.
 */
export function Numpad({
  value,
  onChange,
  onEnter,
  enterLabel = "OK",
  allowDecimal = false,
  decimalSeparator = ",",
  maxLength = 12,
  disabled = false,
  className,
  "aria-label": ariaLabel = "Papan angka",
}: NumpadProps) {
  const opts = { allowDecimal, maxLength };
  const press = (key: Key) => onChange(apply(value, key, opts));
  const digit = (d: string) => (
    <button key={d} type="button" disabled={disabled} className={KEY_CLASS} onClick={() => press({ type: "digit", digit: d })}>
      {d}
    </button>
  );

  return (
    <div role="group" aria-label={ariaLabel} className={cn("grid grid-cols-4 gap-2", className)}>
      {["7", "8", "9"].map(digit)}
      <button
        type="button"
        disabled={disabled}
        aria-label="Hapus angka terakhir"
        className={KEY_CLASS}
        onClick={() => press({ type: "backspace" })}
      >
        <Delete aria-hidden className="size-6" />
      </button>
      {["4", "5", "6"].map(digit)}
      <button
        type="button"
        disabled={disabled || !onEnter}
        aria-label={enterLabel}
        className={cn(
          KEY_CLASS,
          "col-start-4 row-span-3 row-start-2 h-auto border-transparent bg-sidebar text-lg font-semibold text-sidebar-strong active:bg-sidebar-accent",
        )}
        onClick={() => onEnter?.()}
      >
        {enterLabel}
      </button>
      {["1", "2", "3"].map(digit)}
      <button
        type="button"
        disabled={disabled}
        className={KEY_CLASS}
        onClick={() => press({ type: "double-zero" })}
      >
        00
      </button>
      {digit("0")}
      {allowDecimal ? (
        <button
          type="button"
          disabled={disabled}
          aria-label={decimalSeparator === "," ? "Koma" : "Titik"}
          className={KEY_CLASS}
          onClick={() => press({ type: "decimal" })}
        >
          {decimalSeparator}
        </button>
      ) : (
        <span aria-hidden />
      )}
    </div>
  );
}
