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
import type { SearchSelectOption } from "@/shared/components/ui";
import { ArchiveViewToggle, DocNumber, Page, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { customerService, memberTypeService } from "@/modules/party/services/party.service";
import { PartyFormModal, PartyStatusFilter } from "@/modules/party/components/PartyFormModal";
import {
  PARTY_LIST_DEFAULTS,
  partyListFilterSchema,
  partyStatusLabel,
  partyStatusParam,
} from "@/modules/party/schemas/party-list-filter.schema";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import { rowsToAttrs } from "@/shared/lib/key-value";
import type { Customer } from "@/modules/party/types";

const LIMIT = 20;

export default function CustomersPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("customers.create");

  const { filters, setFilters } = useUrlFilters(partyListFilterSchema, PARTY_LIST_DEFAULTS);
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      customerService
        .list({ search: filters.search, status: partyStatusParam(filters.status), page: filters.page, limit: LIMIT }, signal)
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters.search, filters.status, filters.page],
  );
  const items = data?.items ?? null;
  const total = data?.total ?? 0;

  // Opsi member untuk form buat. Gagal muat (mis. tanpa permission
  // member_types.manage) = field member disembunyikan, bukan error halaman.
  const { data: memberData } = useAsyncData(
    async () => {
      if (!canCreate) return { options: [], ok: false };
      try {
        const list = await memberTypeService.list("", "active");
        return {
          options: list.map((m) => ({ value: m.id, label: `${m.code} — ${m.name}` })),
          ok: true,
        };
      } catch {
        return { options: [], ok: false };
      }
    },
    [canCreate],
  );
  const memberOptions: SearchSelectOption[] = memberData?.options ?? [];
  const memberLoadOk = memberData?.ok ?? false;

  function openCreate() {
    setModalError(null);
    setModalOpen(true);
  }

  async function handleCreate(values: PartyFormValues) {
    setSubmitting(true);
    setModalError(null);
    try {
      const res = await customerService.create({
        code: values.code.trim() === "" ? undefined : values.code.trim(),
        name: values.name,
        phone: values.phone === "" ? undefined : values.phone,
        email: values.email === "" ? undefined : values.email,
        address: values.address === "" ? undefined : values.address,
        notes: values.notes === "" ? undefined : values.notes,
        memberTypeId: values.memberTypeId ?? undefined,
        attributes: rowsToAttrs(values.attrs),
      });
      toast.fromServer(res.message, "Customer dibuat");
      setModalOpen(false);
      if (filters.page !== 1) setFilters({ page: 1 });
      else reload();
    } catch (err) {
      setModalError(toApiError(err).message);
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
  const addButton = canCreate ? <Button onClick={openCreate}>Tambah Customer</Button> : undefined;

  return (
    <Page>
      <PageHeader
        title="Pelanggan"
        description="Daftar customer — klik baris untuk melihat detail dan buku alamat."
        actions={addButton}
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari pelanggan"
            placeholder="Cari kode, nama, atau telepon…"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
          />
        }
        primary={<PartyStatusFilter value={filters.status} onChange={(v) => setFilters({ status: v })} />}
        right={
          <ArchiveViewToggle
            showingArchive={filters.status === "archived"}
            onChange={(show) => setFilters({ status: show ? "archived" : "active" })}
          />
        }
        active={active}
        onClearAll={() => setFilters(PARTY_LIST_DEFAULTS)}
      />

      <SectionCard flush>
        <DataTable<Customer>
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          caption="Daftar pelanggan"
          onRowClick={(r) => navigate(`${ROUTE_PATHS.customers}/${r.id}`)}
          emptyTitle={filtered ? "Tidak ada customer yang cocok dengan filter ini" : "Belum ada customer"}
          emptyDescription={
            filtered ? "Ubah kata kunci atau hapus filter." : "Tambah customer pertama untuk mulai berjualan."
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
                <Link to={`${ROUTE_PATHS.customers}/${r.id}`} className="font-medium text-primary hover:underline">
                  {r.name}
                </Link>
              ),
            },
            { header: "Telepon", render: (r) => r.phone || "–" },
            { header: "Member", render: (r) => r.member?.code ?? "–" },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
            // Kolom tanggal arsip hanya di tampilan yang memuat arsip.
            ...(filters.status !== "active"
              ? [{ header: "Diarsipkan pada", render: (r: Customer) => (r.archivedAt ? formatDate(r.archivedAt) : "–") }]
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
          title="Tambah Customer"
          description="Kode opsional — kosongkan untuk kode otomatis (CUS-001, berurutan)."
          initial={null}
          withMember
          memberLoadOk={memberLoadOk}
          memberOptions={memberOptions}
          submitting={submitting}
          serverError={modalError}
          onClose={() => setModalOpen(false)}
          onSubmit={(v) => void handleCreate(v)}
        />
      )}
    </Page>
  );
}
