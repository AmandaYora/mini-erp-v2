import { useState } from "react";
import {
  Button,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  Pagination,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDateTime } from "@/shared/lib/format";
import { auditService } from "@/modules/audit/services/audit.service";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import type { AuditRecord } from "@/modules/audit/types";

const LIMIT = 20;

export default function AuditPage() {
  const [action, setAction] = useState("");
  const [entity, setEntity] = useState("");
  const [applied, setApplied] = useState({ action: "", entity: "" });
  const [page, setPage] = useState(1);
  const { data, loading, error } = useAsyncData(
    () =>
      auditService
        .list({
          action: applied.action,
          entity: applied.entity,
          page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [applied.action, applied.entity, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  function apply() {
    setPage(1);
    setApplied({ action, entity });
  }

  return (
    <div>
      <PageHeader
        eyebrow="Laporan"
        title="Jejak Audit"
        description="Rekam aktivitas terbaru di cabang aktif, dari yang paling baru."
      />

      <FilterBar>
        <FormField label="Aksi">
          <TextInput
            placeholder="cth: sales.confirm…"
            value={action}
            onChange={(e) => setAction(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") apply();
            }}
          />
        </FormField>
        <FormField label="Entitas">
          <TextInput
            placeholder="cth: sales_order…"
            value={entity}
            onChange={(e) => setEntity(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") apply();
            }}
          />
        </FormField>
        <Button variant="secondary" onClick={apply}>
          Cari
        </Button>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat jejak audit">
            {error}
          </Notice>
        </div>
      )}

      <div className="rounded-lg border border-hairline bg-surface">
        {loading ? (
          <p className="text-muted px-6 py-10 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<AuditRecord>
              columns={[
                {
                  header: "Waktu",
                  render: (r) => formatDateTime(r.createdAt),
                },
                {
                  header: "Aksi",
                  render: (r) => (
                    <span className="font-medium">{r.action}</span>
                  ),
                },
                { header: "Entitas", render: (r) => r.entity },
                {
                  header: "ID Entitas",
                  align: "right",
                  render: (r) => (r.entityId === 0 ? "-" : String(r.entityId)),
                },
                {
                  header: "Aktor",
                  render: (r) =>
                    r.actorId === 0 ? "Sistem" : `#${r.actorId}`,
                },
                { header: "Catatan", render: (r) => r.note || "-" },
              ]}
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada jejak audit"
              emptyDescription="Aktivitas yang tercatat akan muncul di sini."
            />
            <Pagination
              page={page}
              limit={LIMIT}
              total={total}
              onPageChange={setPage}
            />
          </>
        )}
      </div>
    </div>
  );
}
