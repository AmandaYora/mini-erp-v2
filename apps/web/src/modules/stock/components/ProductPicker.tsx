import { useCallback, useEffect, useState } from "react";
import {
  Button,
  FormField,
  SearchSelect,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { toast } from "@/shared/stores/toast.store";
import { toApiError } from "@/shared/services/http-client";
import { stockService } from "@/modules/stock/services/stock.service";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import type {
  ProductOption,
  ProductVariantOption,
} from "@/modules/stock/types";

interface ProductPickerProps {
  productId: number | null;
  variantId: number | null;
  onProductChange: (id: number | null) => void;
  onVariantChange: (id: number | null) => void;
  disabled?: boolean;
}

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

/**
 * Pemilih produk + varian. Opsi dari GET /api/v1/products/search-options
 * (maks 20 baris aktif, tanpa varian) sehingga varian diambil dari
 * GET /api/v1/products/{id} setelah produk dipilih.
 */
export function ProductPicker({
  productId,
  variantId,
  onProductChange,
  onVariantChange,
  disabled,
}: ProductPickerProps) {
  const [options, setOptions] = useState<ProductOption[]>([]);
  const [keyword, setKeyword] = useState("");
  const [searching, setSearching] = useState(false);

  const { data: variantsData, loading: loadingVariants } = useAsyncData(
    async () => {
      if (productId === null) {
        return [] as ProductVariantOption[];
      }
      try {
        const p = await stockService.getProduct(productId);
        return p.variants ?? [];
      } catch (err) {
        toast.danger("Gagal memuat varian produk", toApiError(err).message);
        throw err;
      }
    },
    [productId],
  );
  const variants: ProductVariantOption[] = variantsData ?? [];

  const loadOptions = useCallback(
    async (q: string, pinId: number | null) => {
      setSearching(true);
      try {
        const rows = await stockService.searchProductOptions(q);
        setOptions((prev) => {
          if (pinId === null) return rows;
          if (rows.some((r) => r.id === pinId)) return rows;
          const pinned = prev.find((r) => r.id === pinId);
          return pinned ? [pinned, ...rows] : rows;
        });
      } catch (err) {
        toast.danger("Gagal memuat opsi produk", toApiError(err).message);
      } finally {
        setSearching(false);
      }
    },
    [],
  );

  useEffect(() => {
    let alive = true;
    stockService
      .searchProductOptions("")
      .then((rows) => {
        if (alive) setOptions(rows);
      })
      .catch((err) => {
        if (alive)
          toast.danger("Gagal memuat opsi produk", toApiError(err).message);
      });
    return () => {
      alive = false;
    };
    // Muat sekali saat mount; pencarian server dipicu tombol Cari.
  }, []);

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <FormField label="Produk" required>
        <div className="flex gap-2">
          <div className="min-w-0 flex-1">
            <TextInput
              placeholder="Kata kunci…"
              value={keyword}
              disabled={disabled}
              onChange={(e) => setKeyword(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  void loadOptions(keyword, productId);
                }
              }}
            />
          </div>
          <Button
            variant="secondary"
            disabled={disabled || searching}
            onClick={() => void loadOptions(keyword, productId)}
          >
            {searching ? "Mencari…" : "Cari"}
          </Button>
        </div>
        <div className="mt-2">
          <SearchSelect
            options={options.map((o) => ({
              value: o.id,
              label: `${o.code} — ${o.name}`,
            }))}
            value={productId}
            placeholder="Pilih produk…"
            allowClear
            disabled={disabled}
            onChange={(v) => {
              const id = toId(v);
              onProductChange(id);
              onVariantChange(null);
            }}
          />
        </div>
      </FormField>
      <FormField
        label="Varian"
        required
        helperText={
          productId === null
            ? "Pilih produk dulu."
            : loadingVariants
              ? "Memuat varian…"
              : undefined
        }
      >
        <SelectInput
          value={variantId === null ? "" : String(variantId)}
          disabled={disabled || productId === null || loadingVariants}
          onChange={(e) => onVariantChange(toId(e.target.value))}
        >
          <option value="">Pilih varian…</option>
          {variants.map((v) => (
            <option key={v.id} value={String(v.id)}>
              {v.code} — {v.name}
              {v.isDefault ? " (utama)" : ""}
            </option>
          ))}
        </SelectInput>
      </FormField>
    </div>
  );
}
