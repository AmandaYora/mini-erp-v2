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
}

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

/** Dropdown lokasi cabang aktif (GET /api/v1/stock/locations). */
export function LocationSelect({
  label = "Lokasi",
  value,
  onChange,
  required,
  allowClear,
  disabled,
  placeholder = "Pilih lokasi…",
}: LocationSelectProps) {
  const [locations, setLocations] = useState<StockLocation[]>([]);

  useEffect(() => {
    let alive = true;
    stockService
      .listLocations()
      .then((rows) => {
        if (alive) setLocations(rows);
      })
      .catch((err) => {
        if (alive) toast.danger("Gagal memuat lokasi", toApiError(err).message);
      });
    return () => {
      alive = false;
    };
  }, []);

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
        disabled={disabled}
        onChange={(v) => onChange(toId(v))}
      />
    </FormField>
  );
}
