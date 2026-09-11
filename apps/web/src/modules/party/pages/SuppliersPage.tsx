import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { supplierService } from "@/modules/party/services/party.service";
import { PartyFormModal, PartyStatusFilter } from "@/modules/party/components/PartyFormModal";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import type { Supplier } from "@/modules/party/types";

const LIMIT = 20;

export default function SuppliersPage() {
  const can = useAuthStore((s) => s.can);
  const canCreate = can("suppliers.create");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      supplierService
        .list({ search, status, page, limit: LIMIT })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [search, status, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  function applySearch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSearch(searchInput);
    setPage(1);
  }

  function openCreate() {
    setModalError(null);
    setModalOpen(true);
  }

  async function handleCreate(values: PartyFormValues) {
    setSubmitting(true);
    setModalError(null);
    try {
      const res = await supplierService.create({
        code: values.code.trim() === "" ? undefined : values.code.trim(),
        name: values.name,
        phone: values.phone === "" ? undefined : values.phone,
        email: values.email === "" ? undefined : values.email,
        address: values.address === "" ? undefined : values.address,
        notes: values.notes === "" ? undefined : values.notes,
      });
      toast.fromServer(res.message, "Supplier dibuat");
      setModalOpen(false);
      if (page !== 1) setPage(1);
      else reload();
    } catch (err) {
      setModalError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Master"
        title="Pemasok"
        description="Daftar supplier — klik nama untuk melihat detail."
        actions={
          canCreate ? <Button onClick={openCreate}>Tambah Supplier</Button> : undefined
        }
      />

      <FilterBar>
        <form onSubmit={applySearch} className="flex flex-wrap items-end gap-4">
          <div className="min-w-52 flex-1">
            <FormField label="Cari">
              <TextInput
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Kode / nama / telepon…"
              />
            </FormField>
          </div>
          <PartyStatusFilter
            value={status}
            onChange={(v) => {
              setStatus(v);
              setPage(1);
            }}
          />
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
            <DataTable<Supplier>
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada supplier"
              emptyDescription="Tambah supplier baru atau ubah kata kunci pencarian."
              columns={[
                { header: "Kode", render: (r) => <span className="font-medium">{r.code}</span> },
                {
                  header: "Nama",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.suppliers}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.name}
                    </Link>
                  ),
                },
                { header: "Telepon", render: (r) => r.phone || "-" },
                { header: "Email", render: (r) => r.email || "-" },
                {
                  header: "Status",
                  render: (r) => <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge>,
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.suppliers}/${r.id}`}
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

      {modalOpen && (
        <PartyFormModal
          open
          title="Tambah Supplier"
          description="Kode opsional — kosongkan untuk kode otomatis (SUP-…)."
          initial={null}
          withMember={false}
          memberLoadOk={false}
          memberOptions={[]}
          submitting={submitting}
          serverError={modalError}
          onClose={() => setModalOpen(false)}
          onSubmit={(v) => void handleCreate(v)}
        />
      )}
    </div>
  );
}
