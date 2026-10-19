import { FormField, SelectInput } from "@/shared/components/ui";
import { useAccessibleBranches } from "@/modules/auth/hooks/use-accessible-branches";

interface BranchPickerProps {
  id: string;
  /** "0" = otomatis (cabang yang disebut di pesan, atau semua cabang yang boleh diakses). */
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

/**
 * Pemilih cabang untuk konsol uji & simulasi (AS-01). Hanya cabang yang boleh
 * diakses pengguna; server tetap memvalidasi (403 bila cabang di luar akses).
 */
export function BranchPicker({ id, value, onChange, disabled }: BranchPickerProps) {
  const { branches } = useAccessibleBranches(true);
  return (
    <FormField label="Cabang" htmlFor={id} helperText="Otomatis: asisten membaca nama cabang dari pesan, atau memakai semua cabang Anda.">
      <SelectInput id={id} value={value} onChange={(e) => onChange(e.target.value)} disabled={disabled}>
        <option value="0">Otomatis (dari pesan / semua cabang saya)</option>
        {branches.map((b) => (
          <option key={b.id} value={String(b.id)}>
            {b.code} — {b.name}
          </option>
        ))}
      </SelectInput>
    </FormField>
  );
}
