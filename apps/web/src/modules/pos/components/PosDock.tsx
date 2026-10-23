import { ChevronRight, ScanLine } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import { formatIDR, formatNumber } from "@/shared/lib/format";

/**
 * "Dok kasir" di layar < 1024px (brief desain-pos-hp §2/§4b): bingkai navy bawah yang menaruh dua
 * kata kerja kasir di zona ibu jari — Pindai (kiri) dan Keranjang berisi jumlah barang + total
 * berjalan (kanan). Hanya tampilan; logika tetap di usePosCart.
 */
export function PosDock({
  itemCount,
  total,
  started,
  onScan,
  onOpenCart,
}: {
  itemCount: number;
  total: number;
  /** Checkout sudah dimulai (order ada di server): tombol kanan melanjutkan pembayaran. */
  started: boolean;
  onScan: () => void;
  onOpenCart: () => void;
}) {
  const empty = itemCount === 0;
  const label = started
    ? `Lanjutkan pembayaran, total ${formatIDR(total)}`
    : empty
      ? "Keranjang kosong"
      : `Buka keranjang, ${formatNumber(itemCount)} barang, total ${formatIDR(total)}`;
  return (
    <nav
      aria-label="Aksi kasir"
      className="flex flex-none items-center gap-2 bg-sidebar px-3 pt-2 text-sidebar-strong"
      style={{ paddingBottom: "max(0.5rem, env(safe-area-inset-bottom))" }}
    >
      <button
        type="button"
        onClick={onScan}
        className="flex h-12 flex-none touch-manipulation items-center gap-2 rounded-lg bg-sidebar-accent px-4 text-base font-semibold text-sidebar-strong select-none active:bg-sidebar-active focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:outline-none"
      >
        <ScanLine aria-hidden className="size-5" />
        Pindai
      </button>
      <button
        type="button"
        aria-label={label}
        disabled={empty && !started}
        onClick={onOpenCart}
        className={cn(
          "flex h-12 min-w-0 flex-1 touch-manipulation items-center justify-between gap-2 rounded-lg px-4 text-left select-none focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:outline-none",
          empty && !started
            ? "bg-sidebar-accent text-sidebar-muted"
            : "bg-sidebar-primary text-sidebar-primary-foreground active:scale-[0.98]",
        )}
      >
        {empty && !started ? (
          <span className="text-base font-medium">Keranjang kosong</span>
        ) : (
          <span className="flex min-w-0 flex-col leading-tight">
            <span className="text-sm font-semibold">{started ? "Lanjutkan pembayaran" : `${formatNumber(itemCount)} barang`}</span>
            <span className="tabular truncate font-display text-xl font-bold">{formatIDR(total)}</span>
          </span>
        )}
        <ChevronRight aria-hidden className="size-5 flex-none" />
      </button>
      <p role="status" aria-atomic="true" className="sr-only">
        {empty ? "Keranjang kosong" : `${formatNumber(itemCount)} barang, total ${formatIDR(total)}`}
      </p>
    </nav>
  );
}
