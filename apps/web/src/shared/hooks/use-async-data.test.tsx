import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useAsyncData } from "@/shared/hooks/use-async-data";

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("useAsyncData", () => {
  it("mulai loading lalu mengisi data saat resolve", async () => {
    const loader = vi.fn(async () => "ok");
    const { result } = renderHook(() => useAsyncData(loader, []));
    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBe("ok");
    expect(result.current.error).toBeNull();
    expect(loader).toHaveBeenCalledTimes(1);
  });

  it("mengisi error saat loader menolak", async () => {
    const loader = vi.fn(async (): Promise<string> => {
      throw new Error("putus");
    });
    const { result } = renderHook(() => useAsyncData(loader, []));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBeNull();
    // toApiError: non-axios error → pesan generik (kontrak http-client).
    expect(result.current.error).toBe("Terjadi kesalahan tak terduga");
  });

  it("reload memicu fetch ulang", async () => {
    let n = 0;
    const { result } = renderHook(() =>
      useAsyncData(async () => {
        n += 1;
        return n;
      }, []),
    );
    await waitFor(() => expect(result.current.data).toBe(1));
    act(() => {
      result.current.reload();
    });
    await waitFor(() => expect(result.current.data).toBe(2));
  });

  it("fetch ulang saat isi deps berubah, diam saat identitas fungsi berubah", async () => {
    const seen: string[] = [];
    const { rerender } = renderHook(
      ({ tag }: { tag: string }) =>
        useAsyncData(
          // identitas loader baru tiap render — tidak boleh memicu fetch
          async () => {
            seen.push(tag);
            return tag;
          },
          [tag],
        ),
      { initialProps: { tag: "a" } },
    );
    await waitFor(() => expect(seen).toEqual(["a"]));
    rerender({ tag: "a" });
    await act(async () => {});
    expect(seen).toEqual(["a"]);
    rerender({ tag: "b" });
    await waitFor(() => expect(seen).toEqual(["a", "b"]));
  });

  it("aman saat unmount sebelum resolve (tanpa setState basi)", async () => {
    const gate = deferred<string>();
    const loader = vi.fn(() => gate.promise);
    const { result, unmount } = renderHook(() => useAsyncData(loader, []));
    expect(result.current.loading).toBe(true);
    unmount();
    await act(async () => {
      gate.resolve("terlambat");
      await gate.promise;
    });
    expect(loader).toHaveBeenCalledTimes(1);
  });
});
