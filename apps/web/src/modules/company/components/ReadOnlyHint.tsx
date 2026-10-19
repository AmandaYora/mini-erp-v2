import { Lock } from "lucide-react";

/** Penanda gembok per seksi pada mode baca-saja (lapis penjelas bersama Notice di atas halaman). */
export default function ReadOnlyHint({ show }: { show: boolean }) {
  if (!show) return null;
  return (
    <p className="mb-4 flex items-center gap-1.5 text-xs text-muted-foreground">
      <Lock size={12} aria-hidden /> Hanya baca — Anda tidak memiliki izin mengubah bagian ini.
    </p>
  );
}
