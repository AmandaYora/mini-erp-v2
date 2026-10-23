import { useState } from "react";
import { useOutletContext } from "react-router-dom";
import { X } from "lucide-react";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/shared/components/ui";
import { formatNumber } from "@/shared/lib/format";
import type { FocusLayoutContext } from "@/shared/layouts/FocusLayout";
import { usePosCart } from "@/modules/pos/hooks/use-pos-cart";
import { usePosCatalog } from "@/modules/pos/hooks/use-pos-catalog";
import { POS_LANDSCAPE_QUERY, useIsWide, useMediaQuery } from "@/modules/pos/hooks/use-is-wide";
import { PosTopbar } from "@/modules/pos/components/PosTopbar";
import { ProductGrid } from "@/modules/pos/components/ProductGrid";
import { CartPanel } from "@/modules/pos/components/CartPanel";
import { CheckoutSheet } from "@/modules/pos/components/CheckoutSheet";
import { PosDock } from "@/modules/pos/components/PosDock";
import { ScanDialog } from "@/modules/pos/components/ScanDialog";
import { VariantSheet } from "@/modules/pos/components/VariantSheet";

/**
 * Mode Kasir (layar penuh, sentuh). Halaman hanya merangkai: logika di hooks/use-pos-cart +
 * use-pos-catalog, tampilan di components/. Dirender di dalam FocusLayout.
 *
 * Lebar ≥ 1024 (tablet mendatar): katalog + panel keranjang berdampingan. Di bawahnya (HP, tablet
 * tegak; temuan 5, brief desain-pos-hp): katalog penuh, dok bawah Pindai/Keranjang, keranjang dalam
 * sheet bawah; HP mendatar menaruh pencarian di topbar agar katalog tetap terlihat. Checkout, varian, dan pemindai sama untuk keduanya.
 */
export default function PosPage() {
  const outlet = useOutletContext<FocusLayoutContext | null>();
  const pos = usePosCart();
  const catalog = usePosCatalog();
  const wide = useIsWide();
  const landscape = useMediaQuery(POS_LANDSCAPE_QUERY, false);
  const [scannerOpen, setScannerOpen] = useState(false);
  const [cartOpenRaw, setCartOpen] = useState(false);
  // Diputar ke mendatar saat sheet terbuka: panel keranjang tampil di samping, sheet tidak dibawa.
  const cartOpen = cartOpenRaw && !wide;
  const topbar = {
    query: catalog.productQuery,
    onQueryChange: catalog.setProductQuery,
    onScan: () => setScannerOpen(true),
  };

  return (
    <div className={wide ? "flex h-full min-h-0" : "flex h-full min-h-0 flex-col"}>
      {wide ? (
        <>
          <PosTopbar slot={outlet ? outlet.topbarSlot : undefined} {...topbar} />
          <ProductGrid pos={pos} catalog={catalog} />
          <CartPanel pos={pos} />
        </>
      ) : (
        <>
          <PosTopbar compact slot={landscape && outlet ? outlet.topbarSlot : undefined} {...topbar} />
          <ProductGrid pos={pos} catalog={catalog} />
          <PosDock
            itemCount={pos.totals.itemCount}
            total={pos.totals.total}
            started={pos.started}
            onScan={() => setScannerOpen(true)}
            onOpenCart={() => {
              if (pos.started && pos.readiness.canCheckout) pos.openCheckout();
              else setCartOpen(true);
            }}
          />
          <Sheet open={cartOpen} onOpenChange={setCartOpen}>
            <SheetContent side="bottom" showCloseButton={false} className="flex h-dvh gap-0 bg-card p-0">
              <SheetHeader className="flex-row items-center justify-between gap-3 border-b border-border px-4 py-2 text-left">
                <div className="min-w-0">
                  <SheetTitle className="text-lg font-semibold">
                    Keranjang · {formatNumber(pos.totals.itemCount)} barang
                  </SheetTitle>
                  <SheetDescription className="sr-only">Isi keranjang, pelanggan, dan tombol bayar.</SheetDescription>
                </div>
                <button
                  type="button"
                  aria-label="Tutup keranjang"
                  onClick={() => setCartOpen(false)}
                  className="-mr-2 flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg text-muted-foreground select-none active:bg-accent"
                >
                  <X aria-hidden className="size-6" />
                </button>
              </SheetHeader>
              <CartPanel pos={pos} variant="sheet" onBeforeCheckout={() => setCartOpen(false)} />
            </SheetContent>
          </Sheet>
        </>
      )}
      <CheckoutSheet pos={pos} />
      <VariantSheet
        choice={pos.variantChoice}
        onClose={() => pos.setVariantChoice(null)}
        onPick={(productId, variantId) => {
          pos.setVariantChoice(null);
          void pos.addProductById(productId, variantId);
        }}
      />
      <ScanDialog
        open={scannerOpen}
        paused={pos.variantChoice !== null}
        onClose={() => setScannerOpen(false)}
        onScan={pos.scanCode}
      />
    </div>
  );
}
