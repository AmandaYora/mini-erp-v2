import { FormField, SelectInput } from "@/shared/components/ui";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { useAccessibleBranches } from "@/modules/auth/hooks/use-accessible-branches";

interface BranchScopeSelectProps {
  /** "" = cabang sesi, "all" = semua cabang yang boleh diakses, atau id cabang. */
  value: string;
  onChange: (value: string) => void;
}

/**
 * Pemilih cakupan cabang untuk laporan keuangan (?branch=). Hanya tampil bila
 * user punya izin finance.report.all_branches — tanpa izin backend menolak
 * ?branch= selain cabang sesi, jadi selektor disembunyikan sama sekali.
 */
export function BranchScopeSelect({ value, onChange }: BranchScopeSelectProps) {
  const allowed = useAuthStore((s) => s.can("finance.report.all_branches"));
  const sessionBranch = useAuthStore((s) => s.branch);
  const { branches } = useAccessibleBranches(allowed);
  if (!allowed) return null;
  return (
    <FormField label="Cabang">
      <SelectInput value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">Cabang aktif{sessionBranch ? ` (${sessionBranch.name})` : ""}</option>
        <option value="all">Semua cabang</option>
        {branches
          .filter((b) => b.id !== sessionBranch?.id)
          .map((b) => (
            <option key={b.id} value={String(b.id)}>
              {b.code} — {b.name}
            </option>
          ))}
      </SelectInput>
    </FormField>
  );
}
