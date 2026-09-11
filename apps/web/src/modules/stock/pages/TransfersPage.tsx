import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  FilterBar,
  FormField,
  Modal,
  PageHeader,
  Pagination,
  SearchSelect,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatNumber,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import {
  moveLocationSchema,
  transferSchema,
} from "@/modules/stock/schemas/stock.schema";
import { LocationSelect } from "@/modules/stock/components/LocationSelect";
import { ProductPicker } from "@/modules/stock/components/ProductPicker";
import type {
  BranchOption,
  StockTransfer,
} from "@/modules/stock/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

const LIMIT = 20;

interface ItemRow {
  key: number;
  productId: number | null;
  variantId: number | null;
  qty: string;
}

let nextRowKey = 1;

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

export default function TransfersPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.manage");

  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [branches, setBranches] = useState<BranchOption[]>([]);

  const [createOpen, setCreateOpen] = useState(false);
  const [toBranchId, setToBranchId] = useState<number | null>(null);
  const [fromLocationId, setFromLocationId] = useState<number | null>(null);
  const [toLocationId, setToLocationId] = useState<number | null>(null);
  const [notes, setNotes] = useState("");
  const [rows, setRows] = useState<ItemRow[]>([
    { key: 0, productId: null, variantId: null, qty: "" },
  ]);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Pindah lokasi satu langkah (J4) — dalam cabang aktif, langsung diterima.
  const [moveOpen, setMoveOpen] = useState(false);
  const [moveFrom, setMoveFrom] = useState<number | null>(null);
  const [moveTo, setMoveTo] = useState<number | null>(null);
  const [moveNotes, setMoveNotes] = useState("");
  const [moveRows, setMoveRows] = useState<ItemRow[]>([
    { key: 0, productId: null, variantId: null, qty: "" },
  ]);
  const [moveError, setMoveError] = useState<string | null>(null);
  const [moving, setMoving] = useState(false);

  const branchName = useCallback(
    (id: number) => {
      const b = branches.find((x) => x.id === id);
      return b ? `${b.code} — ${b.name}` : `#${id}`;
    },
    [branches],
  );

  const { data, loading, error, reload } = useAsyncData(
    () =>
      stockService
        .listTransfers({
          status: status || undefined,
          page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [status, page],
  );
  const items: StockTransfer[] = data?.items ?? [];
  const total = data?.total ?? 0;

  useEffect(() => {
    stockService
      .listBranches()
      .then(setBranches)
      .catch((err) =>
        toast.danger("Gagal memuat cabang", toApiError(err).message),
      );
  }, []);

  function resetForm() {
    setToBranchId(null);
    setFromLocationId(null);
    setToLocationId(null);
    setNotes("");
    setRows([{ key: nextRowKey++, productId: null, variantId: null, qty: "" }]);
    setFormError(null);
  }

  function resetMoveForm() {
    setMoveFrom(null);
    setMoveTo(null);
    setMoveNotes("");
    setMoveRows([{ key: nextRowKey++, productId: null, variantId: null, qty: "" }]);
    setMoveError(null);
  }

  function patchMoveRow(key: number, patch: Partial<ItemRow>) {
    setMoveRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  async function handleMove(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = moveLocationSchema.safeParse({
      fromLocationId: moveFrom ?? 0,
      toLocationId: moveTo ?? 0,
      notes: moveNotes,
      items: moveRows.map((r) => ({
        productId: r.productId ?? 0,
        variantId: r.variantId ?? 0,
        qty: r.qty,
      })),
    });
    if (!parsed.success) {
      const first = parsed.error.issues[0];
      setMoveError(first?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setMoveError(null);
    setMoving(true);
    try {
      const res = await stockService.moveLocation({
        fromLocationId: parsed.data.fromLocationId,
        toLocationId: parsed.data.toLocationId,
        notes: parsed.data.notes,
        items: parsed.data.items.map((it) => ({ ...it, notes: "" })),
      });
      toast.fromServer(res.message, "Stok dipindahkan", res.data.number);
      setMoveOpen(false);
      resetMoveForm();
      reload();
      navigate(`/stock/transfers/${res.data.id}`);
    } catch (err) {
      const apiErr = toApiError(err);
      setMoveError(apiErr.message);
      toast.danger("Gagal memindahkan stok", apiErr.message);
    } finally {
      setMoving(false);
    }
  }

  function patchRow(key: number, patch: Partial<ItemRow>) {
    setRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  async function handleCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = transferSchema.safeParse({
      toBranchId: toBranchId ?? 0,
      fromLocationId: fromLocationId ?? 0,
      toLocationId: toLocationId ?? 0,
      notes,
      items: rows.map((r) => ({
        productId: r.productId ?? 0,
        variantId: r.variantId ?? 0,
        qty: r.qty,
      })),
    });
    if (!parsed.success) {
      const first = parsed.error.issues[0];
      setFormError(first?.message ?? "Periksa kembali isian formulir.");
      return;
    }
    setFormError(null);
    setSaving(true);
    try {
      const res = await stockService.createTransfer({
        toBranchId: parsed.data.toBranchId,
        fromLocationId: parsed.data.fromLocationId,
        toLocationId: parsed.data.toLocationId,
        notes: parsed.data.notes,
        items: parsed.data.items.map((it) => ({ ...it, notes: "" })),
      });
      toast.fromServer(res.message, "Transfer dibuat", res.data.number);
      setCreateOpen(false);
      resetForm();
      reload();
      navigate(`/stock/transfers/${res.data.id}`);
    } catch (err) {
      const apiErr = toApiError(err);
      setFormError(apiErr.message);
      toast.danger("Gagal membuat transfer", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Stok & Gudang"
        title="Transfer Stok"
        description="Mutasi antar cabang: dibuat, dikirim, lalu diterima."
        actions={
          canManage ? (
            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                onClick={() => {
                  resetMoveForm();
                  setMoveOpen(true);
                }}
              >
                Pindah Lokasi
              </Button>
              <Button
                onClick={() => {
                  resetForm();
                  setCreateOpen(true);
                }}
              >
                Buat Transfer
              </Button>
            </div>
          ) : undefined
        }
      />

      <FilterBar>
        <FormField label="Status">
          <SelectInput
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setPage(1);
            }}
          >
            <option value="">Semua</option>
            <option value="draft">Draf</option>
            <option value="dispatched">Dikirim</option>
            <option value="received">Diterima</option>
            <option value="cancelled">Dibatalkan</option>
          </SelectInput>
        </FormField>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat transfer">
            {error}
          </Notice>
        </div>
      )}

      <div className="rounded-lg border border-hairline bg-surface">
        {loading ? (
          <p className="text-muted px-6 py-10 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<StockTransfer>
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`/stock/transfers/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                { header: "Dari", render: (r) => branchName(r.fromBranchId) },
                { header: "Ke", render: (r) => branchName(r.toBranchId) },
                {
                  header: "Baris",
                  align: "right",
                  render: (r) => formatNumber(r.items.length),
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
                  render: (r) => (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => navigate(`/stock/transfers/${r.id}`)}
                    >
                      Lihat
                    </Button>
                  ),
                },
              ]}
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada transfer"
              emptyDescription="Buat transfer untuk memindahkan stok antar cabang."
            />
            <Pagination
              page={page}
              limit={LIMIT}
              total={total}
              onPageChange={setPage}
            />
          </>
        )}
      </div>

      <Modal
        open={createOpen}
        title="Buat Transfer"
        description="Stok keluar dicatat saat pengiriman, bukan saat pembuatan."
        size="lg"
        onClose={() => {
          if (!saving) setCreateOpen(false);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              disabled={saving}
              onClick={() => setCreateOpen(false)}
            >
              Batal
            </Button>
            <Button type="submit" form="transfer-form" disabled={saving}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="transfer-form" onSubmit={(e) => void handleCreate(e)}>
          {formError && (
            <div className="mb-4">
              <Notice tone="danger" title="Transfer belum tersimpan">
                {formError}
              </Notice>
            </div>
          )}
          <div className="grid gap-4 md:grid-cols-3">
            <FormField label="Cabang Tujuan" required>
              <SearchSelect
                options={branches.map((b) => ({
                  value: b.id,
                  label: `${b.code} — ${b.name}`,
                }))}
                value={toBranchId}
                placeholder="Pilih cabang…"
                onChange={(v) => setToBranchId(toId(v))}
              />
            </FormField>
            <LocationSelect
              label="Lokasi Asal"
              value={fromLocationId}
              onChange={setFromLocationId}
              required
            />
            <LocationSelect
              label="Lokasi Tujuan"
              value={toLocationId}
              onChange={setToLocationId}
              required
            />
          </div>
          <div className="mt-4">
            <FormField label="Catatan">
              <TextArea
                rows={2}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder="Keterangan tambahan (opsional)"
              />
            </FormField>
          </div>
          <div className="mt-4 space-y-4">
            {rows.map((row, idx) => (
              <div
                key={row.key}
                className="rounded-md border border-hairline p-4"
              >
                <div className="mb-3 flex items-center justify-between">
                  <p className="text-sm font-semibold text-ink">
                    Barang {idx + 1}
                  </p>
                  {rows.length > 1 && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() =>
                        setRows((prev) => prev.filter((r) => r.key !== row.key))
                      }
                    >
                      Hapus
                    </Button>
                  )}
                </div>
                <ProductPicker
                  productId={row.productId}
                  variantId={row.variantId}
                  onProductChange={(id) => patchRow(row.key, { productId: id })}
                  onVariantChange={(id) => patchRow(row.key, { variantId: id })}
                />
                <div className="mt-3 max-w-[220px]">
                  <FormField label="Jumlah" required>
                    <TextInput
                      type="number"
                      min="0"
                      step="any"
                      placeholder="cth: 5"
                      value={row.qty}
                      onChange={(e) =>
                        patchRow(row.key, { qty: e.target.value })
                      }
                    />
                  </FormField>
                </div>
              </div>
            ))}
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                setRows((prev) => [
                  ...prev,
                  {
                    key: nextRowKey++,
                    productId: null,
                    variantId: null,
                    qty: "",
                  },
                ])
              }
            >
              Tambah Barang
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={moveOpen}
        title="Pindah Lokasi"
        description="Antar lokasi dalam cabang aktif — stok langsung berpindah, tercatat sebagai surat transfer yang sudah diterima."
        size="lg"
        onClose={() => {
          if (!moving) setMoveOpen(false);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              disabled={moving}
              onClick={() => setMoveOpen(false)}
            >
              Batal
            </Button>
            <Button type="submit" form="move-form" disabled={moving}>
              {moving ? "Memindahkan…" : "Pindahkan"}
            </Button>
          </>
        }
      >
        <form id="move-form" onSubmit={(e) => void handleMove(e)}>
          {moveError && (
            <div className="mb-4">
              <Notice tone="danger" title="Stok belum dipindahkan">
                {moveError}
              </Notice>
            </div>
          )}
          <div className="grid gap-4 md:grid-cols-2">
            <LocationSelect
              label="Lokasi Asal"
              value={moveFrom}
              onChange={setMoveFrom}
              required
            />
            <LocationSelect
              label="Lokasi Tujuan"
              value={moveTo}
              onChange={setMoveTo}
              required
            />
          </div>
          <div className="mt-4">
            <FormField label="Catatan">
              <TextArea
                rows={2}
                value={moveNotes}
                onChange={(e) => setMoveNotes(e.target.value)}
                placeholder="Keterangan tambahan (opsional)"
              />
            </FormField>
          </div>
          <div className="mt-4 space-y-4">
            {moveRows.map((row, idx) => (
              <div
                key={row.key}
                className="rounded-md border border-hairline p-4"
              >
                <div className="mb-3 flex items-center justify-between">
                  <p className="text-sm font-semibold text-ink">
                    Barang {idx + 1}
                  </p>
                  {moveRows.length > 1 && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() =>
                        setMoveRows((prev) => prev.filter((r) => r.key !== row.key))
                      }
                    >
                      Hapus
                    </Button>
                  )}
                </div>
                <ProductPicker
                  productId={row.productId}
                  variantId={row.variantId}
                  onProductChange={(id) => patchMoveRow(row.key, { productId: id })}
                  onVariantChange={(id) => patchMoveRow(row.key, { variantId: id })}
                />
                <div className="mt-3 max-w-[220px]">
                  <FormField label="Jumlah" required>
                    <TextInput
                      type="number"
                      min="0"
                      step="any"
                      placeholder="cth: 5"
                      value={row.qty}
                      onChange={(e) =>
                        patchMoveRow(row.key, { qty: e.target.value })
                      }
                    />
                  </FormField>
                </div>
              </div>
            ))}
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                setMoveRows((prev) => [
                  ...prev,
                  {
                    key: nextRowKey++,
                    productId: null,
                    variantId: null,
                    qty: "",
                  },
                ])
              }
            >
              Tambah Barang
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
