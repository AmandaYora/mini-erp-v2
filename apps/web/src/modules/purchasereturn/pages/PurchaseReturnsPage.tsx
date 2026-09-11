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
import { purchaseReturnService } from "@/modules/purchasereturn/services/purchase-return.service";
import { PURCHASE_RETURN_STATUSES } from "@/modules/purchasereturn/types";
import type { PurchaseReturn } from "@/modules/purchasereturn/types";
import { PurchaseReturnCreateModal } from "@/modules/purchasereturn/components/PurchaseReturnCreateModal";

const LIMIT = 20;

export default function PurchaseReturnsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("purchasereturn.create");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      purchaseReturnService
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
        eyebrow="Pembelian"
        title="Retur Beli"
        description="Retur pembelian terhadap order beli terkonfirmasi — klik nomor untuk detail dan proses."
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
                placeholder="Nomor retur / nomor PO…"
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
              {PURCHASE_RETURN_STATUSES.map((s) => (
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
            <DataTable<PurchaseReturn>
              rows={visible}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada retur pembelian"
              emptyDescription="Buat retur baru atau ubah kata kunci pencarian."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.purchaseReturns}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.returnDate) },
                { header: "Ref PO", render: (r) => r.orderNumber || "-" },
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
                      to={`${ROUTE_PATHS.purchaseReturns}/${r.id}`}
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
        <PurchaseReturnCreateModal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          onCreated={(ret) => navigate(`${ROUTE_PATHS.purchaseReturns}/${ret.id}`)}
        />
      )}
    </div>
  );
}
