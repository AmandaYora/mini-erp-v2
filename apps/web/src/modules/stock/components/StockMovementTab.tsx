import { useId, useMemo, useState } from "react";
import {
  Badge,
  DataTable,
  DateRangeFields,
  FormField,
  Pagination,
  SearchSelect,
  SectionCard,
} from "@/shared/components/ui";
import { Qty, Toolbar } from "@/shared/components/patterns";
import { formatDateTime, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { stockService } from "@/modules/stock/services/stock.service";
import { ProductSelect } from "@/modules/stock/components/ProductSelect";
import { toId } from "@/modules/stock/lib/ids";
import type { ProductDetail, StockLocation, StockMovement } from "@/modules/stock/types";

const LIMIT = 20;

/** Tab Mutasi: aliran stok masuk/keluar cabang aktif (GET /stock/movements, paginasi server). */
export function StockMovementTab({ locations }: { locations: StockLocation[] }) {
  const uid = useId();
  const [dateFrom, setDateFrom] = useState(monthStartWIB());
  const [dateTo, setDateTo] = useState(todayWIB());
  const [productId, setProductId] = useState<number | null>(null);
  const [locationId, setLocationId] = useState<number | null>(null);
  const [type, setType] = useState("");
  const [page, setPage] = useState(1);

  const { data, loading, error, reload } = useAsyncData(async () => {
    const res = await stockService.listMovements({
      productId,
      locationId,
      type: type || undefined,
      dateFrom: dateFrom || undefined,
      dateTo: dateTo || undefined,
      page,
      limit: LIMIT,
    });
    // Nama produk/varian best-effort: mutasi hanya membawa ID.
    const ids = [...new Set(res.items.map((m) => m.productId))];
    const details = new Map<number, ProductDetail>();
    await Promise.all(
      ids.map(async (id) => {
        const d = await stockService.getProduct(id).catch(() => null);
        if (d) details.set(id, d);
      }),
    );
    return { items: res.items, total: res.meta.total, details };
  }, [productId, locationId, type, dateFrom, dateTo, page]);

  const locationMap = useMemo(() => new Map(locations.map((l) => [l.id, l])), [locations]);
  const details = data?.details;

  return (
    <div className="flex flex-col gap-4">
      <Toolbar
        primary={
          <>
            <DateRangeFields
              from={dateFrom}
              to={dateTo}
              onChange={(r) => {
                setDateFrom(r.from);
                setDateTo(r.to);
                setPage(1);
              }}
            />
            <div className="w-64 max-w-full">
              <FormField label="Produk" htmlFor={`${uid}-p`}>
                <ProductSelect
                  id={`${uid}-p`}
                  productId={productId}
                  placeholder="Semua produk"
                  onChange={(id) => {
                    setProductId(id);
                    setPage(1);
                  }}
                />
              </FormField>
            </div>
          </>
        }
        more={
          <>
          <FormField label="Tipe mutasi">
            <SearchSelect
              options={[
                { value: "in", label: "Masuk" },
                { value: "out", label: "Keluar" },
              ]}
              value={type || null}
              placeholder="Semua tipe"
              allowClear
              onChange={(v) => {
                setType(v === null ? "" : String(v));
                setPage(1);
              }}
            />
          </FormField>
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
          </>
        }
        moreActiveCount={(locationId !== null ? 1 : 0) + (type ? 1 : 0)}
        active={[
          ...(type
            ? [
                {
                  key: "type",
                  label: `Tipe: ${type === "in" ? "Masuk" : "Keluar"}`,
                  onRemove: () => {
                    setType("");
                    setPage(1);
                  },
                },
              ]
            : []),
          ...(locationId !== null
            ? [
                {
                  key: "location",
                  label: `Lokasi: ${locationMap.get(locationId)?.code ?? `#${locationId}`}`,
                  onRemove: () => {
                    setLocationId(null);
                    setPage(1);
                  },
                },
              ]
            : []),
        ]}
      />
      <SectionCard flush>
        <DataTable<StockMovement>
          loading={loading}
          error={error}
          onRetry={reload}
          rows={data?.items}
          rowKey={(r) => r.id}
          emptyTitle="Belum ada mutasi"
          emptyDescription="Tidak ada mutasi pada filter ini. Mutasi tercatat dari penerimaan, penjualan, koreksi, dan transfer."
          columns={[
            { header: "Tanggal", render: (r) => formatDateTime(r.createdAt) },
            {
              header: "Produk",
              render: (r) => {
                const d = details?.get(r.productId);
                const v = d?.variants.find((x) => x.id === r.variantId);
                return (
                  <div>
                    <div>{d ? `${d.code} — ${d.name}` : `#${r.productId}`}</div>
                    <div className="text-xs text-muted-foreground">
                      {v ? `${v.code} — ${v.name}` : `V#${r.variantId}`}
                    </div>
                  </div>
                );
              },
            },
            {
              header: "Lokasi",
              render: (r) => locationMap.get(r.locationId)?.code ?? `#${r.locationId}`,
            },
            {
              header: "Masuk",
              align: "right",
              render: (r) => (r.direction === "in" ? <Qty value={r.qty} /> : "–"),
            },
            {
              header: "Keluar",
              align: "right",
              render: (r) => (r.direction === "in" ? "–" : <Qty value={r.qty} />),
            },
            {
              header: "Saldo",
              align: "right",
              render: (r) =>
                r.balanceBefore === null || r.balanceAfter === null ? (
                  "–"
                ) : (
                  <span className="whitespace-nowrap">
                    <Qty value={r.balanceBefore} /> → <Qty value={r.balanceAfter} />
                  </span>
                ),
            },
            {
              header: "Tipe / referensi",
              render: (r) => (
                <div>
                  <Badge tone={r.direction === "in" ? "success" : "warning"}>{r.direction === "in" ? "Masuk" : "Keluar"}</Badge>
                  <div className="text-xs text-muted-foreground">{r.refType ? `${r.refType} #${r.refId}` : "–"}</div>
                </div>
              ),
            },
            { header: "Catatan", render: (r) => <span className="text-sm">{r.notes || "–"}</span> },
          ]}
        />
        {(data?.total ?? 0) > 0 && (
          <Pagination page={page} limit={LIMIT} total={data?.total ?? 0} onPageChange={setPage} />
        )}
      </SectionCard>
    </div>
  );
}
