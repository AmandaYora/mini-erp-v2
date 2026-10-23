import { useState } from "react";
import { Link } from "react-router-dom";
import { X } from "lucide-react";
import { Badge, Button, DataTable, Pagination, SectionCard } from "@/shared/components/ui";
import type { DataTableColumn } from "@/shared/components/ui";
import { Money, Qty } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatNumber } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { dataHealthService } from "@/modules/data-health/services/data-health.service";
import type { AlertItemRow, HealthAlert } from "@/modules/data-health/types";

const PAGE_SIZE = 20;

function num(row: AlertItemRow, key: string): number {
  const v = row.columns[key];
  return typeof v === "number" ? v : Number(v ?? 0);
}

function columnsFor(alertId: string): DataTableColumn<AlertItemRow>[] {
  const product: DataTableColumn<AlertItemRow> = {
    header: "Produk",
    render: (r) => (
      <span className="flex flex-wrap items-center gap-2">
        <span className="text-foreground">{r.label}</span>
        {r.columns.status === "archived" && <Badge tone="muted">Arsip</Badge>}
      </span>
    ),
  };
  const qty: DataTableColumn<AlertItemRow> = { header: "Stok (satuan dasar)", align: "right", render: (r) => <Qty value={num(r, "qtyBase")} /> };
  const value: DataTableColumn<AlertItemRow> = { header: "Nilai persediaan", align: "right", render: (r) => <Money value={num(r, "valueRp")} /> };
  if (alertId === "inventory.cost_above_price") {
    return [
      product,
      { header: "Modal / satuan jual", align: "right", render: (r) => <Money value={num(r, "unitCostRp")} /> },
      { header: "Harga jual", align: "right", render: (r) => <Money value={num(r, "sellingPriceRp")} /> },
      { header: "Rasio", align: "right", render: (r) => <span className="tabular">{`${formatNumber(num(r, "ratio"))}×`}</span> },
      qty,
      value,
    ];
  }
  return [product, qty, { header: "Posisi", align: "right", render: (r) => <span className="tabular">{formatNumber(num(r, "positions"))}</span> }, value];
}

/**
 * Daftar rinci di balik alert yang aksinya halaman ini (A2 harga modal, A7 stok awal): baris
 * berhalaman dari GET /data-health/{id}/items, dengan tautan ke halaman penyelesaian per produk.
 */
export function AlertItemsPanel({ alert, onClose }: { alert: HealthAlert; onClose: () => void }) {
  const can = useAuthStore((s) => s.can);
  const [page, setPage] = useState(1);
  const list = useAsyncData((signal) => dataHealthService.items(alert.id, page, PAGE_SIZE, signal), [alert.id, page]);
  const columns: DataTableColumn<AlertItemRow>[] = [
    ...columnsFor(alert.id),
    {
      header: "Tindakan",
      render: (r) => (
        <span className="flex flex-wrap gap-x-3 gap-y-1">
          {r.links
            .filter((l) => can(l.perm))
            .map((l) => (
              <Link key={l.to} to={l.to} className="text-sm font-medium text-primary hover:underline">
                {l.label}
              </Link>
            ))}
        </span>
      ),
    },
  ];

  return (
    <SectionCard
      title={alert.title}
      description={`Daftar lengkap, nilai terbesar dulu. ${alert.impact}`}
      flush
      actions={
        <Button variant="ghost" size="sm" onClick={onClose}>
          <X size={16} aria-hidden />
          Tutup daftar
        </Button>
      }
    >
      {alert.id === "inventory.cost_above_price" && (
        <div className="border-b border-border p-4">
          <Notice tone="warning" title="Revaluasi di web menjurnal lawan 5100 (laba rugi)">
            Untuk harga modal yang salah sejak saldo awal, koordinasikan dengan akuntan lebih dulu (koreksi ke 3100).
          </Notice>
        </div>
      )}
      <DataTable
        columns={columns}
        rows={list.data?.items}
        rowKey={(r) => r.key}
        loading={list.loading}
        error={list.error}
        onRetry={list.reload}
        emptyTitle="Tidak ada baris"
        emptyDescription="Masalah ini sudah selesai sejak pemeriksaan terakhir."
        caption={alert.title}
      />
      {list.data && list.data.meta.total > PAGE_SIZE && (
        <div className="border-t border-border px-5 py-3">
          <Pagination page={page} limit={PAGE_SIZE} total={list.data.meta.total} onPageChange={setPage} />
        </div>
      )}
    </SectionCard>
  );
}
