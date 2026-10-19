import { Link } from "react-router-dom";
import { Button, DataTable, SectionCard, StatusBadge } from "@/shared/components/ui";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { TableSkeleton } from "@/shared/components/feedback/skeleton";
import { DescriptionList, DocNumber, Money } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { partyActivityService } from "@/modules/party/services/party.service";
import type { PartyOrderBalance, PartyOrderRow } from "@/modules/party/types";

/** Tab "Transaksi": order terbaru pihak ini (penjualan untuk customer, pembelian untuk supplier). */
export function PartyTransactionsPanel({ kind, partyId }: { kind: "customer" | "supplier"; partyId: number }) {
  const base = kind === "customer" ? ROUTE_PATHS.salesOrders : ROUTE_PATHS.purchaseOrders;
  const noun = kind === "customer" ? "order jual" : "order beli";
  const { data, loading, error, reload } = useAsyncData(
    () => partyActivityService.recentOrders(kind, partyId).then((res) => ({ items: res.items, total: res.meta.total })),
    [kind, partyId],
  );
  const total = data?.total ?? 0;

  return (
    <SectionCard
      title="Transaksi"
      description={`Sepuluh ${noun} terbaru untuk pihak ini.`}
      actions={
        total > 0 ? (
          <Button variant="secondary" size="sm" asChild>
            <Link to={`${base}?partyId=${partyId}`}>Lihat semua ({total})</Link>
          </Button>
        ) : undefined
      }
      flush
    >
      <DataTable<PartyOrderRow>
        caption={`Riwayat ${noun}`}
        rows={data?.items ?? null}
        rowKey={(r) => r.id}
        loading={loading}
        error={error}
        onRetry={reload}
        emptyTitle={`Belum ada ${noun}`}
        emptyDescription={`${kind === "customer" ? "Customer" : "Supplier"} ini belum punya transaksi.`}
        columns={[
          {
            header: "Nomor",
            render: (r) => (
              <Link to={`${base}/${r.id}`} className="text-primary hover:underline">
                <DocNumber value={r.number} />
              </Link>
            ),
          },
          { header: "Tanggal", render: (r) => formatDate(r.orderDate) },
          { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
          { header: "Total", align: "right", render: (r) => <Money value={r.grandTotal} /> },
        ]}
      />
    </SectionCard>
  );
}

/** Tab "Saldo": tagihan, pembayaran, dan sisa (perlu izin payment.view). */
export function PartyBalancePanel({ partyId }: { partyId: number }) {
  const { data, loading, error, reload } = useAsyncData(() => partyActivityService.balance(partyId), [partyId]);
  const open = (data?.orders ?? []).filter((o) => o.outstanding !== 0);

  return (
    <div className="flex flex-col gap-6">
      <SectionCard title="Ringkasan saldo" description="Dihitung dari seluruh order dan pembayaran pihak ini.">
        {error ? (
          <ErrorState message={error} onRetry={reload} />
        ) : loading && !data ? (
          <TableSkeleton rows={2} cols={3} />
        ) : data ? (
          <DescriptionList
            columns={3}
            items={[
              { label: "Total tagihan", value: <Money value={data.totalBilled} /> },
              { label: "Sudah dibayar", value: <Money value={data.totalPaid} /> },
              { label: "Retur", value: <Money value={data.totalReturned} /> },
              { label: "Dikembalikan", value: <Money value={data.totalRefunded} /> },
              { label: "Sisa", value: <Money value={data.outstanding} className="font-semibold" /> },
            ]}
          />
        ) : null}
      </SectionCard>

      {data && (
        <SectionCard title="Order belum lunas" flush>
          <DataTable<PartyOrderBalance>
            caption="Order dengan sisa tagihan"
            rows={open}
            rowKey={(r) => `${r.orderType}-${r.orderId}`}
            emptyTitle="Tidak ada sisa tagihan"
            emptyDescription="Semua order pihak ini sudah lunas."
            columns={[
              { header: "Nomor", render: (r) => <DocNumber value={r.number} /> },
              { header: "Total", align: "right", render: (r) => <Money value={r.grandTotal} /> },
              { header: "Dibayar", align: "right", render: (r) => <Money value={r.paid} /> },
              { header: "Sisa", align: "right", render: (r) => <Money value={r.outstanding} className="font-medium" /> },
            ]}
          />
        </SectionCard>
      )}
    </div>
  );
}
