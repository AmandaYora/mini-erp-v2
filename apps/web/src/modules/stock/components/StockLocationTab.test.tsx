import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const createLocation = vi.fn();
const updateLocation = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    createLocation: (...a: unknown[]) => createLocation(...a),
    updateLocation: (...a: unknown[]) => updateLocation(...a),
    archiveLocation: vi.fn(),
  },
}));

import { StockLocationTab } from "@/modules/stock/components/StockLocationTab";
import type { StockLocation } from "@/modules/stock/types";

const loc = (over: Partial<StockLocation>): StockLocation => ({
  id: 1,
  branchId: 1,
  code: "GDG",
  name: "Gudang",
  parentId: null,
  isSystem: false,
  kind: "normal",
  status: "active",
  isPickingArea: false,
  isPrimary: false,
  sortOrder: 0,
  ...over,
});

const locations = [
  loc({ id: 1, code: "GDG", name: "Gudang Utama", isSystem: true }),
  loc({ id: 2, code: "RAK-A", name: "Rak A", isPickingArea: true, isPrimary: true, sortOrder: 3 }),
  loc({ id: 3, code: "RUSAK", name: "Stok Rusak", isSystem: true, kind: "damaged" }),
];

beforeEach(() => {
  vi.clearAllMocks();
  createLocation.mockResolvedValue({ message: "ok", data: {} });
  updateLocation.mockResolvedValue({ message: "ok", data: {} });
});
afterEach(() => cleanup());

function open() {
  return render(<StockLocationTab locations={locations} loading={false} error={null} onReload={() => {}} canManage />);
}

describe("StockLocationTab — atribut lokasi (ST-01)", () => {
  it("menampilkan area picking, lokasi utama, dan urutan di daftar", () => {
    open();
    expect(screen.getByText("Area picking")).toBeTruthy();
    expect(screen.getByText("Utama")).toBeTruthy();
    expect(screen.getByText("urutan 3")).toBeTruthy();
    expect(screen.getByText("Tidak dialokasikan")).toBeTruthy();
  });

  it("mengubah kode dan atribut mengirim code + flag + urutan", async () => {
    open();
    await userEvent.click(screen.getByRole("button", { name: "Aksi untuk lokasi RAK-A" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Ubah" }));
    const code = (await screen.findByLabelText(/Kode/)) as HTMLInputElement;
    expect(code.disabled).toBe(false);
    fireEvent.change(code, { target: { value: "rak-b" } });
    fireEvent.click(screen.getByLabelText("Area picking")); // matikan
    fireEvent.change(screen.getByLabelText("Urutan"), { target: { value: "7" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(updateLocation).toHaveBeenCalled());
    expect(updateLocation).toHaveBeenCalledWith(2, {
      code: "RAK-B",
      name: "Rak A",
      parentId: null,
      isPickingArea: false,
      isPrimary: true,
      sortOrder: 7,
    });
  });

  it("kode tidak dikirim bila tidak berubah", async () => {
    open();
    await userEvent.click(screen.getByRole("button", { name: "Aksi untuk lokasi RAK-A" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Ubah" }));
    fireEvent.click(await screen.findByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(updateLocation).toHaveBeenCalled());
    expect((updateLocation.mock.calls[0][1] as { code?: string }).code).toBeUndefined();
  });

  it("lokasi sistem menjaga kode tetapi atributnya bisa diatur", async () => {
    open();
    await userEvent.click(screen.getByRole("button", { name: "Aksi untuk lokasi GDG" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Ubah" }));
    expect(((await screen.findByLabelText(/Kode/)) as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByLabelText("Lokasi utama") as HTMLInputElement).disabled).toBe(false);
  });

  it("lokasi rusak tidak ikut alokasi: atribut terkunci", async () => {
    open();
    await userEvent.click(screen.getByRole("button", { name: "Aksi untuk lokasi RUSAK" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Ubah" }));
    expect((await screen.findByLabelText("Area picking")).matches(":disabled")).toBe(true);
  });

  it("tambah lokasi mengirim atribut yang dipilih", async () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: "Tambah lokasi" }));
    fireEvent.change(await screen.findByPlaceholderText("cth: GDG-01"), { target: { value: "pick-1" } });
    fireEvent.change(screen.getByPlaceholderText("cth: Gudang Utama"), { target: { value: "Picking 1" } });
    fireEvent.click(screen.getByLabelText("Area picking"));
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(createLocation).toHaveBeenCalled());
    expect(createLocation).toHaveBeenCalledWith(
      expect.objectContaining({ code: "PICK-1", isPickingArea: true, isPrimary: false, sortOrder: 0 }),
    );
  });
});
