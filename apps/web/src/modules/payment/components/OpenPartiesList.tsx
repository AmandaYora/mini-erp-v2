import { useState } from "react";
import { Button, DataTable, Pagination, SearchInput, SectionCard, ToggleGroup, ToggleGroupItem } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { paymentService } from "@/modules/payment/services/payment.service";
import type { OpenParty, OpenPartySide } from "@/modules/payment/types";

const LIMIT = 10;

const SIDES: { value: OpenPartySide; label: string }[] = [
  { value: "receivable", label: "Piutang pelanggan" },
  { value: "payable", label: "Hutang supplier" },
];

interface OpenPartiesListProps {
  canPay: boolean;
  /** Naikkan nilainya setelah pembayaran dicatat agar daftar dimuat ulang. */
  refreshKey: number;
  selectedPartyId: number | null;
  onSelect: (party: OpenParty) => void;
  onPay: (party: OpenParty) => void;
}

/**
 * Daftar semua pihak yang masih berpiutang/berhutang (PY-01, legacy
 * listPartyBalances): cari nama/kode/HP, jumlah dokumen belum lunas, tanggal
 * terlama, sisa, dan aksi langsung Catat bayar. Dihitung server dari order +
 * alokasi pembayaran — tanpa saldo tersimpan.
 */
export function OpenPartiesList({ canPay, refreshKey, selectedPartyId, onSelect, onPay }: OpenPartiesListProps) {
  const [side, setSide] = useState<OpenPartySide>("receivable");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      paymentService.openParties({ side, search, page, limit: LIMIT }).catch((err) => {
        toast.danger("Gagal memuat daftar pihak bersaldo", toApiError(err).message);
        throw err;
      }),
    [side, search, page, refreshKey],
  );
  const rows = data?.items ?? [];
  const sideLabel = side === "receivable" ? "piutang" : "hutang";

  return (
    <SectionCard
      flush
      title="Pihak bersaldo"
      description="Semua pihak yang masih punya tagihan belum lunas, yang tertua lebih dulu."
    >
      <div className="flex flex-wrap items-center gap-3 border-b border-border px-5 py-3">
        <ToggleGroup
          type="single"
          variant="outline"
          aria-label="Sisi saldo"
          value={side}
          onValueChange={(v) => {
            if (!v) return;
            setSide(v as OpenPartySide);
            setPage(1);
          }}
        >
          {SIDES.map((o) => (
            <ToggleGroupItem key={o.value} value={o.value}>
              {o.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        <div className="min-w-56 flex-1">
          <SearchInput
            aria-label="Cari pihak bersaldo"
            value={search}
            onSearch={(v) => {
              setSearch(v);
              setPage(1);
            }}
            placeholder="Nama, kode, atau nomor HP…"
          />
        </div>
        {data && data.total > 0 && (
          <p className="text-sm text-muted-foreground">
            <span className="tabular">{data.total}</span> pihak · total {sideLabel} <Money value={data.totalOutstanding} />
          </p>
        )}
      </div>
      <DataTable<OpenParty>
        rows={rows}
        rowKey={(r) => r.partyId}
        loading={loading}
        error={error}
        onRetry={reload}
        onRowClick={onSelect}
        emptyTitle={search ? "Tidak ada pihak yang cocok" : `Tidak ada ${sideLabel} terbuka`}
        emptyDescription={
          search ? "Ubah kata kunci pencarian." : "Semua tagihan pada sisi ini sudah lunas."
        }
        columns={[
          {
            header: "Pihak",
            render: (r) => (
              <div>
                <p className={r.partyId === selectedPartyId ? "font-semibold text-primary" : "font-medium"}>
                  {r.name || `#${r.partyId}`}
                </p>
                {r.code ? <p className="text-xs text-muted-foreground">{r.code}</p> : null}
              </div>
            ),
          },
          { header: "Dokumen", align: "right", render: (r) => <span className="tabular">{r.orderCount}</span> },
          { header: "Tertua", render: (r) => (r.oldestDate ? formatDate(r.oldestDate) : "-") },
          {
            header: "Sisa",
            align: "right",
            render: (r) => <Money value={r.outstanding} className="font-medium" />,
          },
          {
            header: "",
            align: "right",
            render: (r) =>
              canPay ? (
                <Button
                  size="sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    onPay(r);
                  }}
                >
                  Catat bayar
                </Button>
              ) : null,
          },
        ]}
      />
      <Pagination page={page} limit={LIMIT} total={data?.total ?? 0} onPageChange={setPage} />
    </SectionCard>
  );
}
