import { fromCsv, toggleCsv } from "@/shared/lib/list-query";

export interface MultiToggleOption {
  value: string;
  label: string;
}

export interface MultiToggleProps {
  options: readonly MultiToggleOption[];
  /** Nilai terpilih sebagai CSV ("confirmed,completed"); kosong = semua. */
  value: string;
  onChange: (csv: string) => void;
  "aria-label"?: string;
}

/**
 * MultiToggle — pilihan majemuk berbentuk chip (mis. filter status multi).
 * Nilai disimpan sebagai CSV agar langsung cocok dengan param query `status`.
 */
export function MultiToggle({ options, value, onChange, "aria-label": ariaLabel }: MultiToggleProps) {
  const selected = fromCsv(value);
  return (
    <div role="group" aria-label={ariaLabel} className="flex flex-wrap gap-1.5">
      {options.map((o) => {
        const active = selected.includes(o.value);
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(toggleCsv(value, o.value))}
            className={`cursor-pointer rounded-full border px-3 py-1.5 text-sm font-medium transition-colors ${
              active
                ? "border-primary bg-primary text-primary-foreground"
                : "border-border bg-card text-muted-foreground hover:text-foreground"
            }`}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
