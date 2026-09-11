// Antrian kerja gudang (J3): SO terkonfirmasi yang masih bersisa + SJ draf
// yang fisiknya belum kembali. Satu endpoint, dua tab berhitung.

import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateInput,
  FilterBar,
  FormField,
  PageHeader,
  SectionCard,
  Segmented,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  formatNumber,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS, deliveryPrintPath } from "@/app/routes/route-paths";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { queueAgeLabel, queueAgeTone } from "@/modules/delivery/lib/queue";
import { QUEUE_AGES } from "@/modules/delivery/types";
import type { QueueOrder, WaitingNote } from "@/modules/delivery/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

type Tab = "create" | "waiting";

export default function QueuePage() {
  const can = useAuthStore((s) => s.can);
  const canCreate = can("delivery.create");

  const [tab, setTab] = useState<Tab>("create");
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [age, setAge] = useState("");

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      try {
        // Filter tanggal/umur berlaku untuk tab Menunggu (query SJ); tab Buat
        // memfilter tanggal order di klien dari hasil yang sama.
        const res = await deliveryService.workQueue({
          search: search || undefined,
          from: from || undefined,
          to: to || undefined,
          age: age || undefined,
          limit: 100,
        });
        const inRange =
          (d: string) =>
          (!from || d.slice(0, 10) >= from) && (!to || d.slice(0, 10) <= to);
        return {
          createRows: res.createSj.filter((r) => inRange(r.orderDate)),
          waitingRows: res.waitingReturn,
        };
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

  function applySearch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSearch(searchInput);
  }

  return (
    <div>
      <PageHeader
        eyebrow="Operasional"
        title="Antrian Pengiriman"
        description="Order terkonfirmasi yang masih bersisa + surat jalan yang fisiknya belum kembali."
      />

      <div className="mb-4">
        <Segmented
          value={tab}
          onChange={(v) => setTab(v as Tab)}
          options={[
            { value: "create", label: `Buat SJ (${createRows.length})` },
            { value: "waiting", label: `Menunggu Kembali (${waitingRows.length})` },
          ]}
        />
      </div>

      <FilterBar>
        <form onSubmit={applySearch} className="flex flex-wrap items-end gap-4">
          <div className="min-w-52 flex-1">
            <FormField label="Cari">
              <TextInput
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="No. order, no. SJ, atau customer…"
              />
            </FormField>
          </div>
          <div className="min-w-36">
            <FormField label="Dari">
              <DateInput
                value={from}
                max={to || undefined}
                onChange={(e) => setFrom(e.target.value)}
              />
            </FormField>
          </div>
          <div className="min-w-36">
            <FormField label="Sampai">
              <DateInput
                value={to}
                min={from || undefined}
                onChange={(e) => setTo(e.target.value)}
              />
            </FormField>
          </div>
          <div className="min-w-36">
            <FormField
              label="Umur"
              helperText={tab === "create" ? "Hanya tab Menunggu." : undefined}
            >
              <SelectInput
                value={age}
                disabled={tab !== "waiting"}
                onChange={(e) => setAge(e.target.value)}
              >
                {QUEUE_AGES.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Cari
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setSearchInput("");
              setSearch("");
              setFrom("");
              setTo("");
              setAge("");
            }}
          >
            Reset
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat antrean">
            {error}{" "}
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

      {tab === "create" ? (
        <SectionCard>
          {loading && createRows.length === 0 ? (
            <p className="text-muted py-8 text-center text-sm">
              Memuat antrean…
            </p>
          ) : (
            <DataTable<QueueOrder>
              rows={createRows}
              rowKey={(r) => r.orderId}
              emptyTitle="Tidak ada order yang perlu dibuatkan SJ"
              emptyDescription="Semua order terkonfirmasi sudah terkirim penuh atau sudah dibuatkan SJ."
              columns={[
                {
                  header: "Order / Customer",
                  render: (r) => (
                    <span>
                      <span className="font-semibold">{r.number}</span>{" "}
                      <span className="text-muted">{r.partyName || "-"}</span>
                    </span>
                  ),
                },
                {
                  header: "Tanggal",
                  render: (r) => (
                    <span>
                      {formatDate(r.orderDate)}
                      {r.dueDate ? (
                        <span className="text-muted">
                          {" "}
                          · tempo {formatDate(r.dueDate)}
                        </span>
                      ) : null}
                    </span>
                  ),
                },
                {
                  header: "Sisa",
                  render: (r) => (
                    <span className="block max-w-64 space-y-1">
                      {r.lines.map((l) => (
                        <span key={`${l.productId}/${l.variantId}`} className="block text-sm">
                          {l.productName}: sisa {formatNumber(l.remaining)} {l.uom}
                        </span>
                      ))}
                      {r.pendingDrafts > 0 ? (
                        <span className="text-muted block text-xs">
                          Ada {r.pendingDrafts} SJ draf menunggu kembali
                        </span>
                      ) : null}
                    </span>
                  ),
                },
                {
                  header: "Termin",
                  render: (r) => (
                    <span>
                      {r.paymentTerms || "-"}
                      <span className="text-muted block text-xs">
                        {formatIDR(r.grandTotal)}
                      </span>
                    </span>
                  ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.salesOrders}/${r.orderId}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {canCreate ? "Buat SJ" : "Lihat Order"}
                    </Link>
                  ),
                },
              ]}
            />
          )}
        </SectionCard>
      ) : (
        <SectionCard>
          {loading && waitingRows.length === 0 ? (
            <p className="text-muted py-8 text-center text-sm">
              Memuat antrean…
            </p>
          ) : (
            <DataTable<WaitingNote>
              rows={waitingRows}
              rowKey={(r) => r.id}
              emptyTitle="Tidak ada SJ yang menunggu kembali"
              emptyDescription="Semua surat jalan sudah dikonfirmasi atau belum diterbitkan."
              columns={[
                {
                  header: "SJ / Order",
                  render: (r) => (
                    <span>
                      <Link
                        to={`${ROUTE_PATHS.deliveries}/${r.id}`}
                        className="font-medium text-brand hover:underline"
                      >
                        {r.number}
                      </Link>{" "}
                      <span className="text-muted">{r.orderNumber}</span>
                    </span>
                  ),
                },
                {
                  header: "Customer",
                  render: (r) => r.partyName || "-",
                },
                {
                  header: "Kirim / Umur",
                  render: (r) => (
                    <span>
                      {formatDate(r.deliveryDate)}{" "}
                      <Badge tone={queueAgeTone(r.ageDays)}>
                        {queueAgeLabel(r.ageDays)}
                      </Badge>
                    </span>
                  ),
                },
                {
                  header: "Sopir",
                  render: (r) => (
                    <span>
                      {r.driverName || "-"}
                      {r.vehiclePlate ? (
                        <span className="text-muted block text-xs">
                          {r.vehiclePlate}
                        </span>
                      ) : null}
                    </span>
                  ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <span className="flex justify-end gap-3">
                      <a
                        href={deliveryPrintPath(r.id)}
                        target="_blank"
                        rel="noreferrer"
                        className="font-medium text-brand hover:underline"
                      >
                        Cetak
                      </a>
                      <Link
                        to={`${ROUTE_PATHS.deliveries}/${r.id}`}
                        className="font-medium text-brand hover:underline"
                      >
                        Konfirmasi
                      </Link>
                    </span>
                  ),
                },
              ]}
            />
          )}
        </SectionCard>
      )}
    </div>
  );
}
