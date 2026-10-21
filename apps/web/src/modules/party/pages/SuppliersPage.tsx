import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  DataTable,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  StatusBadge,
} from "@/shared/components/ui";
import { ArchiveViewToggle, DocNumber, Page, SavedViews, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { supplierService } from "@/modules/party/services/party.service";
import { PartyFormModal, PartyStatusFilter } from "@/modules/party/components/PartyFormModal";
import {
  PARTY_LIST_DEFAULTS,
  partyListFilterSchema,
  partyStatusLabel,
  partyStatusParam,
} from "@/modules/party/schemas/party-list-filter.schema";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import { rowsToAttrs } from "@/shared/lib/key-value";
import type { Supplier } from "@/modules/party/types";

const LIMIT = 20;

export default function SuppliersPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("suppliers.create");

  const { filters, setFilters } = useUrlFilters(partyListFilterSchema, PARTY_LIST_DEFAULTS);
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      supplierService
        .list({ search: filters.search, status: partyStatusParam(filters.status), page: filters.page, limit: LIMIT }, signal)
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters.search, filters.status, filters.page],
  );
  const items = data?.items ?? null;
  const total = data?.total ?? 0;

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
        attributes: rowsToAttrs(values.attrs),
      });
      toast.fromServer(res.message, "Supplier dibuat");
      setModalOpen(false);
      if (filters.page !== 1) setFilters({ page: 1 });
      else reload();
    } catch (err) {
      setModalError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  const active: ActiveFilter[] = [];
  if (filters.status !== PARTY_LIST_DEFAULTS.status)
    active.push({
      key: "status",
      label: `Status: ${partyStatusLabel(filters.status)}`,
      onRemove: () => setFilters({ status: PARTY_LIST_DEFAULTS.status }),
    });
  const filtered = active.length > 0 || filters.search !== "";
  const addButton = canCreate ? <Button onClick={openCreate}>Tambah Supplier</Button> : undefined;

  return (
    <Page>
      <PageHeader
        title="Pemasok"
        description="Daftar supplier — klik baris untuk melihat detail."
        actions={addButton}
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari pemasok"
            placeholder="Cari kode, nama, atau telepon…"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
          />
        }
        primary={<PartyStatusFilter value={filters.status} onChange={(v) => setFilters({ status: v })} />}
        right={
          <>
            <ArchiveViewToggle
              showingArchive={filters.status === "archived"}
              onChange={(show) => setFilters({ status: show ? "archived" : "active" })}
            />
            <SavedViews listKey="suppliers" columnsKey="suppliers" />
          </>
        }
        active={active}
        onClearAll={() => setFilters(PARTY_LIST_DEFAULTS)}
      />

      <SectionCard flush>
        <DataTable<Supplier>
          columnsKey="suppliers"
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          caption="Daftar pemasok"
          onRowClick={(r) => navigate(`${ROUTE_PATHS.suppliers}/${r.id}`)}
          emptyTitle={filtered ? "Tidak ada supplier yang cocok dengan filter ini" : "Belum ada supplier"}
          emptyDescription={
            filtered ? "Ubah kata kunci atau hapus filter." : "Tambah supplier pertama untuk mulai membeli."
          }
          emptyAction={
            filtered ? (
              <Button variant="secondary" onClick={() => setFilters(PARTY_LIST_DEFAULTS)}>
                Hapus filter
              </Button>
            ) : (
              addButton
            )
          }
          columns={[
            { header: "Kode", render: (r) => <DocNumber value={r.code} /> },
            {
              header: "Nama",
              render: (r) => (
                <Link to={`${ROUTE_PATHS.suppliers}/${r.id}`} className="font-medium text-primary hover:underline">
                  {r.name}
                </Link>
              ),
            },
            { header: "Telepon", render: (r) => r.phone || "–" },
            { header: "Email", render: (r) => r.email || "–" },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
            // Kolom tanggal arsip hanya di tampilan yang memuat arsip.
            ...(filters.status !== "active"
              ? [{ header: "Diarsipkan pada", render: (r: Supplier) => (r.archivedAt ? formatDate(r.archivedAt) : "–") }]
              : []),
          ]}
        />
        {!error && (
          <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
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
    </Page>
  );
}
