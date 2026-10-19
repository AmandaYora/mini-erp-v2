import { useState } from "react";
import { Link } from "react-router-dom";
import {
  DataTable,
  DateRangeFields,
  ErrorState,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  StatusBadge,
  SummaryCard,
  Tabs,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import { DocNumber, Money, Page, Toolbar } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, formatNumber, monthStartWIB, statusLabel, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { reportingService } from "@/modules/reporting/services/reporting.service";
import { orderDetailPath, validRange } from "@/modules/reporting/lib/order-report";
import type { OrderReportRow, OrderReportType } from "@/modules/reporting/types";

const LIMIT = 50;
const STATUSES = ["draft", "confirmed", "completed", "cancelled"];

export default function OrdersReportPage() {
  const [type, setType] = useState<OrderReportType>("sales");
  const [status, setStatus] = useState("");
  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const [page, setPage] = useState(1);
  const rangeOk = validRange(from, to);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      rangeOk ? reportingService.orders({ type, status, from, to, page, limit: LIMIT }, signal) : Promise.resolve(null),
    [type, status, from, to, page, rangeOk],
  );

  return (
    <Page>
      <PageHeader
        title="Laporan Order"
        description="Order penjualan atau pembelian cabang aktif per rentang tanggal dan status, dengan total keseluruhan."
      />

      <Tabs
        value={type}
        onValueChange={(v) => {
          setType(v as OrderReportType);
          setPage(1);
        }}
      >
        <TabsList aria-label="Jenis order">
          <TabsTrigger value="sales">Penjualan</TabsTrigger>
          <TabsTrigger value="purchasing">Pembelian</TabsTrigger>
        </TabsList>
      </Tabs>

      <Toolbar
        primary={
          <>
            <FormField label="Status">
              <SelectInput
                aria-label="Status"
                value={status}
                onChange={(e) => {
                  setStatus(e.target.value);
                  setPage(1);
                }}
              >
                <option value="">Semua</option>
                {STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {statusLabel(s)}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <DateRangeFields
              from={from}
              to={to}
              onChange={(r) => {
                setFrom(r.from);
                setTo(r.to);
                setPage(1);
              }}
            />
          </>
        }
      />

      {!rangeOk ? (
        <Notice tone="warning" title="Rentang tanggal tidak valid">
          Tanggal akhir sebelum tanggal awal.
        </Notice>
      ) : error ? (
        <ErrorState title="Gagal memuat laporan order" message={error} onRetry={reload} />
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SummaryCard label="Jumlah order" value={data ? formatNumber(data.total) : "-"} />
            <SummaryCard label="Total nilai (semua halaman)" value={data ? formatIDR(data.grandTotal) : "-"} tone="success" />
          </div>
          <SectionCard
            title={type === "sales" ? "Order penjualan" : "Order pembelian"}
            description={`Periode ${formatDate(from)} – ${formatDate(to)}.`}
            flush
          >
            <DataTable<OrderReportRow>
              rows={data?.rows}
              rowKey={(r) => r.id}
              loading={loading}
              emptyTitle="Tidak ada order"
              emptyDescription="Tidak ada order pada filter ini."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link to={orderDetailPath(data?.type ?? type, r.id)} className="font-semibold text-primary hover:underline">
                      <DocNumber value={r.number} />
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.orderDate.slice(0, 10)) },
                { header: type === "sales" ? "Pelanggan" : "Pemasok", render: (r) => r.partyName || "-" },
                { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
                { header: "Total", align: "right", render: (r) => <Money value={r.grandTotal} /> },
              ]}
            />
            {data && <Pagination page={page} limit={LIMIT} total={data.total} onPageChange={setPage} />}
          </SectionCard>
        </>
      )}
    </Page>
  );
}
