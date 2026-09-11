import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
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
    await waitFor(() => expect(loadOptions).toHaveBeenCalledWith(""));
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
      expect(loadOptions).toHaveBeenCalledWith(expect.stringContaining("A")),
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
});
