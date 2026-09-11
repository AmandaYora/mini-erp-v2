import { useMemo, useState } from "react";
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
  formatIDR,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { salesReturnService } from "@/modules/salesreturn/services/sales-return.service";
import { SALES_RETURN_STATUSES } from "@/modules/salesreturn/types";
import type { SalesReturn } from "@/modules/salesreturn/types";
import { SalesReturnCreateModal } from "@/modules/salesreturn/components/SalesReturnCreateModal";

const LIMIT = 20;

export default function SalesReturnsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("salesreturn.create");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      salesReturnService
        .list({ status, page, limit: LIMIT })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [status, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  function applySearch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSearch(searchInput.trim());
    setPage(1);
  }

  // Backend list TIDAK mendukung param search (handler.go: List) —
  // pencarian nomor difilter sisi klien pada halaman aktif.
  const visible = useMemo(() => {
    const rows = data?.items ?? [];
    const q = search.toLowerCase();
    if (q === "") return rows;
    return rows.filter(
      (r) =>
        r.number.toLowerCase().includes(q) ||
        r.orderNumber.toLowerCase().includes(q),
    );
  }, [data, search]);

  return (
    <div>
      <PageHeader
        eyebrow="Operasional"
        title="Retur Jual"
        description="Retur penjualan terhadap order jual terkonfirmasi — klik nomor untuk detail dan proses."
        actions={
          canCreate ? (
            <Button onClick={() => setModalOpen(true)}>Buat Retur</Button>
          ) : undefined
        }
      />

      <FilterBar>
        <form onSubmit={applySearch} className="flex flex-wrap items-end gap-4">
          <div className="min-w-52 flex-1">
            <FormField label="Cari" helperText="Filter nomor pada halaman ini.">
              <TextInput
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Nomor retur / nomor SO…"
              />
            </FormField>
          </div>
          <FormField label="Status">
            <SelectInput
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                setPage(1);
              }}
            >
              <option value="">Semua</option>
              {SALES_RETURN_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {statusLabel(s)}
                </option>
              ))}
            </SelectInput>
          </FormField>
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
            <DataTable<SalesReturn>
              rows={visible}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada retur penjualan"
              emptyDescription="Buat retur baru atau ubah kata kunci pencarian."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.salesReturns}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.returnDate) },
                { header: "Ref SO", render: (r) => r.orderNumber || "-" },
                {
                  header: "Total",
                  align: "right",
                  render: (r) => formatIDR(r.total),
                },
                {
                  header: "Status",
                  render: (r) => (
                    <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge>
                  ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.salesReturns}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      Detail
                    </Link>
                  ),
                },
              ]}
            />
            {search === "" && (
              <Pagination
                page={page}
                limit={LIMIT}
                total={total}
                onPageChange={(p) => setPage(p)}
              />
            )}
          </>
        )}
      </SectionCard>

      {canCreate && (
        <SalesReturnCreateModal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          onCreated={(ret) => navigate(`${ROUTE_PATHS.salesReturns}/${ret.id}`)}
        />
      )}
    </div>
  );
}
