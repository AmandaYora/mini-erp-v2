import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  ConfirmDialog,
  DataTable,
  FormField,
  Modal,
  PageHeader,
  SectionCard,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import {
  damagedMoveSchema,
  damagedWriteOffSchema,
} from "@/modules/stock/schemas/stock.schema";
import { LocationSelect } from "@/modules/stock/components/LocationSelect";
import { ProductPicker } from "@/modules/stock/components/ProductPicker";
import type {
  ProductDetail,
  StockBalance,
  StockLocation,
} from "@/modules/stock/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

export default function DamagedPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.manage");

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      const [damaged, locs] = await Promise.all([
        stockService.damagedList(),
        stockService.listLocations(),
      ]);
      const productIds = [...new Set(damaged.map((d) => d.productId))];
      const fetched = await Promise.all(
        productIds.map((pid) => stockService.getProduct(pid).catch(() => null)),
      );
      const map = new Map<number, ProductDetail>();
      for (const d of fetched) {
        if (d) map.set(d.id, d);
      }
      return {
        rows: damaged,
        locations: locs,
        details: map,
      };
    },
    [],
  );
  const rows: StockBalance[] = useMemo(() => data?.rows ?? [], [data]);
  const locations: StockLocation[] = useMemo(() => data?.locations ?? [], [data]);
  const details: Map<number, ProductDetail> = useMemo(
    () => data?.details ?? new Map(),
    [data],
  );

  const [moveOpen, setMoveOpen] = useState(false);
  const [moveProduct, setMoveProduct] = useState<number | null>(null);
  const [moveVariant, setMoveVariant] = useState<number | null>(null);
  const [moveLocation, setMoveLocation] = useState<number | null>(null);
  const [moveQty, setMoveQty] = useState("");
  const [moveReason, setMoveReason] = useState("");
  const [moveError, setMoveError] = useState<string | null>(null);
  const [moveSaving, setMoveSaving] = useState(false);

  const [restoreTarget, setRestoreTarget] = useState<StockBalance | null>(null);

  const [writeOffTarget, setWriteOffTarget] = useState<StockBalance | null>(null);
  const [writeOffQty, setWriteOffQty] = useState("");
  const [writeOffReason, setWriteOffReason] = useState("");
  const [writeOffError, setWriteOffError] = useState<string | null>(null);
  const [writeOffSaving, setWriteOffSaving] = useState(false);

  const locationMap = useMemo(
    () => new Map(locations.map((l) => [l.id, l])),
    [locations],
  );

  function productText(productId: number): string {
    const d = details.get(productId);
    return d ? `${d.code} — ${d.name}` : `#${productId}`;
  }

  function variantText(productId: number, variantId: number): string {
    const v = details.get(productId)?.variants.find((x) => x.id === variantId);
    return v ? `${v.code} — ${v.name}` : `#${variantId}`;
  }

  function rowKey(r: StockBalance): string {
    return `${r.productId}-${r.variantId}-${r.locationId}`;
  }

  async function handleMoveIn(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = damagedMoveSchema.safeParse({
      productId: moveProduct ?? 0,
      variantId: moveVariant ?? 0,
      locationId: moveLocation ?? 0,
      qty: moveQty,
      reason: moveReason.trim(),
    });
    if (!parsed.success) {
      setMoveError(
        parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.",
      );
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

  async function handleRestore() {
    if (!restoreTarget) return;
    const target = restoreTarget;
    setRestoreTarget(null);
    try {
      const res = await stockService.damagedRestore({
        productId: target.productId,
        variantId: target.variantId,
        locationId: target.locationId,
        qty: target.onHand,
        reason: "",
      });
      toast.fromServer(res.message, "Stok dipulihkan");
      reload();
    } catch (err) {
      toast.danger("Gagal memulihkan stok", toApiError(err).message);
    }
  }

  async function handleWriteOff(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!writeOffTarget) return;
    const parsed = damagedWriteOffSchema.safeParse({
      qty: writeOffQty,
      reason: writeOffReason.trim(),
    });
    if (!parsed.success) {
      setWriteOffError(
        parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.",
      );
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
      toast.danger("Gagal hapusbuku", apiErr.message);
    } finally {
      setWriteOffSaving(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Stok & Gudang"
        title="Stok Rusak"
        description="Saldo di lokasi rusak: catat masuk, pulihkan, atau hapusbukukan."
        actions={
          canManage ? (
            <Button
              onClick={() => {
                setMoveError(null);
                setMoveOpen(true);
              }}
            >
              Catat Rusak
            </Button>
          ) : undefined
        }
      />

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat stok rusak">
            {error}
          </Notice>
        </div>
      )}

      <SectionCard title="Saldo Rusak">
        {loading ? (
          <p className="text-muted px-1 py-6 text-sm">Memuat…</p>
        ) : (
          <DataTable<StockBalance>
            columns={[
              {
                header: "Produk",
                render: (r) => productText(r.productId),
              },
              {
                header: "Varian",
                render: (r) => variantText(r.productId, r.variantId),
              },
              {
                header: "Lokasi",
                render: (r) =>
                  locationMap.get(r.locationId)?.code ?? `#${r.locationId}`,
              },
              {
                header: "Jumlah",
                align: "right",
                render: (r) => formatNumber(r.onHand),
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
                        onClick={() => setRestoreTarget(r)}
                      >
                        Pulihkan
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          setWriteOffError(null);
                          setWriteOffQty(String(r.onHand));
                          setWriteOffReason("");
                          setWriteOffTarget(r);
                        }}
                      >
                        Hapusbuku
                      </Button>
                    </span>
                  ) : (
                    <span className="text-muted text-sm">-</span>
                  ),
              },
            ]}
            rows={rows}
            rowKey={rowKey}
            emptyTitle="Tidak ada stok rusak"
            emptyDescription="Stok yang rusak dicatat lewat tombol Catat Rusak."
          />
        )}
      </SectionCard>

      <Modal
        open={moveOpen}
        title="Catat Stok Rusak"
        description="Memindahkan stok baik ke lokasi rusak."
        size="lg"
        onClose={() => {
          if (!moveSaving) setMoveOpen(false);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              disabled={moveSaving}
              onClick={() => setMoveOpen(false)}
            >
              Batal
            </Button>
            <Button type="submit" form="damaged-move-form" disabled={moveSaving}>
              {moveSaving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="damaged-move-form" onSubmit={(e) => void handleMoveIn(e)}>
          {moveError && (
            <div className="mb-4">
              <Notice tone="danger" title="Belum tersimpan">
                {moveError}
              </Notice>
            </div>
          )}
          <ProductPicker
            productId={moveProduct}
            variantId={moveVariant}
            onProductChange={setMoveProduct}
            onVariantChange={setMoveVariant}
          />
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <LocationSelect
              label="Lokasi Asal"
              value={moveLocation}
              onChange={setMoveLocation}
              required
            />
            <FormField label="Jumlah" required>
              <TextInput
                type="number"
                min="0"
                step="any"
                placeholder="cth: 2"
                value={moveQty}
                onChange={(e) => setMoveQty(e.target.value)}
              />
            </FormField>
          </div>
          <div className="mt-4">
            <FormField label="Alasan">
              <TextArea
                rows={2}
                value={moveReason}
                onChange={(e) => setMoveReason(e.target.value)}
                placeholder="cth: Kemasan bocor saat bongkar"
              />
            </FormField>
          </div>
        </form>
      </Modal>

      <ConfirmDialog
        open={restoreTarget !== null}
        title="Pulihkan Stok"
        message={
          restoreTarget
            ? `Kembalikan ${formatNumber(restoreTarget.onHand)} ${productText(restoreTarget.productId)} dari lokasi rusak ke stok baik?`
            : ""
        }
        confirmLabel="Pulihkan"
        tone="primary"
        onConfirm={() => void handleRestore()}
        onCancel={() => setRestoreTarget(null)}
      />

      <Modal
        open={writeOffTarget !== null}
        title="Hapusbukukan Stok"
        description="Stok keluar permanen dari lokasi rusak. Alasan wajib diisi."
        size="md"
        onClose={() => {
          if (!writeOffSaving) setWriteOffTarget(null);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              disabled={writeOffSaving}
              onClick={() => setWriteOffTarget(null)}
            >
              Batal
            </Button>
            <Button
              type="submit"
              form="damaged-writeoff-form"
              disabled={writeOffSaving}
            >
              {writeOffSaving ? "Menyimpan…" : "Hapusbukukan"}
            </Button>
          </>
        }
      >
        <form
          id="damaged-writeoff-form"
          onSubmit={(e) => void handleWriteOff(e)}
        >
          {writeOffError && (
            <div className="mb-4">
              <Notice tone="danger" title="Belum tersimpan">
                {writeOffError}
              </Notice>
            </div>
          )}
          {writeOffTarget && (
            <p className="mb-4 text-sm text-ink">
              {productText(writeOffTarget.productId)} —{" "}
              {variantText(
                writeOffTarget.productId,
                writeOffTarget.variantId,
              )}{" "}
              (tersedia {formatNumber(writeOffTarget.onHand)})
            </p>
          )}
          <div className="grid gap-4 md:grid-cols-2">
            <FormField label="Jumlah" required>
              <TextInput
                type="number"
                min="0"
                step="any"
                value={writeOffQty}
                onChange={(e) => setWriteOffQty(e.target.value)}
              />
            </FormField>
            <FormField label="Alasan" required>
              <TextInput
                value={writeOffReason}
                onChange={(e) => setWriteOffReason(e.target.value)}
                placeholder="cth: Busuk, dibuang"
              />
            </FormField>
          </div>
        </form>
      </Modal>
    </div>
  );
}
