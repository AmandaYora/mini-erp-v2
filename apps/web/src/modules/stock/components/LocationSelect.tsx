import { useEffect, useState } from "react";
import { FormField, SearchSelect } from "@/shared/components/ui";
import { toast } from "@/shared/stores/toast.store";
import { toApiError } from "@/shared/services/http-client";
import { stockService } from "@/modules/stock/services/stock.service";
import type { StockLocation } from "@/modules/stock/types";

interface LocationSelectProps {
  label?: string;
  value: number | null;
  onChange: (id: number | null) => void;
  required?: boolean;
  allowClear?: boolean;
  disabled?: boolean;
  placeholder?: string;
  /**
   * Cabang pemilik lokasi (K-06). Kosong = cabang sesi. Saat berubah daftar
   * dimuat ulang; memilih cabang tanpa id (null) menonaktifkan pemuatan.
   */
  branchId?: number | null;
  /** Tunda pemuatan sampai cabang dipilih (mis. cabang tujuan transfer). */
  requireBranch?: boolean;
}

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

/** Dropdown lokasi cabang aktif, atau cabang `branchId` (GET /api/v1/stock/locations). */
export function LocationSelect({
  label = "Lokasi",
  value,
  onChange,
  required,
  allowClear,
  disabled,
  placeholder = "Pilih lokasi…",
  branchId,
  requireBranch,
}: LocationSelectProps) {
  // Hasil dikunci ke cabang yang diminta: ganti cabang tak menampilkan daftar lama.
  const [loaded, setLoaded] = useState<{ key: number | null; rows: StockLocation[] }>({ key: null, rows: [] });
  const waiting = requireBranch && !branchId;
  const key = branchId ?? 0;
  const locations = !waiting && loaded.key === key ? loaded.rows : [];

  useEffect(() => {
    if (waiting) return;
    let alive = true;
    stockService
      .listLocations(undefined, branchId)
      .then((rows) => {
        if (alive) setLoaded({ key, rows });
      })
      .catch((err) => {
        if (alive) toast.danger("Gagal memuat lokasi", toApiError(err).message);
      });
    return () => {
      alive = false;
    };
  }, [branchId, waiting, key]);

  return (
    <FormField label={label} required={required}>
      <SearchSelect
        options={locations.map((l) => ({
          value: l.id,
          label: `${l.code} — ${l.name}`,
        }))}
        value={value}
        placeholder={placeholder}
        allowClear={allowClear}
        disabled={disabled || waiting}
        onChange={(v) => onChange(toId(v))}
      />
    </FormField>
  );
}
