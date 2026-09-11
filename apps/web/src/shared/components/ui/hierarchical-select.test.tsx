import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { HierarchicalSelect } from "@/shared/components/ui/hierarchical-select";
import type { SharedSelectOption } from "@/shared/components/ui/select-shared";

afterEach(() => {
  cleanup();
});

const OPTIONS: SharedSelectOption[] = [
  { value: "root", label: "Gudang", isGroup: true, depth: 0, disabled: true, disabledReason: "Pilih lokasi paling bawah" },
  { value: "a", label: "Rak A", depth: 1, path: ["Gudang", "Rak A"], code: "RA" },
  { value: "b", label: "Rak B", depth: 1, path: ["Gudang", "Rak B"], code: "RB" },
];

describe("HierarchicalSelect", () => {
  it("menampilkan path opsi terpilih dan memilih daun", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <HierarchicalSelect options={OPTIONS} value="a" onChange={onChange} placeholder="Pilih…" />,
    );
    expect(screen.getByText("Gudang > Rak A")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { expanded: false }));
    await user.click(screen.getByRole("button", { name: /Rak B/ }));
    expect(onChange).toHaveBeenCalledWith("b");
  });

  it("grup disabled tidak bisa dipilih", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<HierarchicalSelect options={OPTIONS} onChange={onChange} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    const group = screen.getByRole("button", { name: /Gudang.*Grup/ });
    expect(group).toBeDisabled();
    await user.click(group);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("menyaring opsi dari label, kode, dan path", async () => {
    const user = userEvent.setup();
    render(<HierarchicalSelect options={OPTIONS} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await user.type(screen.getByPlaceholderText("Cari…"), "rb");
    expect(screen.queryByText("Rak A")).not.toBeInTheDocument();
    expect(screen.getByText("Rak B")).toBeInTheDocument();
  });

  it("mendukung nilai numerik", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <HierarchicalSelect
        options={[
          { value: 10, label: "Lantai 1", depth: 0 },
          { value: 11, label: "Bin 1", depth: 1 },
        ]}
        onChange={onChange}
      />,
    );
    await user.click(screen.getByRole("button", { expanded: false }));
    await user.click(screen.getByRole("button", { name: "Bin 1" }));
    expect(onChange).toHaveBeenCalledWith(11);
  });
});
