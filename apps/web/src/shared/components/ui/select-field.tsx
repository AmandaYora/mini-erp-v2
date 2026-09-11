import { SearchSelect } from "./search-select";
import type { SearchSelectOption } from "./search-select";

export interface SelectFieldProps {
  id?: string;
  name?: string;
  value?: string | number | null;
  defaultValue?: string | number | null;
  options: SearchSelectOption[];
  disabled?: boolean;
  className?: string;
  placeholder?: string;
  searchPlaceholder?: string;
  allowClear?: boolean;
  onChange?: (value: string | number | null) => void;
}

/**
 * SelectField — varian SearchSelect yang ramah formulir (mendukung
 * `name` untuk hidden input + `defaultValue` tak terkontrol).
 *
 * Dibangun DI ATAS SearchSelect yang sudah ada (bukan implementasi ganda):
 * satu perilaku dropdown sync, satu gaya popover. Dipakai halaman yang butuh
 * select statis dengan semantik field formulir.
 */
export function SelectField({
  id,
  name,
  value,
  defaultValue,
  options,
  disabled,
  className,
  placeholder,
  searchPlaceholder: _searchPlaceholder,
  allowClear,
  onChange,
}: SelectFieldProps) {
  void _searchPlaceholder;
  return (
    <span className={className ? `block min-w-0 ${className}` : "block min-w-0"}>
      <SearchSelect
        options={options}
        value={value ?? defaultValue ?? null}
        onChange={(v) => onChange?.(v)}
        placeholder={placeholder}
        allowClear={allowClear}
        disabled={disabled}
      />
      {name ? <input type="hidden" name={name} id={id} value={value ?? defaultValue ?? ""} /> : null}
    </span>
  );
}
