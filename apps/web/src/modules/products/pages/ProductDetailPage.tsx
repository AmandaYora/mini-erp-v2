import { useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Archive, QrCode } from "lucide-react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  DetailSkeleton,
  PageHeader,
  SectionCard,
  StatusBadge,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import {
  ActionMenu,
  DescriptionList,
  DocNumber,
  FileDropzone,
  Money,
  Page,
  Qty,
  useUrlTab,
} from "@/shared/components/patterns";
import type { DescriptionItem } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { TableSkeleton } from "@/shared/components/feedback/skeleton";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS, stockItemPath } from "@/app/routes/route-paths";
import { toast } from "@/shared/stores/toast.store";
import { productsService } from "@/modules/products/services/products.service";
import QrModal from "@/modules/products/components/QrModal";
import { classifyServerMessage, guardTitle, productTypeLabel } from "@/modules/products/lib/product-form";
import {
  MEDIA_ACCEPT_ATTR,
  formatDimensions,
  validatePhotoFile,
} from "@/modules/products/lib/media-rules";
import type { ProductMedia, ProductStockBalance, ProductVariant } from "@/modules/products/types";

const TABS = ["ringkasan", "harga", "varian", "foto", "stok"] as const;

interface StockRow {
  variantId: number;
  name: string;
  onHand: number;
  reserved: number;
  available: number;
}

