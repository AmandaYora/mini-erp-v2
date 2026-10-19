// Antrian kerja gudang (J3): SO terkonfirmasi yang masih bersisa + SJ draf
// yang fisiknya belum kembali. Satu endpoint, dua tab berhitung (T7 konsol kerja).

import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateRangeFields,
  FormField,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  SelectInput,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import { DocNumber, Money, Page, Qty, Toolbar, useUrlTab } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS, deliveryPrintPath } from "@/app/routes/route-paths";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { queueAgeLabel, queueAgeTone } from "@/modules/delivery/lib/queue";
import { QUEUE_AGES } from "@/modules/delivery/types";
import type { QueueOrder, WaitingNote } from "@/modules/delivery/types";
import { paymentTermsLabel } from "@/modules/sales/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

const TABS = ["create", "waiting"] as const;
const PAGE_SIZE = 20;

export default function QueuePage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("delivery.create");

  const [tab, setTab] = useUrlTab(TABS, "create");
  const [search, setSearch] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [age, setAge] = useState("");
  // Pagination klien: endpoint antrean mengembalikan semua (maks 100) sekaligus.
  const [pages, setPages] = useState<Record<(typeof TABS)[number], number>>({ create: 1, waiting: 1 });

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      try {
        // Pencarian dan filter tanggal dijalankan server SEBELUM batas daftar
        // (DL-02): tab Siap dibuat memfilter tanggal order, tab Menunggu tanggal kirim.
        const res = await deliveryService.workQueue({
          search: search || undefined,
          from: from || undefined,
          to: to || undefined,
          age: age || undefined,
          limit: 100,
        });
        return { createRows: res.createSj, waitingRows: res.waitingReturn };
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat antrean", apiErr.message);
        throw err;
      }
    },
    [search, from, to, age],
  );
  const createRows: QueueOrder[] = data?.createRows ?? [];
  const waitingRows: WaitingNote[] = data?.waitingRows ?? [];

  function resetPages() {
    setPages({ create: 1, waiting: 1 });
  }
  function clearFilters() {
    setSearch("");
    setFrom("");
    setTo("");
    setAge("");
    resetPages();
  }
  const hasFilters = Boolean(search || from || to || age);
  const pageOf = (t: (typeof TABS)[number], rows: unknown[]) => {
    const last = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
    return Math.min(pages[t], last);
  };
  const createPage = pageOf("create", createRows);
  const waitingPage = pageOf("waiting", waitingRows);

  return (
    <Page>
      <PageHeader
        title="Antrian Pengiriman"
        description="Order terkonfirmasi yang masih bersisa dan surat jalan yang fisiknya belum kembali."
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari antrean"
            value={search}
            onSearch={(v) => {
              setSearch(v);
              resetPages();
            }}
            placeholder="No. order, no. SJ, atau pelanggan…"
          />
        }
        primary={
          <DateRangeFields
            from={from}
            to={to}
            fromLabel="Dari"
            toLabel="Sampai"
            onChange={({ from: f, to: t }) => {
              setFrom(f);
              setTo(t);
              resetPages();
            }}
          />
        }
        more={
          <FormField label="Umur" helperText="Berlaku untuk tab Menunggu konfirmasi.">
            <SelectInput
              aria-label="Umur"
              value={age}
              onChange={(e) => {
                setAge(e.target.value);
                resetPages();
              }}
            >
              {QUEUE_AGES.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </SelectInput>
          </FormField>
        }
        moreActiveCount={age ? 1 : 0}
        active={age ? [{ key: "age", label: `Umur: ${QUEUE_AGES.find((o) => o.value === age)?.label ?? age}`, onRemove: () => setAge("") }] : []}
        onClearAll={hasFilters ? clearFilters : undefined}
      />

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-4">
        <TabsList>
          <TabsTrigger value="create">Siap dibuat ({createRows.length})</TabsTrigger>
          <TabsTrigger value="waiting">Menunggu konfirmasi ({waitingRows.length})</TabsTrigger>
        </TabsList>

        <TabsContent value="create">
          <SectionCard flush>
            <DataTable<QueueOrder>
              rows={createRows.slice((createPage - 1) * PAGE_SIZE, createPage * PAGE_SIZE)}
              rowKey={(r) => r.orderId}
              loading={loading}
              error={error}
              onRetry={reload}
              onRowClick={(r) => navigate(`${ROUTE_PATHS.salesOrders}/${r.orderId}`)}
              emptyTitle={hasFilters ? "Tidak ada yang cocok dengan filter ini" : "Tidak ada order yang perlu dibuatkan surat jalan"}
              emptyDescription={
                hasFilters
                  ? "Ubah atau hapus filter untuk melihat order lain."
                  : "Semua order terkonfirmasi sudah terkirim penuh atau sudah dibuatkan surat jalan."
              }
              emptyAction={
                hasFilters ? (
                  <Button variant="secondary" size="sm" onClick={clearFilters}>
                    Hapus filter
                  </Button>
                ) : undefined
              }
              columns={[
                {
                  header: "Order / pelanggan",
                  render: (r) => (
                    <div>
                      <Link to={`${ROUTE_PATHS.salesOrders}/${r.orderId}`} className="font-medium text-primary hover:underline">
                        <DocNumber value={r.number} />
                      </Link>
                      <p className="text-xs text-muted-foreground">{r.partyName || "Walk-in"}</p>
                    </div>
                  ),
                },
                {
                  header: "Tanggal",
                  render: (r) => (
                    <div>
                      {formatDate(r.orderDate)}
                      {r.dueDate ? <p className="text-xs text-muted-foreground">Tempo {formatDate(r.dueDate)}</p> : null}
                    </div>
                  ),
                },
                {
                  header: "Sisa dikirim",
                  render: (r) => (
                    <div className="max-w-64 space-y-0.5">
                      <p className="text-xs font-medium text-muted-foreground">{r.itemCount} item</p>
                      {r.lines.map((l) => (
                        <p key={`${l.productId}/${l.variantId}`} className="text-sm">
                          {l.productName}: sisa <Qty value={l.remaining} unit={l.uom} />
                        </p>
                      ))}
                      {r.pendingDrafts > 0 ? (
                        <p className="text-xs text-muted-foreground">Ada {r.pendingDrafts} SJ draf menunggu kembali</p>
                      ) : null}
                    </div>
                  ),
                },
                {
                  header: "Termin / total",
                  align: "right",
                  render: (r) => (
                    <div>
                      <Money value={r.grandTotal} />
                      <p className="text-xs text-muted-foreground">{paymentTermsLabel(r.paymentTerms)}</p>
                    </div>
                  ),
                },
                {
                  header: "",
                  align: "right",
                  render: (r) =>
                    canCreate ? (
                      <Button asChild size="sm">
                        <Link to={`${ROUTE_PATHS.deliveries}/new?from=${r.orderId}`}>Buat surat jalan</Link>
                      </Button>
                    ) : (
                      <Button asChild size="sm" variant="secondary">
                        <Link to={`${ROUTE_PATHS.salesOrders}/${r.orderId}`}>Lihat order</Link>
                      </Button>
                    ),
                },
              ]}
            />
            <Pagination
              page={createPage}
              limit={PAGE_SIZE}
              total={createRows.length}
              onPageChange={(p) => setPages((s) => ({ ...s, create: p }))}
            />
          </SectionCard>
        </TabsContent>

        <TabsContent value="waiting">
          <SectionCard flush>
            <DataTable<WaitingNote>
              rows={waitingRows.slice((waitingPage - 1) * PAGE_SIZE, waitingPage * PAGE_SIZE)}
              rowKey={(r) => r.id}
              loading={loading}
              error={error}
              onRetry={reload}
              onRowClick={(r) => navigate(`${ROUTE_PATHS.deliveries}/${r.id}`)}
              emptyTitle={hasFilters ? "Tidak ada yang cocok dengan filter ini" : "Tidak ada surat jalan yang menunggu konfirmasi"}
              emptyDescription={
                hasFilters
                  ? "Ubah atau hapus filter untuk melihat surat jalan lain."
                  : "Semua surat jalan sudah dikonfirmasi atau belum diterbitkan."
              }
              emptyAction={
                hasFilters ? (
                  <Button variant="secondary" size="sm" onClick={clearFilters}>
                    Hapus filter
                  </Button>
                ) : undefined
              }
              columns={[
                {
                  header: "SJ / order",
                  render: (r) => (
                    <div>
                      <Link to={`${ROUTE_PATHS.deliveries}/${r.id}`} className="font-medium text-primary hover:underline">
                        <DocNumber value={r.number} />
                      </Link>
                      <p className="text-xs text-muted-foreground">{r.orderNumber}</p>
                      <Badge tone={r.hasSignedDocument ? "success" : "warning"}>
                        {r.hasSignedDocument ? "SJ ttd terunggah" : "SJ ttd belum diunggah"}
                      </Badge>
                    </div>
                  ),
                },
                { header: "Pelanggan", render: (r) => r.partyName || "–" },
                {
                  header: "Kirim / umur",
                  render: (r) => (
                    <span className="inline-flex flex-wrap items-center gap-2">
                      {formatDate(r.deliveryDate)}
                      <Badge tone={queueAgeTone(r.ageDays)}>{queueAgeLabel(r.ageDays)}</Badge>
                    </span>
                  ),
                },
                {
                  header: "Sopir",
                  render: (r) => (
                    <div>
                      {r.driverName || "–"}
                      {r.vehiclePlate ? <p className="text-xs text-muted-foreground">{r.vehiclePlate}</p> : null}
                    </div>
                  ),
                },
                {
                  header: "",
                  align: "right",
                  render: (r) => (
                    <span className="inline-flex justify-end gap-2">
                      <Button asChild size="sm" variant="secondary">
                        <a href={deliveryPrintPath(r.id)} target="_blank" rel="noreferrer">
                          Cetak
                        </a>
                      </Button>
                      <Button asChild size="sm">
                        <Link to={`${ROUTE_PATHS.deliveries}/${r.id}`}>Konfirmasi</Link>
                      </Button>
                    </span>
                  ),
                },
              ]}
            />
            <Pagination
              page={waitingPage}
              limit={PAGE_SIZE}
              total={waitingRows.length}
              onPageChange={(p) => setPages((s) => ({ ...s, waiting: p }))}
            />
          </SectionCard>
        </TabsContent>
      </Tabs>
    </Page>
  );
}
