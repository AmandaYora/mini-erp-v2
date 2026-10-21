import { useState } from "react";
import { Button, CardSkeleton, Modal } from "@/shared/components/ui";
import { FileDropzone } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeService } from "@/modules/finance/services/finance.service";
import type { TaxAdjustment } from "@/modules/finance/types";

interface TaxAdjustmentFilesModalProps {
  adjustment: TaxAdjustment;
  canManage: boolean;
  onClose: () => void;
}

// Lampiran koreksi fiskal (FN-07): daftar gambar pendukung + tambah lampiran baru.
export function TaxAdjustmentFilesModal({ adjustment, canManage, onClose }: TaxAdjustmentFilesModalProps) {
  const { data, loading, error, reload } = useAsyncData(() => financeService.taxAdjustmentFiles(adjustment.id).then((f) => f ?? []), [adjustment.id]);
  const [uploading, setUploading] = useState(false);
  const files = data ?? [];

  async function handleFile(file: File) {
    setUploading(true);
    try {
      await financeService.attachTaxAdjustment(adjustment.id, file);
      toast.success("Lampiran diunggah");
      reload();
    } catch (err) {
      toast.danger("Gagal mengunggah lampiran", formatApiError(err));
    } finally {
      setUploading(false);
    }
  }

  return (
    <Modal
      open
      size="md"
      title={`Lampiran ${adjustment.number}`}
      description="Bukti pendukung koreksi fiskal (gambar)."
      onClose={onClose}
      actions={
        <Button variant="secondary" onClick={onClose}>
          Tutup
        </Button>
      }
    >
      {error ? (
        <ErrorState message={error} onRetry={reload} title="Gagal memuat lampiran" />
      ) : loading ? (
        <CardSkeleton className="h-24 border-0 p-0" />
      ) : files.length === 0 ? (
        <p className="text-sm text-muted-foreground">Belum ada lampiran.</p>
      ) : (
        <ul className="grid grid-cols-2 gap-3">
          {files.map((f) => (
            <li key={f.id} className="overflow-hidden rounded-lg border border-border">
              <a href={f.url} target="_blank" rel="noreferrer" className="block">
                <img src={f.url} alt={f.name} className="h-32 w-full object-cover" />
                <span className="block truncate px-2 py-1 text-xs text-muted-foreground">{f.name}</span>
              </a>
            </li>
          ))}
        </ul>
      )}
      {canManage && (
        <div className="mt-4">
          <FileDropzone
            accept="image/jpeg,image/png,image/webp"
            busy={uploading}
            label="Tambah lampiran"
            hint="JPG, PNG, atau WebP, maks 5 MB"
            onFile={(f) => void handleFile(f)}
          />
        </div>
      )}
    </Modal>
  );
}
