import { useState } from "react";
import { AsyncSearchSelect } from "@/shared/components/ui";
import type { AsyncSearchLoadResult, AsyncSearchOption } from "@/shared/components/ui";
import { searchOptionPage } from "@/modules/products/lib/product-picker";
import { toId } from "@/modules/stock/lib/ids";

export interface ProductSelectProps {
  productId: number | null;
  onChange: (id: number | null) => void;
  disabled?: boolean;
  id?: string;
  placeholder?: string;
  allowClear?: boolean;
}

async function loadProducts(q: string, signal?: AbortSignal, page = 1): Promise<AsyncSearchLoadResult> {
  const { rows, hasMore } = await searchOptionPage(q, page, {}, signal);
  return { options: rows.map((o) => ({ value: o.id, label: `${o.code} — ${o.name}` })), hasMore };
}

/**
 * Pemilih produk saja (tanpa varian), pencarian server lewat
 * GET /products/search-options berhalaman: menggulir memuat halaman berikutnya
 * sehingga setiap produk aktif bisa dipilih. Dipakai ProductPicker, baris barang, dan filter.
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
      paged
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
