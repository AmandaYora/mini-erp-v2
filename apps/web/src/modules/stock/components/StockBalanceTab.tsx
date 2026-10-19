import { useId, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  DataTable,
  FormField,
  Pagination,
  SearchSelect,
  SectionCard,
} from "@/shared/components/ui";
import { Qty, Toolbar } from "@/shared/components/patterns";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { stockItemPath } from "@/app/routes/route-paths";
import { stockService } from "@/modules/stock/services/stock.service";
import { ProductSelect } from "@/modules/stock/components/ProductSelect";
import { clampPage, paginate } from "@/modules/stock/lib/paginate";
import { toId } from "@/modules/stock/lib/ids";
import type { ProductDetail, StockBalance, StockLocation } from "@/modules/stock/types";

const LIMIT = 20;

/**
 * Tab Saldo: saldo satu produk per lokasi. Backend mewajibkan productId (tanpa itu balas []);
 * tidak ada param locationId, jadi lokasi disaring dan dipaginasi di klien.
 */
export function StockBalanceTab({ locations }: { locations: StockLocation[] }) {
  const uid = useId();
  const [productId, setProductId] = useState<number | null>(null);
  const [locationId, setLocationId] = useState<number | null>(null);
  const [page, setPage] = useState(1);

  const { data, loading, error, reload } = useAsyncData(async () => {
    if (productId === null) {
      return { balances: [] as StockBalance[], detail: null as ProductDetail | null };
    }
    const [balances, detail] = await Promise.all([
      stockService.balancesByProduct(productId),
      stockService.getProduct(productId).catch(() => null),
    ]);
    return { balances: balances ?? [], detail };
  }, [productId]);

  const locationMap = useMemo(() => new Map(locations.map((l) => [l.id, l])), [locations]);
  const filtered = useMemo(
    () =>
      (data?.balances ?? []).filter((b) => locationId === null || b.locationId === locationId),
    [data, locationId],
  );
  const current = clampPage(page, filtered.length, LIMIT);
  const rows = paginate(filtered, current, LIMIT);
  const detail = data?.detail ?? null;

  return (
    <div className="flex flex-col gap-4">
      <Toolbar
        primary={
          <>
            <div className="w-72 max-w-full">
              <FormField label="Produk" htmlFor={`${uid}-p`}>
                <ProductSelect
                  id={`${uid}-p`}
                  productId={productId}
                  onChange={(id) => {
                    setProductId(id);
                    setPage(1);
                  }}
                />
              </FormField>
            </div>
            <div className="w-56 max-w-full">
              <FormField label="Lokasi">
                <SearchSelect
                  options={locations.map((l) => ({ value: l.id, label: `${l.code} — ${l.name}` }))}
                  value={locationId}
                  placeholder="Semua lokasi"
                  allowClear
                  onChange={(v) => {
                    setLocationId(toId(v));
                    setPage(1);
                  }}
                />
              </FormField>
            </div>
          </>
        }
        right={
          productId !== null ? (
            <Button asChild variant="secondary" size="sm">
              <Link to={stockItemPath(productId)}>Buka detail produk</Link>
            </Button>
          ) : undefined
        }
      />
      <SectionCard flush>
        <DataTable<StockBalance>
          loading={loading}
          error={error}
          onRetry={reload}
          rows={rows}
          rowKey={(r) => `${r.productId}-${r.variantId}-${r.locationId}`}
          emptyTitle={productId === null ? "Pilih produk dulu" : "Belum ada saldo"}
          emptyDescription={
            productId === null
              ? "Saldo dimuat per produk. Cari produk di atas untuk melihat posisinya per lokasi."
              : locationId !== null
                ? "Tidak ada saldo di lokasi ini untuk produk tersebut."
                : "Belum ada stok tercatat untuk produk ini."
          }
          columns={[
            {
              header: "Produk",
              render: (r) => (
                <span className="font-medium">
                  {detail ? `${detail.code} — ${detail.name}` : `#${r.productId}`}
                </span>
              ),
            },
            {
              header: "Varian",
              render: (r) => {
                const v = detail?.variants.find((x) => x.id === r.variantId);
                return v ? `${v.code} — ${v.name}` : `#${r.variantId}`;
              },
            },
            {
              header: "Lokasi",
              render: (r) => {
                const l = locationMap.get(r.locationId);
                return l ? `${l.code} — ${l.name}` : `#${r.locationId}`;
              },
            },
            { header: "Fisik", align: "right", render: (r) => <Qty value={r.onHand} /> },
            { header: "Ditahan", align: "right", render: (r) => <Qty value={r.reserved} /> },
            { header: "Tersedia", align: "right", render: (r) => <Qty value={r.available} /> },
          ]}
        />
        {filtered.length > 0 && (
          <Pagination page={current} limit={LIMIT} total={filtered.length} onPageChange={setPage} />
        )}
      </SectionCard>
    </div>
  );
}
