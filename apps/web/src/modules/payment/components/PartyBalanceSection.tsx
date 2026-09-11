import { useState } from "react";
import {
  DataTable,
  FormField,
  Pagination,
  SearchSelect,
  SectionCard,
  SummaryCard,
} from "@/shared/components/ui";
import type { SearchSelectOption } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { formatDate, formatIDR, statusLabel } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { paymentService } from "@/modules/payment/services/payment.service";
import type {
  PartyBalance,
  PartyLookup,
  Payment,
} from "@/modules/payment/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { methodLabel, orderTypeLabel } from "@/modules/payment/types";

const LEDGER_LIMIT = 10;

interface PartyBalanceSectionProps {
  parties: PartyLookup[];
}

/**
 * Saldo & buku pembayaran per pihak (read-only). Hutang (supplier) /
 * piutang (customer) dihitung live oleh backend — tanpa saldo tersimpan.
 */
export function PartyBalanceSection({ parties }: PartyBalanceSectionProps) {
  const [partyId, setPartyId] = useState<number | null>(null);
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
      try {
        const [bal, led] = await Promise.all([
          paymentService.balance(partyId),
          paymentService.ledger(partyId, ledgerPage, LEDGER_LIMIT),
        ]);
        return { balance: bal, ledger: led.items, total: led.meta.total };
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat saldo pihak", apiErr.message);
        throw err;
      }
    },
    [partyId, ledgerPage],
  );
  const balance: PartyBalance | null = data?.balance ?? null;
  const ledger: Payment[] = data?.ledger ?? [];
  const ledgerTotal = data?.total ?? 0;

  const partyOptions: SearchSelectOption[] = parties.map((p) => ({
    value: p.id,
    label: `${p.name} (${p.type === "supplier" ? "Supplier" : "Customer"})`,
  }));
  const partyType = parties.find((p) => p.id === partyId)?.type;
  const positionLabel =
    partyType === "supplier" ? "Hutang Usaha (sisa)" : "Piutang Usaha (sisa)";

  return (
    <div className="flex flex-col gap-4">
      <SectionCard
        title="Saldo Pihak"
        description="Pilih pihak untuk melihat posisi hutang/piutang live."
      >
        <div className="max-w-md">
          <FormField label="Pihak">
            <SearchSelect
              options={partyOptions}
              value={partyId}
              allowClear
              onChange={(v) => {
                setPartyId(
                  typeof v === "number" ? v : v === null ? null : Number(v),
                );
                setLedgerPage(1);
              }}
              placeholder="Pilih customer / supplier…"
            />
          </FormField>
        </div>

        {error && (
          <div className="mt-4">
            <Notice tone="danger" title={error}>
              <button
                type="button"
                onClick={() => reload()}
                className="cursor-pointer font-semibold text-brand hover:underline"
              >
                Coba lagi
              </button>
            </Notice>
          </div>
        )}

        {!partyId ? (
          <div className="mt-4">
            <EmptyState
              title="Belum ada pihak dipilih"
              description="Pilih customer atau supplier untuk melihat saldo dan bukunya."
            />
          </div>
        ) : loading && !balance ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : balance ? (
          <div className="mt-4 flex flex-col gap-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <SummaryCard
                label={positionLabel}
                value={formatIDR(balance.outstanding)}
                tone={balance.outstanding > 0 ? "danger" : "success"}
              />
              <SummaryCard
                label="Total Ditagih"
                value={formatIDR(balance.totalBilled)}
                tone="brand"
              />
              <SummaryCard
                label="Sudah Dibayar"
                value={formatIDR(balance.totalPaid)}
                tone="success"
              />
              <SummaryCard
                label="Dikurangi Retur"
                value={formatIDR(balance.totalReturned)}
                hint={`Dikompensasi ${formatIDR(balance.totalRefunded)}`}
                tone="warning"
              />
            </div>

            <DataTable
              rows={balance.orders}
              rowKey={(r) => `${r.orderType}-${r.orderId}`}
              emptyTitle="Tidak ada tagihan"
              emptyDescription="Pihak ini tidak punya order terbuka."
              columns={[
                { header: "Nomor", render: (r) => r.number },
                { header: "Tipe", render: (r) => orderTypeLabel(r.orderType) },
                {
                  header: "Total",
                  align: "right",
                  render: (r) => formatIDR(r.grandTotal),
                },
                {
                  header: "Dibayar",
                  align: "right",
                  render: (r) => formatIDR(r.paid),
                },
                {
                  header: "Sisa",
                  align: "right",
                  render: (r) => (
                    <span className="font-medium">
                      {formatIDR(r.outstanding)}
                    </span>
                  ),
                },
              ]}
            />

            {balance.returns.length > 0 && (
              <DataTable
                rows={balance.returns}
                rowKey={(r) => `${r.orderType}-${r.orderId}`}
                emptyTitle="Tidak ada retur"
                columns={[
                  { header: "Nomor Retur", render: (r) => r.number },
                  { header: "Tipe", render: (r) => orderTypeLabel(r.orderType) },
                  {
                    header: "Total",
                    align: "right",
                    render: (r) => formatIDR(r.grandTotal),
                  },
                  {
                    header: "Dikompensasi",
                    align: "right",
                    render: (r) => formatIDR(r.paid),
                  },
                  {
                    header: "Sisa",
                    align: "right",
                    render: (r) => (
                      <span className="font-medium">
                        {formatIDR(r.outstanding)}
                      </span>
                    ),
                  },
                ]}
              />
            )}
          </div>
        ) : null}
      </SectionCard>

      {partyId && balance && (
        <SectionCard
          title="Buku Pembayaran"
          description="Pembayaran aktif pihak ini, terbaru dulu."
        >
          <DataTable<Payment>
            rows={ledger}
            rowKey={(r) => r.id}
            emptyTitle="Belum ada pembayaran"
            columns={[
              { header: "Nomor", render: (r) => r.number },
              { header: "Tanggal", render: (r) => formatDate(r.paidAt) },
              { header: "Metode", render: (r) => methodLabel(r.method) },
              { header: "Status", render: (r) => statusLabel(r.status) },
              {
                header: "Jumlah",
                align: "right",
                render: (r) => (
                  <span className="font-medium">{formatIDR(r.amount)}</span>
                ),
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
      )}
    </div>
  );
}
