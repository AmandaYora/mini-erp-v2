import { useState } from "react";
import {
  Badge,
  Button,
  DataTable,
  DateRangeFields,
  FormField,
  FormSheet,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Page, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate, formatDateTime } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { useAccessibleBranches } from "@/modules/auth/hooks/use-accessible-branches";
import { UserSearchSelect } from "@/modules/users/components/UserSearchSelect";
import { AuditDetail } from "@/modules/audit/components/AuditDetail";
import { actorLabel, branchLabel, entityLabel, isSystemActor } from "@/modules/audit/lib/audit";
import { ACTION_LABELS, ENTITY_LABELS, actionLabel, entityName } from "@/modules/audit/lib/audit-labels";
import { hasDetails } from "@/modules/audit/lib/diff";
import { AUDIT_LIST_DEFAULTS, auditListFilterSchema } from "@/modules/audit/schemas/audit-list-filter.schema";
import { auditService } from "@/modules/audit/services/audit.service";
import type { AuditRecord } from "@/modules/audit/types";

const LIMIT = 20;

const byLabel = (a: [string, string], b: [string, string]) => a[1].localeCompare(b[1], "id");
const ACTION_OPTIONS = Object.entries(ACTION_LABELS).sort(byLabel);
const ENTITY_OPTIONS = Object.entries(ENTITY_LABELS).sort(byLabel);

