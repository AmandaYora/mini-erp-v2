import { useEffect, useMemo, useState } from "react";
import { HierarchicalSelect } from "@/shared/components/ui";
import { stockService } from "@/modules/stock/services/stock.service";
import type { StockLocation } from "@/modules/stock/types";
import {
  EMPTY_LOCATIONS,
  buildLocationOptions,
} from "@/modules/stock/components/stock-location-tree";

export interface StockLocationSelectProps {
  /** Daftar lokasi. Bila dihilangkan, dimuat sendiri (cabang aktif). */
  locations?: StockLocation[];
  value?: number | null;
  defaultValue?: number | null;
  onChange?: (id: number | null) => void;
  excludeIds?: number[];
  name?: string;
  id?: string;
  required?: boolean;
  disabled?: boolean;
  placeholder?: string;
  emptyOptionLabel?: string;
}

/**
 * StockLocationSelect — pemilih lokasi berhierarki (pohon).
 *
 * Dibangun DI ATAS HierarchicalSelect di dalam modul stock (shared tetap
 * domain-agnostik). Melengkapi LocationSelect datar yang sudah ada: versi
 * pohon ini untuk kasus yang butuh konteks hierarki (mis. pindah lokasi
 * dengan excludeIds). Effect muat-mandiri hanya berisi callback async.
 */
export function StockLocationSelect({
  locations: locationsProp,
  value,
  defaultValue,
  onChange,
  excludeIds = [],
  name,
  id,
  required,
  disabled,
  placeholder = "Pilih lokasi…",
  emptyOptionLabel,
}: StockLocationSelectProps) {
  const [fetched, setFetched] = useState<StockLocation[] | null>(null);

  useEffect(() => {
    if (locationsProp) return;
    let active = true;
    stockService
      .listLocations()
      .then((rows) => {
        if (active) setFetched(rows);
      })
      .catch(() => {
        if (active) setFetched([]);
      });
    return () => {
      active = false;
    };
  }, [locationsProp]);

  const locations = useMemo(
    () => locationsProp ?? fetched ?? EMPTY_LOCATIONS,
    [locationsProp, fetched],
  );
  const options = useMemo(
    () => buildLocationOptions(locations, excludeIds),
    [locations, excludeIds],
  );

  return (
    <HierarchicalSelect
      id={id}
      name={name}
      options={options}
      value={value}
      defaultValue={defaultValue}
      required={required}
      disabled={disabled}
      placeholder={placeholder}
      searchPlaceholder="Cari lokasi…"
      emptyOptionLabel={emptyOptionLabel}
      emptyText="Belum ada lokasi aktif"
      onChange={(v) =>
        onChange?.(typeof v === "number" ? v : v === null ? null : Number(v) || null)
      }
    />
  );
}
