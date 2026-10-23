import { useState } from "react";
import { FormSheet, SearchInput } from "@/shared/components/ui";
import { cn } from "@/shared/lib/cn";
import { formatNumber } from "@/shared/lib/format";
import { formatQty } from "@/modules/pos/lib/qty";
import { variantChoiceLabel } from "@/modules/pos/lib/scan";
import type { VariantChoiceState } from "@/modules/pos/hooks/use-pos-cart";

/** Di atas jumlah ini lembar menampilkan kolom cari (produk cat bisa punya ratusan warna). */
const SEARCH_FROM = 8;

/**
 * Pilih varian: produk dengan lebih dari satu varian layak jual (ketuk tile atau kode produk
 * hasil pindai). Stok tersedia per varian tampil bila diketahui; varian tanpa stok tetap bisa
 * dipilih (aturan stok tetap ditegakkan tahanan stok + server).
 */
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
          ? `${choice.productCode} — ${choice.productName} punya ${formatNumber(choice.variants.length)} varian. Pilih yang dijual.`
          : undefined
      }
      onClose={onClose}
    >
      {/* key: daftar & kata cari dipasang ulang untuk tiap produk. */}
      {choice && <VariantList key={choice.productId} choice={choice} onPick={onPick} />}
    </FormSheet>
  );
}

function VariantList({ choice, onPick }: { choice: VariantChoiceState; onPick: (productId: number, variantId: number) => void }) {
  const [query, setQuery] = useState("");
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const shown = choice.variants.filter((v) => {
    const hay = `${v.code} ${v.name} ${v.barcode}`.toLowerCase();
    return words.every((w) => hay.includes(w));
  });
  return (
    <div className="flex flex-col gap-3">
      {choice.variants.length > SEARCH_FROM && (
        <SearchInput aria-label="Cari varian" placeholder="Cari varian (kode / nama / warna)…" value={query} onSearch={setQuery} debounceMs={150} />
      )}
      {shown.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted-foreground">Tidak ada varian yang cocok.</p>
      ) : (
        <ul className="space-y-2">
          {shown.map((v) => {
            const empty = v.available !== undefined && v.available <= 0;
            return (
              <li key={v.id}>
                <button
                  type="button"
                  className="flex min-h-14 w-full touch-manipulation items-center justify-between gap-3 rounded-lg border border-border bg-card px-4 py-2 text-left text-base text-foreground select-none active:bg-accent"
                  onClick={() => onPick(choice.productId, v.id)}
                >
                  <span className="min-w-0">{variantChoiceLabel(v)}</span>
                  {v.available !== undefined && (
                    <span className={cn("tabular flex-none text-sm", empty ? "text-destructive" : "text-muted-foreground")}>
                      {empty ? "Stok habis" : `Stok ${formatQty(v.available)}`}
                    </span>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
