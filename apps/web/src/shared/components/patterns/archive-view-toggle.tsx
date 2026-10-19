import { Archive, ArchiveRestore } from "lucide-react";
import { Button } from "@/shared/components/ui/button";

/**
 * Pintasan satu klik antara daftar aktif dan tampilan arsip. Daftar master
 * dibuka tanpa arsip (default "Aktif"); tombol ini membuka/menutup arsip tanpa
 * membuka dropdown status. `showingArchive` = tampilan sedang menampilkan arsip.
 */
export function ArchiveViewToggle({
  showingArchive,
  onChange,
  label = "Tampilkan arsip",
  activeLabel = "Kembali ke aktif",
}: {
  showingArchive: boolean;
  onChange: (showArchive: boolean) => void;
  label?: string;
  activeLabel?: string;
}) {
  return (
    <Button
      variant={showingArchive ? "secondary" : "ghost"}
      size="sm"
      aria-pressed={showingArchive}
      onClick={() => onChange(!showingArchive)}
    >
      {showingArchive ? <ArchiveRestore aria-hidden /> : <Archive aria-hidden />}
      {showingArchive ? activeLabel : label}
    </Button>
  );
}
