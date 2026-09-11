import { useCallback, useMemo } from "react";
import { useParams } from "react-router-dom";
import {
  DataTable,
  PageHeader,
  SectionCard,
  SummaryCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDateTime, formatNumber } from "@/shared/lib/format";
import { stockService } from "@/modules/stock/services/stock.service";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import type {
  ProductDetail,
  StockBalance,
  StockLocation,
  StockMovement,
} from "@/modules/stock/types";

// Detail item stok (J5): posisi satu produk per lokasi + 10 mutasi terakhir.
// Data dari GET /stock/balances/{productId} (path-style) + movements yang
// sudah ada — tanpa logika baru di backend.
export default function StockItemPage() {
  const { productId } = useParams<{ productId: string }>();
  const id = Number(productId);

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      if (!Number.isInteger(id) || id <= 0) {
        throw new Error("ID produk tidak valid.");
      }
      const [detail, rows, locs, mov] = await Promise.all([
        stockService.getProduct(id).catch(() => null),
        stockService.balanceDetail(id),
        stockService.listLocations().catch(() => [] as StockLocation[]),
        stockService
          .listMovements({ productId: id, limit: 10 })
          .then((p) => p.items)
          .catch(() => [] as StockMovement[]),
      ]);
      return {
        product: detail,
        balances: rows ?? [],
        locations: locs ?? [],
        movements: mov ?? [],
      };
    },
    [id],
  );
  const product: ProductDetail | null = data?.product ?? null;
  const balances: StockBalance[] = useMemo(() => data?.balances ?? [], [data]);
  const movements: StockMovement[] = useMemo(() => data?.movements ?? [], [data]);
  const locations: StockLocation[] = useMemo(() => data?.locations ?? [], [data]);

  const locationMap = useMemo(
    () => new Map(locations.map((l) => [l.id, l] as const)),
    [locations],
  );
  const variantName = useCallback(
    (variantId: number) =>
      product?.variants.find((v) => v.id === variantId)?.name ??
      `#${variantId}`,
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

  return (
    <div>
      <PageHeader
        eyebrow="Stok & Gudang"
        title={product ? `${product.code} — ${product.name}` : `Item #${productId}`}
        description="Posisi per lokasi dan mutasi terakhir."
      />

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat detail stok">
            {error}{" "}
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      {loading ? (
        <p className="text-muted px-1 py-6 text-sm">Memuat detail stok…</p>
      ) : (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <SummaryCard label="Fisik" value={formatNumber(totals.onHand)} />
            <SummaryCard label="Ditahan" value={formatNumber(totals.reserved)} />
            <SummaryCard
              label="Tersedia"
              value={formatNumber(totals.available)}
              tone="success"
            />
          </div>

          <SectionCard
            title="Saldo per Lokasi"
            description="Satu baris per varian per lokasi di cabang aktif."
          >
            <DataTable<StockBalance>
              rows={balances}
              rowKey={(r) => `${r.variantId}/${r.locationId}`}
              emptyTitle="Tidak ada saldo"
              emptyDescription="Produk ini belum punya saldo di cabang aktif."
              columns={[
                {
                  header: "Varian",
                  render: (r) => variantName(r.variantId),
                },
                {
                  header: "Lokasi",
                  render: (r) => {
                    const l = locationMap.get(r.locationId);
                    return l ? `${l.code} — ${l.name}` : `#${r.locationId}`;
                  },
                },
                {
                  header: "Fisik",
                  align: "right",
                  render: (r) => formatNumber(r.onHand),
                },
                {
                  header: "Ditahan",
                  align: "right",
                  render: (r) => formatNumber(r.reserved),
                },
                {
                  header: "Tersedia",
                  align: "right",
                  render: (r) => formatNumber(r.available),
                },
              ]}
            />
          </SectionCard>

          <SectionCard
            title="Mutasi Terakhir"
            description="10 pergerakan terbaru item ini."
          >
            <DataTable<StockMovement>
              rows={movements}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada mutasi"
              columns={[
                {
                  header: "Waktu",
                  render: (r) => formatDateTime(r.createdAt),
                },
                {
                  header: "Jenis",
                  render: (r) => r.type,
                },
                {
                  header: "Qty",
                  align: "right",
                  render: (r) =>
                    `${r.direction === "out" ? "−" : "+"}${formatNumber(r.qty)}`,
                },
                {
                  header: "Catatan",
                  render: (r) => r.notes || "-",
                },
              ]}
            />
          </SectionCard>
        </div>
      )}
    </div>
  );
}
