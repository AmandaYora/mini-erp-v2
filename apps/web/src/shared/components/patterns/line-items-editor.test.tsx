import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SearchSelect } from "@/shared/components/ui/search-select";
import { LineItemsEditor } from "./line-items-editor";

const columns = [
  { id: "produk", header: "Produk", render: (_l: { id: number }, i: number) => <input aria-label={`Produk ${i + 1}`} /> },
  { id: "qty", header: "Qty", width: "6rem", align: "right" as const, render: (_l: { id: number }, i: number) => <input aria-label={`Qty ${i + 1}`} /> },
];

function setContainerWidth(width: number) {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
    width,
    height: 0,
    top: 0,
    left: 0,
    right: width,
    bottom: 0,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  });
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("LineItemsEditor — mode kartu (wadah sempit, U7-3)", () => {
  it("wadah < 600px: baris jadi kartu berlabel tanpa tabel; Enter di baris terakhir tetap menambah", async () => {
    setContainerWidth(360);
    const onAdd = vi.fn();
    const onRemove = vi.fn();
    render(<LineItemsEditor lines={[{ id: 1 }, { id: 2 }]} getKey={(l) => l.id} columns={columns} onAdd={onAdd} onRemove={onRemove} />);

    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(screen.getByText("Baris 2")).toBeInTheDocument();
    expect(screen.getAllByText("Qty")).toHaveLength(2);

    await userEvent.type(screen.getByLabelText("Qty 1"), "{Enter}");
    expect(onAdd).not.toHaveBeenCalled();
    await userEvent.type(screen.getByLabelText("Qty 2"), "{Enter}");
    expect(onAdd).toHaveBeenCalledTimes(1);
    await userEvent.click(screen.getByRole("button", { name: "Hapus baris 2" }));
    expect(onRemove).toHaveBeenCalledWith(1);
  });

  it("wadah lebar tetap tabel; kosong menampilkan teks kosong di kedua mode", () => {
    setContainerWidth(900);
    const { unmount } = render(<LineItemsEditor lines={[]} getKey={(_l, i) => i} columns={columns} emptyText="Tambahkan produk." />);
    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.getByText("Tambahkan produk.")).toBeInTheDocument();
    unmount();

    setContainerWidth(320);
    render(<LineItemsEditor lines={[]} getKey={(_l, i) => i} columns={columns} emptyText="Tambahkan produk." />);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.getByText("Tambahkan produk.")).toBeInTheDocument();
  });
});

describe("LineItemsEditor — Enter di pemilih (portal) tidak menambah baris", () => {
  it("memilih produk dengan Enter di kotak cari tidak memanggil onAdd", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn();
    const onPick = vi.fn();
    render(
      <LineItemsEditor
        lines={[{ id: 1 }]}
        getKey={(l) => l.id}
        onAdd={onAdd}
        columns={[
          {
            id: "produk",
            header: "Produk",
            render: () => (
              <SearchSelect aria-label="Produk 1" options={[{ value: 7, label: "P007 Sabun" }]} value={null} onChange={onPick} />
            ),
          },
        ]}
      />,
    );
    screen.getByRole("button", { name: "Produk 1" }).focus();
    await user.keyboard("{Enter}");
    await user.keyboard("Sabun{Enter}");
    expect(onPick).toHaveBeenCalledWith(7);
    expect(onAdd).not.toHaveBeenCalled();
  });
});
