import { Button } from "@/shared/components/ui";
import { StickyFooter } from "@/shared/components/patterns";
import { changesLabel } from "@/modules/company/lib/dirty";

/**
 * Footer simpan halaman pengaturan (PLAN T8): muncul HANYA bila ada perubahan.
 * Ditempatkan di dalam kolom isian, bukan di tepi kanvas, jadi margin negatif StickyFooter dinolkan.
 */
export default function SettingsFooter({
  count,
  saving,
  onReset,
  onSave,
  submit = false,
  saveLabel = "Simpan",
}: {
  count: number;
  saving: boolean;
  onReset: () => void;
  onSave?: () => void;
  /** true = tombol simpan bertipe submit (footer di dalam <form>). */
  submit?: boolean;
  saveLabel?: string;
}) {
  if (count === 0) return null;
  return (
    <StickyFooter
      className="mx-0 mt-4 rounded-lg border max-md:mx-0"
      summary={
        <span role="status" aria-live="polite">
          {changesLabel(count)}
        </span>
      }
    >
      <Button variant="secondary" onClick={onReset} disabled={saving}>
        Batalkan
      </Button>
      <Button type={submit ? "submit" : "button"} onClick={submit ? undefined : onSave} loading={saving}>
        {saving ? "Menyimpan…" : saveLabel}
      </Button>
    </StickyFooter>
  );
}
