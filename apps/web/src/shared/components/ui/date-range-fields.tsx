import { dateRangeError } from "@/shared/lib/list-query";
import { DateInput } from "./input";
import { FormField } from "./form-field";

export interface DateRangeFieldsProps {
  from: string;
  to: string;
  onChange: (range: { from: string; to: string }) => void;
  fromLabel?: string;
  toLabel?: string;
}

/**
 * DateRangeFields — dua kolom tanggal (YYYY-MM-DD) dengan batas saling
 * membatasi dan pesan bila terbalik. Rentang terbalik tetap diteruskan apa
 * adanya (server menolak dengan pesan 400 yang ditampilkan halaman).
 */
export function DateRangeFields({
  from,
  to,
  onChange,
  fromLabel = "Dari tanggal",
  toLabel = "Sampai tanggal",
}: DateRangeFieldsProps) {
  const err = dateRangeError(from, to);
  return (
    <>
      <FormField label={fromLabel} errorText={err ?? undefined}>
        <DateInput
          aria-label={fromLabel}
          value={from}
          max={to || undefined}
          onChange={(e) => onChange({ from: e.target.value, to })}
        />
      </FormField>
      <FormField label={toLabel}>
        <DateInput
          aria-label={toLabel}
          value={to}
          min={from || undefined}
          onChange={(e) => onChange({ from, to: e.target.value })}
        />
      </FormField>
    </>
  );
}
