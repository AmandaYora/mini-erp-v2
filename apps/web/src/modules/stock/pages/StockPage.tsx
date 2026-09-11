import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FilterBar,
  FormField,
  Modal,
  PageHeader,
  Pagination,
  SearchSelect,
  SectionCard,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDateTime,
  formatNumber,
  monthStartWIB,
  statusLabel,
  statusTone,
  todayWIB,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import {
  locationCreateSchema,
  locationUpdateSchema,
} from "@/modules/stock/schemas/stock.schema";
import type {
  ProductDetail,
  ProductOption,
  StockBalance,
  StockLocation,
  StockMovement,
} from "@/modules/stock/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

const MOV_LIMIT = 20;

type LocationMode = { kind: "create" } | { kind: "edit"; location: StockLocation };

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

export default function StockPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.manage");

  const [productOptions, setProductOptions] = useState<ProductOption[]>([]);

  const {
    data: locationsData,
    loading: locLoading,
    reload: reloadLocations,
  } = useAsyncData(() => stockService.listLocations().catch((err) => {
    toast.danger("Gagal memuat lokasi", toApiError(err).message);
    throw err;
  }), []);
  const locations: StockLocation[] = useMemo(() => locationsData ?? [], [locationsData]);

  // ---- Saldo ----
  const [saldoProduct, setSaldoProduct] = useState<number | null>(null);
  const [saldoLocation, setSaldoLocation] = useState<number | null>(null);

  const {
    data: saldoData,
    loading: saldoLoading,
    error: saldoError,
  } = useAsyncData(
    async () => {
      if (saldoProduct === null) {
        return { balances: [] as StockBalance[], detail: null as ProductDetail | null };
      }
      const [rows, detail] = await Promise.all([
        stockService.balancesByProduct(saldoProduct),
        stockService.getProduct(saldoProduct).catch(() => null),
      ]);
      return { balances: rows, detail };
    },
    [saldoProduct],
  );
  const balances: StockBalance[] = useMemo(() => saldoData?.balances ?? [], [saldoData]);
  const saldoDetail: ProductDetail | null = saldoData?.detail ?? null;

  // ---- Riwayat mutasi ----
  const [dateFrom, setDateFrom] = useState(monthStartWIB());
  const [dateTo, setDateTo] = useState(todayWIB());
  const [movProduct, setMovProduct] = useState<number | null>(null);
  const [movVariant, setMovVariant] = useState<number | null>(null);
  const [movLocation, setMovLocation] = useState<number | null>(null);
  const [movPage, setMovPage] = useState(1);

  const {
    data: movData,
    loading: movLoading,
    error: movError,
  } = useAsyncData(
    () =>
      stockService
        .listMovements({
          productId: movProduct,
          variantId: movVariant,
          locationId: movLocation,
          dateFrom: dateFrom || undefined,
          dateTo: dateTo || undefined,
          page: movPage,
          limit: MOV_LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [movProduct, movVariant, movLocation, dateFrom, dateTo, movPage],
  );
  const movements: StockMovement[] = movData?.items ?? [];
  const movTotal = movData?.total ?? 0;

  // ---- Lokasi ----
  const [locMode, setLocMode] = useState<LocationMode | null>(null);
  const [locCode, setLocCode] = useState("");
  const [locName, setLocName] = useState("");
  const [locParent, setLocParent] = useState<number | null>(null);
  const [locErrors, setLocErrors] = useState<Record<string, string>>({});
  const [locSaving, setLocSaving] = useState(false);
  const [archiveTarget, setArchiveTarget] = useState<StockLocation | null>(null);

  const locationMap = useMemo(
    () => new Map(locations.map((l) => [l.id, l])),
    [locations],
  );
  const productLabel = useMemo(
    () => new Map(productOptions.map((p) => [p.id, `${p.code} — ${p.name}`])),
    [productOptions],
  );
  const variantMap = useMemo(
    () =>
      new Map(
        (saldoDetail?.variants ?? []).map((v) => [v.id, `${v.code} — ${v.name}`]),
      ),
    [saldoDetail],
  );

  useEffect(() => {
    let alive = true;
    stockService
      .searchProductOptions("")
      .then((rows) => {
        if (alive) setProductOptions(rows);
      })
      .catch((err) => {
        if (alive)
          toast.danger("Gagal memuat opsi produk", toApiError(err).message);
      });
    return () => {
      alive = false;
    };
  }, []);

  // Saldo: backend mewajibkan productId (tanpa itu balas []).
  // Tidak ada param locationId di backend → saring lokasi sisi klien.

  const visibleBalances = useMemo(
    () =>
      saldoLocation === null
        ? balances
        : balances.filter((b) => b.locationId === saldoLocation),
    [balances, saldoLocation],
  );

  function openCreateLocation() {
    setLocCode("");
    setLocName("");
    setLocParent(null);
    setLocErrors({});
    setLocMode({ kind: "create" });
  }

  function openEditLocation(loc: StockLocation) {
    setLocCode(loc.code);
    setLocName(loc.name);
    setLocParent(loc.parentId);
    setLocErrors({});
    setLocMode({ kind: "edit", location: loc });
  }

  async function handleLocationSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!locMode) return;
    if (locMode.kind === "create") {
      const parsed = locationCreateSchema.safeParse({
        code: locCode.trim().toUpperCase(),
        name: locName.trim(),
        parentId: locParent,
      });
      if (!parsed.success) {
        const errs: Record<string, string> = {};
        for (const issue of parsed.error.issues) {
          errs[String(issue.path[0] ?? "")] ??= issue.message;
        }
        setLocErrors(errs);
        return;
      }
      setLocErrors({});
      setLocSaving(true);
      try {
        const res = await stockService.createLocation(parsed.data);
        toast.fromServer(res.message, "Lokasi dibuat");
        setLocMode(null);
        reloadLocations();
      } catch (err) {
        toast.danger("Gagal menyimpan lokasi", toApiError(err).message);
      } finally {
        setLocSaving(false);
      }
      return;
    }
    const parsed = locationUpdateSchema.safeParse({
      name: locName.trim(),
      parentId: locParent,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        errs[String(issue.path[0] ?? "")] ??= issue.message;
      }
      setLocErrors(errs);
      return;
    }
    setLocErrors({});
    setLocSaving(true);
    try {
      // Kode tidak dikirim: backend UpdateLocation hanya menerima name+parentId.
      const res = await stockService.updateLocation(locMode.location.id, parsed.data);
      toast.fromServer(res.message, "Lokasi disimpan");
      setLocMode(null);
      reloadLocations();
    } catch (err) {
      toast.danger("Gagal menyimpan lokasi", toApiError(err).message);
    } finally {
      setLocSaving(false);
    }
  }

  async function handleArchive() {
    if (!archiveTarget) return;
    const target = archiveTarget;
    setArchiveTarget(null);
    try {
      const res = await stockService.archiveLocation(target.id);
      toast.fromServer(res.message, "Lokasi diarsipkan");
      reloadLocations();
    } catch (err) {
      toast.danger("Gagal mengarsipkan lokasi", toApiError(err).message);
    }
  }

  const parentOptions = (selfId?: number) =>
    locations
      .filter((l) => l.id !== selfId)
      .map((l) => ({ value: l.id, label: `${l.code} — ${l.name}` }));

  return (
    <div>
      <PageHeader
        eyebrow="Stok & Gudang"
        title="Stok"
        description="Saldo per produk, riwayat mutasi, dan daftar lokasi penyimpanan."
      />

      <SectionCard
        title="Saldo"
        description="Pilih produk untuk melihat saldo per lokasi. Backend mewajibkan productId."
        actions={
          saldoProduct !== null ? (
            <Link
              to={`/stock/items/${saldoProduct}`}
              className="font-semibold text-brand hover:underline text-sm"
            >
              Buka halaman detail
            </Link>
          ) : undefined
        }
      >
        <FilterBar>
          <div className="min-w-[240px] flex-1">
            <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">
              Produk
            </span>
            <SearchSelect
              options={productOptions.map((p) => ({
                value: p.id,
                label: `${p.code} — ${p.name}`,
              }))}
              value={saldoProduct}
              placeholder="Pilih produk…"
              allowClear
              onChange={(v) => setSaldoProduct(toId(v))}
            />
          </div>
          <div className="min-w-[220px]">
            <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">
              Lokasi
            </span>
            <SearchSelect
              options={locations.map((l) => ({
                value: l.id,
                label: `${l.code} — ${l.name}`,
              }))}
              value={saldoLocation}
              placeholder="Semua lokasi"
              allowClear
              onChange={(v) => setSaldoLocation(toId(v))}
            />
          </div>
        </FilterBar>
        {saldoError && (
          <div className="mb-4">
            <Notice tone="danger" title="Gagal memuat saldo">
              {saldoError}
            </Notice>
          </div>
        )}
        {saldoLoading ? (
          <p className="text-muted px-1 py-6 text-sm">Memuat saldo…</p>
        ) : (
          <DataTable<StockBalance>
            columns={[
              {
                header: "Produk",
                render: (r) => productLabel.get(r.productId) ?? `#${r.productId}`,
              },
              {
                header: "Varian",
                render: (r) => variantMap.get(r.variantId) ?? `#${r.variantId}`,
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
            rows={visibleBalances}
            rowKey={(r) => `${r.productId}-${r.variantId}-${r.locationId}`}
            emptyTitle={
              saldoProduct === null ? "Pilih produk dulu" : "Belum ada saldo"
            }
            emptyDescription={
              saldoProduct === null
                ? "Saldo dimuat per produk karena backend mewajibkan productId."
                : "Belum ada stok tercatat untuk produk ini."
            }
          />
        )}
      </SectionCard>

      <div className="mt-6">
        <SectionCard
          title="Riwayat Mutasi"
          description="Aliran stok masuk dan keluar per cabang yang aktif."
        >
          <FilterBar>
            <FormField label="Dari">
              <TextInput
                type="date"
                value={dateFrom}
                onChange={(e) => {
                  setDateFrom(e.target.value);
                  setMovPage(1);
                }}
              />
            </FormField>
            <FormField label="Sampai">
              <TextInput
                type="date"
                value={dateTo}
                onChange={(e) => {
                  setDateTo(e.target.value);
                  setMovPage(1);
                }}
              />
            </FormField>
            <div className="min-w-[220px] flex-1">
              <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">
                Produk
              </span>
              <SearchSelect
                options={productOptions.map((p) => ({
                  value: p.id,
                  label: `${p.code} — ${p.name}`,
                }))}
                value={movProduct}
                placeholder="Semua produk"
                allowClear
                onChange={(v) => {
                  setMovProduct(toId(v));
                  setMovVariant(null);
                  setMovPage(1);
                }}
              />
            </div>
            <div className="min-w-[200px]">
              <span className="text-heading mb-1.5 block text-[0.85rem] font-medium">
                Lokasi
              </span>
              <SearchSelect
                options={locations.map((l) => ({
                  value: l.id,
                  label: `${l.code} — ${l.name}`,
                }))}
                value={movLocation}
                placeholder="Semua lokasi"
                allowClear
                onChange={(v) => {
                  setMovLocation(toId(v));
                  setMovPage(1);
                }}
              />
            </div>
          </FilterBar>
          {movError && (
            <div className="mb-4">
              <Notice tone="danger" title="Gagal memuat mutasi">
                {movError}
              </Notice>
            </div>
          )}
          {movLoading ? (
            <p className="text-muted px-1 py-6 text-sm">Memuat mutasi…</p>
          ) : (
            <>
              <DataTable<StockMovement>
                columns={[
                  {
                    header: "Tanggal",
                    render: (r) => formatDateTime(r.createdAt),
                  },
                  {
                    header: "Produk",
                    render: (r) =>
                      `${productLabel.get(r.productId) ?? `#${r.productId}`} / V#${r.variantId}`,
                  },
                  {
                    header: "Lokasi",
                    render: (r) =>
                      locationMap.get(r.locationId)?.code ??
                      `#${r.locationId}`,
                  },
                  {
                    header: "Masuk",
                    align: "right",
                    render: (r) =>
                      r.direction === "in" ? formatNumber(r.qty) : "-",
                  },
                  {
                    header: "Keluar",
                    align: "right",
                    render: (r) =>
                      r.direction === "in" ? "-" : formatNumber(r.qty),
                  },
                  {
                    header: "Referensi",
                    render: (r) =>
                      r.refType ? `${r.refType} #${r.refId}` : "-",
                  },
                ]}
                rows={movements}
                rowKey={(r) => r.id}
                emptyTitle="Belum ada mutasi"
                emptyDescription="Mutasi tercatat dari penerimaan, penjualan, koreksi, dan transfer."
              />
              <Pagination
                page={movPage}
                limit={MOV_LIMIT}
                total={movTotal}
                onPageChange={setMovPage}
              />
            </>
          )}
        </SectionCard>
      </div>

      <div className="mt-6">
        <SectionCard
          title="Lokasi"
          description="Gudang dan rak penyimpanan cabang yang aktif."
          actions={
            canManage ? (
              <Button size="sm" onClick={openCreateLocation}>
                Tambah Lokasi
              </Button>
            ) : undefined
          }
        >
          {locLoading ? (
            <p className="text-muted px-1 py-6 text-sm">Memuat lokasi…</p>
          ) : (
            <DataTable<StockLocation>
              columns={[
                {
                  header: "Kode",
                  render: (r) => <span className="font-medium">{r.code}</span>,
                },
                { header: "Nama", render: (r) => r.name },
                {
                  header: "Induk",
                  render: (r) =>
                    r.parentId === null
                      ? "-"
                      : (locationMap.get(r.parentId)?.name ?? `#${r.parentId}`),
                },
                {
                  header: "Status",
                  render: (r) => (
                    <Badge tone={statusTone(r.status)}>
                      {statusLabel(r.status)}
                    </Badge>
                  ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) =>
                    canManage ? (
                      <span className="flex justify-end gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => openEditLocation(r)}
                        >
                          Ubah
                        </Button>
                        <Button
                          variant="secondary"
                          size="sm"
                          disabled={r.isSystem}
                          title={
                            r.isSystem
                              ? "Lokasi sistem tidak dapat diarsipkan"
                              : undefined
                          }
                          onClick={() => setArchiveTarget(r)}
                        >
                          Arsipkan
                        </Button>
                      </span>
                    ) : (
                      <span className="text-muted text-sm">-</span>
                    ),
                },
              ]}
              rows={locations}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada lokasi"
              emptyDescription="Tambah lokasi penyimpanan untuk mulai mencatat stok."
            />
          )}
        </SectionCard>
      </div>

      <Modal
        open={locMode !== null}
        title={locMode?.kind === "edit" ? "Ubah Lokasi" : "Tambah Lokasi"}
        size="md"
        onClose={() => {
          if (!locSaving) setLocMode(null);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              disabled={locSaving}
              onClick={() => setLocMode(null)}
            >
              Batal
            </Button>
            <Button type="submit" form="location-form" disabled={locSaving}>
              {locSaving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="location-form" onSubmit={(e) => void handleLocationSubmit(e)}>
          <div className="grid gap-4 md:grid-cols-2">
            <FormField label="Kode" required errorText={locErrors.code}>
              <TextInput
                value={locCode}
                disabled={locMode?.kind === "edit"}
                placeholder="cth: GDG-01"
                onChange={(e) => setLocCode(e.target.value.toUpperCase())}
              />
            </FormField>
            <FormField label="Nama" required errorText={locErrors.name}>
              <TextInput
                value={locName}
                placeholder="cth: Gudang Utama"
                onChange={(e) => setLocName(e.target.value)}
              />
            </FormField>
          </div>
          <div className="mt-4">
            <FormField label="Lokasi Induk" errorText={locErrors.parentId}>
              <SearchSelect
                options={parentOptions(
                  locMode?.kind === "edit" ? locMode.location.id : undefined,
                )}
                value={locParent}
                placeholder="Tanpa induk"
                allowClear
                onChange={(v) => setLocParent(toId(v))}
              />
            </FormField>
          </div>
        </form>
      </Modal>

      <ConfirmDialog
        open={archiveTarget !== null}
        title="Arsipkan Lokasi"
        message={`Arsipkan lokasi ${archiveTarget?.code ?? ""} — ${archiveTarget?.name ?? ""}? Lokasi arsip tidak bisa dipakai transaksi baru.`}
        confirmLabel="Arsipkan"
        onConfirm={() => void handleArchive()}
        onCancel={() => setArchiveTarget(null)}
      />
    </div>
  );
}
