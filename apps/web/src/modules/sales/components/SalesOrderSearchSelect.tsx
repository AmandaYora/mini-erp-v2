import { useCallback, useEffect, useState } from "react";
import {
  AsyncSearchSelect,
  type AsyncSearchLoadResult,
  type AsyncSearchOption,
} from "@/shared/components/ui";
import { formatDate, formatIDR } from "@/shared/lib/format";
import { salesService } from "@/modules/sales/services/sales.service";
import type { SalesOrder } from "@/modules/sales/types";

type OrderOption = AsyncSearchOption & { order: SalesOrder };

export interface SalesOrderSearchSelectProps {
  id?: string;
  value: number | null;
  onChange: (value: number | null, order?: SalesOrder) => void;
  /** CSV status yang boleh dipilih, mis. "confirmed" atau "confirmed,completed". */
  status?: string;
  /** Batasi ke order milik satu pelanggan. */
  partyId?: number;
  /** Seed objek terpilih (mis. dari tautan/prefill) agar langsung tampil. */
  selectedOrder?: SalesOrder | null;
  disabled?: boolean;
  allowClear?: boolean;
  placeholder?: string;
  limit?: number;
}

function formatSalesOrderOption(order: SalesOrder): OrderOption {
  return {
    value: order.id,
    label: `${order.number} — ${order.partyName || "Tunai"}`,
    description: `${formatDate(order.orderDate)} · ${formatIDR(order.grandTotal)}`,
    order,
  };
}

/**
 * SalesOrderSearchSelect — pemilih order jual dengan pencarian SERVER-SIDE
 * (nomor atau nama/kontak pelanggan) + filter status CSV + partyId. Menggantikan
 * daftar tetap 50 baris: order lama tetap bisa dijangkau dengan mengetik.
 */
export function SalesOrderSearchSelect({
  id,
  value,
  onChange,
  status,
  partyId,
  selectedOrder,
  disabled,
  allowClear = false,
  placeholder = "Cari nomor order atau pelanggan…",
  limit = 20,
}: SalesOrderSearchSelectProps) {
  const [resolved, setResolved] = useState<OrderOption | null>(
    selectedOrder ? formatSalesOrderOption(selectedOrder) : null,
  );
  const [prevSeed, setPrevSeed] = useState(selectedOrder);
  if (selectedOrder !== prevSeed) {
    setPrevSeed(selectedOrder);
    if (selectedOrder) setResolved(formatSalesOrderOption(selectedOrder));
  }
  if ((value === null || value === undefined) && resolved !== null) setResolved(null);

  const loadOptions = useCallback(
    async (search: string, signal?: AbortSignal): Promise<AsyncSearchLoadResult> => {
      const res = await salesService.list(
        { search, status, partyId, page: 1, limit },
        signal,
      );
      return {
        options: res.items.map(formatSalesOrderOption),
        hasMore: res.meta.total > res.items.length,
      };
    },
    [status, partyId, limit],
  );

  // Nilai awal tanpa seed: ambil satu order agar labelnya tampil.
  useEffect(() => {
    if (!value || resolved?.value === value) return;
    let active = true;
    salesService
      .get(value)
      .then((order) => {
        if (active) setResolved(formatSalesOrderOption(order));
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [value, resolved]);

  return (
    <AsyncSearchSelect
      // Ganti filter pihak/status = daftar lain: pemilih dibuat ulang.
      key={`${status ?? ""}|${partyId ?? 0}`}
      id={id}
      value={value}
      disabled={disabled}
      allowClear={allowClear}
      placeholder={placeholder}
      searchPlaceholder="Ketik nomor atau nama pelanggan…"
      selectedOption={resolved}
      loadOptions={loadOptions}
      onChange={(next, option) => {
        const o = option as OrderOption | undefined;
        setResolved(o ?? null);
        onChange(
          typeof next === "number" ? next : next === null ? null : Number(next) || null,
          o?.order,
        );
      }}
    />
  );
}
