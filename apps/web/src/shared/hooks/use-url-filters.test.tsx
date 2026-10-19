import { act, cleanup, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";
import { z } from "zod";
import { parseFilters, serializeFilters, useUrlFilters } from "./use-url-filters";

afterEach(cleanup);

const schema = z.object({
  search: z.string(),
  status: z.enum(["", "draft", "confirmed"]),
  partyId: z.number().int().nonnegative(),
  page: z.number().int().min(1),
});
const defaults = { search: "", status: "" as const, partyId: 0, page: 1 };

describe("parseFilters / serializeFilters", () => {
  it("membaca nilai valid dari query string, angka dikonversi", () => {
    const f = parseFilters(schema, defaults, new URLSearchParams("search=abc&status=draft&partyId=7&page=3"));
    expect(f).toEqual({ search: "abc", status: "draft", partyId: 7, page: 3 });
  });

  it("nilai tidak valid jatuh ke default per-field", () => {
    const f = parseFilters(schema, defaults, new URLSearchParams("status=zzz&partyId=-4&page=0&search=ok"));
    expect(f).toEqual({ search: "ok", status: "", partyId: 0, page: 1 });
  });

  it("serialisasi melewatkan default dan nilai kosong", () => {
    const params = serializeFilters(defaults, { search: "abc", status: "", partyId: 0, page: 2 });
    expect(params.toString()).toBe("search=abc&page=2");
  });

  it("mempertahankan param lain di base", () => {
    const base = new URLSearchParams("keep=1&search=old");
    const params = serializeFilters(defaults, { search: "", status: "draft", partyId: 0, page: 1 }, base);
    expect(params.get("keep")).toBe("1");
    expect(params.has("search")).toBe(false);
    expect(params.get("status")).toBe("draft");
  });
});

function wrapper(initial: string) {
  return function W({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={[initial]}>{children}</MemoryRouter>;
  };
}

describe("useUrlFilters", () => {
  it("menerapkan filter ke URL dan mereset page ke 1 saat filter lain berubah", () => {
    const { result } = renderHook(
      () => ({ f: useUrlFilters(schema, defaults), loc: useLocation() }),
      { wrapper: wrapper("/x?page=4") },
    );
    expect(result.current.f.filters.page).toBe(4);
    act(() => result.current.f.setFilters({ status: "confirmed" }));
    expect(result.current.f.filters.status).toBe("confirmed");
    expect(result.current.f.filters.page).toBe(1);
    expect(result.current.loc.search).toBe("?status=confirmed");
  });

  it("page eksplisit tidak direset", () => {
    const { result } = renderHook(() => useUrlFilters(schema, defaults), { wrapper: wrapper("/x") });
    act(() => result.current.setFilters({ page: 3 }));
    expect(result.current.filters.page).toBe(3);
  });

  it("reset mengembalikan semua ke default", () => {
    const { result } = renderHook(
      () => ({ f: useUrlFilters(schema, defaults), loc: useLocation() }),
      { wrapper: wrapper("/x?search=a&status=draft&page=2") },
    );
    act(() => result.current.f.reset());
    expect(result.current.f.filters).toEqual(defaults);
    expect(result.current.loc.search).toBe("");
  });
});
