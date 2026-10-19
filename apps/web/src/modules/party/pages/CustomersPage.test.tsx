import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { Party } from "@/modules/party/types";

const customerList = vi.fn();
const supplierList = vi.fn();
const memberList = vi.fn();
vi.mock("@/modules/party/services/party.service", () => ({
  customerService: { list: (...a: unknown[]) => customerList(...a), create: vi.fn() },
  supplierService: { list: (...a: unknown[]) => supplierList(...a), create: vi.fn() },
  memberTypeService: { list: (...a: unknown[]) => memberList(...a) },
}));

import CustomersPage from "@/modules/party/pages/CustomersPage";
import SuppliersPage from "@/modules/party/pages/SuppliersPage";

const party = (over: Partial<Party>): Party => ({
  id: 1,
  code: "CUS-001",
  type: "customer",
  name: "Budi Santoso",
  phone: "0812",
  email: null,
  address: null,
  notes: null,
  member: null,
  status: "active",
  attributes: {},
  ...over,
});
const page = (items: Party[]) => ({ items, meta: { page: 1, limit: 20, total: items.length, totalPages: 1 } });

function renderAt(node: React.ReactNode, path: string) {
  return render(<MemoryRouter initialEntries={[path]}>{node}</MemoryRouter>);
}

beforeEach(() => {
  for (const m of [customerList, supplierList, memberList]) m.mockReset();
  memberList.mockResolvedValue([]);
  customerList.mockResolvedValue(page([party({})]));
  supplierList.mockResolvedValue(page([party({ id: 5, code: "SUP-001", type: "supplier", name: "PT Sumber" })]));
  useAuthStore.setState({ permissions: ["customers.view", "suppliers.view"], ready: true });
});
afterEach(cleanup);

describe("daftar pelanggan — default Aktif + akses arsip", () => {
  it("tampilan awal status=active tanpa chip dan tanpa kolom arsip", async () => {
    renderAt(<CustomersPage />, "/customers");
    await screen.findByText("Budi Santoso");
    expect(customerList.mock.calls[0][0]).toMatchObject({ status: "active" });
    expect(screen.queryByText(/^Status:/)).not.toBeInTheDocument();
    expect(screen.queryByText("Diarsipkan pada")).not.toBeInTheDocument();
  });

  it("'Tampilkan arsip' membuka arsip dengan kolom tanggal arsip; 'Semua status' tak mengirim filter", async () => {
    const user = userEvent.setup();
    customerList.mockImplementation(async (p: { status: string }) =>
      page(
        p.status === "archived"
          ? [party({ id: 8, code: "CUS-008", name: "Siti Lama", status: "archived", archivedAt: "2026-08-15T03:00:00Z" })]
          : [party({})],
      ),
    );
    renderAt(<CustomersPage />, "/customers");
    await screen.findByText("Budi Santoso");
    await user.click(screen.getByRole("button", { name: "Tampilkan arsip" }));
    await screen.findByText("Siti Lama");
    expect(screen.getByText("Diarsipkan pada")).toBeInTheDocument();
    expect(screen.getByText(/15 Agu 2026|15\/08\/2026|15 Agustus 2026/)).toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText("Status"), "all");
    await waitFor(() => expect(customerList).toHaveBeenLastCalledWith(expect.objectContaining({ status: "" }), expect.anything()));
    expect(screen.getByText("Status: Semua status")).toBeInTheDocument();
  });
});

describe("daftar pemasok — konsisten dengan pelanggan (PM-02)", () => {
  it("default Aktif, tombol arsip, dan kolom tanggal arsip", async () => {
    const user = userEvent.setup();
    supplierList.mockImplementation(async (p: { status: string }) =>
      page(
        p.status === "archived"
          ? [party({ id: 6, code: "SUP-006", type: "supplier", name: "PT Lama", status: "archived", archivedAt: "2026-07-01T03:00:00Z" })]
          : [party({ id: 5, code: "SUP-001", type: "supplier", name: "PT Sumber" })],
      ),
    );
    renderAt(<SuppliersPage />, "/suppliers");
    await screen.findByText("PT Sumber");
    expect(supplierList.mock.calls[0][0]).toMatchObject({ status: "active" });
    expect(screen.queryByText("Diarsipkan pada")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Tampilkan arsip" }));
    await screen.findByText("PT Lama");
    expect(screen.getByText("Diarsipkan pada")).toBeInTheDocument();
  });
});
