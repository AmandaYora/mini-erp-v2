import { useMemo, useState } from "react";
import { Archive, ArchiveRestore, SquarePen } from "lucide-react";
import {
  Button,
  ConfirmDialog,
  DataTable,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  StatusBadge,
} from "@/shared/components/ui";
import { ActionMenu, ArchiveViewToggle, DocNumber, Page, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatIDR } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { archiveConfirmMessage } from "@/modules/party/lib/member-type-impact";
import { memberTypeService } from "@/modules/party/services/party.service";
import { MemberTypeFormModal } from "@/modules/party/components/MemberTypeFormModal";
import { PartyStatusFilter } from "@/modules/party/components/PartyFormModal";
import {
  PARTY_LIST_DEFAULTS,
  partyListFilterSchema,
  partyStatusLabel,
  partyStatusParam,
} from "@/modules/party/schemas/party-list-filter.schema";
import {
  basisOptions,
  optionLabel,
  roundingSummary,
} from "@/modules/party/schemas/member-type.schema";
import type {
  MemberTypeCreateValues,
  MemberTypeUpdateValues,
} from "@/modules/party/schemas/member-type.schema";
import type { MemberType } from "@/modules/party/types";

const LIMIT = 20;

/** Ringkasan aturan untuk kolom tabel, mis. "−10% dari Harga jual · Terdekat 100"
 *  (plus boleh di atas 100%, mis. "+150% dari Harga beli"). */
function ruleSummary(m: MemberType): string {
  const dir = m.direction === "plus" ? "+" : "−";
  const val = m.type === "percent" ? `${m.value}%` : formatIDR(m.value);
  return `${dir}${val} dari ${optionLabel(basisOptions, m.basis)} · ${roundingSummary(m.roundingMode, m.roundingStep)}`;
}

