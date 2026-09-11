import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  FilterBar,
  PageHeader,
  Pagination,
  SearchSelect,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR, statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { toast } from "@/shared/stores/toast.store";
import { productsService } from "@/modules/products/services/products.service";
import QrModal from "@/modules/products/components/QrModal";
import type { Product, ProductCategory } from "@/modules/products/types";

const LIMIT = 20;

export default function ProductsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const [search, setSearch] = useState("");
  const [appliedSearch, setAppliedSearch] = useState("");
  const [categoryId, setCategoryId] = useState<number | null>(null);
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [qrProductId, setQrProductId] = useState<number | null>(null);
  const [qrZipping, setQrZipping] = useState(false);

  // Filter kategori opsional — gagal muat tak menghalangi daftar produk.
  const { data: categoryData } = useAsyncData(
    () => productsService.listCategories().then(
      (cats) => cats,
      () => [] as ProductCategory[],
    ),
    [],
  );
  const categories = categoryData ?? [];

  const { data, loading, error } = useAsyncData(
    () =>
      productsService
        .list({
          search: appliedSearch,
          categoryId,
          status,
          page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [appliedSearch, categoryId, status, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  function applySearch() {
    setPage(1);
    setAppliedSearch(search);
  }

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

  return (
    <div>
      <PageHeader
        eyebrow="Master"
        title="Produk"
        description="Katalog barang dan jasa beserta harga jual."
        actions={
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              disabled={qrZipping}
              onClick={() => void handleQrZip()}
            >
              {qrZipping ? "Menyiapkan…" : "Unduh QR (ZIP)"}
            </Button>
            {can("products.create") ? (
              <Button onClick={() => navigate("/products/new")}>Tambah</Button>
            ) : undefined}
          </div>
        }
      />

      <FilterBar>
        <div className="min-w-[220px] flex-1">
          <label htmlFor="product-search" className="text-heading mb-1.5 block text-[0.85rem] font-medium">
            Cari
          </label>
          <TextInput
            id="product-search"
            placeholder="Kode atau nama…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") applySearch();
            }}
          />
        </div>
        <div className="min-w-[200px]">
          <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">Kategori</span>
          <SearchSelect
            options={categories.map((c) => ({ value: c.id, label: `${c.code} — ${c.name}` }))}
            value={categoryId}
            onChange={(v) => {
              setPage(1);
              setCategoryId(typeof v === "number" ? v : null);
            }}
            placeholder="Semua kategori"
            allowClear
          />
        </div>
        <div className="min-w-[160px]">
          <label htmlFor="product-status" className="text-heading mb-1.5 block text-[0.85rem] font-medium">
            Status
          </label>
          <SelectInput
            id="product-status"
            value={status}
            onChange={(e) => {
              setPage(1);
              setStatus(e.target.value);
            }}
          >
            <option value="">Semua</option>
            <option value="active">Aktif</option>
            <option value="archived">Diarsipkan</option>
          </SelectInput>
        </div>
        <Button variant="secondary" onClick={applySearch}>
          Cari
        </Button>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat produk">
            {error}
          </Notice>
        </div>
      )}

      <div className="rounded-lg border border-hairline bg-surface">
        {loading ? (
          <p className="text-muted px-6 py-10 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<Product>
              columns={[
                { header: "Kode", render: (r) => <span className="font-medium">{r.code}</span> },
                {
                  header: "Nama",
                  render: (r) => (
                    <Link to={`/products/${r.id}`} className="font-medium text-brand hover:underline">
                      {r.name}
                    </Link>
                  ),
                },
                { header: "Kategori", render: (r) => r.categoryName || "-" },
                {
                  header: "Harga Jual",
                  align: "right",
                  render: (r) => formatIDR(r.sellingPrice),
                },
                {
                  header: "Status",
                  render: (r) => <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge>,
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <span className="flex justify-end gap-1">
                      <Button variant="ghost" size="sm" onClick={() => setQrProductId(r.id)}>
                        QR
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => navigate(`/products/${r.id}`)}>
                        Lihat
                      </Button>
                    </span>
                  ),
                },
              ]}
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada produk"
              emptyDescription="Tambah produk baru atau impor dari Excel."
            />
            <Pagination page={page} limit={LIMIT} total={total} onPageChange={setPage} />
          </>
        )}
      </div>

      <QrModal
        productId={qrProductId}
        open={qrProductId !== null}
        onClose={() => setQrProductId(null)}
      />
    </div>
  );
}
