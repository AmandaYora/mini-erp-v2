import { useId } from "react";
import { FormField } from "@/shared/components/ui";
import { ProductSelect } from "@/modules/stock/components/ProductSelect";
import { VariantSelect } from "@/modules/stock/components/VariantSelect";

interface ProductPickerProps {
  productId: number | null;
  variantId: number | null;
  onProductChange: (id: number | null) => void;
  onVariantChange: (id: number | null) => void;
  disabled?: boolean;
}

/**
 * Pemilih produk + varian. Opsi produk dicari di server
 * (GET /api/v1/products/search-options, maks 20 baris aktif, tanpa varian) sehingga
 * varian diambil dari GET /api/v1/products/{id} setelah produk dipilih.
 */
export function ProductPicker({
  productId,
  variantId,
  onProductChange,
  onVariantChange,
  disabled,
}: ProductPickerProps) {
  const uid = useId();
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <FormField
        label="Produk"
        htmlFor={`${uid}-product`}
        required
        helperText="Ketik kode atau nama untuk mencari."
      >
        <ProductSelect
          id={`${uid}-product`}
          productId={productId}
          disabled={disabled}
          onChange={(id) => {
            onProductChange(id);
            onVariantChange(null);
          }}
        />
      </FormField>
      <FormField
        label="Varian"
        htmlFor={`${uid}-variant`}
        required
        helperText={productId === null ? "Pilih produk dulu." : undefined}
      >
        <VariantSelect
          id={`${uid}-variant`}
          productId={productId}
          variantId={variantId}
          disabled={disabled}
          onChange={onVariantChange}
        />
      </FormField>
    </div>
  );
}
