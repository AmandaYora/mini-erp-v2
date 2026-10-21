import { useState } from "react";
import { useParams } from "react-router-dom";
import {
  Button,
  ConfirmDialog,
  DataTable,
  DetailSkeleton,
  ErrorState,
  PageHeader,
  SectionCard,
  StatusBadge,
} from "@/shared/components/ui";
import {
  ActionMenu,
  cancellationItems,
  DescriptionList,
  DocNumber,
  DocumentRail,
  Page,
  PageColumns,
  Qty,
  ReasonDialog,
} from "@/shared/components/patterns";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS, transferPrintPath } from "@/app/routes/route-paths";
import { stockService } from "@/modules/stock/services/stock.service";
import { transferRail } from "@/modules/stock/lib/transfer-rail";
import type {
  BranchOption,
  ProductDetail,
  StockLocation,
  StockTransfer,
  TransferItem,
} from "@/modules/stock/types";

type Action = "dispatch" | "receive" | "cancel";

const ACTION_META: Record<Action, { title: string; message: string; confirmLabel: string; done: string }> = {
  dispatch: {
    title: "Kirim transfer",
    message:
      "Kirim transfer ini? Stok keluar dicatat di lokasi asal dan tidak bisa dibatalkan sepihak setelah diterima.",
    confirmLabel: "Kirim transfer",
    done: "Transfer dikirim",
  },
  receive: {
    title: "Terima transfer",
    message: "Tandai transfer ini sudah diterima? Stok masuk dicatat di lokasi tujuan.",
    confirmLabel: "Terima transfer",
    done: "Transfer diterima",
  },
  cancel: {
    title: "Batalkan transfer",
    message:
      "Transfer dibatalkan; bila sudah dikirim, stok kembali ke lokasi asal. Alasan dicatat pada transfer dan jejak audit.",
    confirmLabel: "Batalkan transfer",
    done: "Transfer dibatalkan",
  },
};

