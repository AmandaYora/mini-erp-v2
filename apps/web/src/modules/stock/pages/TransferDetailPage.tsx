import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  PageHeader,
  SectionCard,
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
import type {
  BranchOption,
  ProductDetail,
  StockTransfer,
} from "@/modules/stock/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

type Action = "dispatch" | "receive" | "cancel";

const ACTION_META: Record<
  Action,
  { title: string; message: string; confirmLabel: string }
> = {
  dispatch: {
    title: "Kirim Transfer",
    message:
      "Kirim transfer ini? Stok keluar dicatat di lokasi asal dan tidak bisa dibatalkan sepihak setelah diterima.",
    confirmLabel: "Kirim",
  },
  receive: {
    title: "Terima Transfer",
    message: "Tandai transfer ini sudah diterima? Stok masuk dicatat di lokasi tujuan.",
    confirmLabel: "Terima",
  },
  cancel: {
    title: "Batalkan Transfer",
    message: "Batalkan transfer ini? Hanya transfer yang belum diterima yang bisa dibatalkan.",
    confirmLabel: "Batalkan",
  },
};

export default function TransferDetailPage() {
  const { id } = useParams<{ id: string }>();
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.manage");

  const [action, setAction] = useState<Action | null>(null);

  const transferId = Number(id);

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      if (!Number.isInteger(transferId) || transferId <= 0) {
        throw new Error("ID transfer tidak valid.");
      }
      const [t, branchRows] = await Promise.all([
        stockService.getTransfer(transferId),
        stockService.listBranches().catch(() => [] as BranchOption[]),
      ]);
      // Nama produk/varian best-effort: transferView hanya membawa ID.
      const productIds = [...new Set(t.items.map((it) => it.productId))];
      const fetched = await Promise.all(
        productIds.map((pid) =>
          stockService.getProduct(pid).catch(() => null),
        ),
      );
      const map = new Map<number, ProductDetail>();
      for (const d of fetched) {
        if (d) map.set(d.id, d);
      }
      return { transfer: t, details: map, branches: branchRows };
    },
    [transferId],
  );
  const transfer: StockTransfer | null = data?.transfer ?? null;
  const details: Map<number, ProductDetail> = data?.details ?? new Map();
  const branches: BranchOption[] = data?.branches ?? [];

  function branchName(branchId: number): string {
    const b = branches.find((x) => x.id === branchId);
    return b ? `${b.code} — ${b.name}` : `#${branchId}`;
  }

  async function runAction() {
    if (!action || !transfer) return;
    const current = action;
    setAction(null);
    try {
      const res =
        current === "dispatch"
          ? await stockService.dispatchTransfer(transfer.id)
          : current === "receive"
            ? await stockService.receiveTransfer(transfer.id)
            : await stockService.cancelTransfer(transfer.id);
      toast.fromServer(
        res.message,
        current === "dispatch"
          ? "Transfer dikirim"
          : current === "receive"
            ? "Transfer diterima"
            : "Transfer dibatalkan",
      );
      reload();
    } catch (err) {
      toast.danger("Aksi gagal", toApiError(err).message);
    }
  }

  const status = transfer?.status ?? "";
  const showActions =
    canManage && (status === "draft" || status === "dispatched");

  return (
    <div>
      <PageHeader
        eyebrow="Stok & Gudang"
        title={transfer ? `Transfer ${transfer.number}` : "Detail Transfer"}
        description={
          transfer
            ? `${branchName(transfer.fromBranchId)} → ${branchName(transfer.toBranchId)}`
            : "Memuat detail transfer…"
        }
        actions={
          <Link to="/stock/transfers">
            <Button variant="secondary">Kembali</Button>
          </Link>
        }
      />

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat transfer">
            {error}
          </Notice>
        </div>
      )}

      {loading ? (
        <p className="text-muted px-1 py-6 text-sm">Memuat…</p>
      ) : transfer ? (
        <>
          <SectionCard
            title="Ringkasan"
            actions={
              <Badge tone={statusTone(transfer.status)}>
                {statusLabel(transfer.status)}
              </Badge>
            }
          >
            <dl className="grid gap-3 text-sm md:grid-cols-2">
              <div>
                <dt className="text-muted">Nomor</dt>
                <dd className="font-medium text-ink">{transfer.number}</dd>
              </div>
              <div>
                <dt className="text-muted">Status</dt>
                <dd className="font-medium text-ink">
                  {statusLabel(transfer.status)}
                </dd>
              </div>
              <div>
                <dt className="text-muted">Dari</dt>
                <dd className="text-ink">
                  {branchName(transfer.fromBranchId)} (lokasi #
                  {transfer.fromLocationId})
                </dd>
              </div>
              <div>
                <dt className="text-muted">Ke</dt>
                <dd className="text-ink">
                  {branchName(transfer.toBranchId)} (lokasi #
                  {transfer.toLocationId})
                </dd>
              </div>
            </dl>
            {showActions && (
              <div className="mt-4 flex flex-wrap gap-2">
                {status === "draft" && (
                  <Button size="sm" onClick={() => setAction("dispatch")}>
                    Kirim
                  </Button>
                )}
                {status === "dispatched" && (
                  <Button size="sm" onClick={() => setAction("receive")}>
                    Terima
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => setAction("cancel")}
                >
                  Batalkan
                </Button>
              </div>
            )}
          </SectionCard>

          <div className="mt-6">
            <SectionCard title="Barang">
              <DataTable
                columns={[
                  {
                    header: "Produk",
                    render: (r) =>
                      details.get(r.productId)?.code ??
                      `#${r.productId}`,
                  },
                  {
                    header: "Varian",
                    render: (r) => {
                      const v = details
                        .get(r.productId)
                        ?.variants.find((x) => x.id === r.variantId);
                      return v ? `${v.code} — ${v.name}` : `#${r.variantId}`;
                    },
                  },
                  {
                    header: "Jumlah",
                    align: "right",
                    render: (r) => formatNumber(r.qty),
                  },
                  {
                    header: "Catatan",
                    render: (r) => r.notes || "-",
                  },
                ]}
                rows={transfer.items}
                rowKey={(r) => `${r.productId}-${r.variantId}`}
                emptyTitle="Tanpa barang"
              />
            </SectionCard>
          </div>
        </>
      ) : null}

      <ConfirmDialog
        open={action !== null}
        title={action ? ACTION_META[action].title : ""}
        message={action ? ACTION_META[action].message : ""}
        confirmLabel={action ? ACTION_META[action].confirmLabel : "Konfirmasi"}
        tone={action === "cancel" ? "danger" : "primary"}
        onConfirm={() => void runAction()}
        onCancel={() => setAction(null)}
      />
    </div>
  );
}
