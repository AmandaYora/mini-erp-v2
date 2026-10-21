import { useCallback, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  DataTable,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  StatusBadge,
} from "@/shared/components/ui";
import { DocNumber, Page, Qty, SavedViews, Toolbar } from "@/shared/components/patterns";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS, transferPrintPath } from "@/app/routes/route-paths";
import { stockService } from "@/modules/stock/services/stock.service";
import { MoveLocationSheet } from "@/modules/stock/components/MoveLocationSheet";
import {
  TRANSFER_DIRECTIONS,
  TRANSFER_LIST_DEFAULTS,
  TRANSFER_STATUSES,
  transferListFilterSchema,
} from "@/modules/stock/schemas/transfer-list-filter.schema";
import type { BranchOption, StockTransfer } from "@/modules/stock/types";

const LIMIT = 20;
const DIRECTION_LABEL = { out: "Keluar", in: "Masuk", internal: "Dalam cabang" } as const;
const NEW_TRANSFER = ROUTE_PATHS.stockTransfersNew;

export default function TransfersPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canManage = can("stock.transfer");

  const { filters, setFilters } = useUrlFilters(transferListFilterSchema, TRANSFER_LIST_DEFAULTS);
  const [moveOpen, setMoveOpen] = useState(false);
  // Naik tiap dibuka agar isian sheet mulai bersih.
  const [moveKey, setMoveKey] = useState(0);

  const { data: branches } = useAsyncData(
    () =>
      // Tanpa branches.view (mis. peran gudang) daftar cabang diganti cabang yang boleh diakses (login saja);
      // nama cabang di luar itu jatuh ke "#id". Sebelumnya peran tanpa izin itu mendapat toast 403 (N-50).
      (can("branches.view") ? stockService.listBranches() : stockService.listAccessibleBranches()).catch((err) => {
        toast.danger("Gagal memuat cabang", toApiError(err).message);
        return [] as BranchOption[];
      }),
    [can],
  );

  const branchName = useCallback(
    (id: number) => {
      const b = (branches ?? []).find((x) => x.id === id);
      return b ? `${b.code} — ${b.name}` : `#${id}`;
    },
    [branches],
  );

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      stockService
        .listTransfers(
          { status: filters.status || undefined, direction: filters.direction || undefined, page: filters.page, limit: LIMIT },
          signal,
        )
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters],
  );
  const total = data?.total ?? 0;
  const hasFilter = filters.status !== "" || filters.direction !== "";

  return (
    <Page>
      <PageHeader
        title="Transfer Stok"
        description="Mutasi antar cabang: dibuat, dikirim, lalu diterima."
        actions={
          canManage ? (
            <>
              <Button variant="secondary" onClick={() => {
                  setMoveKey((k) => k + 1);
                  setMoveOpen(true);
                }}>
                Pindah lokasi
              </Button>
              <Button asChild>
                <Link to={NEW_TRANSFER}>Buat transfer</Link>
              </Button>
            </>
          ) : undefined
        }
      />

      <Toolbar
        right={<SavedViews listKey="stock-transfers" columnsKey="stock-transfers" />}
        primary={
          <>
          <div className="w-48 max-w-full">
            <FormField label="Arah" htmlFor="transfer-direction">
              <SelectInput
                id="transfer-direction"
                value={filters.direction}
                onChange={(e) => setFilters({ direction: e.target.value as typeof filters.direction })}
              >
                <option value="">Semua arah</option>
                {TRANSFER_DIRECTIONS.map((d) => (
                  <option key={d} value={d}>
                    {DIRECTION_LABEL[d]}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
          <div className="w-48 max-w-full">
            <FormField label="Status" htmlFor="transfer-status">
              <SelectInput
                id="transfer-status"
                value={filters.status}
                onChange={(e) => setFilters({ status: e.target.value as typeof filters.status })}
              >
                <option value="">Semua status</option>
                {TRANSFER_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {statusLabel(s)}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
          </>
        }
        active={[
          ...(filters.status
            ? [{ key: "status", label: `Status: ${statusLabel(filters.status)}`, onRemove: () => setFilters({ status: "" }) }]
            : []),
          ...(filters.direction !== ""
            ? [{ key: "direction", label: `Arah: ${DIRECTION_LABEL[filters.direction as keyof typeof DIRECTION_LABEL]}`, onRemove: () => setFilters({ direction: "" }) }]
            : []),
        ]}
      />

      <SectionCard flush>
        <DataTable<StockTransfer>
          columnsKey="stock-transfers"
          loading={loading}
          error={error}
          onRetry={reload}
          rows={data?.items}
          rowKey={(r) => r.id}
          onRowClick={(r) => navigate(`${ROUTE_PATHS.stockTransfers}/${r.id}`)}
          emptyTitle={hasFilter ? "Tidak ada transfer yang cocok" : "Belum ada transfer"}
          emptyDescription={
            hasFilter
              ? "Tidak ada transfer dengan filter ini."
              : "Buat transfer untuk memindahkan stok antar cabang."
          }
          emptyAction={
            canManage && !hasFilter ? (
              <Button asChild>
                <Link to={NEW_TRANSFER}>Buat transfer pertama</Link>
              </Button>
            ) : undefined
          }
          columns={[
            {
              header: "Nomor",
              render: (r) => (
                <Link to={`${ROUTE_PATHS.stockTransfers}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Dari", render: (r) => branchName(r.fromBranchId) },
            { header: "Ke", render: (r) => branchName(r.toBranchId) },
            { header: "Baris", align: "right", render: (r) => <Qty value={r.items.length} /> },
            { header: "Tanggal", render: (r) => (r.transferDate ? formatDate(r.transferDate) : "–") },
            { header: "Sopir", render: (r) => r.driverName || "–" },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
            {
              header: "",
              align: "right" as const,
              width: "5rem",
              render: (r: StockTransfer) => (
                <Button asChild size="sm" variant="secondary">
                  <a href={transferPrintPath(r.id)} target="_blank" rel="noreferrer" onClick={(e) => e.stopPropagation()}>
                    Cetak
                  </a>
                </Button>
              ),
            },
          ]}
        />
        {total > 0 && (
          <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
        )}
      </SectionCard>

      <MoveLocationSheet
        key={moveKey}
        open={moveOpen}
        onClose={() => setMoveOpen(false)}
        onMoved={(t) => {
          setMoveOpen(false);
          reload();
          navigate(`${ROUTE_PATHS.stockTransfers}/${t.id}`);
        }}
      />
    </Page>
  );
}
