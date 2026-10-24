import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  AsyncSearchSelect,
  type AsyncSearchOption,
} from "@/shared/components/ui/async-search-select";

afterEach(() => {
  cleanup();
});

const OPTIONS: AsyncSearchOption[] = [
  { value: 1, label: "Gudang A", description: "Cabang 1" },
  { value: 2, label: "Gudang B", description: "Cabang 2" },
];

describe("AsyncSearchSelect", () => {
  it("memuat opsi saat dibuka dan memilih opsi", async () => {
    const user = userEvent.setup();
    const loadOptions = vi.fn(async () => OPTIONS);
    const onChange = vi.fn();
    render(
      <AsyncSearchSelect
        loadOptions={loadOptions}
        onChange={onChange}
        placeholder="Pilih gudang…"
        debounceMs={0}
      />,
    );
    expect(screen.getByText("Pilih gudang…")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { expanded: false }));
    // Pemuat kini menerima AbortSignal kedua: request usang dihentikan,
    // bukan sekadar diabaikan.
    await waitFor(() =>
      expect(loadOptions).toHaveBeenCalledWith("", expect.any(AbortSignal)),
    );
    expect(await screen.findByText("Gudang A")).toBeInTheDocument();
    await user.click(screen.getByRole("option", { name: /Gudang B/ }));
    expect(onChange).toHaveBeenCalledWith(
      2,
      expect.objectContaining({ value: 2, label: "Gudang B" }),
    );
    expect(screen.getByText("Gudang B")).toBeInTheDocument();
  });

  it("mencari server-side per ketikan dan menampilkan hasMore", async () => {
    const user = userEvent.setup();
    const loadOptions = vi.fn(async (q: string) =>
      q === "" ? { options: OPTIONS, hasMore: true } : { options: [OPTIONS[0]], hasMore: false },
    );
    render(<AsyncSearchSelect loadOptions={loadOptions} debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    expect(await screen.findByText("Ketik lebih spesifik untuk melihat hasil lainnya")).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText("Cari…"), "A");
    await waitFor(() =>
      expect(loadOptions).toHaveBeenCalledWith(
        expect.stringContaining("A"),
        expect.any(AbortSignal),
      ),
    );
    expect(await screen.findByText("Gudang A")).toBeInTheDocument();
    expect(screen.queryByText("Gudang B")).not.toBeInTheDocument();
  });

  it("menampilkan galat muat dan teks kosong", async () => {
    const user = userEvent.setup();
    const failing = vi.fn(async (): Promise<AsyncSearchOption[]> => {
      throw new Error("down");
    });
    const { unmount } = render(<AsyncSearchSelect loadOptions={failing} debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    expect(await screen.findByText("Gagal memuat pilihan")).toBeInTheDocument();
    unmount();

    const empty = vi.fn(async () => [] as AsyncSearchOption[]);
    render(<AsyncSearchSelect loadOptions={empty} debounceMs={0} emptyText="Kosong" />);
    await user.click(screen.getByRole("button", { expanded: false }));
    expect(await screen.findByText("Kosong")).toBeInTheDocument();
  });

  it("allowClear mengosongkan pilihan", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <AsyncSearchSelect
        loadOptions={async () => OPTIONS}
        onChange={onChange}
        value={1}
        selectedOption={OPTIONS[0]}
        allowClear
        debounceMs={0}
      />,
    );
    expect(screen.getByText("Gudang A")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Hapus pilihan" }));
    expect(onChange).toHaveBeenCalledWith(null, undefined);
  });
  it("membatalkan request yang tergantikan ketikan berikutnya", async () => {
    const user = userEvent.setup();
    const signals: AbortSignal[] = [];
    const loadOptions = vi.fn(async (_q: string, signal?: AbortSignal) => {
      if (signal) signals.push(signal);
      return OPTIONS;
    });
    render(<AsyncSearchSelect loadOptions={loadOptions} debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(signals.length).toBeGreaterThan(0));
    await user.type(screen.getByPlaceholderText("Cari…"), "AB");
    await waitFor(() => expect(signals.length).toBeGreaterThan(1));
    // Semua kecuali yang terakhir harus sudah dibatalkan: request usang tidak
    // boleh terus memakai koneksi dan kerja server.
    const stale = signals.slice(0, -1);
    expect(stale.length).toBeGreaterThan(0);
    expect(stale.every((s) => s.aborted)).toBe(true);
    expect(signals[signals.length - 1].aborted).toBe(false);
  });
  it("minChars: kata kunci 1 huruf tidak memanggil loadOptions, 2 huruf memanggil", async () => {
    const user = userEvent.setup();
    const loadOptions = vi.fn(async () => OPTIONS);
    render(<AsyncSearchSelect loadOptions={loadOptions} debounceMs={0} minChars={2} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    // Daftar awal (kata kunci kosong) tetap dimuat.
    await waitFor(() => expect(loadOptions).toHaveBeenCalledTimes(1));
    expect(loadOptions).toHaveBeenLastCalledWith("", expect.any(AbortSignal));

    const input = screen.getByPlaceholderText("Cari…");
    await user.type(input, "a");
    expect(await screen.findByText("Ketik minimal 2 huruf untuk mencari")).toBeInTheDocument();
    expect(loadOptions).toHaveBeenCalledTimes(1);

    await user.type(input, "b");
    await waitFor(() =>
      expect(loadOptions).toHaveBeenLastCalledWith("ab", expect.any(AbortSignal)),
    );
    expect(loadOptions).toHaveBeenCalledTimes(2);
    expect(screen.queryByText("Ketik minimal 2 huruf untuk mencari")).not.toBeInTheDocument();
  });
});

