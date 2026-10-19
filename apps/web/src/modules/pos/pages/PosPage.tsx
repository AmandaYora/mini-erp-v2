import { useState } from "react";
import { useOutletContext } from "react-router-dom";
import type { FocusLayoutContext } from "@/shared/layouts/FocusLayout";
import { usePosCart } from "@/modules/pos/hooks/use-pos-cart";
import { usePosCatalog } from "@/modules/pos/hooks/use-pos-catalog";
import { PosTopbar } from "@/modules/pos/components/PosTopbar";
import { ProductGrid } from "@/modules/pos/components/ProductGrid";
import { CartPanel } from "@/modules/pos/components/CartPanel";
import { CheckoutSheet } from "@/modules/pos/components/CheckoutSheet";
import { ScanDialog } from "@/modules/pos/components/ScanDialog";
import { VariantSheet } from "@/modules/pos/components/VariantSheet";

/**
 * Mode Kasir (layar penuh, tablet sentuh landscape). Halaman hanya merangkai: logika di
 * hooks/use-pos-cart + use-pos-catalog, tampilan di components/. Dirender di dalam FocusLayout.
 */
export default function PosPage() {
  const outlet = useOutletContext<FocusLayoutContext | null>();
  const pos = usePosCart();
  const catalog = usePosCatalog();
  const [scannerOpen, setScannerOpen] = useState(false);

  return (
    <div className="flex h-full min-h-0">
      <PosTopbar
        slot={outlet ? outlet.topbarSlot : undefined}
        query={catalog.productQuery}
        onQueryChange={catalog.setProductQuery}
        onScan={() => setScannerOpen(true)}
      />
      <ProductGrid pos={pos} catalog={catalog} />
      <CartPanel pos={pos} />
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
