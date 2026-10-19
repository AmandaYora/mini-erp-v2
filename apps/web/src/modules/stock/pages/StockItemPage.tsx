import { useCallback, useMemo } from "react";
import { useParams } from "react-router-dom";
import {
  DataTable,
  DetailSkeleton,
  ErrorState,
  PageHeader,
  SectionCard,
  SummaryCard,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import { Page, Qty, useUrlTab } from "@/shared/components/patterns";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatDateTime, formatNumber } from "@/shared/lib/format";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { stockService } from "@/modules/stock/services/stock.service";
import type { ProductDetail, StockBalance, StockLocation, StockMovement } from "@/modules/stock/types";

const TABS = ["lokasi", "kartu"] as const;

// Detail item stok (J5): posisi satu produk per lokasi + 10 mutasi terakhir.
// Data dari GET /stock/balances/{productId} (path-style) + movements yang
// sudah ada — tanpa logika baru di backend.
export default function StockItemPage() {
  const { productId } = useParams<{ productId: string }>();
  const id = Number(productId);
  const validId = Number.isInteger(id) && id > 0;
  const [tab, setTab] = useUrlTab(TABS, "lokasi");

  const { data, loading, error, errorStatus, reload } = useAsyncData(async () => {
    if (!validId) throw new Error("ID produk tidak valid.");
    const [detail, rows, locs, mov] = await Promise.all([
      stockService.getProduct(id).catch(() => null),
      stockService.balanceDetail(id),
      stockService.listLocations().catch(() => [] as StockLocation[]),
      stockService
        .listMovements({ productId: id, limit: 10 })
        .then((p) => p.items)
        .catch(() => [] as StockMovement[]),
    ]);
    return { product: detail, balances: rows ?? [], locations: locs ?? [], movements: mov ?? [] };
  }, [id]);
  const product: ProductDetail | null = data?.product ?? null;
  const balances: StockBalance[] = useMemo(() => data?.balances ?? [], [data]);
  const movements: StockMovement[] = useMemo(() => data?.movements ?? [], [data]);
  const locations: StockLocation[] = useMemo(() => data?.locations ?? [], [data]);
  usePageCrumb(product?.code);

  const locationMap = useMemo(() => new Map(locations.map((l) => [l.id, l] as const)), [locations]);
  const variantName = useCallback(
    (variantId: number) => product?.variants.find((v) => v.id === variantId)?.name ?? `#${variantId}`,
    [product],
  );

  const totals = useMemo(
    () =>
      balances.reduce(
        (s, b) => ({
          onHand: s.onHand + b.onHand,
          reserved: s.reserved + b.reserved,
          available: s.available + b.available,
        }),
        { onHand: 0, reserved: 0, available: 0 },
      ),
    [balances],
  );

  if (!validId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Produk tidak ditemukan"
        message="Produk ini tidak ada atau belum punya data stok."
        backTo={ROUTE_PATHS.stockBalances}
        backLabel="Ke saldo stok"
      />
    );
  }
  if (loading && !data) return <DetailSkeleton />;
  if (error && !data) return <ErrorState message={error} onRetry={reload} title="Gagal memuat detail stok" />;

  return (
    <Page>
      <PageHeader
        title={product ? product.name : `Item #${productId}`}
        subtitle={product ? <span className="font-mono">{product.code}</span> : undefined}
        description="Posisi per lokasi dan mutasi terakhir."
      />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <SummaryCard label="Fisik" value={formatNumber(totals.onHand)} />
        <SummaryCard label="Ditahan" value={formatNumber(totals.reserved)} tone="warning" />
        <SummaryCard label="Tersedia" value={formatNumber(totals.available)} tone="success" />
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-4">
        <TabsList variant="line" aria-label="Bagian detail stok">
          <TabsTrigger value="lokasi">Per lokasi</TabsTrigger>
          <TabsTrigger value="kartu">Kartu stok</TabsTrigger>
        </TabsList>
        <TabsContent value="lokasi">
          <SectionCard flush>
            <DataTable<StockBalance>
              rows={balances}
              rowKey={(r) => `${r.variantId}/${r.locationId}`}
              emptyTitle="Tidak ada saldo"
              emptyDescription="Produk ini belum punya saldo di cabang aktif."
              columns={[
                { header: "Varian", render: (r) => variantName(r.variantId) },
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
          </SectionCard>
        </TabsContent>
        <TabsContent value="kartu">
          <SectionCard description="10 pergerakan terbaru item ini." flush>
            <DataTable<StockMovement>
              rows={movements}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada mutasi"
              columns={[
                { header: "Waktu", render: (r) => formatDateTime(r.createdAt) },
                { header: "Jenis", render: (r) => r.type },
                {
                  header: "Jumlah",
                  align: "right",
                  render: (r) => (
                    <span className="tabular">
                      {r.direction === "out" ? "−" : "+"}
                      <Qty value={r.qty} />
                    </span>
                  ),
                },
                { header: "Catatan", render: (r) => r.notes || "–" },
              ]}
            />
          </SectionCard>
        </TabsContent>
      </Tabs>
    </Page>
  );
}
