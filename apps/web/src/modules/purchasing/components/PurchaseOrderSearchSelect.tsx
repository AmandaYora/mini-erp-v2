import { useCallback, useEffect, useState } from "react";
import {
  AsyncSearchSelect,
  type AsyncSearchLoadResult,
  type AsyncSearchOption,
} from "@/shared/components/ui";
import { formatDate, formatIDR } from "@/shared/lib/format";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import type { PurchaseOrder } from "@/modules/purchasing/types";

type OrderOption = AsyncSearchOption & { order: PurchaseOrder };

export interface PurchaseOrderSearchSelectProps {
  id?: string;
  value: number | null;
  onChange: (value: number | null, order?: PurchaseOrder) => void;
  /** CSV status yang boleh dipilih, mis. "confirmed" atau "confirmed,completed". */
  status?: string;
  /** Batasi ke order milik satu supplier. */
  partyId?: number;
  /** Seed objek terpilih (mis. dari tautan/prefill) agar langsung tampil. */
  selectedOrder?: PurchaseOrder | null;
  disabled?: boolean;
  allowClear?: boolean;
  placeholder?: string;
  limit?: number;
}

function formatPurchaseOrderOption(order: PurchaseOrder): OrderOption {
  return {
    value: order.id,
    label: `${order.number} — ${order.partyName || "-"}`,
    description: `${formatDate(order.orderDate)} · ${formatIDR(order.grandTotal)}`,
    order,
  };
}

/**
 * PurchaseOrderSearchSelect — pemilih order beli dengan pencarian SERVER-SIDE
 * (nomor atau nama/kontak supplier) + filter status CSV + partyId. Menggantikan
 * daftar tetap 50 baris: order lama tetap bisa dijangkau dengan mengetik.
 */
export function PurchaseOrderSearchSelect({
  id,
  value,
  onChange,
  status,
  partyId,
  selectedOrder,
  disabled,
  allowClear = false,
  placeholder = "Cari nomor PO atau supplier…",
  limit = 20,
}: PurchaseOrderSearchSelectProps) {
  const [resolved, setResolved] = useState<OrderOption | null>(
    selectedOrder ? formatPurchaseOrderOption(selectedOrder) : null,
  );
  const [prevSeed, setPrevSeed] = useState(selectedOrder);
  if (selectedOrder !== prevSeed) {
    setPrevSeed(selectedOrder);
    if (selectedOrder) setResolved(formatPurchaseOrderOption(selectedOrder));
  }
  if ((value === null || value === undefined) && resolved !== null) setResolved(null);

  const loadOptions = useCallback(
    async (search: string, signal?: AbortSignal): Promise<AsyncSearchLoadResult> => {
      const res = await purchaseOrderService.list(
        { search, status, partyId, page: 1, limit },
        signal,
      );
      return {
        options: res.items.map(formatPurchaseOrderOption),
        hasMore: res.meta.total > res.items.length,
      };
    },
    [status, partyId, limit],
  );

  // Nilai awal tanpa seed: ambil satu order agar labelnya tampil.
  useEffect(() => {
    if (!value || resolved?.value === value) return;
    let active = true;
    purchaseOrderService
      .get(value)
      .then((order) => {
        if (active) setResolved(formatPurchaseOrderOption(order));
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
      searchPlaceholder="Ketik nomor atau nama supplier…"
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