// Observer palsu: tes memicu "sentinel terlihat" sendiri (jsdom tidak punya layout).
class FakeObserver {
  static live: FakeObserver[] = [];
  constructor(private readonly cb: IntersectionObserverCallback) {
    FakeObserver.live.push(this);
  }
  observe() {}
  unobserve() {}
  takeRecords() {
    return [];
  }
  disconnect() {
    FakeObserver.live = FakeObserver.live.filter((o) => o !== this);
  }
  fire() {
    this.cb([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver);
  }
}

/** Sentinel terlihat (gulir mendekati bawah daftar). */
function scrollToEnd(times = 1) {
  const observer = FakeObserver.live.at(-1);
  expect(observer).toBeDefined();
  act(() => {
    for (let i = 0; i < times; i++) observer?.fire();
  });
}

const opt = (n: number): AsyncSearchOption => ({ value: n, label: `Barang ${n}` });
const range = (from: number, to: number) => Array.from({ length: to - from + 1 }, (_, i) => opt(from + i));

describe("AsyncSearchSelect paged (auto-load saat gulir)", () => {
  beforeEach(() => {
    FakeObserver.live = [];
    vi.stubGlobal("IntersectionObserver", FakeObserver);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("memuat halaman berikutnya saat digulir, membuang duplikat, berhenti saat habis", async () => {
    const user = userEvent.setup();
    const pages: Record<number, { options: AsyncSearchOption[]; hasMore: boolean }> = {
      1: { options: range(1, 3), hasMore: true },
      2: { options: [opt(3), ...range(4, 5)], hasMore: true },
      3: { options: [opt(6)], hasMore: false },
    };
    const loadOptions = vi.fn(async (_q: string, _s?: AbortSignal, page = 1) => pages[page]);
    render(<AsyncSearchSelect loadOptions={loadOptions} paged debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    expect(await screen.findByText("Barang 3")).toBeInTheDocument();
    expect(loadOptions).toHaveBeenLastCalledWith("", expect.any(AbortSignal), 1);
    // Mode paged tidak menampilkan "ketik lebih spesifik".
    expect(screen.queryByText("Ketik lebih spesifik untuk melihat hasil lainnya")).not.toBeInTheDocument();

    scrollToEnd();
    expect(await screen.findByText("Barang 5")).toBeInTheDocument();
    expect(loadOptions).toHaveBeenLastCalledWith("", expect.any(AbortSignal), 2);
    expect(screen.getAllByText("Barang 3")).toHaveLength(1);

    scrollToEnd();
    expect(await screen.findByText("Barang 6")).toBeInTheDocument();
    expect(screen.getAllByRole("option")).toHaveLength(6);
    // Habis: sentinel dilepas, tidak ada request lagi.
    await waitFor(() => expect(FakeObserver.live).toHaveLength(0));
    expect(loadOptions).toHaveBeenCalledTimes(3);
  });

  it("indikator memuat dan pemicu ganda hanya mengirim satu request", async () => {
    const user = userEvent.setup();
    let release: (() => void) | undefined;
    const loadOptions = vi.fn((_q: string, _s?: AbortSignal, page = 1) =>
      page === 1
        ? Promise.resolve({ options: range(1, 2), hasMore: true })
        : new Promise<{ options: AsyncSearchOption[]; hasMore: boolean }>((resolve) => {
            release = () => resolve({ options: range(3, 4), hasMore: false });
          }),
    );
    render(<AsyncSearchSelect loadOptions={loadOptions} paged debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await screen.findByText("Barang 2");
    scrollToEnd(2);
    expect(await screen.findByText("Memuat lainnya…")).toBeInTheDocument();
    expect(loadOptions).toHaveBeenCalledTimes(2);
    await act(async () => release?.());
    expect(await screen.findByText("Barang 4")).toBeInTheDocument();
    expect(screen.queryByText("Memuat lainnya…")).not.toBeInTheDocument();
  });

  it("kata kunci baru mengulang dari halaman 1 dan halaman berikutnya memakai kata kunci itu", async () => {
    const user = userEvent.setup();
    const loadOptions = vi.fn(async (q: string, _s?: AbortSignal, page = 1) =>
      q === ""
        ? { options: range(page * 10, page * 10 + 1), hasMore: true }
        : { options: [opt(100 + page)], hasMore: page < 2 },
    );
    render(<AsyncSearchSelect loadOptions={loadOptions} paged debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await screen.findByText("Barang 10");
    scrollToEnd();
    await screen.findByText("Barang 20");

    await user.type(screen.getByPlaceholderText("Cari…"), "x");
    expect(await screen.findByText("Barang 101")).toBeInTheDocument();
    expect(loadOptions).toHaveBeenLastCalledWith("x", expect.any(AbortSignal), 1);
    expect(screen.queryByText("Barang 20")).not.toBeInTheDocument();

    scrollToEnd();
    expect(await screen.findByText("Barang 102")).toBeInTheDocument();
    expect(loadOptions).toHaveBeenLastCalledWith("x", expect.any(AbortSignal), 2);
  });

  it("galat halaman berikutnya: opsi tetap tampil, Coba lagi memuat ulang halaman itu", async () => {
    const user = userEvent.setup();
    let failNext = true;
    const loadOptions = vi.fn(async (_q: string, _s?: AbortSignal, page = 1) => {
      if (page === 2 && failNext) {
        failNext = false;
        throw new Error("down");
      }
      return page === 1 ? { options: range(1, 2), hasMore: true } : { options: range(3, 4), hasMore: false };
    });
    render(<AsyncSearchSelect loadOptions={loadOptions} paged debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await screen.findByText("Barang 2");
    scrollToEnd();
    expect(await screen.findByText("Gagal memuat pilihan berikutnya")).toBeInTheDocument();
    expect(screen.getByText("Barang 1")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Coba lagi" }));
    expect(await screen.findByText("Barang 4")).toBeInTheDocument();
    expect(loadOptions).toHaveBeenLastCalledWith("", expect.any(AbortSignal), 2);
  });

  it("halaman tanpa nilai baru dianggap akhir daftar", async () => {
    const user = userEvent.setup();
    const loadOptions = vi.fn(async () => ({ options: range(1, 2), hasMore: true }));
    render(<AsyncSearchSelect loadOptions={loadOptions} paged debounceMs={0} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await screen.findByText("Barang 2");
    scrollToEnd();
    await waitFor(() => expect(FakeObserver.live).toHaveLength(0));
    expect(loadOptions).toHaveBeenCalledTimes(2);
    expect(screen.getAllByRole("option")).toHaveLength(2);
  });

  it("label pilihan tersimpan tampil dari selectedOption walau tidak ada di halaman pertama", () => {
    render(
      <AsyncSearchSelect
        loadOptions={vi.fn(async () => ({ options: range(1, 2), hasMore: true }))}
        paged
        value={99}
        selectedOption={{ value: 99, label: "Barang 99" }}
      />,
    );
    expect(screen.getByText("Barang 99")).toBeInTheDocument();
  });
});
