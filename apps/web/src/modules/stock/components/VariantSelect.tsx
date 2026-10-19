import { useEffect, useMemo } from "react";
import { SelectInput } from "@/shared/components/ui";
import { toast } from "@/shared/stores/toast.store";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { stockService } from "@/modules/stock/services/stock.service";
import { toId } from "@/modules/stock/lib/ids";
import type { ProductVariantOption } from "@/modules/stock/types";

export interface VariantSelectProps {
  productId: number | null;
  variantId: number | null;
  onChange: (id: number | null) => void;
  disabled?: boolean;
  id?: string;
  "aria-label"?: string;
  /** Pilih otomatis bila produk hanya punya satu varian (untuk baris barang). */
  autoPick?: boolean;
}

/** Pilihan varian suatu produk (GET /products/{id}). */
export function VariantSelect({
  productId,
  variantId,
  onChange,
  disabled,
  id,
  "aria-label": ariaLabel,
  autoPick = false,
}: VariantSelectProps) {
  const { data, loading } = useAsyncData(async () => {
    if (productId === null) return { productId, variants: [] as ProductVariantOption[] };
    try {
      const p = await stockService.getProduct(productId);
      return { productId, variants: p.variants ?? [] };
    } catch (err) {
      toast.danger("Gagal memuat varian produk", toApiError(err).message);
      throw err;
    }
  }, [productId]);
  // Data basi (produk sebelumnya) tidak boleh dipakai selama produk baru dimuat.
  const fresh = data !== null && data.productId === productId && !loading;
  const variants = useMemo(() => (fresh ? data.variants : []), [fresh, data]);

  useEffect(() => {
    if (autoPick && fresh && variantId === null && variants.length === 1) {
      onChange(variants[0].id);
    }
  }, [autoPick, fresh, variantId, variants, onChange]);

  return (
    <SelectInput
      id={id}
      aria-label={ariaLabel ?? "Varian"}
      value={variantId === null ? "" : String(variantId)}
      disabled={disabled || productId === null || loading}
      onChange={(e) => onChange(toId(e.target.value))}
    >
      <option value="">{productId === null ? "Pilih produk dulu" : loading ? "Memuat varian…" : "Pilih varian…"}</option>
      {variants.map((v) => (
        <option key={v.id} value={String(v.id)}>
          {v.code} — {v.name}
          {v.isDefault ? " (utama)" : ""}
        </option>
      ))}
    </SelectInput>
  );
}
