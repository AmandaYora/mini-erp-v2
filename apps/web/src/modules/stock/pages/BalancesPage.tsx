import { useId, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Checkbox,
  DataTable,
  FormField,
  PageHeader,
  Pagination,
  SearchInput,
  SearchSelect,
  SectionCard,
} from "@/shared/components/ui";
import { Page, Qty, Toolbar } from "@/shared/components/patterns";
import { TextInput } from "@/shared/components/ui";
import type { ActiveFilter } from "@/shared/components/patterns";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { stockItemPath } from "@/app/routes/route-paths";
import { stockService } from "@/modules/stock/services/stock.service";
import { ProductSelect } from "@/modules/stock/components/ProductSelect";
import { VariantSelect } from "@/modules/stock/components/VariantSelect";
import { distinctProductIds, filterBalances } from "@/modules/stock/lib/balances";
import { dualUomLabel } from "@/modules/stock/lib/uom";
import type { UomInfo } from "@/modules/stock/lib/uom";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { toId } from "@/modules/stock/lib/ids";
import type { StockBalance, StockLocation } from "@/modules/stock/types";

// Mode daftar membawa productCode/productName dari server; mode per-produk tidak.
type BalanceRow = StockBalance & { productCode?: string; productName?: string };

const LIMIT = 25;

function ToggleFilter({
  checked,
  disabled,
  onChange,
  children,
}: {
  checked: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
  children: string;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
      <Checkbox checked={checked} disabled={disabled} onCheckedChange={(v) => onChange(v === true)} />
      {children}
    </label>
  );
}

