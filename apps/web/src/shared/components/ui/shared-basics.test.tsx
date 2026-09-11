import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PasswordInput } from "@/shared/components/ui/password-input";
import { FieldHint } from "@/shared/components/ui/field-hint";
import { SelectField } from "@/shared/components/ui/select-field";
import { GlobalLoader } from "@/shared/components/ui/global-loader";
import { loadingBus } from "@/shared/services/loading-bus";

afterEach(() => {
  cleanup();
  loadingBus.resetForTest();
});

describe("PasswordInput", () => {
  it("menyembunyikan lalu menampilkan kata sandi", async () => {
    const user = userEvent.setup();
    render(<PasswordInput aria-label="Kata sandi" />);
    const input = screen.getByLabelText("Kata sandi");
    expect(input).toHaveAttribute("type", "password");
    await user.click(screen.getByRole("button", { name: "Tampilkan password" }));
    expect(input).toHaveAttribute("type", "text");
    expect(screen.getByRole("button", { name: "Sembunyikan password" })).toBeInTheDocument();
  });
});

describe("FieldHint", () => {
  it("membuka popover saat diklik dan menutup via Escape", async () => {
    const user = userEvent.setup();
    render(<FieldHint>Isi dengan kode unik.</FieldHint>);
    expect(screen.queryByText("Isi dengan kode unik.")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Keterangan" }));
    expect(screen.getByText("Isi dengan kode unik.")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByText("Isi dengan kode unik.")).not.toBeInTheDocument();
  });
});

describe("SelectField", () => {
  it("memilih opsi dan meneruskan hidden input name", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const { container } = render(
      <SelectField
        name="lokasi"
        options={[
          { value: 1, label: "A" },
          { value: 2, label: "B" },
        ]}
        onChange={onChange}
        placeholder="Pilih…"
      />,
    );
    // SearchSelect sync (bukan Async): trigger tanpa aria-expanded.
    await user.click(screen.getByRole("button", { name: "Pilih…" }));
    await user.click(screen.getByRole("button", { name: "B" }));
    expect(onChange).toHaveBeenCalledWith(2);
    expect(container.querySelector('input[type="hidden"][name="lokasi"]')).not.toBeNull();
  });
});

describe("GlobalLoader", () => {
  it("hanya tampil saat ada request berjalan", () => {
    loadingBus.resetForTest();
    const { unmount } = render(<GlobalLoader />);
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    act(() => {
      loadingBus.begin();
    });
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    act(() => {
      loadingBus.end();
    });
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    unmount();
    loadingBus.resetForTest();
  });
});
