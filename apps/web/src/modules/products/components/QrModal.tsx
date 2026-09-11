// Modal QR label tunggal (J2, legacy F-08.5): pratinjau + unduh 1 PNG +
// tautan cetak label. Produk bervarian memilih target per varian (tanpa
// target induk, legacy F-08.1); isi QR = barcode bila ada, else kode.

import { useEffect, useState } from "react";
import {
  Button,
  FormField,
  Modal,
  SelectInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { productsService } from "@/modules/products/services/products.service";
import { productLabelsPrintPath } from "@/app/routes/route-paths";
import type { ProductVariant } from "@/modules/products/types";

interface QrModalProps {
  productId: number | null;
  open: boolean;
  onClose: () => void;
}

function variantPayload(v: ProductVariant): string {
  return v.barcode?.trim() ? v.barcode.trim() : v.code;
}

export default function QrModal({ productId, open, onClose }: QrModalProps) {
  const [variantId, setVariantId] = useState<number | null>(null);
  const [downloading, setDownloading] = useState(false);

  const { data: fetchedProduct, loading, error: productError } = useAsyncData(
    () => (!open || productId === null ? Promise.resolve(null) : productsService.detail(productId)),
    [open, productId],
  );

  // Tampilkan hanya produk yang cocok dengan target — saat target berganti,
  // produk lama disembunyikan sampai yang baru tiba (seperti reset semula).
  const product = fetchedProduct && fetchedProduct.id === productId ? fetchedProduct : null;

  // Varian default mengikuti produk yang dimuat; pilihan manual user
  // dipertahankan selama produknya sama.
  const loadedProductId = product?.id ?? null;
  const [prevProductId, setPrevProductId] = useState<number | null>(null);
  if (loadedProductId !== prevProductId) {
    setPrevProductId(loadedProductId);
    const def = product
      ? (product.variants.find((v) => v.isDefault) ?? product.variants[0] ?? null)
      : null;
    setVariantId(def ? def.id : null);
  }

  const { data: imgUrl, error: qrError } = useAsyncData(
    async () => {
      if (!open || product === null || variantId === null) return null;
      try {
        return await productsService.qrPngUrl(product.id, variantId);
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat QR", apiErr.message);
        throw err;
      }
    },
    [open, loadedProductId, variantId],
  );
  const error = productError ?? qrError;

  // Bersihkan blob URL QR saat berganti/ditutup — tanpa setState.
  useEffect(() => {
    return () => {
      if (imgUrl) URL.revokeObjectURL(imgUrl);
    };
  }, [imgUrl]);

  const variant: ProductVariant | null =
    product?.variants.find((v) => v.id === variantId) ?? null;

  async function handleDownload() {
    if (product === null || variant === null) return;
    setDownloading(true);
    try {
      await productsService.downloadQr(
        product.id,
        `${variant.code || product.code}.png`,
        variant.id,
      );
    } catch (err) {
      toast.danger("Gagal mengunduh QR", toApiError(err).message);
    } finally {
      setDownloading(false);
    }
  }

  return (
    <Modal
      open={open}
      title="Label QR"
      description="Pindai untuk mengisi keranjang POS atau pencarian — sama dengan input barcode."
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" onClick={onClose}>
            Tutup
          </Button>
          {product !== null ? (
            <Button
              variant="secondary"
              onClick={() =>
                window.open(productLabelsPrintPath([product.id]), "_blank", "noreferrer")
              }
            >
              Cetak Label
            </Button>
          ) : null}
          <Button
            onClick={() => void handleDownload()}
            disabled={downloading || variant === null}
          >
            {downloading ? "Mengunduh…" : "Unduh PNG"}
          </Button>
        </>
      }
    >
      {loading ? (
        <p className="text-muted py-6 text-center text-sm">Memuat…</p>
      ) : error !== null ? (
        <Notice tone="danger" title="Gagal memuat QR">
          {error}
        </Notice>
      ) : product !== null ? (
        <div className="space-y-4">
          {product.variants.length > 1 ? (
            <FormField label="Varian">
              <SelectInput
                value={variantId === null ? "" : String(variantId)}
                onChange={(e) =>
                  setVariantId(e.target.value === "" ? null : Number(e.target.value))
                }
              >
                {product.variants.map((v) => (
                  <option key={v.id} value={v.id}>
                    {v.name || v.code}
                    {v.isDefault ? " (utama)" : ""}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          ) : null}
          {imgUrl !== null && variant !== null ? (
            <div className="flex flex-col items-center gap-2">
              <img
                src={imgUrl}
                alt={`QR ${variant.code}`}
                className="h-60 w-60 border border-hairline bg-white"
              />
              <p className="text-sm font-bold text-ink">
                {product.name}
                {product.variants.length > 1 ? ` - ${variant.name || variant.code}` : ""}
              </p>
              <p className="font-mono text-sm text-muted">{variantPayload(variant)}</p>
            </div>
          ) : (
            <p className="text-muted py-6 text-center text-sm">Memuat QR…</p>
          )}
        </div>
      ) : null}
    </Modal>
  );
}
