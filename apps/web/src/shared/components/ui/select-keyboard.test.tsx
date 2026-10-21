import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AsyncSearchSelect } from "./async-search-select";
import { SearchSelect } from "./search-select";

// U7-2: pemilih bisa dipakai penuh tanpa tetikus — buka dengan Enter, ketik, Enter memilih hasil
// pertama, ↓/↑ berpindah antar opsi dan kembali ke kotak cari.
afterEach(cleanup);

const OPTIONS = [
  { value: 1, label: "Gudang A" },
  { value: 2, label: "Gudang B" },
  { value: 3, label: "Gudang C" },
];

describe("navigasi keyboard select", () => {
  it("AsyncSearchSelect: ketik lalu Enter memilih hasil pertama (form tidak terkirim)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const onSubmit = vi.fn((e: React.FormEvent) => e.preventDefault());
    const loadOptions = vi.fn(async (q: string) => OPTIONS.filter((o) => o.label.includes(q)));
    render(
      <form onSubmit={onSubmit}>
        <AsyncSearchSelect aria-label="Gudang" loadOptions={loadOptions} onChange={onChange} debounceMs={0} />
      </form>,
    );
    screen.getByRole("button", { name: "Gudang" }).focus();
    await user.keyboard("{Enter}");
    const input = await screen.findByPlaceholderText("Cari…");
    await waitFor(() => expect(input).toHaveFocus());
    await user.keyboard("B");
    await waitFor(() => expect(screen.queryByRole("option", { name: "Gudang A" })).not.toBeInTheDocument());
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledWith(2, expect.objectContaining({ label: "Gudang B" }));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("AsyncSearchSelect: ↓ ke opsi, ↓ berikutnya, ↑ dua kali kembali ke kotak cari, Enter di opsi memilihnya", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<AsyncSearchSelect aria-label="Gudang" loadOptions={async () => OPTIONS} onChange={onChange} debounceMs={0} />);
    screen.getByRole("button", { name: "Gudang" }).focus();
    await user.keyboard("{Enter}");
    const input = await screen.findByPlaceholderText("Cari…");
    await screen.findByRole("option", { name: "Gudang C" });
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("option", { name: "Gudang A" })).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("option", { name: "Gudang B" })).toHaveFocus();
    await user.keyboard("{ArrowUp}{ArrowUp}");
    expect(input).toHaveFocus();
    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}{Enter}");
    expect(onChange).toHaveBeenCalledWith(3, expect.objectContaining({ label: "Gudang C" }));
  });

  it("SearchSelect: ketik lalu Enter memilih opsi tersaring pertama", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<SearchSelect aria-label="Kategori" options={OPTIONS} value={null} onChange={onChange} />);
    screen.getByRole("button", { name: "Kategori" }).focus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(screen.getByPlaceholderText("Cari…")).toHaveFocus());
    await user.keyboard("C{Enter}");
    expect(onChange).toHaveBeenCalledWith(3);
  });
});

describe("AsyncSearchSelect — Enter saat hasil belum tiba", () => {
  it("tidak memilih dari daftar basi; memilih hasil pertama kata kunci terbaru begitu tiba", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    let release: () => void = () => {};
    const loadOptions = vi.fn((q: string) =>
      q === ""
        ? Promise.resolve(OPTIONS)
        : new Promise<typeof OPTIONS>((resolve) => {
            release = () => resolve(OPTIONS.filter((o) => o.label.includes(q)));
          }),
    );
    render(<AsyncSearchSelect aria-label="Gudang" loadOptions={loadOptions} onChange={onChange} debounceMs={0} />);
    screen.getByRole("button", { name: "Gudang" }).focus();
    await user.keyboard("{Enter}");
    await screen.findByRole("option", { name: "Gudang A" });
    await user.keyboard("C");
    await waitFor(() => expect(loadOptions).toHaveBeenCalledWith("C", expect.any(AbortSignal)));
    expect(screen.getByRole("listbox")).toHaveAttribute("aria-busy", "true");
    await user.keyboard("{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    release();
    await waitFor(() => expect(onChange).toHaveBeenCalledWith(3, expect.objectContaining({ label: "Gudang C" })));
  });
});
