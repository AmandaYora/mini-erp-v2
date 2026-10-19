import { FormSheet } from "@/shared/components/ui";
import { variantChoiceLabel } from "@/modules/pos/lib/scan";
import type { VariantChoiceState } from "@/modules/pos/hooks/use-pos-cart";

/** Pilih varian saat kode produk memuat lebih dari satu varian (perilaku sama dengan sebelumnya). */
export function VariantSheet({
  choice,
  onPick,
  onClose,
}: {
  choice: VariantChoiceState | null;
  onPick: (productId: number, variantId: number) => void;
  onClose: () => void;
}) {
  return (
    <FormSheet
      open={choice !== null}
      title="Pilih varian"
      description={
        choice
          ? `${choice.productCode} — ${choice.productName} punya beberapa varian. Pilih yang dijual.`
          : undefined
      }
      onClose={onClose}
    >
      <ul className="space-y-2">
        {(choice?.variants ?? []).map((v) => (
          <li key={v.id}>
            <button
              type="button"
              className="flex min-h-14 w-full touch-manipulation items-center rounded-lg border border-border bg-card px-4 py-2 text-left text-base text-foreground select-none active:bg-accent"
              onClick={() => choice && onPick(choice.productId, v.id)}
            >
              {variantChoiceLabel(v)}
            </button>
          </li>
        ))}
      </ul>
    </FormSheet>
  );
}
