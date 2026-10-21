import { useState } from "react";
import { Link } from "react-router-dom";
import { SectionCard } from "@/shared/components/ui";
import { FileDropzone } from "@/shared/components/patterns";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { paymentService } from "@/modules/payment/services/payment.service";
import type { ReceiptPayment } from "@/modules/goodsreceipt/types";

interface ReceiptPaymentProofProps {
  payment: ReceiptPayment;
}

/**
 * Bukti bayar pelunasan saat terima (GR-02): pembayaran pay_now sudah tercatat,
 * slip transfer dilampirkan setelah posting lewat endpoint bukti pembayaran.
 */
export function ReceiptPaymentProof({ payment }: ReceiptPaymentProofProps) {
  const [uploading, setUploading] = useState(false);
  const [uploaded, setUploaded] = useState<string[]>([]);

  async function handleFile(file: File) {
    setUploading(true);
    try {
      const res = await paymentService.uploadProof(payment.id, file);
      toast.fromServer(res.message, "Bukti bayar diunggah");
      setUploaded((prev) => [...prev, file.name]);
    } catch (err) {
      toast.danger("Gagal mengunggah bukti", formatApiError(err));
    } finally {
      setUploading(false);
    }
  }

  return (
    <SectionCard
      title="Bukti bayar"
      description={`Pembayaran ${payment.number} tercatat. Lampirkan slip transfer bila ada.`}
    >
      <div className="flex flex-col gap-3">
        {uploaded.length > 0 && (
          <ul className="flex flex-col gap-1 text-sm">
            {uploaded.map((n, i) => (
              <li key={`${n}-${i}`} className="truncate">
                {n}
              </li>
            ))}
          </ul>
        )}
        <FileDropzone
          accept="image/*"
          busy={uploading}
          label="Seret bukti bayar ke sini atau pilih"
          hint="Gambar slip atau kwitansi"
          onFile={(f) => void handleFile(f)}
        />
        <Link to={`${ROUTE_PATHS.payments}/${payment.id}`} className="text-sm text-primary hover:underline">
          Buka pembayaran
        </Link>
      </div>
    </SectionCard>
  );
}
