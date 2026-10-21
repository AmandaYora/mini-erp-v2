import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const listAccessibleBranches = vi.fn();
const listLocations = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    listAccessibleBranches: (...a: unknown[]) => listAccessibleBranches(...a),
    listLocations: (...a: unknown[]) => listLocations(...a),
    createTransfer: vi.fn(),
  },
}));
vi.mock("@/modules/stock/components/ProductSelect", () => ({ ProductSelect: () => <div /> }));
vi.mock("@/modules/stock/components/VariantSelect", () => ({ VariantSelect: () => <div /> }));

import TransferNewPage from "@/modules/stock/pages/TransferNewPage";

const loc = (id: number, branchId: number, code: string) => ({
  id,
  branchId,
  code,
  name: `Gudang ${code}`,
  parentId: null,
  isSystem: false,
  kind: "storage",
  status: "active",
});

beforeEach(() => {
  vi.clearAllMocks();
  listAccessibleBranches.mockResolvedValue([
    { id: 1, code: "B1", name: "Cabang Satu", status: "active" },
    { id: 2, code: "B2", name: "Cabang Dua", status: "active" },
    { id: 3, code: "B3", name: "Cabang Tiga", status: "active" },
  ]);
  // Tanpa branchId = cabang sesi (1); dengan branchId = cabang itu.
  listLocations.mockImplementation((_status?: string, branchId?: number | null) =>
    Promise.resolve(
      branchId === 2
        ? [loc(20, 2, "K2")]
        : branchId === 3
          ? [loc(30, 3, "K3")]
          : [loc(10, 1, "K1")],
    ),
  );
});
afterEach(() => cleanup());

function open() {
  return render(
    <MemoryRouter>
      <TransferNewPage />
    </MemoryRouter>,
  );
}

async function pickBranch(label: string) {
  const trigger = await screen.findByRole("button", { name: /Cabang tujuan/ });
  fireEvent.click(trigger);
  fireEvent.click(await screen.findByRole("option", { name: label }));
}

describe("TransferNewPage — lokasi tujuan mengikuti cabang tujuan (K-06)", () => {
  it("memuat cabang yang bisa diakses, lokasi tujuan menunggu cabang dipilih", async () => {
    open();
    await waitFor(() => expect(listAccessibleBranches).toHaveBeenCalled());
    // Lokasi asal = cabang sesi (tanpa branchId); lokasi tujuan belum dimuat.
    await waitFor(() => expect(listLocations).toHaveBeenCalledWith(undefined, undefined));
    expect(listLocations.mock.calls.some((c) => c[1] === 2)).toBe(false);
    expect(screen.getByRole("button", { name: /Lokasi tujuan/ })).toBeDisabled();
  });

  it("memuat lokasi cabang tujuan dan mengosongkan pilihan saat cabang berganti", async () => {
    open();
    await pickBranch("B2 — Cabang Dua");
    await waitFor(() => expect(listLocations).toHaveBeenCalledWith(undefined, 2));

    // Pilih lokasi di cabang 2.
    const destTrigger = await screen.findByRole("button", { name: /Lokasi tujuan/ });
    await waitFor(() => expect(destTrigger).toBeEnabled());
    fireEvent.click(destTrigger);
    fireEvent.click(await screen.findByRole("option", { name: "K2 — Gudang K2" }));
    expect(await screen.findByText("K2 — Gudang K2")).toBeInTheDocument();

    // Ganti cabang tujuan: pilihan lama dikosongkan, daftar cabang 3 dimuat.
    await pickBranch("B3 — Cabang Tiga");
    await waitFor(() => expect(listLocations).toHaveBeenCalledWith(undefined, 3));
    await waitFor(() => expect(screen.queryByText("K2 — Gudang K2")).not.toBeInTheDocument());
  });
});
