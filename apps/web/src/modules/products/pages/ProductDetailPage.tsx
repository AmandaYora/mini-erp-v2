import { useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ActionRow,
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, formatNumber, statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { toast } from "@/shared/stores/toast.store";
import { productsService } from "@/modules/products/services/products.service";
import QrModal from "@/modules/products/components/QrModal";
import type { ProductMedia } from "@/modules/products/types";

export default function ProductDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const fileRef = useRef<HTMLInputElement>(null);

  const productId = Number(id);
  const invalidId = !Number.isInteger(productId) || productId <= 0;

  const [archiveOpen, setArchiveOpen] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [qrOpen, setQrOpen] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [mediaBusyId, setMediaBusyId] = useState<number | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ProductMedia | null>(null);

  const canUpdate = can("products.update");
  const canArchive = can("products.archive");

  const { data, loading, error: fetchError, reload } = useAsyncData(
    () => {
      if (invalidId) return Promise.resolve(null);
      return (async () => {
        const p = await productsService.detail(productId);
        let photos: ProductMedia[];
        try {
          photos = await productsService.listMedia(productId);
        } catch {
          photos = [];
        }
        return { product: p, media: photos };
      })();
    },
    [productId],
  );
  const product = data?.product ?? null;
  const media = data?.media ?? [];
  const error = invalidId ? "ID produk tidak valid." : fetchError;

  async function handleArchive() {
    setArchiving(true);
    try {
      const res = await productsService.archive(productId);
      toast.fromServer(res.message, "Produk diarsipkan");
      setArchiveOpen(false);
      reload();
    } catch (err) {
      toast.danger("Gagal mengarsipkan", toApiError(err).message);
    } finally {
      setArchiving(false);
    }
  }

  async function handleUpload(file: File | undefined) {
    if (!file) return;
    setUploading(true);
    try {
      const res = await productsService.uploadMedia(productId, file);
      toast.fromServer(res.message, "Foto diunggah");
      reload();
    } catch (err) {
      toast.danger("Gagal mengunggah foto", toApiError(err).message);
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function handleSetPrimary(mediaId: number) {
    setMediaBusyId(mediaId);
    try {
      const res = await productsService.setPrimaryMedia(productId, mediaId);
      toast.fromServer(res.message, "Foto utama diganti");
      reload();
    } catch (err) {
      toast.danger("Gagal mengganti foto utama", toApiError(err).message);
    } finally {
      setMediaBusyId(null);
    }
  }

  async function handleDeleteMedia() {
    if (!deleteTarget) return;
    setMediaBusyId(deleteTarget.id);
    try {
      const res = await productsService.deleteMedia(productId, deleteTarget.id);
      toast.fromServer(res.message, "Foto dihapus");
      setDeleteTarget(null);
      reload();
    } catch (err) {
      toast.danger("Gagal menghapus foto", toApiError(err).message);
    } finally {
      setMediaBusyId(null);
    }
  }

  if (loading) {
    return <p className="text-muted py-10 text-center text-sm">Memuat…</p>;
  }

  if (error || !product) {
    return (
      <div className="space-y-4">
        <PageHeader title="Detail Produk" description="Rincian produk." />
        <Notice tone="danger" title="Gagal memuat produk">
          {error ?? "Produk tidak ditemukan."}
        </Notice>
        <Button variant="secondary" onClick={() => navigate("/products")}>
          Kembali ke daftar
        </Button>
      </div>
    );
  }

  const archived = product.status === "archived";

  return (
    <div className="space-y-4">
      <PageHeader
        eyebrow={product.code}
        title={product.name}
        description={`${product.type === "jasa" ? "Jasa" : "Barang"}${product.categoryName ? ` • ${product.categoryName}` : ""}`}
        actions={
          <>
            <Button variant="secondary" onClick={() => navigate("/products")}>
              Kembali
            </Button>
            <Button variant="secondary" onClick={() => setQrOpen(true)}>
              Label QR
            </Button>
            {canUpdate && !archived && (
              <Button onClick={() => navigate(`/products/${product.id}/edit`)}>Ubah</Button>
            )}
            {canArchive && !archived && (
              <Button variant="danger" onClick={() => setArchiveOpen(true)}>
                Arsipkan
              </Button>
            )}
          </>
        }
      />

      {archived && (
        <Notice tone="danger" title="Produk diarsipkan">
          Produk ini sudah diarsipkan — data dan foto dikunci, riwayat tidak berubah.
        </Notice>
      )}

      <SectionCard title="Informasi">
        <dl className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Kode</dt>
            <dd className="mt-1 text-sm font-medium text-ink">{product.code}</dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Kategori</dt>
            <dd className="mt-1 text-sm text-ink">{product.categoryName || "-"}</dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Status</dt>
            <dd className="mt-1">
              <Badge tone={statusTone(product.status)}>{statusLabel(product.status)}</Badge>
            </dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Tipe</dt>
            <dd className="mt-1 text-sm text-ink">{product.type === "jasa" ? "Jasa" : "Barang"}</dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Terpantau</dt>
            <dd className="mt-1 text-sm text-ink">{product.tracked ? "Ya" : "Tidak"}</dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Stok Minimum</dt>
            <dd className="mt-1 text-sm text-ink">{formatNumber(product.minStock)}</dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Satuan</dt>
            <dd className="mt-1 text-sm text-ink">
              Dasar: {product.baseUom} • Beli: {product.purchaseUom} ({formatNumber(product.purchaseFactor)}) •
              Jual: {product.salesUom} ({formatNumber(product.salesFactor)})
            </dd>
          </div>
        </dl>
      </SectionCard>

      <SectionCard title="Harga" description="Rupiah penuh, tanpa desimal.">
        <dl className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Harga Beli</dt>
            <dd className="mt-1 text-sm font-medium text-ink">{formatIDR(product.purchasePrice)}</dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Harga Jual</dt>
            <dd className="mt-1 text-sm font-medium text-ink">{formatIDR(product.sellingPrice)}</dd>
          </div>
          <div>
            <dt className="text-muted text-xs font-semibold uppercase">Harga Jual Minimal</dt>
            <dd className="mt-1 text-sm font-medium text-ink">{formatIDR(product.minSellingPrice)}</dd>
          </div>
        </dl>
      </SectionCard>

      <SectionCard title="Varian" description="Setiap produk memiliki minimal satu varian.">
        <DataTable
          columns={[
            { header: "Kode", render: (r) => <span className="font-medium">{r.code}</span> },
            { header: "Nama", render: (r) => r.name || "-" },
            { header: "Barcode", render: (r) => r.barcode || "-" },
            {
              header: "Utama",
              render: (r) =>
                r.isDefault ? <Badge tone="info">Utama</Badge> : <span className="text-muted">-</span>,
            },
            {
              header: "Status",
              render: (r) => <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge>,
            },
          ]}
          rows={product.variants}
          rowKey={(r) => r.id}
          emptyTitle="Belum ada varian"
        />
      </SectionCard>

      <SectionCard
        title="Foto"
        description="Foto pertama yang diunggah menjadi foto utama. Maksimal 5MB per berkas."
      >
        {media.length === 0 ? (
          <p className="text-muted text-sm">Belum ada foto untuk produk ini.</p>
        ) : (
          <ul className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            {media.map((m) => (
              <li key={m.id} className="overflow-hidden rounded-lg border border-hairline bg-surface-subtle">
                <img src={m.url} alt={m.originalName} className="h-36 w-full object-cover" loading="lazy" />
                <div className="space-y-2 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <p className="truncate text-xs text-muted" title={m.originalName}>
                      {m.originalName}
                    </p>
                    {m.isPrimary && <Badge tone="success">Utama</Badge>}
                  </div>
                  {canUpdate && !archived && (
                    <div className="flex gap-2">
                      {!m.isPrimary && (
                        <Button
                          variant="secondary"
                          size="sm"
                          disabled={mediaBusyId === m.id}
                          onClick={() => handleSetPrimary(m.id)}
                        >
                          Jadikan utama
                        </Button>
                      )}
                      <Button
                        variant="danger"
                        size="sm"
                        disabled={mediaBusyId === m.id}
                        onClick={() => setDeleteTarget(m)}
                      >
                        Hapus
                      </Button>
                    </div>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}

        {canUpdate && !archived && (
          <div className="mt-4 flex flex-wrap items-center gap-3">
            <div>
              <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">Unggah foto (gambar saja)</span>
              <input
                ref={fileRef}
                type="file"
                accept="image/*"
                disabled={uploading}
                onChange={(e) => handleUpload(e.target.files?.[0])}
                className="text-sm text-ink"
              />
            </div>
            {uploading && <p className="text-muted text-sm">Mengunggah…</p>}
          </div>
        )}
      </SectionCard>

      <ActionRow>
        <Button variant="secondary" onClick={() => navigate("/products")}>
          Kembali ke daftar
        </Button>
      </ActionRow>

      <ConfirmDialog
        open={archiveOpen}
        title="Arsipkan produk?"
        message={`Produk ${product.code} — ${product.name} akan diarsipkan dan tidak bisa diubah lagi.`}
        confirmLabel={archiving ? "Mengarsipkan…" : "Arsipkan"}
        onConfirm={handleArchive}
        onCancel={() => setArchiveOpen(false)}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Hapus foto?"
        message={`Foto ${deleteTarget?.originalName ?? ""} akan dihapus permanen.`}
        confirmLabel="Hapus"
        onConfirm={handleDeleteMedia}
        onCancel={() => setDeleteTarget(null)}
      />

      <QrModal
        productId={qrOpen ? product.id : null}
        open={qrOpen}
        onClose={() => setQrOpen(false)}
      />
    </div>
  );
}