export default function ProductDetailPage() {
  const { id } = useParams<{ id: string }>();
  const can = useAuthStore((s) => s.can);

  const productId = Number(id);
  const invalidId = !Number.isInteger(productId) || productId <= 0;

  const [archiveOpen, setArchiveOpen] = useState(false);
  const [archiveError, setArchiveError] = useState<{ title: string; message: string } | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [archiving, setArchiving] = useState(false);
  const [qrOpen, setQrOpen] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [mediaBusyId, setMediaBusyId] = useState<number | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ProductMedia | null>(null);

  const canUpdate = can("products.update");
  const canArchive = can("products.archive");
  const canStock = can("stock.view");

  const [rawTab, setTab] = useUrlTab(TABS, "ringkasan");
  const tab = rawTab === "stok" && !canStock ? "ringkasan" : rawTab;

  const {
    data,
    loading,
    error: fetchError,
    errorStatus,
    reload,
  } = useAsyncData(
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
  usePageCrumb(product?.name);

  // Saldo stok dimuat saat tab Stok dibuka (dan hanya untuk produk terpantau).
  const stockWanted = tab === "stok" && canStock && product !== null && product.tracked;
  const {
    data: balances,
    loading: stockLoading,
    error: stockError,
    reload: reloadStock,
  } = useAsyncData(
    () => (stockWanted ? productsService.stockBalances(productId) : Promise.resolve(null)),
    [stockWanted, productId],
  );

  const stockRows: StockRow[] = useMemo(() => {
    if (!product || !balances) return [];
    const names = new Map<number, string>(
      product.variants.map((v: ProductVariant) => [v.id, v.isHidden ? "Varian bawaan" : v.name || v.code]),
    );
    const sum = new Map<number, StockRow>();
    for (const b of balances as ProductStockBalance[]) {
      const row = sum.get(b.variantId) ?? {
        variantId: b.variantId,
        name: names.get(b.variantId) ?? `Varian #${b.variantId}`,
        onHand: 0,
        reserved: 0,
        available: 0,
      };
      row.onHand += b.onHand;
      row.reserved += b.reserved;
      row.available += b.available;
      sum.set(b.variantId, row);
    }
    return [...sum.values()];
  }, [product, balances]);

  async function handleArchive() {
    setArchiving(true);
    try {
      const res = await productsService.archive(productId);
      toast.fromServer(res.message, "Produk diarsipkan");
      setArchiveOpen(false);
      setArchiveError(null);
      reload();
    } catch (err) {
      const message = toApiError(err).message;
      setArchiveOpen(false);
      setArchiveError({ title: guardTitle(classifyServerMessage(message)), message });
      toast.danger("Gagal mengarsipkan", message);
    } finally {
      setArchiving(false);
    }
  }

  async function handleUpload(file: File) {
    const rejected = validatePhotoFile(file);
    if (rejected) {
      setUploadError(rejected);
      toast.warning("Foto ditolak", rejected);
      return;
    }
    setUploadError(null);
    setUploading(true);
    try {
      const res = await productsService.uploadMedia(productId, file);
      toast.fromServer(res.message, "Foto diunggah");
      reload();
    } catch (err) {
      const message = toApiError(err).message;
      setUploadError(message);
      toast.danger("Gagal mengunggah foto", message);
    } finally {
      setUploading(false);
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

  if (invalidId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Produk tidak ditemukan"
        message={invalidId ? "ID produk tidak valid." : "Produk ini tidak ada atau sudah dihapus."}
        backTo={ROUTE_PATHS.products}
        backLabel="Kembali ke daftar produk"
      />
    );
  }

  if (loading && !product) return <DetailSkeleton />;

  if (fetchError || !product) {
    return (
      <Page>
        <ErrorState title="Gagal memuat produk" message={fetchError ?? "Produk tidak ditemukan."} onRetry={reload} />
      </Page>
    );
  }

  const archived = product.status === "archived";
  const attrEntries = Object.entries(product.attributes ?? {});

  const info: DescriptionItem[] = [
    { label: "Kode", value: <DocNumber value={product.code} copyable /> },
    { label: "Kategori", value: product.categoryName },
    { label: "Status", value: <StatusBadge status={product.status} /> },
    { label: "Tipe", value: productTypeLabel(product.type) },
    { label: "Stok dipantau", value: product.tracked ? "Ya" : "Tidak" },
    { label: "Stok minimum", value: <Qty value={product.minStock} unit={product.baseUom} /> },
    {
      label: "Satuan",
      wide: true,
      value: (
        <>
          Dasar: {product.baseUom} · Beli: {product.purchaseUom} (<Qty value={product.purchaseFactor} />) · Jual:{" "}
          {product.salesUom} (<Qty value={product.salesFactor} />)
        </>
      ),
    },
    ...(product.summary
      ? [{ label: "Ringkasan", wide: true, value: <span className="whitespace-pre-line">{product.summary}</span> }]
      : []),
  ];

  return (
    <Page>
      <PageHeader
        title={product.name}
        meta={<StatusBadge status={product.status} />}
        subtitle={
          <>
            <DocNumber value={product.code} /> · {productTypeLabel(product.type)}
            {product.categoryName ? ` · ${product.categoryName}` : ""}
          </>
        }
        actions={
          <>
            <Button variant="secondary" onClick={() => setQrOpen(true)}>
              <QrCode aria-hidden /> Label QR
            </Button>
            {canArchive && !archived && (
              <ActionMenu
                items={[
                  {
                    key: "archive",
                    label: "Arsipkan",
                    icon: <Archive aria-hidden />,
                    destructive: true,
                    onSelect: () => setArchiveOpen(true),
                  },
                ]}
              />
            )}
            {canUpdate && !archived && (
              <Button asChild>
                <Link to={`${ROUTE_PATHS.products}/${product.id}/edit`}>Ubah</Link>
              </Button>
            )}
          </>
        }
      />

      {archiveError && (
        <Notice tone="warning" title={archiveError.title}>
          {archiveError.message}
        </Notice>
      )}

      {archived && (
        <Notice tone="muted" title="Produk diarsipkan">
          Produk ini sudah diarsipkan — data dan foto dikunci, riwayat tidak berubah.
        </Notice>
      )}

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-5">
        <TabsList variant="line" className="w-full justify-start overflow-x-auto border-b border-border">
          <TabsTrigger value="ringkasan" className="flex-none px-3">
            Ringkasan
          </TabsTrigger>
          <TabsTrigger value="harga" className="flex-none px-3">
            Harga
          </TabsTrigger>
          <TabsTrigger value="varian" className="flex-none px-3">
            Varian
          </TabsTrigger>
          <TabsTrigger value="foto" className="flex-none px-3">
            Foto
          </TabsTrigger>
          {canStock && (
            <TabsTrigger value="stok" className="flex-none px-3">
              Stok
            </TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="ringkasan" className="flex flex-col gap-6">
          <SectionCard title="Informasi">
            <DescriptionList items={info} />
          </SectionCard>
          {attrEntries.length > 0 && (
            <SectionCard title="Info tambahan">
              <DescriptionList items={attrEntries.map(([k, v]) => ({ label: k, value: v }))} />
            </SectionCard>
          )}
        </TabsContent>

        <TabsContent value="harga">
          <SectionCard title="Harga" description="Rupiah penuh, tanpa desimal.">
            <DescriptionList
              items={[
                { label: "Harga beli", value: <Money value={product.purchasePrice} className="font-medium" /> },
                { label: "Harga jual", value: <Money value={product.sellingPrice} className="font-medium" /> },
                {
                  label: "Harga jual minimal",
                  value: <Money value={product.minSellingPrice} className="font-medium" />,
                },
              ]}
            />
          </SectionCard>
        </TabsContent>

        <TabsContent value="varian">
          <SectionCard
            title="Varian"
            description="Varian bawaan (tersembunyi) dipakai produk tanpa varian pilihan dan ikut kode produk."
            flush
          >
            <DataTable<ProductVariant>
              caption="Varian produk"
              columns={[
                { header: "Kode", render: (r) => <DocNumber value={r.code} /> },
                { header: "Nama", render: (r) => r.name || "–" },
                { header: "Barcode", render: (r) => (r.barcode ? <span className="font-mono">{r.barcode}</span> : "–") },
                {
                  header: "Utama",
                  render: (r) =>
                    r.isHidden ? (
                      <Badge tone="neutral">Bawaan</Badge>
                    ) : r.isDefault ? (
                      <Badge tone="info">Utama</Badge>
                    ) : (
                      <span className="text-muted-foreground">–</span>
                    ),
                },
                {
                  header: "Info",
                  render: (r) =>
                    Object.keys(r.attributes ?? {}).length > 0
                      ? Object.entries(r.attributes)
                          .map(([k, v]) => `${k}: ${v}`)
                          .join(", ")
                      : "–",
                },
                { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
              ]}
              rows={product.variants}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada varian"
            />
          </SectionCard>
        </TabsContent>

        <TabsContent value="foto" className="flex flex-col gap-6">
          <SectionCard
            title="Foto"
            description="Foto pertama yang diunggah menjadi foto utama; bila foto utama dihapus, foto lain otomatis menjadi utama. Format JPEG, PNG, atau WebP, maksimal 5 MB per berkas."
          >
            {media.length === 0 ? (
              <EmptyState
                title="Belum ada foto"
                description={
                  canUpdate && !archived
                    ? "Unggah foto pertama lewat kotak di bawah."
                    : "Produk ini belum punya foto."
                }
              />
            ) : (
              <ul className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                {media.map((m) => (
                  <li key={m.id} className="overflow-hidden rounded-lg border border-border bg-background">
                    <img src={m.url} alt={m.originalName} className="h-36 w-full object-cover" loading="lazy" />
                    <div className="space-y-2 p-3">
                      <div className="flex items-center justify-between gap-2">
                        <p className="truncate text-xs text-muted-foreground" title={m.originalName}>
                          {m.originalName}
                        </p>
                        {m.isPrimary && <Badge tone="success">Utama</Badge>}
                      </div>
                      {formatDimensions(m.width, m.height) && (
                        <p className="text-xs text-muted-foreground">
                          {formatDimensions(m.width, m.height)}
                          {m.originalSizeBytes ? ` · asli ${(m.originalSizeBytes / 1024).toFixed(0)} KB` : ""}
                        </p>
                      )}
                      {canUpdate && !archived && (
                        <div className="flex flex-wrap gap-2">
                          {!m.isPrimary && (
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={mediaBusyId === m.id}
                              onClick={() => void handleSetPrimary(m.id)}
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
          </SectionCard>

          {canUpdate && !archived && (
            <div className="space-y-2">
              <FileDropzone
                accept={MEDIA_ACCEPT_ATTR}
                busy={uploading}
                label="Seret foto ke sini atau pilih berkas"
                hint="JPEG, PNG, atau WebP, maksimal 5 MB"
                onFile={(f) => void handleUpload(f)}
              />
              {uploadError && (
                <p role="alert" className="text-sm font-medium text-destructive">
                  {uploadError}
                </p>
              )}
            </div>
          )}
        </TabsContent>

        {canStock && (
          <TabsContent value="stok">
            <SectionCard
              title="Stok"
              description="Saldo gabungan seluruh lokasi di cabang aktif."
              actions={
                product.tracked ? (
                  <Button variant="secondary" size="sm" asChild>
                    <Link to={stockItemPath(product.id)}>Rincian per lokasi</Link>
                  </Button>
                ) : undefined
              }
              flush
            >
              {!product.tracked ? (
                <EmptyState
                  title="Stok tidak dipantau"
                  description="Produk ini tidak dipantau stoknya, jadi tidak punya saldo."
                />
              ) : stockError ? (
                <ErrorState message={stockError} onRetry={reloadStock} className="m-4" />
              ) : stockLoading && balances === null ? (
                <TableSkeleton rows={3} cols={4} />
              ) : (
                <DataTable<StockRow>
                  caption="Saldo stok per varian"
                  rows={stockRows}
                  rowKey={(r) => r.variantId}
                  loading={stockLoading}
                  emptyTitle="Belum ada saldo stok"
                  emptyDescription="Saldo muncul setelah ada penerimaan, saldo awal, atau penyesuaian stok."
                  columns={[
                    { header: "Varian", render: (r) => r.name },
                    { header: "Di tangan", align: "right", render: (r) => <Qty value={r.onHand} unit={product.baseUom} /> },
                    { header: "Dipesan", align: "right", render: (r) => <Qty value={r.reserved} unit={product.baseUom} /> },
                    { header: "Tersedia", align: "right", render: (r) => <Qty value={r.available} unit={product.baseUom} /> },
                  ]}
                />
              )}
            </SectionCard>
          </TabsContent>
        )}
      </Tabs>

      <ConfirmDialog
        open={archiveOpen}
        title="Arsipkan produk?"
        message={`Produk ${product.code} — ${product.name} akan diarsipkan dan tidak bisa diubah lagi.`}
        confirmLabel={archiving ? "Mengarsipkan…" : "Arsipkan"}
        onConfirm={() => {
          if (!archiving) void handleArchive();
        }}
        onCancel={() => setArchiveOpen(false)}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Hapus foto?"
        message={`Foto ${deleteTarget?.originalName ?? ""} akan dihapus permanen.`}
        confirmLabel="Hapus"
        onConfirm={() => void handleDeleteMedia()}
        onCancel={() => setDeleteTarget(null)}
      />

      <QrModal productId={qrOpen ? product.id : null} open={qrOpen} onClose={() => setQrOpen(false)} />
    </Page>
  );
}
