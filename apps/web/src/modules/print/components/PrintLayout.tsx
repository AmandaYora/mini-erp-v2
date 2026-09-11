import { useId, type ReactNode } from "react";
import type { PrintPaperMode } from "@/modules/print/paper";

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
 * Kerangka halaman cetak: toolbar layar (selalu .no-print) + injeksi <style>
 * + wadah <main>. Toolbar tidak pernah ikut tercetak.
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
  const group = useId();
  const isContinuous = paperMode === "continuous";
  return (
    <>
      <style>{baseCss}</style>
      {continuousCss !== null ? <style>{continuousCss}</style> : null}
      <div className="no-print print-toolbar">
        <label className="print-toolbar-option">
          <input
            type="radio"
            name={`${group}-paper`}
            checked={paperMode === "a4"}
            onChange={() => onPaperModeChange("a4")}
          />
          <span>Kertas A4 (biasa)</span>
        </label>
        <label className="print-toolbar-option">
          <input
            type="radio"
            name={`${group}-paper`}
            checked={isContinuous}
            onChange={() => onPaperModeChange("continuous")}
          />
          <span>Kontinu 3-Ply (Dot-Matrix)</span>
        </label>
        {isContinuous && letterheadToggle !== null ? (
          <label className="print-toolbar-option">
            <input
              type="checkbox"
              checked={letterheadToggle.hidden}
              onChange={(e) => letterheadToggle.onChange(e.target.checked)}
            />
            <span>Kop/rekening sudah preprinted (sembunyikan)</span>
          </label>
        ) : null}
        {extraToolbar}
        <button type="button" onClick={onPrint ?? (() => window.print())}>
          Cetak / Print
        </button>
      </div>
      {notice}
      <main className={mainClassName}>{children}</main>
    </>
  );
}