// Saldo satu cabang. Tanpa produk: GET /stock/balances paginasi (filter
// locationId + negative=true di server). Dengan produk (picker -> productId):
// saldo produk itu saja, disaring di klien. "Sembunyikan nol" hanya menyaring
// baris pada halaman yang sedang dimuat (server tidak punya filternya).
export default function BalancesPage() {
  const uid = useId();
  const [productId, setProductId] = useState<number | null>(null);
  const [variantId, setVariantId] = useState<number | null>(null);
  const [locationId, setLocationId] = useState<number | null>(null);
  const [negativeOnly, setNegativeOnly] = useState(false);
  const [criticalOnly, setCriticalOnly] = useState(false);
  const [includeDamaged, setIncludeDamaged] = useState(false);
  const [hideZero, setHideZero] = useState(false);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [scanCode, setScanCode] = useState("");
  const [scanning, setScanning] = useState(false);

  // Pindai QR label (berisi barcode varian / kode) atau barcode pabrik: langsung menyaring saldo produk itu (ST-06).
  async function handleScan() {
    const code = scanCode.trim();
    if (!code || scanning) return;
    setScanning(true);
    try {
      const res = await stockService.scan(code);
      if (res.status === "inactive") {
        toast.warning("Produk tidak aktif", res.product.name);
      } else {
        setProductId(res.product.id);
        setVariantId(res.status === "variant_required" ? null : (res.variant?.id ?? null));
        setPage(1);
        if (res.status === "variant_required") toast.info("Pilih varian", `${res.product.name} punya beberapa varian.`);
      }
      setScanCode("");
    } catch (err) {
      toast.danger("Kode tidak dikenal", toApiError(err).message);
    } finally {
      setScanning(false);
    }
  }

  const { data: locData } = useAsyncData(
    () => stockService.listLocations().catch(() => [] as StockLocation[]),
    [],
  );
  const locations = useMemo(() => locData ?? [], [locData]);
  const locationMap = useMemo(() => new Map(locations.map((l) => [l.id, l] as const)), [locations]);

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      let rows: BalanceRow[];
      let total: number;
      if (productId !== null) {
        const all = await stockService.balancesByProduct(productId);
        rows = filterBalances(all ?? [], { locationId, negativeOnly }).filter(
          (b) => variantId === null || b.variantId === variantId,
        );
        total = rows.length;
      } else {
        const res = await stockService.listBalances({
          locationId,
          negative: negativeOnly,
          search,
          criticalOnly,
          includeDamaged,
          page,
          limit: LIMIT,
        });
        rows = res.items;
        total = res.meta.total;
      }
      const names = new Map<number, string>();
      const uoms = new Map<number, UomInfo>();
      await Promise.all(
        distinctProductIds(rows).map(async (id) => {
          const p = await stockService.getProduct(id).catch(() => null);
          if (p) {
            names.set(id, `${p.code} — ${p.name}`);
            if (p.baseUom) {
              uoms.set(id, {
                baseUom: p.baseUom,
                units: [
                  { uom: p.purchaseUom ?? "", factor: p.purchaseFactor ?? 1 },
                  { uom: p.salesUom ?? "", factor: p.salesFactor ?? 1 },
                ],
              });
            }
            for (const v of p.variants) {
              names.set(-v.id, v.name);
            }
          }
        }),
      );
      return { rows, total, names, uoms };
    },
    [productId, variantId, locationId, negativeOnly, criticalOnly, includeDamaged, search, page],
  );
  const names = data?.names;
  const uoms = data?.uoms;
  // Angka satuan dasar + label dual-satuan ("1 dus 5 pcs") bila produk punya satuan besar.
  const qtyCell = (productId: number, v: number) => {
    const label = dualUomLabel(v, uoms?.get(productId));
    return (
      <div>
        <Qty value={v} />
        {label ? <p className="text-xs text-muted-foreground">{label}</p> : null}
      </div>
    );
  };
  const total = data?.total ?? 0;
  const rows = useMemo(
    () => (data?.rows ?? []).filter((r) => !hideZero || r.onHand !== 0 || r.reserved !== 0),
    [data, hideZero],
  );

  const perProduct = productId !== null;
  const active: ActiveFilter[] = [];
  if (criticalOnly && !perProduct)
    active.push({ key: "critical", label: "Hanya stok kritis", onRemove: () => setCriticalOnly(false) });
  if (negativeOnly)
    active.push({ key: "negative", label: "Hanya saldo negatif", onRemove: () => setNegativeOnly(false) });
  if (includeDamaged && !perProduct)
    active.push({ key: "damaged", label: "Termasuk lokasi rusak", onRemove: () => setIncludeDamaged(false) });
  if (hideZero) active.push({ key: "zero", label: "Sembunyikan nol", onRemove: () => setHideZero(false) });
  const filtered = active.length > 0 || search !== "" || perProduct || locationId !== null;

  function resetAll() {
    setCriticalOnly(false);
    setNegativeOnly(false);
    setIncludeDamaged(false);
    setHideZero(false);
    setPage(1);
  }

  return (
    <Page>
      <PageHeader
        title="Saldo Stok Cabang"
        description="Seluruh saldo cabang aktif, atau satu produk lewat pemilih produk."
      />

      <Toolbar
        search={
          perProduct ? undefined : (
            <SearchInput
              aria-label="Cari produk"
              placeholder="Cari kode atau nama produk…"
              value={search}
              onSearch={(v) => {
                setSearch(v);
                setPage(1);
              }}
            />
          )
        }
        primary={
          <>
            <div className="w-56 max-w-full">
              <FormField label="Pindai QR / kode" htmlFor={`${uid}-scan`}>
                <TextInput
                  id={`${uid}-scan`}
                  value={scanCode}
                  disabled={scanning}
                  placeholder="Pindai lalu Enter"
                  onChange={(e) => setScanCode(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void handleScan();
                    }
                  }}
                />
              </FormField>
            </div>
            <div className="w-64 max-w-full">
              <FormField label="Produk" htmlFor={`${uid}-p`}>
                <ProductSelect
                  id={`${uid}-p`}
                  productId={productId}
                  placeholder="Semua produk"
                  onChange={(id) => {
                    setProductId(id);
                    setVariantId(null);
                    setPage(1);
                  }}
                />
              </FormField>
            </div>
            {perProduct && (
              <div className="w-56 max-w-full">
                <FormField label="Varian" htmlFor={`${uid}-v`}>
                  <VariantSelect
                    id={`${uid}-v`}
                    productId={productId}
                    variantId={variantId}
                    onChange={(id) => {
                      setVariantId(id);
                      setPage(1);
                    }}
                  />
                </FormField>
              </div>
            )}
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
        more={
          <>
            <ToggleFilter
              checked={criticalOnly}
              disabled={perProduct}
              onChange={(v) => {
                setCriticalOnly(v);
                setPage(1);
              }}
            >
              Hanya stok kritis
            </ToggleFilter>
            <ToggleFilter
              checked={negativeOnly}
              onChange={(v) => {
                setNegativeOnly(v);
                setPage(1);
              }}
            >
              Hanya saldo negatif
            </ToggleFilter>
            <ToggleFilter
              checked={includeDamaged}
              disabled={perProduct}
              onChange={(v) => {
                setIncludeDamaged(v);
                setPage(1);
              }}
            >
              Sertakan lokasi rusak
            </ToggleFilter>
            <ToggleFilter checked={hideZero} onChange={setHideZero}>
              Sembunyikan nol
            </ToggleFilter>
          </>
        }
        moreActiveCount={active.length}
        active={active}
        onClearAll={active.length > 1 ? resetAll : undefined}
      />

      <SectionCard flush>
        <DataTable<BalanceRow>
          loading={loading}
          error={error}
          onRetry={reload}
          rows={rows}
          rowKey={(r) => `${r.productId}/${r.variantId}/${r.locationId}`}
          emptyTitle={filtered ? "Tidak ada saldo yang cocok" : "Belum ada saldo"}
          emptyDescription={
            filtered
              ? "Tidak ada saldo yang cocok dengan filter ini. Ubah atau hapus filter."
              : "Saldo muncul setelah ada penerimaan, saldo awal, atau koreksi stok."
          }
          columns={[
            {
              header: "Produk",
              render: (r) => (
                <Link to={stockItemPath(r.productId)} className="font-medium text-primary hover:underline">
                  {r.productCode
                    ? `${r.productCode} — ${r.productName ?? ""}`
                    : (names?.get(r.productId) ?? `#${r.productId}`)}
                </Link>
              ),
            },
            { header: "Varian", render: (r) => names?.get(-r.variantId) ?? `#${r.variantId}` },
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
              render: (r) =>
                r.onHand < 0 ? (
                  <Badge tone="danger">
                    <Qty value={r.onHand} />
                  </Badge>
                ) : (
                  qtyCell(r.productId, r.onHand)
                ),
            },
            { header: "Ditahan", align: "right", render: (r) => <Qty value={r.reserved} /> },
            { header: "Tersedia", align: "right", render: (r) => qtyCell(r.productId, r.available) },
          ]}
        />
        {!perProduct && total > 0 && <Pagination page={page} limit={LIMIT} total={total} onPageChange={setPage} />}
      </SectionCard>
    </Page>
  );
}