export default function MemberTypesPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("member_types.manage");

  const { filters, setFilters } = useUrlFilters(partyListFilterSchema, PARTY_LIST_DEFAULTS);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<MemberType | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<{ target: MemberType; action: "archive" | "restore" } | null>(null);
  const [busy, setBusy] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    () => memberTypeService.list(filters.search, partyStatusParam(filters.status)),
    [filters.search, filters.status],
  );

  // Dampak arsip (D3): jumlah pelanggan aktif dimuat saat dialog arsip dibuka.
  const impact = useAsyncData(
    () =>
      confirm?.action === "archive"
        ? memberTypeService.customerCount(confirm.target.id)
        : Promise.resolve(null),
    [confirm?.action, confirm?.target.id],
  );

  // Endpoint mengembalikan seluruh tipe (bukan paginasi): halaman dipotong di klien.
  const all = useMemo(() => data ?? [], [data]);
  const totalPages = Math.max(1, Math.ceil(all.length / LIMIT));
  const page = Math.min(filters.page, totalPages);
  const rows = data === null ? null : all.slice((page - 1) * LIMIT, page * LIMIT);

  function openCreate() {
    setEditing(null);
    setModalError(null);
    setModalOpen(true);
  }

  function openEdit(row: MemberType) {
    setEditing(row);
    setModalError(null);
    setModalOpen(true);
  }

  async function handleSubmit(values: MemberTypeCreateValues | MemberTypeUpdateValues) {
    setSubmitting(true);
    setModalError(null);
    try {
      if (editing) {
        const v = values as MemberTypeUpdateValues;
        const res = await memberTypeService.update(editing.id, {
          name: v.name,
          description: v.description === "" ? undefined : v.description,
          basis: v.basis,
          direction: v.direction,
          type: v.type,
          value: v.value,
          mode: v.mode,
          step: v.step,
          confirmed: v.confirmed,
        });
        toast.fromServer(res.message, "Tipe member disimpan");
      } else {
        const v = values as MemberTypeCreateValues;
        const res = await memberTypeService.create({
          code: v.code,
          name: v.name,
          description: v.description === "" ? undefined : v.description,
          basis: v.basis,
          direction: v.direction,
          type: v.type,
          value: v.value,
          mode: v.mode,
          step: v.step,
          confirmed: v.confirmed,
        });
        toast.fromServer(res.message, "Tipe member dibuat");
      }
      setModalOpen(false);
      setEditing(null);
      reload();
    } catch (err) {
      setModalError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleConfirm() {
    if (!confirm) return;
    setBusy(true);
    try {
      if (confirm.action === "archive") {
        const res = await memberTypeService.archive(confirm.target.id);
        toast.fromServer(res.message, "Tipe member diarsipkan");
      } else {
        const res = await memberTypeService.restore(confirm.target.id);
        toast.fromServer(res.message, "Tipe member dipulihkan");
      }
      setConfirm(null);
      reload();
    } catch (err) {
      toast.danger("Gagal", toApiError(err).message);
    } finally {
      setBusy(false);
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
  const addButton = canManage ? <Button onClick={openCreate}>Tambah Tipe Member</Button> : undefined;

  return (
    <Page>
      <PageHeader
        title="Tipe member"
        description="Aturan harga member — basis, penyesuaian, dan pembulatan."
        actions={addButton}
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari tipe member"
            placeholder="Cari kode atau nama…"
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
        <DataTable<MemberType>
          rows={rows}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          caption="Daftar tipe member"
          emptyTitle={filtered ? "Tidak ada tipe member yang cocok dengan filter ini" : "Belum ada tipe member"}
          emptyDescription={
            filtered ? "Ubah kata kunci atau hapus filter." : "Buat tipe member pertama untuk aturan harga member."
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
            { header: "Nama", render: (r) => <span className="font-medium">{r.name}</span> },
            { header: "Aturan", render: (r) => ruleSummary(r) },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
            ...(canManage
              ? [
                  {
                    header: "Aksi",
                    width: 56,
                    align: "right" as const,
                    render: (r: MemberType) => (
                      <ActionMenu
                        variant="ghost"
                        label={`Aksi untuk ${r.name}`}
                        items={[
                          { key: "edit", label: "Ubah", icon: <SquarePen aria-hidden />, onSelect: () => openEdit(r) },
                          r.status === "archived"
                            ? {
                                key: "restore",
                                label: "Pulihkan",
                                icon: <ArchiveRestore aria-hidden />,
                                onSelect: () => setConfirm({ target: r, action: "restore" }),
                              }
                            : {
                                key: "archive",
                                label: "Arsipkan",
                                icon: <Archive aria-hidden />,
                                destructive: true,
                                onSelect: () => setConfirm({ target: r, action: "archive" }),
                              },
                        ]}
                      />
                    ),
                  },
                ]
              : []),
          ]}
        />
        {!error && <Pagination page={page} limit={LIMIT} total={all.length} onPageChange={(p) => setFilters({ page: p })} />}
      </SectionCard>

      {modalOpen && (
        <MemberTypeFormModal
          open
          initial={editing}
          submitting={submitting}
          serverError={modalError}
          onClose={() => {
            setModalOpen(false);
            setEditing(null);
          }}
          onSubmit={(v) => void handleSubmit(v)}
        />
      )}

      <ConfirmDialog
        open={confirm !== null}
        title={confirm?.action === "archive" ? "Arsipkan tipe member?" : "Pulihkan tipe member?"}
        message={
          confirm?.action === "archive"
            ? archiveConfirmMessage(confirm.target.code, {
                loading: impact.loading,
                error: impact.error,
                activeCustomers: impact.data?.activeCustomers ?? null,
              })
            : `Tipe ${confirm?.target.code} akan aktif kembali.`
        }
        confirmLabel={confirm?.action === "archive" ? "Arsipkan" : "Pulihkan"}
        tone={confirm?.action === "archive" ? "danger" : "primary"}
        onConfirm={() => {
          if (!busy) void handleConfirm();
        }}
        onCancel={() => setConfirm(null)}
      />
    </Page>
  );
}
