import { useCallback, useEffect, useState } from "react";
import {
  AsyncSearchSelect,
  type AsyncSearchLoadResult,
  type AsyncSearchOption,
} from "@/shared/components/ui";
import { productsService } from "@/modules/products/services/products.service";
import type { Product } from "@/modules/products/types";

type ProductOption = AsyncSearchOption & { product: Product };

export interface ProductSearchSelectProps {
  id?: string;
  name?: string;
  value?: number | null;
  defaultValue?: number | null;
  onChange?: (value: number | null, product?: Product) => void;
  onResolve?: (product?: Product) => void;
  status?: string;
  trackedOnly?: boolean;
  placeholder?: string;
  searchPlaceholder?: string;
  disabled?: boolean;
  limit?: number;
}

function formatProductOption(product: Product): ProductOption {
  return {
    value: product.id,
    label: `${product.code} — ${product.name}`,
    description: `Satuan: ${product.baseUom}`,
    product,
  };
}

/**
 * ProductSearchSelect — pemilih produk dengan pencarian SERVER-SIDE.
 *
 * Dibangun DI ATAS AsyncSearchSelect di dalam modul products (shared tetap
 * domain-agnostik), memakai productsService milik modul sendiri. Effect
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
  placeholder = "Cari kode atau nama produk…",
  searchPlaceholder = "Ketik kode atau nama produk…",
  disabled,
  limit = 20,
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

  const loadOptions = useCallback(
    async (search: string): Promise<AsyncSearchLoadResult> => {
      const res = await productsService.list({
        search: search || undefined,
        status,
        page: 1,
        limit,
      });
      const items = trackedOnly ? res.items.filter((p) => p.tracked) : res.items;
      return {
        options: items.map(formatProductOption),
        hasMore: res.meta.total > res.items.length,
      };
    },
    [status, limit, trackedOnly],
  );

  useEffect(() => {
    if (!selectedValue) {
      onResolve?.(undefined);
      return;
    }
    if (selected?.value === selectedValue) return;
    let active = true;
    productsService
      .detail(selectedValue)
      .then((product) => {
        if (!active) return;
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
