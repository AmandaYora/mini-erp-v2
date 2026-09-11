import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatNumber,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import { CreateDeliveryModal } from "@/modules/delivery/components/CreateDeliveryModal";
import { DELIVERY_STATUSES } from "@/modules/delivery/types";
import type { DeliveryNote } from "@/modules/delivery/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

const LIMIT = 20;

// Kolom tabel hanya dari kunci noteView yang benar-benar ada: number,
// deliveryDate, orderNumber, status, items. Backend TIDAK menyediakan customer
// maupun pencarian teks — filter = status + salesOrderId (param List).

export default function DeliveriesPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("delivery.create");

  const [status, setStatus] = useState("");
  const [soInput, setSoInput] = useState("");
  const [salesOrderId, setSalesOrderId] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      deliveryService
        .list({
          salesOrderId: salesOrderId ?? undefined,
          status: status || undefined,
          page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat pengiriman", apiErr.message);
          throw err;
        }),
    [salesOrderId, status, page],
  );
  const items: DeliveryNote[] = data?.items ?? [];
  const total = data?.total ?? 0;

  function applyFilter(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const n = Number(soInput.trim());
    setSalesOrderId(
      soInput.trim() !== "" && Number.isInteger(n) && n > 0 ? n : null,
    );
    setPage(1);
  }

  return (
    <div>
      <PageHeader
        eyebrow="Operasional"
        title="Pengiriman"
        description="Surat jalan per sales order — klik nomor untuk detail, konfirmasi, dan bukti kirim."
        actions={
          canCreate ? (
            <Button onClick={() => setModalOpen(true)}>
              Buat Surat Jalan
            </Button>
          ) : undefined
        }
      />

      <FilterBar>
        <form onSubmit={applyFilter} className="flex flex-wrap items-end gap-4">
          <div className="min-w-44">
            <FormField label="Status">
              <SelectInput
                value={status}
                onChange={(e) => {
                  setStatus(e.target.value);
                  setPage(1);
                }}
              >
                <option value="">Semua</option>
                {DELIVERY_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {statusLabel(s)}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
          <div className="min-w-52 flex-1">
            <FormField
              label="Sales Order ID"
              helperText="Backend hanya mendukung filter salesOrderId."
            >
              <TextInput
                value={soInput}
                onChange={(e) => setSoInput(e.target.value)}
                placeholder="cth: 12 (kosongkan = semua)"
                inputMode="numeric"
              />
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Cari
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
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

      <SectionCard>
        {loading && items.length === 0 ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<DeliveryNote>
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada surat jalan"
              emptyDescription="Buat surat jalan baru atau ubah filter."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.deliveries}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                {
                  header: "Tanggal",
                  render: (r) => formatDate(r.deliveryDate),
                },
                { header: "No. SO", render: (r) => r.orderNumber || "-" },
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
                    <Link
                      to={`${ROUTE_PATHS.deliveries}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      Detail
                    </Link>
                  ),
                },
              ]}
            />
            <Pagination
              page={page}
              limit={LIMIT}
              total={total}
              onPageChange={(p) => setPage(p)}
            />
          </>
        )}
      </SectionCard>

      <CreateDeliveryModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onCreated={(id) => {
          setModalOpen(false);
          reload();
          navigate(`${ROUTE_PATHS.deliveries}/${id}`);
        }}
      />
    </div>
  );
}
