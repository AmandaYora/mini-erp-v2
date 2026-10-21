import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const searchDocuments = vi.fn();
vi.mock("@/app/routes/document-search", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/app/routes/document-search")>();
  return { ...actual, searchDocuments: (...a: unknown[]) => searchDocuments(...a) };
});

import { CommandPalette, openCommandPalette } from "./command-palette";

function Probe() {
  return <div data-testid="path">{useLocation().pathname}</div>;
}

function renderPalette() {
  return render(
    <MemoryRouter initialEntries={["/dashboard"]}>
      <CommandPalette groups={[]} />
      <Probe />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  searchDocuments.mockReset();
  useAuthStore.setState({ permissions: ["delivery.view"], ready: true });
});
afterEach(cleanup);

describe("CommandPalette — lompat ke dokumen", () => {
  it("mencari nomor setelah jeda ketik lalu membuka dokumen yang dipilih", async () => {
    searchDocuments.mockResolvedValue([
      { key: "sj-5", kind: "Surat jalan", number: "SJ-HQ/2026/00012", detail: "ORD-1 · Draf", path: "/deliveries/5" },
    ]);
    const user = userEvent.setup();
    renderPalette();
    act(() => openCommandPalette());

    await user.type(screen.getByRole("combobox"), "SJ-HQ/2026/00012");
    const item = await screen.findByRole("option", { name: /SJ-HQ\/2026\/00012/ });
    expect(item).toHaveTextContent("Surat jalan · ORD-1 · Draf");
    // Satu panggilan untuk kueri akhir (debounce), dengan izin peran dan AbortSignal.
    expect(searchDocuments).toHaveBeenCalledTimes(1);
    expect(searchDocuments.mock.calls[0][0]).toBe("SJ-HQ/2026/00012");
    expect(searchDocuments.mock.calls[0][2]).toBeInstanceOf(AbortSignal);

    await user.click(item);
    expect(screen.getByTestId("path")).toHaveTextContent("/deliveries/5");
  });

  it("teks pendek atau bukan nomor tidak mencari dokumen", async () => {
    const user = userEvent.setup();
    renderPalette();
    act(() => openCommandPalette());
    await user.type(screen.getByRole("combobox"), "stok");
    await new Promise((r) => setTimeout(r, 400));
    expect(searchDocuments).not.toHaveBeenCalled();
    expect(screen.getByText(/Untuk dokumen, ketik nomornya/)).toBeInTheDocument();
  });
});
