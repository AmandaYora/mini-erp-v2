import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import type { z } from "zod";

// Filter daftar yang tersinkron ke URL (dapat dibagikan / bertahan setelah
// refresh). Domain-agnostik: skema Zod (ZodObject dengan field string/number/
// enum) + nilai default. Nilai yang sama dengan default tidak ditulis ke URL
// supaya tautan tetap pendek. Nilai URL yang tidak lolos skema jatuh ke
// default per-field, bukan membatalkan seluruh filter.

type Shape = z.ZodRawShape;
type Values<S extends Shape> = z.infer<z.ZodObject<S>>;

/** Baca URLSearchParams -> objek filter bertipe; field tak valid -> default. */
export function parseFilters<S extends Shape>(
  schema: z.ZodObject<S>,
  defaults: Values<S>,
  params: URLSearchParams,
): Values<S> {
  const out: Record<string, unknown> = { ...defaults };
  for (const key of Object.keys(schema.shape)) {
    const raw = params.get(key);
    if (raw === null) continue;
    const field = schema.shape[key] as z.ZodTypeAny;
    // Param URL selalu string: angka dicoba dulu bila default-nya angka.
    const candidate = typeof (defaults as Record<string, unknown>)[key] === "number" ? Number(raw) : raw;
    const parsed = field.safeParse(candidate);
    if (parsed.success) out[key] = parsed.data;
  }
  return out as Values<S>;
}

/** Objek filter -> URLSearchParams (default/kosong dilewati). */
export function serializeFilters<S extends Shape>(
  defaults: Values<S>,
  filters: Values<S>,
  base?: URLSearchParams,
): URLSearchParams {
  const next = new URLSearchParams(base);
  const def = defaults as Record<string, unknown>;
  for (const [key, value] of Object.entries(filters as Record<string, unknown>)) {
    const isDefault = value === def[key];
    if (isDefault || value === undefined || value === null || value === "") {
      next.delete(key);
    } else {
      next.set(key, String(value));
    }
  }
  return next;
}

export interface UrlFilters<S extends Shape> {
  filters: Values<S>;
  /** Terapkan sebagian filter; `page` kembali ke 1 kecuali ikut diubah. */
  setFilters: (patch: Partial<Values<S>>) => void;
  reset: () => void;
}

/**
 * useUrlFilters — [filter, setFilter] yang bersumber dari query string.
 * Bila skema punya field `page`, perubahan filter lain me-reset `page`.
 */
export function useUrlFilters<S extends Shape>(
  schema: z.ZodObject<S>,
  defaults: Values<S>,
): UrlFilters<S> {
  const [params, setParams] = useSearchParams();
  const filters = useMemo(
    () => parseFilters(schema, defaults, params),
    // defaults/schema adalah konstanta modul pada setiap pemakaian.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [params],
  );

  const setFilters = useCallback(
    (patch: Partial<Values<S>>) => {
      setParams(
        (prev) => {
          const current = parseFilters(schema, defaults, prev);
          const merged = { ...current, ...patch } as Record<string, unknown>;
          if ("page" in schema.shape && !("page" in patch)) merged.page = (defaults as Record<string, unknown>).page;
          return serializeFilters(defaults, merged as Values<S>, prev);
        },
        { replace: true },
      );
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [setParams],
  );

  const reset = useCallback(() => {
    setParams(serializeFilters(defaults, defaults), { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [setParams]);

  return { filters, setFilters, reset };
}
