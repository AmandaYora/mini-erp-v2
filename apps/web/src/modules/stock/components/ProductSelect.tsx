import { useState } from "react";
import { AsyncSearchSelect } from "@/shared/components/ui";
import type { AsyncSearchOption } from "@/shared/components/ui";
import { stockService } from "@/modules/stock/services/stock.service";
import { toId } from "@/modules/stock/lib/ids";

export interface ProductSelectProps {
  productId: number | null;
  onChange: (id: number | null) => void;
  disabled?: boolean;
  id?: string;
  placeholder?: string;
  allowClear?: boolean;
}

async function loadProducts(q: string): Promise<AsyncSearchOption[]> {
  const rows = await stockService.searchProductOptions(q);
  return rows.map((o) => ({ value: o.id, label: `${o.code} — ${o.name}` }));
}

/**
 * Pemilih produk saja (tanpa varian), pencarian server lewat
 * GET /products/search-options. Dipakai ProductPicker, baris barang, dan filter.
 */
export function ProductSelect({
  productId,
  onChange,
  disabled,
  id,
  placeholder = "Cari kode atau nama produk…",
  allowClear = true,
}: ProductSelectProps) {
  const [picked, setPicked] = useState<AsyncSearchOption | null>(null);
  return (
    <AsyncSearchSelect
      id={id}
      loadOptions={loadProducts}
      value={productId}
      selectedOption={productId === null ? null : picked}
      placeholder={placeholder}
      searchPlaceholder="Kode atau nama produk…"
      allowClear={allowClear}
      disabled={disabled}
      onChange={(v, option) => {
        setPicked(option ?? null);
        onChange(toId(v));
      }}
    />
  );
}
