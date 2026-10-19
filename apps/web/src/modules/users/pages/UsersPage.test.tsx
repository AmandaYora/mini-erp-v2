import { cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
vi.mock("@/modules/users/services/user.service", () => ({
  userService: {
    list: (...a: unknown[]) => list(...a),
    get: vi.fn(),
    branchOptions: vi.fn().mockResolvedValue([]),
  },
}));
vi.mock("@/modules/users/services/role.service", () => ({
  roleService: { list: vi.fn().mockResolvedValue([]) },
}));

import UsersPage from "@/modules/users/pages/UsersPage";

const rows = [
  {
    id: 1,
    username: "sinta",
    email: "sinta@toko.id",
    fullName: "Sinta Kasir",
    status: "active",
    displayTitle: "Kepala Toko",
    roles: [
      { id: 7, name: "Kasir" },
      { id: 8, name: "Gudang" },
    ],
    branches: [
      { id: 1, name: "Cabang Pusat", isDefault: true },
      { id: 2, name: "Cabang Bandung", isDefault: false },
    ],
    lastLoginAt: "2026-09-10 14:30:00",
  },
  {
    id: 2,
    username: "baru",
    email: null,
    fullName: "Pengguna Baru",
    status: "active",
    roles: [],
    branches: [],
    lastLoginAt: null,
  },
];

beforeEach(() => {
  list.mockReset();
  list.mockResolvedValue({ items: rows, meta: { page: 1, limit: 25, total: 2, totalPages: 1 } });
  useAuthStore.setState({ permissions: ["users.view"], ready: true });
});
afterEach(cleanup);

describe("halaman pengguna — kolom peran/cabang/jabatan/login terakhir", () => {
  it("menampilkan peran, cabang, jabatan, dan login terakhir tanpa membuka pengguna", async () => {
    render(
      <MemoryRouter>
        <UsersPage />
      </MemoryRouter>,
    );
    const row = (await screen.findByText("sinta")).closest("tr")!;
    expect(within(row).getByText("Kasir, Gudang")).toBeInTheDocument();
    expect(within(row).getByText("Cabang Pusat, Cabang Bandung")).toBeInTheDocument();
    expect(within(row).getByText("Kepala Toko")).toBeInTheDocument();
    expect(within(row).getByText(/10 Sep 2026/)).toBeInTheDocument();
    const empty = screen.getByText("baru").closest("tr")!;
    expect(within(empty).getByText("Belum pernah")).toBeInTheDocument();
  });
});
