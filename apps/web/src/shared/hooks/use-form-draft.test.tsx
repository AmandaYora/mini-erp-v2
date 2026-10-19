import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDraftAutosave, useDraftInitial } from "@/shared/hooks/use-form-draft";
import { loadDraft, saveDraft } from "@/shared/lib/draft-storage";

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
  localStorage.clear();
});

describe("draf formulir", () => {
  it("initial membaca draf tersimpan sekali", () => {
    saveDraft("mini-erp.draft:1:f", { n: 5 });
    const { result } = renderHook(() => useDraftInitial<{ n: number }>("mini-erp.draft:1:f"));
    expect(result.current).toEqual({ n: 5 });
  });

  it("key null = nonaktif", () => {
    const { result } = renderHook(() => useDraftInitial<{ n: number }>(null));
    expect(result.current).toBeNull();
  });

  it("autosave menyimpan dengan debounce dan menghapus saat kosong", () => {
    const key = "mini-erp.draft:1:g";
    const isEmpty = (v: { n: number }) => v.n === 0;
    const { rerender } = renderHook(({ v }) => useDraftAutosave(key, v, isEmpty, 500), {
      initialProps: { v: { n: 1 } },
    });
    expect(loadDraft(key)).toBeNull();
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(loadDraft(key)).toEqual({ n: 1 });
    rerender({ v: { n: 0 } });
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(loadDraft(key)).toBeNull();
  });
});
