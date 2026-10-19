import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const searchCustomers = vi.fn();
const createCustomer = vi.fn();
const memberTypes = vi.fn();
vi.mock("@/modules/pos/services/pos.service", () => ({
  posService: {
    searchCustomers: (...a: unknown[]) => searchCustomers(...a),
    createCustomer: (...a: unknown[]) => createCustomer(...a),
    memberTypes: (...a: unknown[]) => memberTypes(...a),
  },
}));

import { CustomerSheet } from "@/modules/pos/components/CustomerSheet";

beforeEach(() => {
  searchCustomers.mockReset();
  createCustomer.mockReset();
  memberTypes.mockReset();
  searchCustomers.mockResolvedValue({ items: [], meta: { page: 1, limit: 20, total: 0, totalPages: 1 } });
  memberTypes.mockResolvedValue([
    { id: 4, code: "GOLD", name: "Gold", status: "active" },
    { id: 5, code: "OLD", name: "Lama", status: "archived" },
  ]);
  createCustomer.mockResolvedValue({ id: 42, code: "CUS-042", name: "Siti", phone: "0812345678", member: null });
  useAuthStore.setState({ permissions: ["customers.create", "customers.view"], ready: true });
});
afterEach(cleanup);

function renderSheet(requirePhone = false) {
  const onSelect = vi.fn();
  const onClose = vi.fn();
  render(<CustomerSheet open selectedId={null} requirePhone={requirePhone} onSelect={onSelect} onClose={onClose} />);
  return { onSelect, onClose };
}

describe("CustomerSheet - pelanggan baru (PR-02)", () => {
  it("membuat pelanggan lalu langsung terpilih dan sheet menutup", async () => {
    const { onSelect, onClose } = renderSheet();
    fireEvent.click(await screen.findByRole("button", { name: /Pelanggan baru/ }));
    fireEvent.change(screen.getByLabelText(/Nama/), { target: { value: "Siti" } });
    fireEvent.change(screen.getByLabelText(/Telepon/), { target: { value: "0812345678" } });
    fireEvent.change(await screen.findByLabelText("Tipe member"), { target: { value: "4" } });
    // Hanya tipe member aktif yang ditawarkan.
    expect(screen.queryByRole("option", { name: /Lama/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Simpan & pilih/ }));
    await waitFor(() => expect(onSelect).toHaveBeenCalledWith(42));
    expect(createCustomer).toHaveBeenCalledWith({ name: "Siti", phone: "0812345678", memberTypeId: 4 });
    expect(onClose).toHaveBeenCalled();
  });

  it("Bayar Nanti: telepon wajib, pelanggan tidak dibuat tanpa telepon", async () => {
    const { onSelect } = renderSheet(true);
    fireEvent.click(await screen.findByRole("button", { name: /Pelanggan baru/ }));
    fireEvent.change(screen.getByLabelText(/Nama/), { target: { value: "Tanpa HP" } });
    fireEvent.click(screen.getByRole("button", { name: /Simpan & pilih/ }));
    expect(await screen.findByText(/wajib memakai pelanggan ber-telepon/)).toBeInTheDocument();
    expect(createCustomer).not.toHaveBeenCalled();
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("tanpa izin customers.create tombol Pelanggan baru tidak ada", async () => {
    useAuthStore.setState({ permissions: ["customers.view"], ready: true });
    renderSheet();
    await screen.findByText("Walk-in (tanpa customer)");
    expect(screen.queryByRole("button", { name: /Pelanggan baru/ })).not.toBeInTheDocument();
  });
});

