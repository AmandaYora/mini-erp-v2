import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  DataTable,
  FormField,
  FormSheet,
  Modal,
  PageHeader,
  Pagination,
  SectionCard,
  Tabs,
  TabsList,
  TabsTrigger,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { ActionMenu, Money, Page, Qty, useUrlTab } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import { damagedMoveInSchema, damagedMoveSchema, damagedWriteOffSchema } from "@/modules/stock/schemas/stock.schema";
import { LocationSelect } from "@/modules/stock/components/LocationSelect";
import { ProductPicker } from "@/modules/stock/components/ProductPicker";
import type { DamagedBalance, ProductDetail, StockLocation } from "@/modules/stock/types";

const LIMIT = 20;
const TABS = ["ada", "nol"] as const;

export default function DamagedPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.adjust");
  const [tab, setTab] = useUrlTab(TABS, "ada");
  const [page, setPage] = useState(1);

  // Paginasi dan estimasi kerugian dihitung server (ST-03); tab = himpunan "ada stok" / "sudah nol".
  const { data, loading, error, reload } = useAsyncData(async () => {
    const [damaged, locs] = await Promise.all([
      stockService.damagedList({ page, limit: LIMIT, empty: tab === "nol" }),
      stockService.listLocations(),
    ]);
    const productIds = [...new Set(damaged.items.map((d) => d.productId))];
    const fetched = await Promise.all(productIds.map((pid) => stockService.getProduct(pid).catch(() => null)));
    const details = new Map<number, ProductDetail>();
    for (const d of fetched) if (d) details.set(d.id, d);
    return { damaged, locations: locs, details };
  }, [page, tab]);
  const pageRows: DamagedBalance[] = useMemo(() => data?.damaged.items ?? [], [data]);
  const locations: StockLocation[] = useMemo(() => data?.locations ?? [], [data]);
  const details = data?.details;
  const locationMap = useMemo(() => new Map(locations.map((l) => [l.id, l])), [locations]);
  const withStockCount = data?.damaged.withStock ?? 0;
  const emptyCount = data?.damaged.empty ?? 0;
  const shownTotal = data?.damaged.total ?? 0;
  const totalLoss = data?.damaged.totalEstimatedLoss ?? 0;
  const current = page;

  // Catat rusak (FormSheet: butuh pemilih produk)
  const [moveOpen, setMoveOpen] = useState(false);
  const [moveProduct, setMoveProduct] = useState<number | null>(null);
  const [moveVariant, setMoveVariant] = useState<number | null>(null);
  const [moveLocation, setMoveLocation] = useState<number | null>(null);
  const [moveQty, setMoveQty] = useState("");
  const [moveReason, setMoveReason] = useState("");
  const [moveError, setMoveError] = useState<string | null>(null);
  const [moveSaving, setMoveSaving] = useState(false);

  // Pulihkan (Dialog)
  const [restoreTarget, setRestoreTarget] = useState<DamagedBalance | null>(null);
  const [restoreReason, setRestoreReason] = useState("");
  const [restoreError, setRestoreError] = useState<string | null>(null);
  const [restoreSaving, setRestoreSaving] = useState(false);

  // Hapusbukukan (Dialog)
  const [writeOffTarget, setWriteOffTarget] = useState<DamagedBalance | null>(null);
  const [writeOffQty, setWriteOffQty] = useState("");
  const [writeOffReason, setWriteOffReason] = useState("");
  const [writeOffError, setWriteOffError] = useState<string | null>(null);
  const [writeOffSaving, setWriteOffSaving] = useState(false);

  function productText(productId: number): string {
    const d = details?.get(productId);
    return d ? `${d.code} — ${d.name}` : `#${productId}`;
  }
  function variantText(productId: number, variantId: number): string {
    const v = details?.get(productId)?.variants.find((x) => x.id === variantId);
    return v ? `${v.code} — ${v.name}` : `#${variantId}`;
  }

  async function handleMoveIn(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = damagedMoveInSchema.safeParse({
      productId: moveProduct ?? 0,
      variantId: moveVariant ?? 0,
      locationId: moveLocation ?? 0,
      qty: moveQty,
      reason: moveReason.trim(),
    });
    if (!parsed.success) {
      setMoveError(parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setMoveError(null);
    setMoveSaving(true);
    try {
      const res = await stockService.damagedMoveIn(parsed.data);
      toast.fromServer(res.message, "Stok dicatat rusak");
      setMoveOpen(false);
      setMoveProduct(null);
      setMoveVariant(null);
      setMoveLocation(null);
      setMoveQty("");
      setMoveReason("");
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      setMoveError(apiErr.message);
      toast.danger("Gagal mencatat rusak", apiErr.message);
    } finally {
      setMoveSaving(false);
    }
  }

  async function handleRestore(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!restoreTarget) return;
    const target = restoreTarget;
    // Alasan wajib (backend moveDamaged menolak alasan kosong).
    const parsed = damagedMoveSchema.safeParse({
      productId: target.productId,
      variantId: target.variantId,
      locationId: target.locationId,
      qty: target.onHand,
      reason: restoreReason,
    });
    if (!parsed.success) {
      setRestoreError(parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setRestoreError(null);
    setRestoreSaving(true);
    try {
      const res = await stockService.damagedRestore(parsed.data);
      toast.fromServer(res.message, "Stok dipulihkan");
      setRestoreTarget(null);
      setRestoreReason("");
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      setRestoreError(apiErr.message);
      toast.danger("Gagal memulihkan stok", apiErr.message);
    } finally {
      setRestoreSaving(false);
    }
  }

  async function handleWriteOff(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!writeOffTarget) return;
    const parsed = damagedWriteOffSchema.safeParse({ qty: writeOffQty, reason: writeOffReason.trim() });
    if (!parsed.success) {
      setWriteOffError(parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setWriteOffError(null);
    setWriteOffSaving(true);
    try {
      // Write-off tanpa locationId: selalu memakan lokasi rusak itu sendiri.
      const res = await stockService.damagedWriteOff({
        productId: writeOffTarget.productId,
        variantId: writeOffTarget.variantId,
        qty: parsed.data.qty,
        reason: parsed.data.reason,
      });
      toast.fromServer(res.message, "Stok dihapusbukukan");
      setWriteOffTarget(null);
      setWriteOffQty("");
      setWriteOffReason("");
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      setWriteOffError(apiErr.message);
      toast.danger("Gagal hapus buku", apiErr.message);
    } finally {
      setWriteOffSaving(false);
    }
  }

  function openMove() {
    setMoveError(null);
    setMoveOpen(true);
  }

  return (
    <Page>
      <PageHeader
        title="Stok Rusak"
        description="Saldo di lokasi rusak: catat masuk, pulihkan, atau hapusbukukan."
        actions={canManage ? <Button onClick={openMove}>Catat rusak</Button> : undefined}
      />

      <Tabs
        value={tab}
        onValueChange={(v) => {
          setTab(v as (typeof TABS)[number]);
          setPage(1);
        }}
        className="gap-4"
      >
        <TabsList variant="line" aria-label="Status stok rusak">
          <TabsTrigger value="ada">Ada stok ({formatNumber(withStockCount)})</TabsTrigger>
          <TabsTrigger value="nol">Sudah nol ({formatNumber(emptyCount)})</TabsTrigger>
        </TabsList>
      </Tabs>

      <SectionCard flush>
        {tab === "ada" && withStockCount > 0 && (
          <p className="border-b border-border px-5 py-3 text-sm text-muted-foreground">
            Potensi nilai (harga modal rata-rata) seluruh stok rusak: <Money value={totalLoss} className="font-medium" />
          </p>
        )}
        <DataTable<DamagedBalance>
          loading={loading}
          error={error}
          onRetry={reload}
          rows={pageRows}
          rowKey={(r) => `${r.productId}-${r.variantId}-${r.locationId}`}
          emptyTitle={tab === "ada" ? "Tidak ada stok rusak" : "Belum ada saldo rusak yang habis"}
          emptyDescription={
            tab === "ada" ? "Stok yang rusak dicatat lewat tombol Catat rusak." : "Saldo rusak yang sudah nol tampil di sini."
          }
          emptyAction={tab === "ada" && canManage ? <Button onClick={openMove}>Catat rusak</Button> : undefined}
          columns={[
            { header: "Produk", render: (r) => <span className="font-medium">{productText(r.productId)}</span> },
            { header: "Varian", render: (r) => variantText(r.productId, r.variantId) },
            {
              header: "Lokasi",
              render: (r) => locationMap.get(r.locationId)?.code ?? `#${r.locationId}`,
            },
            { header: "Jumlah", align: "right", render: (r) => <Qty value={r.onHand} /> },
            { header: "Harga modal", align: "right", render: (r) => <Money value={r.estimatedUnitCost} /> },
            { header: "Perkiraan rugi", align: "right", render: (r) => <Money value={r.estimatedLoss} className="font-medium" /> },
            ...(canManage
              ? [
                  {
                    header: "",
                    id: "actions",
                    align: "right" as const,
                    width: "3rem",
                    render: (r: DamagedBalance) => (
                      <ActionMenu
                        variant="ghost"
                        label={`Aksi untuk ${productText(r.productId)}`}
                        items={[
                          {
                            key: "restore",
                            label: "Pulihkan ke stok baik",
                            disabled: r.onHand <= 0,
                            onSelect: () => {
                              setRestoreReason("");
                              setRestoreError(null);
                              setRestoreTarget(r);
                            },
                          },
                          {
                            key: "writeoff",
                            label: "Hapusbukukan",
                            destructive: true,
                            disabled: r.onHand <= 0,
                            onSelect: () => {
                              setWriteOffError(null);
                              setWriteOffQty(String(r.onHand));
                              setWriteOffReason("");
                              setWriteOffTarget(r);
                            },
                          },
                        ]}
                      />
                    ),
                  },
                ]
              : []),
          ]}
        />
        {shownTotal > 0 && <Pagination page={current} limit={LIMIT} total={shownTotal} onPageChange={setPage} />}
      </SectionCard>

      <FormSheet
        open={moveOpen}
        title="Catat stok rusak"
        description="Memindahkan stok baik ke lokasi rusak. Tanpa lokasi asal, barang dicatat langsung masuk sebagai rusak (datang rusak)."
        size="lg"
        onClose={() => {
          if (!moveSaving) setMoveOpen(false);
        }}
        actions={
          <>
            <Button variant="secondary" disabled={moveSaving} onClick={() => setMoveOpen(false)}>
              Batal
            </Button>
            <Button type="submit" form="damaged-move-form" loading={moveSaving}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="damaged-move-form" className="flex flex-col gap-4" onSubmit={(e) => void handleMoveIn(e)}>
          {moveError && (
            <Notice tone="danger" title="Belum tersimpan">
              {moveError}
            </Notice>
          )}
          <ProductPicker
            productId={moveProduct}
            variantId={moveVariant}
            onProductChange={setMoveProduct}
            onVariantChange={setMoveVariant}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <LocationSelect
              label="Lokasi asal (opsional)"
              value={moveLocation}
              onChange={setMoveLocation}
              allowClear
              placeholder="Kosongkan bila barang datang sudah rusak"
            />
            <FormField label="Jumlah" htmlFor="damaged-move-qty" required>
              <TextInput
                id="damaged-move-qty"
                type="number"
                min="0"
                step="any"
                inputMode="decimal"
                placeholder="cth: 2"
                value={moveQty}
                onChange={(e) => setMoveQty(e.target.value)}
              />
            </FormField>
          </div>
          <FormField label="Alasan" htmlFor="damaged-move-reason" required>
            <TextArea
              id="damaged-move-reason"
              rows={2}
              value={moveReason}
              onChange={(e) => setMoveReason(e.target.value)}
              placeholder="cth: Kemasan bocor saat bongkar"
            />
          </FormField>
        </form>
      </FormSheet>

      <Modal
        open={restoreTarget !== null}
        title="Pulihkan stok"
        description="Stok kembali dari lokasi rusak ke stok baik. Alasan wajib diisi."
        size="md"
        onClose={() => {
          if (!restoreSaving) setRestoreTarget(null);
        }}
        actions={
          <>
            <Button variant="secondary" disabled={restoreSaving} onClick={() => setRestoreTarget(null)}>
              Batal
            </Button>
            <Button type="submit" form="damaged-restore-form" loading={restoreSaving}>
              Pulihkan
            </Button>
          </>
        }
      >
        <form id="damaged-restore-form" className="flex flex-col gap-4" onSubmit={(e) => void handleRestore(e)}>
          {restoreError && (
            <Notice tone="danger" title="Belum tersimpan">
              {restoreError}
            </Notice>
          )}
          {restoreTarget && (
            <p className="text-sm text-foreground">
              Kembalikan {formatNumber(restoreTarget.onHand)} {productText(restoreTarget.productId)} ke stok baik.
            </p>
          )}
          <FormField label="Alasan" htmlFor="damaged-restore-reason" required>
            <TextInput
              id="damaged-restore-reason"
              value={restoreReason}
              onChange={(e) => setRestoreReason(e.target.value)}
              placeholder="cth: Kemasan diganti, layak jual"
            />
          </FormField>
        </form>
      </Modal>

      <Modal
        open={writeOffTarget !== null}
        title="Hapusbukukan stok"
        description="Stok keluar permanen dari lokasi rusak. Alasan wajib diisi."
        size="md"
        onClose={() => {
          if (!writeOffSaving) setWriteOffTarget(null);
        }}
        actions={
          <>
            <Button variant="secondary" disabled={writeOffSaving} onClick={() => setWriteOffTarget(null)}>
              Batal
            </Button>
            <Button type="submit" variant="danger" form="damaged-writeoff-form" loading={writeOffSaving}>
              Hapusbukukan
            </Button>
          </>
        }
      >
        <form id="damaged-writeoff-form" className="flex flex-col gap-4" onSubmit={(e) => void handleWriteOff(e)}>
          {writeOffError && (
            <Notice tone="danger" title="Belum tersimpan">
              {writeOffError}
            </Notice>
          )}
          {writeOffTarget && (
            <p className="text-sm text-foreground">
              {productText(writeOffTarget.productId)} — {variantText(writeOffTarget.productId, writeOffTarget.variantId)}{" "}
              (tersedia {formatNumber(writeOffTarget.onHand)})
            </p>
          )}
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="Jumlah" htmlFor="damaged-writeoff-qty" required>
              <TextInput
                id="damaged-writeoff-qty"
                type="number"
                min="0"
                step="any"
                inputMode="decimal"
                value={writeOffQty}
                onChange={(e) => setWriteOffQty(e.target.value)}
              />
            </FormField>
            <FormField label="Alasan" htmlFor="damaged-writeoff-reason" required>
              <TextInput
                id="damaged-writeoff-reason"
                value={writeOffReason}
                onChange={(e) => setWriteOffReason(e.target.value)}
                placeholder="cth: Busuk, dibuang"
              />
            </FormField>
          </div>
        </form>
      </Modal>
    </Page>
  );
}
