import { useCallback, useEffect, useState } from "react";
import {
  AsyncSearchSelect,
  type AsyncSearchLoadResult,
  type AsyncSearchOption,
} from "@/shared/components/ui";
import { productsService } from "@/modules/products/services/products.service";
import { PRODUCT_PICKER_PAGE_SIZE, searchOptionPage } from "@/modules/products/lib/product-picker";
import type { ProductOptionRow } from "@/modules/products/types";

type ProductOption = AsyncSearchOption & { product: ProductOptionRow };

export interface ProductSearchSelectProps {
  id?: string;
  name?: string;
  value?: number | null;
  defaultValue?: number | null;
  onChange?: (value: number | null, product?: ProductOptionRow) => void;
  onResolve?: (product?: ProductOptionRow) => void;
  status?: string;
  trackedOnly?: boolean;
  /** Stok tersedia di cabang aktif naik ke atas (dropdown transaksi); relevansi tetap menang. */
  preferStock?: boolean;
  placeholder?: string;
  searchPlaceholder?: string;
  disabled?: boolean;
  limit?: number;
}

function formatProductOption(product: ProductOptionRow): ProductOption {
  const parts = [`Satuan: ${product.baseUom}`];
  const visible = product.variants.filter((v) => !v.isHidden && v.status === "active").length;
  if (visible > 1) parts.push(`${visible} varian`);
  if (!product.tracked) parts.push("tak dipantau");
  return {
    value: product.id,
    label: `${product.code} — ${product.name}`,
    description: parts.join(" · "),
    product,
  };
}

/**
 * ProductSearchSelect — pemilih produk dengan pencarian SERVER-SIDE.
 *
 * Dibangun DI ATAS AsyncSearchSelect di dalam modul products (shared tetap
 * domain-agnostik). Opsi dari GET /products/search-options (satu panggilan:
 * satuan, faktor, harga, tipe, varian — tanpa detail per pilihan). Effect
 * resolve hanya berisi callback async sehingga bebas peringatan
 * set-state-in-effect.
 */
export function ProductSearchSelect({
  id,
  name,
  value,
  defaultValue,
  onChange,
  onResolve,
  status = "active",
  trackedOnly,
  preferStock,
  placeholder = "Cari kode atau nama produk…",
  searchPlaceholder = "Ketik kode atau nama produk…",
  disabled,
  limit = PRODUCT_PICKER_PAGE_SIZE,
}: ProductSearchSelectProps) {
  const [uncontrolled, setUncontrolled] = useState<number | null>(defaultValue ?? null);
  const [prevDefault, setPrevDefault] = useState(defaultValue);
  if (defaultValue !== undefined && defaultValue !== prevDefault && value === undefined) {
    setPrevDefault(defaultValue);
    setUncontrolled(defaultValue);
  }
  const selectedValue = value ?? uncontrolled;
  const [selected, setSelected] = useState<ProductOption | null>(null);
  // Pengosongan saat value dikosongkan — render-phase adjustment.
  if ((selectedValue === null || selectedValue === undefined) && selected !== null) {
    setSelected(null);
  }

  // Berhalaman: menggulir memuat halaman berikutnya sehingga setiap produk yang cocok
  // bisa dipilih (bukan hanya baris pertama search-options).
  const loadOptions = useCallback(
    async (search: string, signal?: AbortSignal, page = 1): Promise<AsyncSearchLoadResult> => {
      const { rows, hasMore } = await searchOptionPage(
        search,
        page,
        { status, tracked: trackedOnly ? true : undefined, preferStock, limit },
        signal,
      );
      return { options: rows.map(formatProductOption), hasMore };
    },
    [status, limit, trackedOnly, preferStock],
  );

  useEffect(() => {
    if (!selectedValue) {
      onResolve?.(undefined);
      return;
    }
    if (selected?.value === selectedValue) return;
    let active = true;
    // Satu panggilan ringkas dengan productId — tanpa pencarian ulang atau detail penuh.
    productsService
      .searchOptions({ productId: selectedValue, limit: 1 })
      .then((rows) => {
        if (!active) return;
        const product = rows[0];
        if (!product) {
          setSelected(null);
          onResolve?.(undefined);
          return;
        }
        const option = formatProductOption(product);
        setSelected(option);
        onResolve?.(option.product);
      })
      .catch(() => {
        if (!active) return;
        setSelected(null);
        onResolve?.(undefined);
      });
    return () => {
      active = false;
    };
  }, [selectedValue, selected, onResolve]);

  return (
    <AsyncSearchSelect
      id={id}
      name={name}
      value={selectedValue}
      disabled={disabled}
      placeholder={placeholder}
      searchPlaceholder={searchPlaceholder}
      selectedOption={selected}
      loadOptions={loadOptions}
      paged
      onChange={(next, option) => {
        const productOption = option as ProductOption | undefined;
        const numeric =
          typeof next === "number" ? next : next === null ? null : Number(next) || null;
        setUncontrolled(numeric);
        setSelected(productOption ?? null);
        onChange?.(numeric, productOption?.product);
      }}
    />
  );
}