export default function TransferDetailPage() {
  const { id } = useParams<{ id: string }>();
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.transfer");
  const [action, setAction] = useState<Action | null>(null);
  const [running, setRunning] = useState(false);

  const transferId = Number(id);
  const validId = Number.isInteger(transferId) && transferId > 0;

  const { data, loading, error, errorStatus, reload } = useAsyncData(
    async () => {
      if (!validId) throw new Error("ID transfer tidak valid.");
      const [t, branchRows, locationRows] = await Promise.all([
        stockService.getTransfer(transferId),
        stockService.listBranches().catch(() => [] as BranchOption[]),
        // Hanya lokasi cabang aktif; lokasi cabang lain jatuh ke "#id".
        stockService.listLocations().catch(() => [] as StockLocation[]),
      ]);
      // Nama produk/varian best-effort: transferView hanya membawa ID.
      const productIds = [...new Set(t.items.map((it) => it.productId))];
      const fetched = await Promise.all(productIds.map((pid) => stockService.getProduct(pid).catch(() => null)));
      const details = new Map<number, ProductDetail>();
      for (const d of fetched) if (d) details.set(d.id, d);
      return { transfer: t, details, branches: branchRows, locations: locationRows };
    },
    [transferId],
  );
  const transfer: StockTransfer | null = data?.transfer ?? null;
  usePageCrumb(transfer?.number);

  const branchName = (branchId: number) => {
    const b = data?.branches.find((x) => x.id === branchId);
    return b ? `${b.code} — ${b.name}` : `#${branchId}`;
  };
  const locationName = (locationId: number) => {
    const l = data?.locations.find((x) => x.id === locationId);
    return l ? `${l.code} — ${l.name}` : `Lokasi #${locationId}`;
  };

  async function runAction(reason = "") {
    if (!action || !transfer) return;
    const current = action;
    setRunning(true);
    try {
      const res =
        current === "dispatch"
          ? await stockService.dispatchTransfer(transfer.id)
          : current === "receive"
            ? await stockService.receiveTransfer(transfer.id)
            : await stockService.cancelTransfer(transfer.id, reason);
      toast.fromServer(res.message, ACTION_META[current].done);
      setAction(null);
      reload();
    } catch (err) {
      // Dialog alasan tetap terbuka agar alasan yang sudah diketik tidak hilang.
      if (current !== "cancel") setAction(null);
      toast.danger("Aksi gagal", formatApiError(err));
    } finally {
      setRunning(false);
    }
  }

  if (!validId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Transfer tidak ditemukan"
        message="Transfer ini tidak ada atau sudah dihapus."
        backTo={ROUTE_PATHS.stockTransfers}
        backLabel="Ke daftar transfer"
      />
    );
  }
  if (loading && !transfer) return <DetailSkeleton />;
  if (error && !transfer) {
    return <ErrorState message={error} onRetry={reload} title="Gagal memuat transfer" />;
  }
  if (!transfer || !data) return null;

  const status = transfer.status;
  const rail = transferRail(status);
  const canDispatch = canManage && status === "draft";
  const canReceive = canManage && status === "dispatched";
  const canCancel = canManage && (status === "draft" || status === "dispatched");

  return (
    <Page>
      <PageHeader
        title={transfer.number}
        meta={<StatusBadge status={status} />}
        subtitle={`${branchName(transfer.fromBranchId)} → ${branchName(transfer.toBranchId)}`}
        actions={
          <>
            <Button variant="secondary" asChild>
              <a href={transferPrintPath(transfer.id)} target="_blank" rel="noreferrer">
                Cetak surat transfer
              </a>
            </Button>
            {canCancel && (
              <ActionMenu
                items={[{ key: "cancel", label: "Batalkan transfer", destructive: true, onSelect: () => setAction("cancel") }]}
              />
            )}
            {canDispatch && <Button onClick={() => setAction("dispatch")}>Kirim</Button>}
            {canReceive && <Button onClick={() => setAction("receive")}>Terima</Button>}
          </>
        }
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} />
      </PageHeader>

      {status === "draft" && (
        <Notice tone="info" title="Stok belum bergerak">
          Stok keluar dari lokasi asal baru dicatat saat transfer dikirim.
        </Notice>
      )}
      {error && (
        <ErrorState message={error} onRetry={reload} title="Gagal menyegarkan transfer" />
      )}

      <PageColumns
        main={
          <SectionCard title="Barang" flush>
            <DataTable<TransferItem>
              rows={transfer.items}
              rowKey={(r) => `${r.productId}-${r.variantId}`}
              emptyTitle="Tanpa barang"
              columns={[
                {
                  header: "Produk",
                  render: (r) => {
                    const d = data.details.get(r.productId);
                    const v = d?.variants.find((x) => x.id === r.variantId);
                    return (
                      <div>
                        <div className="font-medium">{d ? `${d.code} — ${d.name}` : `#${r.productId}`}</div>
                        <div className="text-xs text-muted-foreground">
                          {v ? `${v.code} — ${v.name}` : `Varian #${r.variantId}`}
                        </div>
                      </div>
                    );
                  },
                },
                { header: "Jumlah", align: "right", render: (r) => <Qty value={r.qty} /> },
                {
                  header: "Tujuan baris",
                  render: (r) =>
                    r.toLocationId && r.toLocationId !== transfer.toLocationId
                      ? locationName(r.toLocationId)
                      : "Sesuai dokumen",
                },
                { header: "Catatan", render: (r) => r.notes || "–" },
              ]}
            />
          </SectionCard>
        }
        aside={
          <SectionCard title="Info dokumen">
            <DescriptionList
              columns={1}
              items={[
                { label: "Nomor", value: <DocNumber value={transfer.number} copyable /> },
                { label: "Status", value: <StatusBadge status={status} /> },
                { label: "Cabang asal", value: branchName(transfer.fromBranchId) },
                { label: "Lokasi asal", value: locationName(transfer.fromLocationId) },
                { label: "Cabang tujuan", value: branchName(transfer.toBranchId) },
                { label: "Lokasi tujuan", value: locationName(transfer.toLocationId) },
                { label: "Tanggal transfer", value: transfer.transferDate ? formatDate(transfer.transferDate) : null },
                { label: "Sopir", value: transfer.driverName },
                { label: "Kendaraan", value: transfer.vehiclePlate },
                ...cancellationItems(transfer),
                { label: "Catatan", value: transfer.notes, wide: true },
              ]}
            />
          </SectionCard>
        }
      />

      <ConfirmDialog
        open={action === "dispatch" || action === "receive"}
        title={action ? ACTION_META[action].title : ""}
        message={action ? ACTION_META[action].message : ""}
        confirmLabel={running ? "Memproses…" : action ? ACTION_META[action].confirmLabel : "Konfirmasi"}
        tone="primary"
        onConfirm={() => {
          if (!running) void runAction();
        }}
        onCancel={() => {
          if (!running) setAction(null);
        }}
      />
      <ReasonDialog
        open={action === "cancel"}
        title={ACTION_META.cancel.title}
        description={`${transfer.number}: ${ACTION_META.cancel.message}`}
        confirmLabel={ACTION_META.cancel.confirmLabel}
        placeholder="Mis. salah lokasi tujuan, jumlah salah"
        submitting={running}
        onClose={() => {
          if (!running) setAction(null);
        }}
        onSubmit={(reason) => void runAction(reason)}
      />
    </Page>
  );
}