export default function AuditPage() {
  const can = useAuthStore((s) => s.can);
  const canPickUser = can("users.view");
  const { filters, setFilters, reset } = useUrlFilters(auditListFilterSchema, AUDIT_LIST_DEFAULTS);
  const { branches } = useAccessibleBranches(true);
  const [selected, setSelected] = useState<AuditRecord | null>(null);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      auditService.list({ ...filters, limit: LIMIT }, signal).then((res) => ({ items: res.items, total: res.meta.total })),
    [filters],
  );
  const total = data?.total ?? 0;

  // Filter di luar toolbar utama (entitas, id entitas, pengguna) masuk popover "Filter".
  const moreActiveCount =
    (filters.entity ? 1 : 0) + (filters.entityId > 0 ? 1 : 0) + (canPickUser && filters.actorId > 0 ? 1 : 0);

  const branchName = (id: string) => branches.find((b) => String(b.id) === id)?.name ?? `#${id}`;
  const active: ActiveFilter[] = [];
  if (filters.branchId === "all") {
    active.push({ key: "branch", label: "Cabang: semua", onRemove: () => setFilters({ branchId: "" }) });
  } else if (filters.branchId) {
    active.push({
      key: "branch",
      label: `Cabang: ${branchName(filters.branchId)}`,
      onRemove: () => setFilters({ branchId: "" }),
    });
  }
  if (filters.action) {
    active.push({ key: "action", label: `Aksi: ${actionLabel(filters.action)}`, onRemove: () => setFilters({ action: "" }) });
  }
  if (filters.entity) {
    active.push({ key: "entity", label: `Entitas: ${entityName(filters.entity)}`, onRemove: () => setFilters({ entity: "" }) });
  }
  if (filters.entityId > 0) {
    active.push({ key: "entityId", label: `ID entitas: ${filters.entityId}`, onRemove: () => setFilters({ entityId: 0 }) });
  }
  if (canPickUser && filters.actorId > 0) {
    active.push({ key: "actor", label: `Pengguna #${filters.actorId}`, onRemove: () => setFilters({ actorId: 0 }) });
  }
  if (filters.from || filters.to) {
    active.push({
      key: "range",
      label: `Tanggal: ${filters.from ? formatDate(filters.from) : "…"} – ${filters.to ? formatDate(filters.to) : "…"}`,
      onRemove: () => setFilters({ from: "", to: "" }),
    });
  }

  return (
    <Page>
      <PageHeader
        title="Jejak Audit"
        description="Rekam aktivitas, dari yang paling baru. Pilih baris atau tombol Rincian untuk melihat perubahan sebelum/sesudah."
      />

      <Toolbar
        search={
          <SelectInput
            aria-label="Aksi"
            value={filters.action}
            onChange={(e) => setFilters({ action: e.target.value })}
          >
            <option value="">Semua aksi</option>
            {/* Nilai dari URL yang bukan kunci dikenal tetap tampil agar filternya terlihat. */}
            {filters.action && !(filters.action in ACTION_LABELS) && (
              <option value={filters.action}>{filters.action}</option>
            )}
            {ACTION_OPTIONS.map(([key, label]) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </SelectInput>
        }
        primary={
          <>
            <FormField label="Cabang">
              <SelectInput
                aria-label="Cabang"
                value={filters.branchId}
                onChange={(e) => setFilters({ branchId: e.target.value })}
              >
                <option value="">Cabang aktif + global</option>
                <option value="all">Semua cabang</option>
                {branches.map((b) => (
                  <option key={b.id} value={String(b.id)}>
                    {b.code} — {b.name}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <DateRangeFields
              from={filters.from}
              to={filters.to}
              onChange={({ from, to }) => setFilters({ from, to })}
            />
          </>
        }
        more={
          <>
            <FormField label="Entitas">
              <SelectInput
                aria-label="Entitas"
                value={filters.entity}
                onChange={(e) => setFilters({ entity: e.target.value })}
              >
                <option value="">Semua entitas</option>
                {filters.entity && !(filters.entity in ENTITY_LABELS) && (
                  <option value={filters.entity}>{filters.entity}</option>
                )}
                {ENTITY_OPTIONS.map(([key, label]) => (
                  <option key={key} value={key}>
                    {label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField label="ID entitas">
              <TextInput
                aria-label="ID entitas"
                inputMode="numeric"
                value={filters.entityId > 0 ? String(filters.entityId) : ""}
                onChange={(e) => {
                  const n = Number(e.target.value.replace(/\D/g, ""));
                  setFilters({ entityId: Number.isFinite(n) ? n : 0 });
                }}
                placeholder="cth: 12"
              />
            </FormField>
            {canPickUser && (
              <FormField label="Pengguna">
                <UserSearchSelect value={filters.actorId || null} onChange={(id) => setFilters({ actorId: id ?? 0 })} />
              </FormField>
            )}
          </>
        }
        moreActiveCount={moreActiveCount}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<AuditRecord>
          caption="Jejak audit"
          rows={data?.items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={(r) => {
            if (hasDetails(r)) setSelected(r);
          }}
          emptyTitle="Tidak ada jejak audit"
          emptyDescription="Tidak ada jejak audit untuk filter ini."
          emptyAction={
            active.length > 0 ? (
              <Button variant="secondary" size="sm" onClick={reset}>
                Atur ulang filter
              </Button>
            ) : undefined
          }
          columns={[
            { header: "Waktu", render: (r) => <span className="whitespace-nowrap">{formatDateTime(r.createdAt)}</span> },
            {
              header: "Aksi",
              // Kunci mentah tetap tersedia di tooltip (untuk pencarian log/dukungan).
              render: (r) => (
                <span className="font-medium" title={r.action}>
                  {actionLabel(r.action)}
                </span>
              ),
            },
            { header: "Entitas", render: (r) => <span title={r.entity}>{entityLabel(r)}</span> },
            {
              header: "Aktor",
              render: (r) => (isSystemActor(r) ? <Badge tone="neutral">Sistem</Badge> : actorLabel(r)),
            },
            { header: "Cabang", render: (r) => branchLabel(r) },
            { header: "Catatan", render: (r) => r.note || "-" },
            {
              header: "",
              id: "rincian",
              align: "right",
              render: (r) =>
                hasDetails(r) ? (
                  <Button variant="ghost" size="sm" onClick={() => setSelected(r)}>
                    Rincian
                  </Button>
                ) : null,
            },
          ]}
        />
        {data && data.items.length > 0 && (
          <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
        )}
      </SectionCard>

      <FormSheet
        open={selected !== null}
        size="lg"
        title={selected ? actionLabel(selected.action) : "Rincian audit"}
        description={
          selected
            ? `${formatDateTime(selected.createdAt)} · ${actorLabel(selected)} · ${selected.action}`
            : undefined
        }
        onClose={() => setSelected(null)}
        actions={
          <Button variant="secondary" onClick={() => setSelected(null)}>
            Tutup
          </Button>
        }
      >
        {selected && <AuditDetail record={selected} />}
      </FormSheet>
    </Page>
  );
}
