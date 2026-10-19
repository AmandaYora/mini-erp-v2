import { useId, type ReactNode } from "react";
import { Printer } from "lucide-react";
import { Button, Checkbox, RadioGroup, RadioGroupItem } from "@/shared/components/ui";
import type { PrintPaperMode } from "@/modules/print/paper";
import { LEGACY_UA_CSS } from "@/modules/print/styles";

interface PrintLayoutProps {
  /** CSS dasar halaman (mode A4). */
  baseCss: string;
  /** CSS tambahan mode kontinu (sudah termasuk @page profil) — null = mode A4. */
  continuousCss: string | null;
  paperMode: PrintPaperMode;
  onPaperModeChange: (mode: PrintPaperMode) => void;
  /** Toggle preprinted — hanya relevan di mode kontinu. */
  letterheadToggle: {
    hidden: boolean;
    onChange: (hidden: boolean) => void;
  } | null;
  /** Peringatan non-blokir, mis. profil rusak -> default dipakai. */
  notice?: ReactNode;
  /** Kontrol toolbar tambahan (mis. textarea notes bebas). */
  extraToolbar?: ReactNode;
  /** Override aksi cetak (mis. cetak native Android). Default: window.print(). */
  onPrint?: () => void;
  /** Kelas <main> halaman, mis. "sj-page continuous". */
  mainClassName: string;
  children: ReactNode;
}

/**
 * Kerangka halaman cetak: toolbar layar (selalu .no-print, sticky) + injeksi <style>
 * + wadah <main>. Toolbar tidak pernah ikut tercetak; kertas fisik tidak disentuh (PLAN U12).
 */
export function PrintLayout({
  baseCss,
  continuousCss,
  paperMode,
  onPaperModeChange,
  letterheadToggle,
  notice,
  extraToolbar,
  onPrint,
  mainClassName,
  children,
}: PrintLayoutProps) {
  const id = useId();
  const isContinuous = paperMode === "continuous";
  return (
    <>
      <style>{LEGACY_UA_CSS + baseCss}</style>
      {continuousCss !== null ? <style>{continuousCss}</style> : null}
      <div
        role="toolbar"
        aria-label="Opsi cetak"
        className="no-print sticky top-0 z-30 flex flex-wrap items-center gap-x-6 gap-y-3 border-b border-border bg-card px-4 py-3 text-base text-foreground print:hidden"
      >
        <RadioGroup
          aria-label="Jenis kertas"
          value={paperMode}
          onValueChange={(v) => onPaperModeChange(v as PrintPaperMode)}
          className="flex flex-wrap items-center gap-x-5 gap-y-2"
        >
          <label htmlFor={`${id}-a4`} className="flex cursor-pointer items-center gap-2 text-sm font-medium">
            <RadioGroupItem id={`${id}-a4`} value="a4" />
            Kertas A4 (biasa)
          </label>
          <label htmlFor={`${id}-continuous`} className="flex cursor-pointer items-center gap-2 text-sm font-medium">
            <RadioGroupItem id={`${id}-continuous`} value="continuous" />
            Kontinu 3-ply (dot-matrix)
          </label>
        </RadioGroup>
        {isContinuous && letterheadToggle !== null ? (
          <label htmlFor={`${id}-preprinted`} className="flex cursor-pointer items-center gap-2 text-sm font-medium">
            <Checkbox
              id={`${id}-preprinted`}
              checked={letterheadToggle.hidden}
              onCheckedChange={(v) => letterheadToggle.onChange(v === true)}
            />
            Kop/rekening sudah preprinted (sembunyikan)
          </label>
        ) : null}
        {extraToolbar}
        <Button className="ml-auto" onClick={onPrint ?? (() => window.print())}>
          <Printer aria-hidden /> Cetak
        </Button>
      </div>
      {notice}
      <main className={mainClassName}>{children}</main>
    </>
  );
}
