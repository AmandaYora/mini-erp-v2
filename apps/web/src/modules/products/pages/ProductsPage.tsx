import { useState } from "react";
import type { MouseEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Download, ImageOff, LayoutGrid, List, QrCode, ScanLine, SquarePen } from "lucide-react";
import {
  Button,
  DataTable,
  FormField,
  PageHeader,
  Pagination,
  SearchInput,
  SearchSelect,
  SectionCard,
  SelectInput,
  StatusBadge,
} from "@/shared/components/ui";
import { ActionMenu, ArchiveViewToggle, DocNumber, Money, Page, SavedViews, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { TableSkeleton } from "@/shared/components/feedback/skeleton";
import { formatDate } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { toast } from "@/shared/stores/toast.store";
import { productsService } from "@/modules/products/services/products.service";
import QrModal from "@/modules/products/components/QrModal";
import { ScanDialog } from "@/modules/pos/components/ScanDialog";
import type { ScanOutcome } from "@/modules/pos/lib/scan-session";
import { minStockText, stockCell } from "@/modules/products/lib/product-list-view";
import { PRODUCT_TYPE_OPTIONS, productTypeLabel } from "@/modules/products/lib/product-form";
import {
  PRODUCT_LIST_DEFAULTS,
  productListFilterSchema,
  productStatusParam,
} from "@/modules/products/schemas/product-list-filter.schema";
import type { ProductListFilter } from "@/modules/products/schemas/product-list-filter.schema";
import type { Product, ProductCategory } from "@/modules/products/types";

const LIMIT = 20;

const STATUS_LABEL: Record<string, string> = {
  active: "Aktif",
  inactive: "Nonaktif",
  archived: "Diarsipkan",
  all: "Semua status",
};
const TRACKED_LABEL: Record<string, string> = { true: "Terpantau", false: "Tidak terpantau" };

/** Klik di dalam menu aksi (portal) tidak boleh ikut memicu klik baris. */
function stopRowClick(e: MouseEvent) {
  e.stopPropagation();
}

function Thumb({ product, size }: { product: Product; size: "sm" | "lg" }) {
  const box = size === "sm" ? "size-8 rounded-md" : "aspect-square w-full";
  if (product.primaryImageUrl) {
    return (
      <img
        src={product.primaryImageUrl}
        alt={`Foto ${product.name}`}
        className={`${box} border border-border object-cover`}
        loading="lazy"
      />
    );
  }
  return (
    <span
      role="img"
      aria-label="Tanpa foto"
      className={`${box} flex items-center justify-center border border-border bg-muted text-muted-foreground`}
    >
      <ImageOff size={size === "sm" ? 14 : 28} aria-hidden />
    </span>
  );
}

export default function ProductsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("products.create");
  const canUpdate = can("products.update");
  // Kolom stok & harga beli hanya untuk yang berhak; server juga tidak mengirim datanya tanpa izin.
  const showStock = can("stock.view");
  const showCost = can("products.update") || can("purchasing.view");

  const { filters, setFilters } = useUrlFilters(productListFilterSchema, PRODUCT_LIST_DEFAULTS);
  const [qrProductId, setQrProductId] = useState<number | null>(null);
  const [qrZipping, setQrZipping] = useState(false);
  const [scanOpen, setScanOpen] = useState(false);

  // Filter kategori opsional — gagal muat tak menghalangi daftar produk.
  const { data: categoryData } = useAsyncData(
    () =>
      productsService.listCategories().then(
        (cats) => cats,
        () => [] as ProductCategory[],
      ),
    [],
  );
  const categories = categoryData ?? [];

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      productsService
        .list(
          {
            search: filters.search,
            categoryId: filters.categoryId || null,
            status: productStatusParam(filters.status),
            type: filters.type,
            tracked: filters.tracked,
            page: filters.page,
            limit: LIMIT,
          },
          signal,
        )
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters.search, filters.categoryId, filters.status, filters.type, filters.tracked, filters.page],
  );
  const items = data?.items ?? null;
  const total = data?.total ?? 0;

  async function handleQrZip() {
    setQrZipping(true);
    try {
      await productsService.downloadQrZip();
    } catch (err) {
      toast.danger("Gagal mengunduh QR", toApiError(err).message);
    } finally {
      setQrZipping(false);
    }
  }

  /** Pindai satu kali: kode dikenal -> buka detail produk (dialog ikut tertutup). */
  async function handleScan(code: string): Promise<ScanOutcome> {
    try {
      const hit = await productsService.scanLookup(code);
      setScanOpen(false);
      navigate(`${ROUTE_PATHS.products}/${hit.id}`);
      return { kind: "added", name: `${hit.code} — ${hit.name}`, qty: 1 };
    } catch (err) {
      const apiErr = toApiError(err);
      return apiErr.status === 404
        ? { kind: "unknown", code, detail: `Kode "${code}" tidak ditemukan di katalog produk.` }
        : { kind: "rejected", detail: apiErr.message };
    }
  }

  const category = categories.find((c) => c.id === filters.categoryId);
  const active: ActiveFilter[] = [];
  if (filters.categoryId)
    active.push({
      key: "category",
      label: `Kategori: ${category ? category.name : `#${filters.categoryId}`}`,
      onRemove: () => setFilters({ categoryId: 0 }),
    });
  if (filters.status !== PRODUCT_LIST_DEFAULTS.status)
    active.push({
      key: "status",
      label: `Status: ${STATUS_LABEL[filters.status] ?? filters.status}`,
      onRemove: () => setFilters({ status: PRODUCT_LIST_DEFAULTS.status }),
    });
  if (filters.type)
    active.push({
      key: "type",
      label: `Tipe: ${productTypeLabel(filters.type)}`,
      onRemove: () => setFilters({ type: "" }),
    });
  if (filters.tracked)
    active.push({
      key: "tracked",
      label: `Stok: ${TRACKED_LABEL[filters.tracked] ?? filters.tracked}`,
      onRemove: () => setFilters({ tracked: "" }),
    });
  const filtered = active.length > 0 || filters.search !== "";
  const resetFilters = () => setFilters({ ...PRODUCT_LIST_DEFAULTS, view: filters.view });

  const emptyTitle = filtered ? "Tidak ada produk yang cocok dengan filter ini" : "Belum ada produk";
  const emptyDescription = filtered
    ? "Ubah kata kunci atau hapus filter untuk melihat produk lain."
    : "Tambah produk baru atau impor dari Excel.";
  const emptyAction = filtered ? (
    <Button variant="secondary" onClick={resetFilters}>
      Hapus filter
    </Button>
  ) : canCreate ? (
    <Button asChild>
      <Link to={`${ROUTE_PATHS.products}/new`}>Tambah produk</Link>
    </Button>
  ) : undefined;

  const gallery = filters.view === "gallery";

  return (
    <Page>
      <PageHeader
        title="Produk"
        description="Katalog barang dan jasa beserta harga jual."
        actions={
          <>
            <Button variant="secondary" onClick={() => setScanOpen(true)}>
              <ScanLine aria-hidden /> Pindai
            </Button>
            <Button variant="secondary" loading={qrZipping} onClick={() => void handleQrZip()}>
              <Download aria-hidden /> Unduh QR (ZIP)
            </Button>
            {canCreate && (
              <Button asChild>
                <Link to={`${ROUTE_PATHS.products}/new`}>Tambah produk</Link>
              </Button>
            )}
          </>
        }
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari produk"
            placeholder="Cari kode atau nama…"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
          />
        }
        primary={
          <>
            <div className="min-w-48">
              <FormField label="Kategori">
                <SearchSelect
                  options={categories.map((c) => ({ value: c.id, label: `${c.code} — ${c.name}` }))}
                  value={filters.categoryId || null}
                  onChange={(v) => setFilters({ categoryId: typeof v === "number" ? v : 0 })}
                  placeholder="Semua kategori"
                  allowClear
                />
              </FormField>
            </div>
            <div className="min-w-36">
              <FormField label="Status" htmlFor="product-status">
                <SelectInput
                  id="product-status"
                  value={filters.status}
                  onChange={(e) => setFilters({ status: e.target.value as ProductListFilter["status"] })}
                >
                  <option value="active">Aktif</option>
                  <option value="inactive">Nonaktif</option>
                  <option value="archived">Diarsipkan</option>
                  <option value="all">Semua status</option>
                </SelectInput>
              </FormField>
            </div>
            <div className="min-w-32">
              <FormField label="Tipe" htmlFor="product-type">
                <SelectInput
                  id="product-type"
                  value={filters.type}
                  onChange={(e) => setFilters({ type: e.target.value })}
                >
                  <option value="">Semua</option>
                  {PRODUCT_TYPE_OPTIONS.map((o) => (
                    <option key={o.value} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </SelectInput>
              </FormField>
            </div>
          </>
        }
        more={
          <FormField label="Pantau stok" htmlFor="product-tracked">
            <SelectInput
              id="product-tracked"
              value={filters.tracked}
              onChange={(e) => setFilters({ tracked: e.target.value })}
            >
              <option value="">Semua</option>
              <option value="true">Terpantau</option>
              <option value="false">Tidak terpantau</option>
            </SelectInput>
          </FormField>
        }
        moreActiveCount={filters.tracked ? 1 : 0}
        active={active}
        onClearAll={resetFilters}
        right={
          <>
            <ArchiveViewToggle
              showingArchive={filters.status === "archived"}
              onChange={(show) => setFilters({ status: show ? "archived" : "active" })}
            />
          <div role="group" aria-label="Tampilan daftar" className="flex items-center gap-1">
            <Button
              variant={gallery ? "ghost" : "secondary"}
              size="sm"
              aria-pressed={!gallery}
              onClick={() => setFilters({ view: "table", page: filters.page })}
            >
              <List aria-hidden /> Tabel
            </Button>
            <Button
              variant={gallery ? "secondary" : "ghost"}
              size="sm"
              aria-pressed={gallery}
              onClick={() => setFilters({ view: "gallery", page: filters.page })}
            >
              <LayoutGrid aria-hidden /> Galeri
            </Button>
          </div>
            <SavedViews listKey="products" columnsKey="products" />
          </>
        }
      />

      <SectionCard flush>
        {gallery ? (
          error ? (
            <ErrorState message={error} onRetry={reload} className="m-4" />
          ) : items === null ? (
            <TableSkeleton />
          ) : items.length === 0 ? (
            <EmptyState title={emptyTitle} description={emptyDescription} action={emptyAction} />
          ) : (
            <ul
              aria-busy={loading || undefined}
              className={`grid grid-cols-2 gap-4 p-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 ${loading ? "opacity-60" : ""}`}
            >
              {items.map((p) => (
                <li
                  key={p.id}
                  className="overflow-hidden rounded-lg border border-border bg-card transition-colors hover:bg-background"
                >
                  <Link to={`${ROUTE_PATHS.products}/${p.id}`} className="block hover:no-underline">
                    <Thumb product={p} size="lg" />
                    <div className="space-y-1 p-3">
                      <p className="truncate text-sm font-medium text-foreground" title={p.name}>
                        {p.name}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        <DocNumber value={p.code} />
                      </p>
                      <div className="flex items-center justify-between gap-2 pt-1">
                        <Money value={p.sellingPrice} className="text-sm font-medium text-foreground" />
                        <StatusBadge status={p.status} />
                      </div>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )
        ) : (
          <DataTable<Product>
            columnsKey="products"
            rows={items}
            rowKey={(r) => r.id}
            loading={loading}
            error={error}
            onRetry={reload}
            caption="Daftar produk"
            onRowClick={(r) => navigate(`${ROUTE_PATHS.products}/${r.id}`)}
            emptyTitle={emptyTitle}
            emptyDescription={emptyDescription}
            emptyAction={emptyAction}
            columns={[
              { header: "Foto", width: 56, render: (r) => <Thumb product={r} size="sm" /> },
              { header: "Kode", render: (r) => <DocNumber value={r.code} /> },
              {
                header: "Nama",
                render: (r) => (
                  <Link to={`${ROUTE_PATHS.products}/${r.id}`} className="font-medium text-primary hover:underline">
                    {r.name}
                  </Link>
                ),
              },
              {
                header: "Tipe",
                render: (r) => (
                  <>
                    {productTypeLabel(r.type)}
                    {r.type === "barang" && !r.tracked && (
                      <span className="block text-xs text-muted-foreground">Stok tidak dipantau</span>
                    )}
                  </>
                ),
              },
              { header: "Kategori", render: (r) => r.categoryName || "–" },
              ...(showCost
                ? [
                    {
                      header: "Harga beli",
                      align: "right" as const,
                      render: (r: Product) =>
                        r.purchasePrice === undefined ? "–" : <Money value={r.purchasePrice} />,
                    },
                  ]
                : []),
              { header: "Harga jual", align: "right", render: (r) => <Money value={r.sellingPrice} /> },
              ...(showStock
                ? [
                    {
                      header: "Stok",
                      align: "right" as const,
                      render: (r: Product) => {
                        const cell = stockCell(r);
                        if (!cell.tracked || cell.text === "")
                          return <span className="text-muted-foreground">–</span>;
                        const min = minStockText(r);
                        return (
                          <span className="block">
                            <span className={cell.below ? "font-medium text-destructive" : undefined}>{cell.text}</span>
                            {min && (
                              <span className={`block text-xs ${cell.below ? "text-destructive" : "text-muted-foreground"}`}>
                                {min}
                              </span>
                            )}
                          </span>
                        );
                      },
                    },
                  ]
                : []),
              { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
              // Tanggal arsip hanya di tampilan yang memuat arsip.
              ...(filters.status === "archived" || filters.status === "all"
                ? [
                    {
                      header: "Diarsipkan pada",
                      render: (r: Product) => (r.archivedAt ? formatDate(r.archivedAt) : "–"),
                    },
                  ]
                : []),
              {
                header: "Aksi",
                id: "aksi",
                width: 56,
                align: "right",
                render: (r) => (
                  <span onClick={stopRowClick} className="inline-flex">
                    <ActionMenu
                      variant="ghost"
                      label={`Aksi untuk ${r.name}`}
                      items={[
                        { key: "qr", label: "Label QR", icon: <QrCode aria-hidden />, onSelect: () => setQrProductId(r.id) },
                        ...(canUpdate && r.status !== "archived"
                          ? [
                              {
                                key: "edit",
                                label: "Ubah",
                                icon: <SquarePen aria-hidden />,
                                onSelect: () => navigate(`${ROUTE_PATHS.products}/${r.id}/edit`),
                              },
                            ]
                          : []),
                      ]}
                    />
                  </span>
                ),
              },
            ]}
          />
        )}
        <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
      </SectionCard>

      <ScanDialog open={scanOpen} paused={false} onClose={() => setScanOpen(false)} onScan={handleScan} />
      <QrModal productId={qrProductId} open={qrProductId !== null} onClose={() => setQrProductId(null)} />
    </Page>
  );
}
