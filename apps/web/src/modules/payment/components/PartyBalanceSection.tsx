import { useState } from "react";
import {
  CardSkeleton,
  DataTable,
  ErrorState,
  FormField,
  Pagination,
  SectionCard,
  StatusBadge,
  SummaryCard,
  TableSkeleton,
} from "@/shared/components/ui";
import { DocNumber, Money, Page } from "@/shared/components/patterns";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { formatDate, formatIDR } from "@/shared/lib/format";
import { paymentService } from "@/modules/payment/services/payment.service";
import { OpenPartiesList } from "@/modules/payment/components/OpenPartiesList";
import type { OpenParty, OrderBalance, PartyBalance, Payment } from "@/modules/payment/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { methodLabel, orderTypeLabel } from "@/modules/payment/types";

const LEDGER_LIMIT = 10;

/**
 * Saldo & buku pembayaran per pihak (read-only). Hutang (supplier) /
 * piutang (customer) dihitung live oleh backend — tanpa saldo tersimpan.
 */
export function PartyBalanceSection({
  canPay = false,
  refreshKey = 0,
  onPay,
}: {
  canPay?: boolean;
  /** Naik setelah pembayaran dicatat: daftar dan saldo dimuat ulang. */
  refreshKey?: number;
  /** "Catat bayar" pada daftar pihak bersaldo; induk membuka form pembayaran. */
  onPay?: (party: OpenParty) => void;
}) {
  const [partyId, setPartyId] = useState<number | null>(null);
  const [partyType, setPartyType] = useState<string | undefined>(undefined);
  const [ledgerPage, setLedgerPage] = useState(1);

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      if (!partyId) {
        return {
          balance: null as PartyBalance | null,
          ledger: [] as Payment[],
          total: 0,
        };
      }
      const [bal, led] = await Promise.all([
        paymentService.balance(partyId),
        paymentService.ledger(partyId, ledgerPage, LEDGER_LIMIT),
      ]);
      return { balance: bal, ledger: led.items, total: led.meta.total };
    },
    [partyId, ledgerPage, refreshKey],
  );
  const balance: PartyBalance | null = data?.balance ?? null;
  const ledger: Payment[] = data?.ledger ?? [];
  const ledgerTotal = data?.total ?? 0;

  const positionLabel = partyType === "supplier" ? "Hutang usaha (sisa)" : "Piutang usaha (sisa)";

  return (
    <Page>
      <OpenPartiesList
        canPay={canPay && onPay !== undefined}
        refreshKey={refreshKey}
        selectedPartyId={partyId}
        onSelect={(p) => {
          setPartyId(p.partyId);
          setPartyType(p.side === "payable" ? "supplier" : "customer");
          setLedgerPage(1);
        }}
        onPay={(p) => onPay?.(p)}
      />
      <SectionCard title="Saldo pihak" description="Pilih pihak untuk melihat posisi hutang/piutang live.">
        <div className="max-w-md">
          <FormField label="Pihak">
            <PartySearchSelect
              partyKind="any"
              value={partyId}
              onChange={(id, party) => {
                setPartyId(id);
                setPartyType(party?.type);
                setLedgerPage(1);
              }}
              placeholder="Cari customer / supplier…"
            />
          </FormField>
        </div>
      </SectionCard>

      {error && <ErrorState title="Gagal memuat saldo pihak" message={error} onRetry={reload} />}

      {!partyId ? (
        <SectionCard>
          <EmptyState
            title="Belum ada pihak dipilih"
            description="Pilih customer atau supplier untuk melihat saldo dan bukunya."
          />
        </SectionCard>
      ) : loading && !balance ? (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <CardSkeleton />
            <CardSkeleton />
            <CardSkeleton />
            <CardSkeleton />
          </div>
          <SectionCard flush>
            <TableSkeleton rows={4} cols={5} />
          </SectionCard>
        </div>
      ) : balance ? (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryCard
              label={positionLabel}
              value={formatIDR(balance.outstanding)}
              tone={balance.outstanding > 0 ? "danger" : "success"}
            />
            <SummaryCard label="Total ditagih" value={formatIDR(balance.totalBilled)} tone="brand" />
            <SummaryCard label="Sudah dibayar" value={formatIDR(balance.totalPaid)} tone="success" />
            <SummaryCard
              label="Dikurangi retur"
              value={formatIDR(balance.totalReturned)}
              hint={`Dikompensasi ${formatIDR(balance.totalRefunded)}`}
              tone="warning"
            />
          </div>

          <SectionCard flush title="Tagihan terbuka" description="Order beserta total, pembayaran, dan sisanya.">
            <BalanceTable rows={balance.orders} numberHeader="Nomor" paidHeader="Dibayar" emptyTitle="Tidak ada tagihan" emptyDescription="Pihak ini tidak punya order terbuka." />
          </SectionCard>

          {balance.returns.length > 0 && (
            <SectionCard flush title="Retur" description="Retur yang mengurangi tagihan pihak ini.">
              <BalanceTable rows={balance.returns} numberHeader="Nomor retur" paidHeader="Dikompensasi" emptyTitle="Tidak ada retur" />
            </SectionCard>
          )}

          <SectionCard flush title="Buku pembayaran" description="Pembayaran aktif pihak ini, terbaru dulu.">
            <DataTable<Payment>
              rows={ledger}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada pembayaran"
              columns={[
                { header: "Nomor", render: (r) => <DocNumber value={r.number} /> },
                { header: "Tanggal", render: (r) => formatDate(r.paidAt) },
                { header: "Metode", render: (r) => methodLabel(r.method) },
                { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
                {
                  header: "Jumlah",
                  align: "right",
                  render: (r) => <Money value={r.amount} className="font-medium" />,
                },
              ]}
            />
            <Pagination
              page={ledgerPage}
              limit={LEDGER_LIMIT}
              total={ledgerTotal}
              onPageChange={(p) => setLedgerPage(p)}
            />
          </SectionCard>
        </>
      ) : null}
    </Page>
  );
}

function BalanceTable({
  rows,
  numberHeader,
  paidHeader,
  emptyTitle,
  emptyDescription,
}: {
  rows: OrderBalance[];
  numberHeader: string;
  paidHeader: string;
  emptyTitle: string;
  emptyDescription?: string;
}) {
  return (
    <DataTable<OrderBalance>
      rows={rows}
      rowKey={(r) => `${r.orderType}-${r.orderId}`}
      emptyTitle={emptyTitle}
      emptyDescription={emptyDescription}
      columns={[
        { header: numberHeader, render: (r) => <DocNumber value={r.number} /> },
        { header: "Jenis", render: (r) => orderTypeLabel(r.orderType) },
        { header: "Total", align: "right", render: (r) => <Money value={r.grandTotal} /> },
        { header: paidHeader, align: "right", render: (r) => <Money value={r.paid} /> },
        { header: "Sisa", align: "right", render: (r) => <Money value={r.outstanding} className="font-medium" /> },
      ]}
    />
  );
}
