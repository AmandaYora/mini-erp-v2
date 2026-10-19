import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";

/**
 * Uang: rupiah bulat (ADR-0005), tabular, rata kanan. `accounting` menulis negatif `(1.250.000)`
 * berwarna merah dan nol sebagai `–` (laporan keuangan).
 */
export function Money({
  value,
  accounting = false,
  className,
}: {
  value: number | null | undefined;
  accounting?: boolean;
  className?: string;
}) {
  if (accounting && typeof value === "number" && !Number.isNaN(value)) {
    if (value === 0) return <span className={cn("tabular text-right", className)}>–</span>;
    if (value < 0) {
      return <span className={cn("tabular text-right text-destructive", className)}>({formatIDR(Math.abs(value))})</span>;
    }
  }
  return <span className={cn("tabular text-right", className)}>{formatIDR(value)}</span>;
}

/** Kuantitas: angka id-ID tabular, jumlah desimal tetap bila `decimals` diberikan. */
export function Qty({
  value,
  decimals,
  unit,
  className,
}: {
  value: number | null | undefined;
  decimals?: number;
  unit?: string;
  className?: string;
}) {
  const text =
    decimals !== undefined && typeof value === "number" && !Number.isNaN(value)
      ? new Intl.NumberFormat("id-ID", { minimumFractionDigits: decimals, maximumFractionDigits: decimals }).format(value)
      : formatNumber(value);
  return (
    <span className={cn("tabular", className)}>
      {text}
      {unit ? <span className="ml-1 text-muted-foreground">{unit}</span> : null}
    </span>
  );
}

/** Nomor dokumen (SO-2026-00012): monospasi, dengan tombol salin opsional. */
export function DocNumber({ value, copyable = false, className }: { value: string; copyable?: boolean; className?: string }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard tidak tersedia (konteks tak aman): abaikan */
    }
  }
  return (
    <span className={cn("inline-flex items-center gap-1 font-mono text-[0.92em]", className)}>
      {value}
      {copyable && (
        <button
          type="button"
          onClick={copy}
          aria-label={copied ? "Tersalin" : `Salin ${value}`}
          className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          {copied ? <Check size={13} aria-hidden /> : <Copy size={13} aria-hidden />}
        </button>
      )}
    </span>
  );
}
